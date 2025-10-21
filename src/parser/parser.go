package parser

import (
	"fmt"
	"strings"

	"eyot/ast"
	"eyot/errors"
	"eyot/token"
)

type ModuleProvider interface {
	GetModule(Id ast.ModuleId, disallowedIds map[string]bool) *ast.Module
}

// The parsing context at a given level
type Frame struct {
	Position int
}

type Parser struct {
	tokens  []token.Token
	imports []*ast.ImportElement

	// These are parser save/restore type frames
	frames []Frame

	// This is the current parser scope
	scope *ast.Scope

	/*
	   When >= 0, braces can attach to an identifier to form a struct literal
	   When <0 they cannot
	*/
	structLiteralOk int

	/*
	   When > 0 we are in a loop that can be broken out of (with a 'break' statement)
	*/
	breakOk int

	// the name to report in errors
	fileName string

	id ast.ModuleId

	es *errors.Errors

	mp ModuleProvider

	disallowedIds map[string]bool

	ffi *ast.FfiDefinitions
}

func NewParser(mp ModuleProvider, id ast.ModuleId, tkns []token.Token, es *errors.Errors, noImportIds map[string]bool, ffid *ast.FfiDefinitions) *Parser {
	rootScope := ast.NewScope(nil)
	if ffid != nil {
		for _, cf := range ffid.Functions {
			rootScope.AddCFunction(cf)
		}
	}

	return &Parser{
		ffi:             ffid,
		id:              id,
		es:              es,
		tokens:          tkns,
		structLiteralOk: 0,
		breakOk:         0,
		fileName:        id.Key(),
		frames: []Frame{
			Frame{Position: 0},
		},
		scope:         rootScope,
		mp:            mp,
		imports:       []*ast.ImportElement{},
		disallowedIds: noImportIds,
	}
}

// description of current parse location
func (p *Parser) CurrentLocation() errors.SourceLocation {
	cp := p.CurrentFrame().Position
	if cp >= len(p.tokens) {
		return errors.SourceLocation{
			Filename: p.fileName,
			Line:     -1,
		}
	} else {
		return errors.SourceLocation{
			Filename: p.fileName,
			Line:     p.tokens[cp].Line,
		}
	}
}

func (p *Parser) FindImport(module string) *ast.Module {
	for _, ie := range p.imports {
		if ie.ImportAs == module {
			return ie.Mod
		}
	}

	return nil
}

// Log an error at the current position
func (p *Parser) LogError(description string, args ...interface{}) {
	p.es.SetCurrentLocation(p.CurrentLocation())
	p.es.Errorf(description, args...)
}

func (p *Parser) LogExpectingError(expecting, context string) {
	p.LogError(fmt.Sprintf("Expecting '%v' in %v, have %v", expecting, context, p.DebugPeekToken()))
}

// Current parse context
func (p *Parser) CurrentFrame() *Frame {
	return &p.frames[len(p.frames)-1]
}

// return true if there are no more tokens left
func (p *Parser) Eof() bool {
	return p.CurrentFrame().Position >= len(p.tokens)
}

func (p *Parser) Save() {
	p.frames = append(p.frames, *p.CurrentFrame())
}

func (p *Parser) StartScope() {
	p.scope = ast.NewScope(p.scope)
}

func (p *Parser) EndScope() {
	p.scope = p.scope.Parent
}

func (p *Parser) Accept() {
	p.frames[len(p.frames)-2] = p.frames[len(p.frames)-1]
	p.frames = p.frames[:len(p.frames)-1]
}

func (p *Parser) Reject() {
	p.frames = p.frames[:len(p.frames)-1]
}

func (p *Parser) DebugPeekToken() token.Token {
	p.Save()
	t, _ := p.GetToken()
	p.Reject()
	return t
}

func (p *Parser) CurrentModuleId() ast.ModuleId {
	return p.id
}

// Look at the current token without consuming it
func (p *Parser) GetToken() (token.Token, bool) {
	if p.Eof() {
		return token.Token{}, false
	}

	f := p.CurrentFrame()

	tok := p.tokens[f.Position]
	f.Position += 1
	return tok, true
}

func (p *Parser) Token(ty token.TokenType) (token.Token, bool) {
	p.Save()
	tok, ok := p.GetToken()
	if !ok {
		p.Reject()
		return token.Token{}, false
	}

	if tok.Type == ty {
		p.Accept()
		return tok, true
	}
	p.Reject()

	return token.Token{}, false
}

// using https://alx71hub.github.io/hcb/

/*
Grab a type

This is a non-pointer type
*/
func (p *Parser) WholeType() (ast.Type, bool) {
	var fnd bool
	var tok token.Token

	_, fnd = p.Token(token.OpenSquare)
	if fnd {
		// vector type
		ity, ok := p.Type()
		if !ok {
			p.LogError("No open type found after '['")
			return ast.Type{}, false
		}

		_, fnd = p.Token(token.CloseSquare)
		if !fnd {
			p.LogError("No close found for vector type")
			return ast.Type{}, false
		}

		// vectors are always inherently pointers
		return ast.MakePointer(ast.Type{
			Selector: ast.KTypeVector,
			Types:    []ast.Type{ity},
		}), true
	}

	_, fnd = p.Token(token.Worker)
	if fnd {
		_, fnd = p.Token(token.OpenCurved)
		if !fnd {
			p.LogError("Expecting '(' after worker")
			return ast.Type{}, false
		}

		t := ast.Type{
			Selector: ast.KTypeWorker,
			Types:    []ast.Type{},
			Return:   nil,
		}

		firstType, firstTypeFound := p.Type()
		if firstTypeFound {
			t.Types = append(t.Types, firstType)
		}

		_, fnd = p.Token(token.CloseCurved)
		if !fnd {
			p.LogError("Expecting ')' after type")
			return ast.Type{}, false
		}

		returnType, returnFnd := p.Type()
		if returnFnd {
			t.Types = append(t.Types, returnType)
		} else {
			t.Types = append(t.Types, ast.Type{Selector: ast.KTypeVoid})
		}

		return t, true
	}

	_, fnd = p.Token(token.IntegerKeyword)
	if fnd {
		return ast.Type{Selector: ast.KTypeInteger}, true
	}

	_, fnd = p.Token(token.Float32Keyword)
	if fnd {
		return ast.Type{Selector: ast.KTypeFloat, Width: 32}, true
	}

	_, fnd = p.Token(token.Float64Keyword)
	if fnd {
		return ast.Type{Selector: ast.KTypeFloat, Width: 64}, true
	}

	_, fnd = p.Token(token.CharKeyword)
	if fnd {
		return ast.Type{Selector: ast.KTypeCharacter}, true
	}

	_, fnd = p.Token(token.BoolKeyword)
	if fnd {
		return ast.Type{Selector: ast.KTypeBoolean}, true
	}

	_, fnd = p.Token(token.StringKeyword)
	if fnd {
		return ast.Type{Selector: ast.KTypeString}, true
	}

	_, fnd = p.Token(token.OpenCurved)
	if fnd {
		tuple := ast.Type{
			Selector: ast.KTypeTuple,
			Types:    []ast.Type{},
		}

		for {
			if len(tuple.Types) > 0 {
				_, fnd := p.Token(token.Comma)
				if !fnd {
					break
				}
			}

			ty, fnd := p.Type()
			if !fnd {
				p.LogError("Did not find type in tuple")
				return ast.Type{}, false
			}
			tuple.Types = append(tuple.Types, ty)
		}

		_, fnd = p.Token(token.CloseCurved)
		if !fnd {
			p.LogError("Expecting close curved at the end of a tuple")
			return ast.Type{}, false
		}

		// This is a tuple type
		return tuple, true
	}

	tok, fnd = p.Token(token.Identifier)
	if fnd {
		// not sure this is always going to be the right thing to do
		return ast.Type{
			Selector: ast.KTypeStruct,
			StructId: ast.StructId{
				Module: p.CurrentModuleId(),
				Name:   tok.Tval,
			},
		}, true
	}

	return ast.Type{}, false
}

/*
Type (including pointer types)
*/
func (p *Parser) Type() (ast.Type, bool) {
	p.Save()

	_, isPointer := p.Token(token.Multiply)

	ty, ok := p.WholeType()
	if !ok {
		p.Reject()
		return ty, false
	}

	p.Accept()
	if isPointer {
		ty = ast.MakePointer(ty)
	}
	return ty, true
}

func (p *Parser) ResolvedId() (*ast.Module, string, bool) {
	p.Save()
	tok, fnd := p.Token(token.Identifier)
	if !fnd {
		p.Reject()
		return nil, "", false
	}

	_, ok := p.Token(token.ScopeResolution)
	if !ok {
		p.Reject()
		return nil, tok.Tval, false
	}

	sym, ok := p.Token(token.Identifier)
	if !ok {
		p.LogError("No identifier found after scope resolution operator")
		p.Reject()
		return nil, "", false
	}

	mod := p.FindImport(tok.Tval)
	if mod == nil {
		p.LogModuleNotFound(tok.Tval)
		p.Reject()
		return nil, "", false
	}

	p.Accept()
	return mod, sym.Tval, true
}

func (p *Parser) LogModuleNotFound(name string) {
	p.LogError("Parser failed to find module %v", name)
}

func (p *Parser) StructLiteralBody(moduleId ast.ModuleId, name string) (*ast.StructLiteralExpression, bool) {
	_, fnd := p.Token(token.OpenCurly)
	if !fnd {
		return nil, false
	}

	literalPairs := []ast.StructLiteralPair{}

	firstPass := true
	for {
		p.EatSemicolons()
		_, fnd = p.Token(token.CloseCurly)
		if fnd {
			return &ast.StructLiteralExpression{
				Id: ast.StructId{
					Module: moduleId,
					Name:   name,
				},
				Pairs: literalPairs,
			}, true
		}

		if len(literalPairs) > 0 {
			p.EatSemicolons()
			_, fnd = p.Token(token.Comma)
			if !fnd {
				p.LogExpectingError("comma", "struct literal")
				return nil, false
			}
		} else if !firstPass {
			p.LogExpectingError("value or closing '}'", "struct literal")
			return nil, false
		}

		p.EatSemicolons()
		slp, fnd := p.LiteralPair()
		if fnd {
			literalPairs = append(literalPairs, slp)
		}

		firstPass = false
	}
}

func (p *Parser) LiteralValueExpression() (ast.Expression, bool) {
	tok, fnd := p.Token(token.Integer)
	if fnd {
		return &ast.IntegerTerminal{Value: tok.Ival}, true
	}

	tok, fnd = p.Token(token.Null)
	if fnd {
		return &ast.NullLiteral{}, true
	}

	tok, fnd = p.Token(token.Self)
	if fnd {
		return &ast.SelfTerminal{}, true
	}

	tok, fnd = p.Token(token.Float32)
	if fnd {
		return &ast.FloatTerminal{LValue: tok.Ival, Zeros: tok.FvalZeros, RValue: tok.Fval, Width: 32}, true
	}

	tok, fnd = p.Token(token.Float64)
	if fnd {
		return &ast.FloatTerminal{LValue: tok.Ival, Zeros: tok.FvalZeros, RValue: tok.Fval, Width: 64}, true
	}

	tok, fnd = p.Token(token.String)
	if fnd {
		return &ast.StringTerminal{Value: tok.Tval}, true
	}

	tok, fnd = p.Token(token.Character)
	if fnd {
		return &ast.CharacterTerminal{CodePoint: tok.Ival}, true
	}

	_, fnd = p.Token(token.True)
	if fnd {
		return &ast.BooleanTerminal{Value: true}, true
	}

	_, fnd = p.Token(token.False)
	if fnd {
		return &ast.BooleanTerminal{Value: false}, true
	}

	_, fnd = p.Token(token.GpuBuiltin)
	if fnd {
		_, fnd = p.Token(token.ScopeResolution)
		if !fnd {
			p.LogExpectingError("::", "gpubuiltin")
			return nil, false
		}

		it, fnd := p.Token(token.Identifier)
		if !fnd {
			p.LogExpectingError("identifier", "gpubuiltin")
			return nil, false
		}

		return &ast.GpuBuiltinTerminal { Name: it.Tval }, true
	}

	mod, name, fnd := p.ResolvedId()
	if fnd {
		def, foundFunction := mod.LookupFunction(name)
		sd, foundStruct := mod.LookupStruct(name)
		if foundFunction {
			fid := def.Id
			if !def.Exported {
				p.LogError("Function %v in module %v is not exported", name, mod.Id.DisplayName())
				return nil, false
			}

			return &ast.IdentifierTerminal{
				Name:           name,
				DontNamespace:  false,
				CachedType:     def.OurType(),
				TypeSetInParse: true,
				Fid:            &fid,
			}, true
		} else if p.structLiteralOk < 0 {
			p.LogError("Cannot parse a struct literal in this context (not a function, so interpreted that way)")
			return nil, false
		} else if sl, fnd := p.StructLiteralBody(mod.Id, name); fnd && foundStruct {
			if !sd.Exported {
				p.LogError("struct %v in module %v is not exported", name, mod.Id.DisplayName())
				return nil, false
			}

			return sl, true
		} else {
			p.LogError("Do not recognise the scoped identifier in this context")
			return nil, false
		}
	}

	tok, fnd = p.Token(token.Identifier)
	if fnd {
		if p.structLiteralOk < 0 {
			// force a basic identifier
			return &ast.IdentifierTerminal{Name: tok.Tval, DontNamespace: false}, true
		} else {
			sl, fnd := p.StructLiteralBody(p.CurrentModuleId(), tok.Tval)
			if fnd {
				return sl, true
			} else {
				// basic identifier
				return &ast.IdentifierTerminal{Name: tok.Tval, DontNamespace: false}, true
			}
		}
	}

	// vector literal
	_, fnd = p.Token(token.OpenSquare)
	if fnd {
		// vector type
		ity, ok := p.Type()
		if !ok {
			p.LogError("No type found after '['")
			return nil, false
		}

		_, fnd = p.Token(token.CloseSquare)
		if !fnd {
			p.LogError("No close square found for vector type")
		}

		_, fnd = p.Token(token.OpenCurly)
		if !fnd {
			p.LogError("No open curly found for vector type")
		}

		es, fnd := p.ExpressionList(false, true)
		if !fnd {
			p.LogError("Expecting expression list in vector literal")
		}

		_, fnd = p.Token(token.CloseCurly)
		if !fnd {
			p.LogError("Expecting closed curly after vector literal")
		}

		return &ast.VectorLiteralExpression{
			ElementType:  ity,
			Initialisers: es,
		}, true
	}

	return nil, false
}

func (p *Parser) PrimaryExpression() (ast.Expression, bool) {
	if _, fnd := p.Token(token.OpenCurved); fnd {
		innerExpression, fnd := p.Expression()
		if !fnd {
			p.LogError("Expecting an expression after the curved brackets")
			return nil, false
		}

		if _, fnd := p.Token(token.CloseCurved); !fnd {
			p.LogError("Missing closing curved bracket")
			return nil, false
		}

		return innerExpression, true
	}

	return p.LiteralValueExpression()
}

// Parse an expression list
//
// When allowPlaceholders is true, it will parse for _, and leave nil in those cases
func (p *Parser) ExpressionList(allowPlaceholders, allowTrailingComma bool) ([]ast.Expression, bool) {
	el := []ast.Expression{}

	checkLeading := true
	var fnd bool

	if allowPlaceholders {
		_, fnd = p.Token(token.Placeholder)
		if fnd {
			el = append(el, nil)
			checkLeading = false
		}
	}
	if checkLeading {
		leadingExpression, fnd := p.Expression()
		if !fnd {
			// an empty list is a valid list
			return el, true
		}
		el = append(el, leadingExpression)
	}

	for {
		_, fnd = p.Token(token.Comma)
		if !fnd {
			break
		}

		if allowPlaceholders {
			_, fnd = p.Token(token.Placeholder)
			if fnd {
				el = append(el, nil)
				continue
			}
		}

		nextExpression, fnd := p.Expression()
		if !fnd {
			if allowTrailingComma {
				break
			} else {
				p.LogError("Expecting expression after command in expression list")
				return nil, false
			}
		}

		el = append(el, nextExpression)
	}

	return el, true
}

// ident: exp
func (p *Parser) LiteralPair() (ast.StructLiteralPair, bool) {
	ident, fnd := p.Token(token.Identifier)
	if !fnd {
		return ast.StructLiteralPair{}, false
	}

	_, fnd = p.Token(token.Colon)
	if !fnd {
		p.LogError("Expecting colon after identifier in struct (" + ident.Tval + ")")
		return ast.StructLiteralPair{}, false
	}

	e, fnd := p.Expression()
	if !fnd {
		p.LogError("Expecting expression after colon in struct")
		return ast.StructLiteralPair{}, false
	}

	return ast.StructLiteralPair{
		FieldName: ident.Tval,
		Value:     e,
	}, true
}

func (p *Parser) PostfixExpression() (ast.Expression, bool) {
	pe, fnd := p.PrimaryExpression()
	if !fnd {
		return nil, false
	}

	for {
		_, fnd = p.Token(token.As)
		if fnd {
			ty, fnd := p.Type()
			if !fnd {
				p.LogExpectingError("type", "cast expression")
			}

			pe = &ast.CastExpression {
				NewType: ty,
				Casted: pe,
				CheckCastable: true,
			}
		}
		
		_, fnd = p.Token(token.OpenCurved)
		if fnd {
			el, fnd := p.ExpressionList(false, false)
			if !fnd {
				// this should already have printed a more useful error than we can generate here
				return nil, false
			}

			_, fnd = p.Token(token.CloseCurved)
			if !fnd {
				p.LogExpectingError("')'", "call statement")
				return nil, false
			}

			pe = &ast.CallExpression{
				CalledExpression: pe,
				Arguments:        el,
			}
			continue
		}

		_, fnd = p.Token(token.OpenSquare)
		if fnd {
			e, fnd := p.Expression()
			if !fnd {
				// this should already have printed a more useful error than we can generate here
				return nil, false
			}

			_, fnd = p.Token(token.CloseSquare)
			if !fnd {
				p.LogError(fmt.Sprintf("Expecting ']' in access statement, have %v", p.DebugPeekToken()))
				return nil, false
			}

			pe = &ast.IndexExpression{
				Indexed: pe,
				Index:   e,
			}
			continue
		}

		_, fnd = p.Token(token.Dot)
		if fnd {
			ident, fnd := p.Token(token.Identifier)
			if !fnd {
				p.LogError("Expected an identifier after '.'")
				return nil, false
			}

			pe = &ast.AccessExpression{
				Accessed:   pe,
				AllowRaw:   false,
				Identifier: ident.Tval,
			}
			continue
		}

		break
	}

	return pe, true
}

func (p *Parser) UnaryExpression() (ast.Expression, bool) {
	tokMap := map[token.TokenType]ast.UnaryOperator{
		token.Not:   ast.KOperatorNot,
		token.Minus: ast.KOperatorNegate,
	}

	for tokType, op := range tokMap {
		_, fnd := p.Token(tokType)
		if !fnd {
			continue
		}

		e, fnd := p.PostfixExpression()
		if !fnd {
			p.LogError(fmt.Sprintf("Expecting expression after %v", tokType))
			return nil, false
		}

		return &ast.UnaryExpression{
			Operator: op,
			Rhs:      e,
		}, true
	}

	return p.PostfixExpression()
}

// A factory method for infix expressions
func (p *Parser) binaryInfixExpression(tkns map[token.TokenType]ast.BinaryOperator, next func() (ast.Expression, bool)) (ast.Expression, bool) {
	pe, fnd := next()
	if !fnd {
		return nil, false
	}

	for {
		fnd = false
		var binop ast.BinaryOperator
		for tkn, op := range tkns {
			_, fnd = p.Token(tkn)
			if fnd {
				binop = op
				break
			}
		}
		if !fnd {
			break
		}

		rhs, fnd := next()
		if !fnd {
			p.LogError("Expecting RHS expression")
			return nil, false
		}

		pe = &ast.BinaryExpression{
			Lhs:      pe,
			Rhs:      rhs,
			Operator: binop,
		}
	}

	return pe, true
}

func (p *Parser) MultiplicativeExpression() (ast.Expression, bool) {
	t := map[token.TokenType]ast.BinaryOperator{
		token.Multiply: ast.KOperatorMultiply,
		token.Divide:   ast.KOperatorDivide,
		token.Percent:  ast.KOperatorMod,
	}
	return p.binaryInfixExpression(t, p.UnaryExpression)
}

func (p *Parser) AdditiveExpression() (ast.Expression, bool) {
	t := map[token.TokenType]ast.BinaryOperator{
		token.Plus:  ast.KOperatorAdd,
		token.Minus: ast.KOperatorSubtract,
	}
	return p.binaryInfixExpression(t, p.MultiplicativeExpression)
}

func (p *Parser) RelationalExpression() (ast.Expression, bool) {
	t := map[token.TokenType]ast.BinaryOperator{
		token.GreaterThan:        ast.KOperatorGT,
		token.GreaterThanOrEqual: ast.KOperatorGTE,
		token.LessThan:           ast.KOperatorLT,
		token.LessThanOrEqual:    ast.KOperatorLTE,
	}
	return p.binaryInfixExpression(t, p.AdditiveExpression)
}

func (p *Parser) EqualityExpression() (ast.Expression, bool) {
	t := map[token.TokenType]ast.BinaryOperator{
		token.Equality:   ast.KOperatorEquality,
		token.Inequality: ast.KOperatorInequality,
	}
	return p.binaryInfixExpression(t, p.RelationalExpression)
}

func (p *Parser) LogicalAndExpression() (ast.Expression, bool) {
	t := map[token.TokenType]ast.BinaryOperator{
		token.And: ast.KOperatorAnd,
	}
	return p.binaryInfixExpression(t, p.EqualityExpression)
}

func (p *Parser) LogicalOrExpression() (ast.Expression, bool) {
	t := map[token.TokenType]ast.BinaryOperator{
		token.Or: ast.KOperatorOr,
	}
	return p.binaryInfixExpression(t, p.LogicalAndExpression)
}

func (p *Parser) AllocationExpression() (ast.Expression, bool) {
	p.Save()

	_, isAllocated := p.Token(token.New)
	lor, ok := p.LogicalOrExpression()
	if !ok {
		p.Reject()
		return nil, false
	}
	p.Accept()

	if isAllocated {
		return &ast.NewExpression{
			Initialiser: lor,
		}, true
	} else {
		return lor, true
	}
}

func (p *Parser) PrefixedExpression() (ast.Expression, bool) {
	p.Save()

	_, isRange := p.Token(token.Range)
	if isRange {
		// fail from here on
		p.Accept()

		_, ok := p.Token(token.OpenCurved)
		if !ok {
			p.LogError("Expecting '(' after 'range'")
			return nil, false
		}

		e, ok := p.Expression()
		if !ok {
			p.LogError("Expecting expression after 'range('")
			return nil, false
		}
		vals := []ast.Expression{e}

		for i := 0; i < 2; i += 1 {
			_, ok = p.Token(token.Comma)
			if ok {
				e, ok = p.Expression()
				if !ok {
					p.LogError("Expecting expression after ','")
					return nil, false
				}

				vals = append(vals, e)
			}
		}

		_, ok = p.Token(token.CloseCurved)
		if !ok {
			p.LogError("Expecting ')' after 'range'")
			return nil, false
		}
		r := &ast.RangeExpression{
			Count: nil,
			Start: nil,
			Step:  nil,
		}
		switch len(vals) {
		case 1:
			r.Count = vals[0]
			r.Start = &ast.IntegerTerminal{Value: 0}
			r.Step = &ast.IntegerTerminal{Value: 1}

		case 2:
			r.Start = vals[0]
			r.Count = vals[1]
			r.Step = &ast.IntegerTerminal{Value: 1}

		case 3:
			r.Start = vals[0]
			r.Count = vals[1]
			r.Step = vals[2]
		}

		return r, true
	}

	_, isDeref := p.Token(token.Multiply)
	if isDeref {
		alloc, ok := p.AllocationExpression()
		if !ok {
			p.Reject()
			return nil, false
		}
		p.Accept()

		return &ast.DereferenceExpression{
			Pointer: alloc,
		}, true
	}

	_, isDrain := p.Token(token.Drain)
	if isDrain {
		_, ok := p.Token(token.OpenCurved)
		if !ok {
			p.LogError("Expecting '(' after 'drain'")
			p.Reject()
			return nil, false
		}

		pipe, ok := p.AllocationExpression()
		if !ok {
			p.Reject()
			return nil, false
		}
		p.Accept()

		_, ok = p.Token(token.CloseCurved)
		if !ok {
			p.LogError("Expecting ')' after expression in 'drain'")
			return nil, false
		}

		return &ast.ReceiveWorkerExpression{
			Worker: pipe,
			All:    true,
		}, true
	}

	_, isPartial := p.Token(token.Partial)
	if isPartial {
		pe, fnd := p.PrimaryExpression()
		if !fnd {
			p.LogExpectingError("primary expression", "partial expression")
			return nil, false
		}

		_, fnd = p.Token(token.OpenCurved)
		if !fnd {
			p.LogExpectingError("(", "partial expression")
			return nil, false
		}

		el, fnd := p.ExpressionList(true, false)
		if !fnd {
			// this should already have printed a more useful error than we can generate here
			p.Reject()
			return nil, false
		}

		_, fnd = p.Token(token.CloseCurved)
		if !fnd {
			p.LogExpectingError(")", "partial expression")
			return nil, false
		}

		if len(el) == 0 {
			// maybe this shouldn't be an error
			p.LogError("There is no reason to partially apply a function of 0 arguments")
			return nil, false
		}

		p.Accept()
		return &ast.ClosureExpression{
			CalledExpression:  pe,
			SuppliedArguments: el,
		}, true
	}

	_, isPipeline := p.Token(token.Pipeline)
	if isPipeline {
		lhs, ok := p.Expression()
		if !ok {
			p.LogExpectingError("expression", "pipeline (1)")
			return nil, false
		}

		rhs, ok := p.Expression()
		if !ok {
			p.LogExpectingError("expression", "pipeline (2)")
			return nil, false
		}

		p.Accept()
		return &ast.CreatePipelineExpression{
			LhsWorker: lhs,
			RhsWorker: rhs,
		}, true
	}

	_, isReceive := p.Token(token.Receive)
	if isReceive {
		_, ok := p.Token(token.OpenCurved)
		if !ok {
			p.LogError("Expecting '(' after 'receive'")
			p.Reject()
			return nil, false
		}

		pipe, ok := p.AllocationExpression()
		if !ok {
			p.Reject()
			return nil, false
		}
		p.Accept()

		_, ok = p.Token(token.CloseCurved)
		if !ok {
			p.LogError("Expecting ')' after expression in 'receive'")
			p.Reject()
			return nil, false
		}

		return &ast.ReceiveWorkerExpression{
			Worker: pipe,
			All:    false,
		}, true
	}

	var isCpu = false
	var isGpu = false
	_, isCpu = p.Token(token.Cpu)
	if !isCpu {
		_, isGpu = p.Token(token.Gpu)
	}
	if isCpu || isGpu {
		worker, ok := p.Expression()
		if !ok {
			p.Reject()
			return nil, false
		}
		p.Accept()
		dest := ast.KDestinationCpu
		if isGpu {
			dest = ast.KDestinationGpu
		}

		return &ast.CreateWorkerExpression{
			Worker:      worker,
			Destination: dest,
		}, true
	}

	p.Reject()

	return p.AllocationExpression()
}

/*
This is an expression, disallowing tuple expressions, which are not exposed types to the user
*/
func (p *Parser) Expression() (ast.Expression, bool) {
	p.structLiteralOk += 1
	e, ok := p.PrefixedExpression()
	p.structLiteralOk -= 1
	return e, ok
}

/*
Parse out a potential tuple
*/
func (p *Parser) TupleExpression() (ast.Expression, bool) {
	leadingExpression, fnd := p.Expression()
	if !fnd {
		return nil, false
	}

	es := []ast.Expression{
		leadingExpression,
	}

	for {
		_, fnd = p.Token(token.Comma)
		if !fnd {
			if len(es) == 1 {
				return leadingExpression, true
			} else {
				return &ast.TupleExpression{
					Expressions: es,
				}, true
			}
		}

		nextExpression, fnd := p.Expression()
		if !fnd {
			p.LogError("Expecting expression after comma in expression list")
			return nil, false
		}

		es = append(es, nextExpression)
	}
}

// single token lvalue
func (p *Parser) SimpleLValue() (ast.LValue, bool) {
	_, fnd := p.Token(token.Self)
	if fnd {
		return &ast.SelfLValue{}, true
	}

	ident, fnd := p.Token(token.Identifier)
	if !fnd {
		return nil, false
	}

	return &ast.IdentifierLValue{Name: ident.Tval}, true

}

/*
a.b
*/
func (p *Parser) AccessLValue() (ast.LValue, bool) {
	lv, fnd := p.SimpleLValue()
	if !fnd {
		return nil, false
	}

	_, fnd = p.Token(token.Dot)
	if fnd {
		ident, fnd := p.Token(token.Identifier)
		if !fnd {
			p.LogError("No identifier found after dot in LValue")
			return nil, false
		}

		return &ast.AccessorLValue{Inner: lv, FieldName: ident.Tval}, true
	}

	return lv, true
}

/*
a[b] =
*/
func (p *Parser) IndexingLValue() (ast.LValue, bool) {
	lv, fnd := p.AccessLValue()
	if !fnd {
		return nil, false
	}

	for {
		_, fnd = p.Token(token.OpenSquare)
		if !fnd {
			break
		}

		ind, fnd := p.Expression()
		if !fnd {
			p.LogError("Expected an expression after '[' in accessor lvalue")
			return nil, false
		}

		_, fnd = p.Token(token.CloseSquare)
		if !fnd {
			p.LogError("Expected an ']' in accessor lvalue")
			return nil, false
		}

		lv = &ast.IndexLValue{
			Indexed: lv,
			Index:   ind,
		}
	}

	return lv, true
}

/*
 *x =
 */
func (p *Parser) DeferenceLValue() (ast.LValue, bool) {
	p.Save()
	_, isDeref := p.Token(token.Multiply)

	lv, fnd := p.IndexingLValue()
	if !fnd {
		p.Reject()
		return nil, false
	}
	p.Accept()

	if isDeref {
		return &ast.DerefLValue{Inner: lv}, true
	} else {
		return lv, true
	}
}

/*
multiple lvalues
*/
func (p *Parser) LValue() (ast.LValue, bool) {
	lv, fnd := p.DeferenceLValue()
	if !fnd {
		return nil, false
	}

	lvalues := []ast.LValue{lv}
	for {
		_, fnd = p.Token(token.Comma)
		if !fnd {
			if len(lvalues) == 1 {
				return lv, true
			} else {
				return &ast.MultipleLValue{LValues: lvalues}, true
			}
		}

		nextLv, fnd := p.DeferenceLValue()
		if !fnd {
			p.LogError("Expecting l-values after comma")
			return nil, false
		}

		lvalues = append(lvalues, nextLv)
	}
}

func (p *Parser) ImportLine() (ast.TopLevelElement, bool) {
	_, fnd := p.Token(token.Import)
	if !fnd {
		return nil, false
	}

	id, fnd := p.Token(token.Identifier)
	if !fnd {
		p.LogError("Expecting identifer after import")
		return nil, false
	}

	ie := &ast.ImportElement{
		Names:    []string{id.Tval},
		ImportAs: id.Tval,
	}

	for {
		_, fnd = p.Token(token.ScopeResolution)
		if !fnd {
			break
		}

		id, fnd = p.Token(token.Identifier)
		if !fnd {
			p.LogError("Expecting identifer after dot in import")
		}

		ie.Names = append(ie.Names, id.Tval)
		ie.ImportAs = id.Tval
	}

	if _, fnd := p.disallowedIds[ie.ImportedId().Key()]; fnd {
		p.LogError("Import cycle found when importing '%v' from '%v'", ie.ImportedId(), p.id)
		return nil, false
	}

	_, fnd = p.Token(token.As)
	if fnd {
		id, fnd = p.Token(token.Identifier)
		if !fnd {
			p.LogExpectingError("identifier", "import statement")
			return nil, false
		}

		ie.ImportAs = id.Tval
	}

	ie.Mod = p.mp.GetModule(ie.Names, p.disallowedIds)
	if ie.Mod == nil {
		p.LogError("Parser failed to find module %v", strings.Join(ie.Names, "."))
		return nil, false
	}
	p.imports = append(p.imports, ie)

	for _, s := range ie.Mod.Structs {
		p.scope.SetStruct(s.Id, s.Definition)
	}

	return ie, true
}

func (p *Parser) StructDefinition() (ast.TopLevelElement, bool) {
	p.Save()
	_, exported := p.Token(token.Export)

	_, fnd := p.Token(token.Struct)
	if !fnd {
		p.Reject()
		return nil, false
	}
	p.Accept()

	structNameTok, fnd := p.Token(token.Identifier)
	if !fnd {
		p.LogError("Expecting identifier after struct keyword")
		return nil, false
	}
	structId := ast.StructId{
		Name:   structNameTok.Tval,
		Module: p.CurrentModuleId(),
	}

	p.StartScope()
	defer p.EndScope()
	ourScope := p.scope

	ourScope.SetVariable("__self__", ast.MakePointer(ast.Type{Selector: ast.KTypeStruct, StructId: structId}), false)

	_, fnd = p.Token(token.OpenCurly)
	if !fnd {
		p.LogError("Expecting '{' after struct name")
		return nil, false
	}

	sd := ast.StructDefinition{
		Scope:     ourScope,
		Fields:    []ast.StructField{},
		Functions: []*ast.FunctionDefinition{},
	}

	for {
		p.EatSemicolons()

		fn, fnd := p.FunctionDefinition()
		if fnd {
			fn.Id.Struct = structId
			sd.Functions = append(sd.Functions, fn)
			continue
		}

		fps, fnd := p.ParameterListSegment()
		if !fnd || len(fps) == 0 {
			break
		}

		for _, fp := range fps {
			sd.Fields = append(sd.Fields, ast.StructField{
				Name: fp.Name,
				Type: fp.Type,
			})
		}
	}

	p.EatSemicolons()
	_, fnd = p.Token(token.CloseCurly)
	if !fnd {
		p.LogError("Expecting '}' after struct")
		return nil, false
	}

	return &ast.StructDefinitionStatement{
		Exported:   exported,
		Id:         structId,
		Definition: sd,
	}, true
}

// send pipe expression
func (p *Parser) SendStatement() (ast.Statement, bool) {
	_, fnd := p.Token(token.Send)
	if !fnd {
		return nil, false
	}

	_, fnd = p.Token(token.OpenCurved)
	if !fnd {
		p.LogError("Expecting ( after 'send' first expression")
		return nil, false
	}

	pipe, fnd := p.Expression()
	if !fnd {
		p.LogError("Expecting expression after 'send'")
		return nil, false
	}

	_, fnd = p.Token(token.Comma)
	if !fnd {
		p.LogError("Expecting comma after 'send' first expression")
		return nil, false
	}

	value, fnd := p.Expression()
	if !fnd {
		p.LogError("Expecting second values after 'send'")
		return nil, false
	}

	_, fnd = p.Token(token.CloseCurved)
	if !fnd {
		p.LogError("Expecting ) after 'send' first expression")
		return nil, false
	}

	return &ast.SendPipeStatement{
		Pipe:  pipe,
		Value: value,
	}, true
}

func (p *Parser) ForeachStatement() (ast.Statement, bool) {
	_, fnd := p.Token(token.Foreach)
	if !fnd {
		return nil, false
	}

	identifierToken, fnd := p.Token(token.Identifier)
	if !fnd {
		p.LogError("Expecting identifier after 'for'")
		return nil, false
	}

	_, fnd = p.Token(token.Colon)
	if !fnd {
		p.LogError("Expecting ':' after identifier in 'for'")
		return nil, false
	}

	p.structLiteralOk -= 2
	iterableExpression, fnd := p.Expression()
	p.structLiteralOk += 2
	if !fnd {
		p.LogError("Expecting expression after ':' in 'for'")
		return nil, false
	}

	p.breakOk += 1
	block, fnd := p.StatementBlock()
	if !fnd {
		p.LogError("Statement block expected after for statement")
		return nil, false
	}
	p.breakOk -= 1

	return &ast.ForeachStatement{
		TemporaryVariableName: identifierToken.Tval,
		Iterable:              iterableExpression,
		Body:                  block,
		Variant:               ast.KForEach,
	}, true
}

// let x = Expression
func (p *Parser) LetLikeStatement(tt token.TokenType, at ast.AssignType) (*ast.AssignStatement, bool) {
	_, fnd := p.Token(tt)
	if !fnd {
		return nil, false
	}

	lv, fnd := p.LValue()
	if !fnd {
		p.LogError("No lvalue found after let or const")
		return nil, false
	}

	_, fnd = p.Token(token.Equals)
	if !fnd {
		p.LogError("No equals found in let statement")
		return nil, false
	}

	e, fnd := p.Expression()
	if !fnd {
		p.LogError("No expression found in let statement")
		return nil, false
	}

	return &ast.AssignStatement{
		Type:        at,
		Lhs:         lv,
		Rhs:         e,
		PinPointers: true,
	}, true
}

// let x = Expression
func (p *Parser) LetStatement() (ast.Statement, bool) {
	return p.LetLikeStatement(token.Let, ast.KAssignLet)
}

// const x = Expression
func (p *Parser) ConstStatement() (ast.Statement, bool) {
	return p.LetLikeStatement(token.Const, ast.KAssignConst)
}

func (p *Parser) ModifyOperator() (ast.ModifyOperator, bool) {
	tokenTypes := []token.TokenType{
		token.PlusEquals,
		token.MinusEquals,
		token.TimesEquals,
		token.DivideEquals,
	}

	operators := []ast.ModifyOperator{
		ast.KModifyPlus,
		ast.KModifyMinus,
		ast.KModifyTimes,
		ast.KModifyDivide,
	}

	for i, tt := range tokenTypes {
		_, fnd := p.Token(tt)
		if fnd {
			return operators[i], true
		}
	}

	return ast.KModifyPlus, false
}

func (p *Parser) ModifyInPlaceStatement() (ast.Statement, bool) {
	p.Save()
	lv, fnd := p.LValue()
	if !fnd {
		p.Reject()
		return nil, false
	}

	mo, fnd := p.ModifyOperator()
	if !fnd {
		p.Reject()
		return nil, false
	}

	p.Accept()

	e, fnd := p.Expression()
	if !fnd {
		p.LogError("Expression expected after modify in place operator")
		return nil, false
	}

	return &ast.ModifyInPlaceStatement{
		Operator:   mo,
		Modified:   lv,
		Expression: e,
	}, true
}

func (p *Parser) AssignStatement() (ast.Statement, bool) {
	p.Save()
	lv, fnd := p.LValue()
	if !fnd {
		p.Reject()
		return nil, false
	}

	_, fnd = p.Token(token.Equals)
	if !fnd {
		p.Reject()
		return nil, false
	}

	rhs, fnd := p.TupleExpression()
	if !fnd {
		p.Reject()
		p.LogError("Expression expected after assignment")
		return nil, false
	}

	p.Accept()

	return &ast.AssignStatement{
		Type:        ast.KAssignNormal,
		Lhs:         lv,
		Rhs:         rhs,
		PinPointers: true,
	}, true
}

func (p *Parser) BreakStatement() (ast.Statement, bool) {
	_, fnd := p.Token(token.Break)
	if !fnd {
		return nil, false
	}

	if p.breakOk == 0 {
		p.LogError("Cannot break outside of a breakable block (e.g. for or while)")
		return nil, false
	}

	return &ast.BreakStatement{}, true
}

func (p *Parser) WhileStatement() (ast.Statement, bool) {
	_, fnd := p.Token(token.While)
	if !fnd {
		return nil, false
	}

	p.structLiteralOk -= 2

	condition, fnd := p.Expression()
	if !fnd {
		p.LogError("Expression expected after while statement")
		return nil, false
	}

	p.structLiteralOk += 2

	p.breakOk += 1
	block, fnd := p.StatementBlock()
	if !fnd {
		p.LogError("Statement block expected after while statement")
		return nil, false
	}
	p.breakOk -= 1

	return &ast.WhileStatement{
		Condition: condition,
		Block:     block,
	}, true
}

func (p *Parser) IfStatement() (ast.Statement, bool) {
	_, fnd := p.Token(token.If)
	if !fnd {
		return nil, false
	}

	p.structLiteralOk -= 2
	ifCondition, fnd := p.Expression()
	p.structLiteralOk += 2
	if !fnd {
		p.LogError("Expression expected after if statement")
		return nil, false
	}

	ifBlock, fnd := p.StatementBlock()
	if !fnd {
		p.LogError("Statement block expected after if statement")
		return nil, false
	}

	stmt := &ast.IfStatement{
		Segments: []ast.IfStatementSegment{
			ast.IfStatementSegment{
				Condition: ifCondition,
				Block:     ifBlock,
			},
		},
	}

	for {
		_, elseIfFnd := p.Token(token.ElseIf)
		if !elseIfFnd {
			break
		}

		elseIfCondition, fnd := p.Expression()
		if !fnd {
			p.LogError("Expression expected after elseif statement")
			return nil, false
		}

		elseIfBlock, fnd := p.StatementBlock()
		if !fnd {
			p.LogError("Statement block expected after elseif statement")
			return nil, false
		}

		stmt.Segments = append(stmt.Segments, ast.IfStatementSegment{
			Condition: elseIfCondition,
			Block:     elseIfBlock,
		})
	}

	_, elseFnd := p.Token(token.Else)
	if elseFnd {
		elseBlock, fnd := p.StatementBlock()
		if !fnd {
			p.LogError("Statement block expected after else statement")
			return nil, false
		}

		stmt.Segments = append(stmt.Segments, ast.IfStatementSegment{
			Condition: nil,
			Block:     elseBlock,
		})
	}

	return stmt, true
}

// parse out a return statement
func (p *Parser) ReturnStatement() (ast.Statement, bool) {
	_, fnd := p.Token(token.Return)
	if !fnd {
		return nil, false
	}

	p.Save()
	e, fnd := p.TupleExpression()
	if fnd {
		p.Accept()
		return &ast.ReturnStatement{
			ReturnedValue: e,
		}, true
	} else {
		p.Reject()
		return &ast.ReturnStatement{
			ReturnedValue: nil,
		}, true
	}

}

func (p *Parser) ExpressionStatement() (ast.Statement, bool) {
	e, fnd := p.Expression()
	if !fnd {
		return nil, false
	}

	return &ast.ExpressionStatement{
		Expression: e,
	}, true
}

func (p *Parser) EatSemicolons() {
	for {
		p.Save()

		_, ok := p.Token(token.Semicolon)
		if !ok {
			p.Reject()
			return
		}
		p.Accept()
	}
}

func (p *Parser) Statement() (ast.Statement, bool) {
	p.EatSemicolons()

	// this will be approximate at best
	statements := []func() (ast.Statement, bool){
		// immediately identifiable by a keyword
		p.ForeachStatement,
		p.LetStatement,
		p.ConstStatement,
		p.SendStatement,
		p.ReturnStatement,
		p.IfStatement,
		p.WhileStatement,
		p.BreakStatement,

		p.ModifyInPlaceStatement,
		p.AssignStatement,
		p.ExpressionStatement,
	}

	for _, statement := range statements {
		cs, ok := statement()
		if ok {
			p.EatSemicolons()
			return cs, true
		}
	}

	return nil, false
}

// list of statements in curly parents
func (p *Parser) StatementBlock() (*ast.StatementBlock, bool) {
	_, fnd := p.Token(token.OpenCurly)
	if !fnd {
		return nil, false
	}

	// success or fail we need to nuke the scope after this
	p.StartScope()
	defer p.EndScope()

	statements := []ast.StatementContainer{}

	for {
		loc := p.CurrentLocation()

		s, fnd := p.Statement()
		if !fnd {
			break
		}

		lsc := ast.StatementContainer{
			Statement: &ast.DummyStatement{
				Location: loc,
			},
			Context: p.scope,
		}

		sc := ast.StatementContainer{
			Statement: s,
			Context:   p.scope,
		}

		statements = append(statements, lsc, sc)
	}

	_, fnd = p.Token(token.CloseCurly)
	if !fnd {
		p.LogError("StatementBlock(): No close curly found")
		return nil, false
	}

	return &ast.StatementBlock{
		Statements: statements,
		Context:    p.scope,
	}, true
}

// a single batch of params a,b,c int
func (p *Parser) ParameterListSegment() ([]ast.FunctionParameter, bool) {
	names := []ast.FunctionParameter{}

	leadingName, fnd := p.Token(token.Identifier)
	if !fnd {
		return names, true
	}

	names = append(names, ast.FunctionParameter{Name: leadingName.Tval})
	for {
		_, fnd = p.Token(token.Comma)
		if !fnd {
			break
		}

		nextName, fnd := p.Token(token.Identifier)
		if !fnd {
			p.LogError("Expecting identifier after command in parameter list")
			return nil, false
		}

		names = append(names, ast.FunctionParameter{Name: nextName.Tval})
	}

	ty, fnd := p.Type()
	if !fnd {
		p.LogError("Expecting type after parameters in parameter list")
		return nil, false
	}

	for i, _ := range names {
		names[i].Type = ty
	}

	return names, true
}

// an enttire list a,b,c int, d string
func (p *Parser) ParameterList() ([]ast.FunctionParameter, bool) {
	ps := []ast.FunctionParameter{}

	leadingSegment, fnd := p.ParameterListSegment()
	if len(leadingSegment) == 0 {
		// don't allow a list to start with a comma
		return ps, true
	}

	ps = append(ps, leadingSegment...)
	for {
		_, fnd = p.Token(token.Comma)
		if !fnd {
			break
		}

		nextSegment, fnd := p.ParameterListSegment()
		if !fnd {
			p.LogError("Expecting parameters after command in parameter list")
			return nil, false
		}

		ps = append(ps, nextSegment...)
	}

	return ps, true
}

func (p *Parser) FunctionDefinition() (*ast.FunctionDefinition, bool) {
	p.Save()

	loc := ast.KLocationAnywhere

	_, exported := p.Token(token.Export)

	if _, cpuKeyword := p.Token(token.Cpu); cpuKeyword {
		loc = ast.KLocationCpu
	} else if _, gpuKeyword := p.Token(token.Gpu); gpuKeyword {
		loc = ast.KLocationGpu
	}

	_, fnd := p.Token(token.Function)
	if !fnd {
		p.Reject()
		return &ast.FunctionDefinition{}, false
	}
	// at this point it is function or failure, so may as well accept
	p.Accept()

	p.StartScope()
	defer p.EndScope()
	ourScope := p.scope

	ident, fnd := p.Token(token.Identifier)
	if !fnd {
		p.LogError("FunctionDefinition(): No identifier found")
		return nil, false
	}

	_, fnd = p.Token(token.OpenCurved)
	if !fnd {
		p.LogError("FunctionDefinition(): No open paren found")
		return nil, false
	}

	parameters, fnd := p.ParameterList()
	if !fnd {
		// there should be a better error already
		return nil, false
	}

	// TODO should this be in the Check instead?
	for _, param := range parameters {
		ourScope.SetVariable(param.Name, param.Type, true)
	}

	_, fnd = p.Token(token.CloseCurved)
	if !fnd {
		p.LogError("FunctionDefinition(): No close curved found")
		return nil, false
	}

	returnType, fnd := p.Type()
	if !fnd {
		returnType = ast.Type{Selector: ast.KTypeVoid}
	}

	statements, fnd := p.StatementBlock()
	if !fnd {
		p.LogError("FunctionDefinition(): No statement block found following definition")
	}

	return &ast.FunctionDefinition{
		Id: ast.FunctionId{
			Name:   ident.Tval,
			Struct: ast.BlankStructId(),
			Module: p.id,
		},
		Exported: exported,
		Return:          returnType,
		AvoidCheckPhase: false,
		Scope:           ourScope,
		Block:           statements,
		Parameters:      parameters,
		Location:        loc,
	}, true
}

func (p *Parser) ConstTle() (ast.TopLevelElement, bool) {
	cs, ok := p.LetLikeStatement(token.Const, ast.KAssignConst)
	if !ok {
		return nil, false
	}

	return &ast.ConstTle{
		Assign: cs,
	}, true
}

func (p *Parser) FunctionDefinitionTle() (ast.TopLevelElement, bool) {
	fd, ok := p.FunctionDefinition()
	if !ok {
		return nil, false
	}

	return &ast.FunctionDefinitionTle{
		Definition: fd,
	}, true
}

func (p *Parser) TopLevelElement() (ast.TopLevelElement, bool) {
	tles := []func() (ast.TopLevelElement, bool){
		p.StructDefinition,
		p.FunctionDefinitionTle,
		p.ConstTle,
		p.ImportLine,
	}

	for _, tle := range tles {
		p.EatSemicolons()

		el, ok := tle()
		if ok {
			return el, true
		}
	}

	p.EatSemicolons()
	return nil, false
}

// Try parsing out a file
//
// This will act on Errors. If return is nil, then the errors are not clean
func (p *Parser) Module() *ast.Module {
	p.StartScope()
	defer p.EndScope()

	f := &ast.Module{
		TopLevelElements: []ast.TopLevelElementContainer{},
		Scope:            p.scope,
		Ffid:             p.ffi,
	}

	for {
		dtlec := ast.TopLevelElementContainer{
			TopLevelElement: &ast.DummyTle{
				Location: p.CurrentLocation(),
			},
			Context: p.scope,
		}

		tle, fnd := p.TopLevelElement()
		if !fnd {
			break
		}
		if !p.es.Clean() {
			return nil
		}

		tlec := ast.TopLevelElementContainer{
			TopLevelElement: tle,
			Context:         p.scope,
		}

		f.TopLevelElements = append(f.TopLevelElements, dtlec, tlec)
	}

	t, fnd := p.Token(token.Eof)
	if !fnd {
		p.LogError("Expecting EOF, got %v", t)
		return nil
	}

	return f
}
