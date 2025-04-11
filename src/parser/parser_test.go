package parser

import (
	"eyot/ast"
	"eyot/errors"
	"eyot/token"
	"testing"
)

// NB tests are fairly minimal and written where needed, ideally the stdout-tests would be the primary suite. That will continue to be useable as the implementation involves

func pf(t *testing.T, src string) *Parser {
	tkns, err := token.Tokenise(src)
	if err != nil {
		t.Errorf("Tokenise failed with error: %v", err)
	}

	return NewParser(nil, []string{"<unittest>"}, tkns, errors.NewErrors(), map[string]bool{}, nil)
}

func TestTerminals(t *testing.T) {
	p := pf(t, "12 \"hi\" true")

	e, ok := p.Expression()
	if !ok {
		t.Fatal("Could not parse expression 1")
	}
	it, ok := e.(*ast.IntegerTerminal)
	if !ok {
		t.Fatal("integer terminal expression wrong type")
	}
	if it.Value != 12 {
		t.Fatal("integer terminal wrong type")
	}

	e, ok = p.Expression()
	if !ok {
		t.Fatal("Could not parse expression 2")
	}
	st, ok := e.(*ast.StringTerminal)
	if !ok {
		t.Fatal("String terminal wrong type")
	}
	if st.Value != "hi" {
		t.Fatal("string terminal wrong value")
	}

	e, ok = p.Expression()
	if !ok {
		t.Fatal("Could not parse expression 3")
	}
	bt, ok := e.(*ast.BooleanTerminal)
	if !ok {
		t.Fatal("Boolean terminal wrong type")
	}
	if !bt.Value {
		t.Fatal("Boolean terminal wrong value")
	}
}

/*
Call statements are gone (should be expression statements with call expressions)
func TestCallStatementNoArgs(t *testing.T) {
	p := pf(t, "hello()")
	s, ok := p.Statement()
	if !ok {
		t.Fatal("Could not parse call statement 1")
	}
	cs, ok := s.(*ast.CallStatement)
	if !ok {
		t.Fatal("call statement 1 of wrong type")
	}
	if len(cs.Arguments) != 0 {
		t.Fatal("call statement 1 wrong arg count")
	}
}

func TestCallStatementOneArg(t *testing.T) {
	p := pf(t, "hello(1)")
	s, ok := p.Statement()
	if !ok {
		t.Fatal("Could not parse call statement 1")
	}
	cs, ok := s.(*ast.CallStatement)
	if !ok {
		t.Fatal("call statement 1 of wrong type")
	}
	if len(cs.Arguments) != 1 {
		t.Fatal("call statement 1 wrong arg count")
	}
	_, ok = cs.Arguments[0].(ast.IntegerTerminal)
	if !ok {
		t.Fatal("call statement 1 arg of wrong type")
	}
}

func TestCallStatementTwoArgs(t *testing.T) {
	p := pf(t, "hello(1, \"there\")")
	s, ok := p.Statement()
	if !ok {
		t.Fatal("Could not parse call statement 1")
	}
	cs, ok := s.(*ast.CallStatement)
	if !ok {
		t.Fatal("call statement 1 of wrong type")
	}
	if len(cs.Arguments) != 2 {
		t.Fatal("call statement 1 wrong arg count")
	}

	_, ok = cs.Arguments[0].(ast.IntegerTerminal)
	if !ok {
		t.Fatal("call statement 1 arg of wrong type")
	}
	_, ok = cs.Arguments[1].(ast.StringTerminal)
	if !ok {
		t.Fatal("call statement 1 arg two of wrong type")
	}
}
*/

func TestEmptyStatementBlock(t *testing.T) {
	p := pf(t, "{}")
	s, ok := p.StatementBlock()
	if !ok {
		t.Fatal("Could not parse call statement 1")
	}
	if len(s.Statements) != 0 {
		t.Fatal("nonempty block")
	}
}

func TestTwoStatements(t *testing.T) {
	p := pf(t, "{\nhello(12)\nthere(13)\n }")
	s, ok := p.StatementBlock()
	if !ok {
		t.Fatal("Could not parse call statement 1")
	}
	if len(s.Statements) != 4 {
		t.Fatal("Wrong number of statements in block")
	}
}

func TestFunctionDefinition(t *testing.T) {
	p := pf(t, "fn foo() {\nhello(12)\n }")
	tle, ok := p.TopLevelElement()
	if !ok {
		t.Fatal("Could not parse function definition")
	}
	fn, ok := tle.(*ast.FunctionDefinitionTle)
	if !ok {
		t.Fatal("Tle did not return function defn")
	}
	if len(fn.Definition.Block.Statements) != 2 {
		t.Fatal("Wrong number of statements in function")
	}
}

func TestBinop1(t *testing.T) {
	p := pf(t, "1 + 2 * 3")

	e, ok := p.Expression()
	if !ok {
		t.Fatal("Could not parse expression")
	}
	top, ok := e.(*ast.BinaryExpression)
	if !ok {
		t.Fatal("binary expression wrong type")
	}
	if top.Operator != ast.KOperatorAdd {
		t.Fatal("operator of wrong type")
	}
}

func TestBinop2(t *testing.T) {
	p := pf(t, "1 * 2 + 3")

	e, ok := p.Expression()
	if !ok {
		t.Fatal("Could not parse expression")
	}
	top, ok := e.(*ast.BinaryExpression)
	if !ok {
		t.Fatalf("binary expression wrong type: %v", e)
	}
	if top.Operator != ast.KOperatorAdd {
		t.Fatal("operator of wrong type")
	}
}

func TestParen(t *testing.T) {
	p := pf(t, "(1 + 2) * 3")

	e, ok := p.Expression()
	if !ok {
		t.Fatal("Could not parse expression")
	}
	top, ok := e.(*ast.BinaryExpression)
	if !ok {
		t.Fatal("binary expression wrong type")
	}
	if top.Operator != ast.KOperatorMultiply {
		t.Fatal("operator of wrong type")
	}
}

func TestLValue(t *testing.T) {
	// there was a bug with this
	p := pf(t, "a.b[c]")

	lv, ok := p.LValue()
	if !ok {
		t.Fatal("Could not parse lvalue")
	}

	ilv, ok := lv.(*ast.IndexLValue)
	if !ok {
		t.Fatal("Top level was not index")
	}

	_, ok = ilv.Indexed.(*ast.AccessorLValue)
	if !ok {
		t.Fatal("Top level was not accessor")
	}
}
