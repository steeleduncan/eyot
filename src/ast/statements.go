package ast

import (
	"eyot/errors"
)

type Statement interface {
	Check(*CheckContext, *Scope)
}

type StatementContainer struct {
	Statement Statement
	Context   *Scope
}

func (sc *StatementContainer) Check(ctx *CheckContext) {
	sc.Statement.Check(ctx, sc.Context)
}

type StatementBlock struct {
	Statements []StatementContainer
	Context    *Scope
}

func (sb *StatementBlock) Check(ctx *CheckContext) {
	i := 0
	newStatements := []StatementContainer{}

	for i < len(sb.Statements) {
		ctx.StartStatementCollectionBlock()

		sc := sb.Statements[i]
		sc.Check(ctx)

		if !ctx.Errors.Clean() {
			return
		}

		stmts := ctx.StopStatementCollectionBlock()

		if stmts != nil {
			for _, st := range stmts {
				newStatements = append(newStatements, StatementContainer{
					Statement: st,
					Context:   sb.Context,
				})
			}
		}

		if !ctx.ShouldRemoveStatement() {
			newStatements = append(newStatements, sc)
		}

		i += 1
	}

	sb.Statements = newStatements
}

type AssignType int

const (
	// let x = 1
	KAssignLet AssignType = iota

	// const x = 1
	KAssignConst

	// x = 1
	KAssignNormal
)

type AssignStatement struct {
	// The identifier we are assigning
	Lhs LValue

	// The rhs we assign it to
	Rhs Expression

	// The type of assign statement
	Type AssignType

	// the type being created
	NewType Type

	// If true we should pin the pointers in the GC
	PinPointers bool
}

var _ Statement = &AssignStatement{}

func (as *AssignStatement) Check(ctx *CheckContext, scope *Scope) {
	// this is simply a declaration
	if as.Rhs != nil {
		as.Rhs.Check(ctx, scope)
	}

	/*
	   In the normal case we can (and should) grab our type from the context
	   In the let case it needs to be set
	*/
	if as.Type == KAssignNormal {
		assignable := as.Lhs.CheckAssignable(ctx, scope)

		if ctx.CurrentPass() == KPassCheckTypes {
			if !assignable {
				ctx.Errors.Errorf("Unable to reassign %v", as.Lhs)
				return
			}
		}
	}
	if !ctx.Errors.Clean() {
		return
	}

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		if as.Rhs != nil {
			rhsType := as.Rhs.Type()
			ctx.RequireType(rhsType, scope)
			as.NewType = rhsType
		}

		switch as.Type {
		case KAssignLet, KAssignConst:
			err := as.Lhs.UpdateScope(scope, as.NewType, as.Type == KAssignLet)
			if err != nil {
				ctx.Errors.Errorf("Unable to update scope: %v", err)
			}

		case KAssignNormal:

		}

	case KPassCheckTypes:
		if as.NewType.Selector == KTypeVoid {
			ctx.Errors.Errorf("Cannot assign to void")
			return
		}

		// The let statement sets the type, the assign needs to match it
		switch as.Type {
		case KAssignNormal:
			lt := as.Lhs.Type()

			if !lt.CanAssignTo(as.NewType) {
				ctx.Errors.Errorf("Cannot assign a variable of type '%v' to type '%v'", lt, as.NewType)
			}
		}
	}
}

type ExpressionStatement struct {
	Expression Expression
}

var _ Statement = &ExpressionStatement{}

func (es *ExpressionStatement) Check(ctx *CheckContext, scope *Scope) {
	es.Expression.Check(ctx, scope)
}

type StructDefinitionStatement struct {
	Id         StructId
	Definition StructDefinition
}

func (sds *StructDefinitionStatement) Check(ctx *CheckContext, scope *Scope) {
	if ctx.CurrentPass() == KPassSetTypes {
		scope.SetStruct(sds.Id, sds.Definition)

		// without this the struct type would never be written (but references to it may be)
		ctx.RequireType(Type{Selector: KTypeStruct, StructId: sds.Id}, scope)
	}
	sds.Definition.Check(ctx)
}

func (sd *StructDefinitionStatement) String() string {
	return "StructDefinitionStatement(TODO)"
}

var _ TopLevelElement = &StructDefinitionStatement{}

type IfStatementSegment struct {
	/*
	   The condition that is evaluated for this segment

	   NB this can be nil in the "else" case
	*/
	Condition Expression
	Block     *StatementBlock
}

type IfStatement struct {
	Segments []IfStatementSegment
}

var _ Statement = &IfStatement{}

func (is *IfStatement) Check(ctx *CheckContext, scope *Scope) {
	for _, seg := range is.Segments {
		if seg.Condition != nil {
			seg.Condition.Check(ctx, scope)
			if !ctx.Errors.Clean() {
				return
			}

			if ctx.CurrentPass() == KPassSetTypes {
				ty := seg.Condition.Type()
				if ty.Selector != KTypeBoolean {
					ctx.Errors.Errorf("IfStatement condition not of boolean type")
					return
				}
			}
		}
		seg.Block.Check(ctx)
		if !ctx.Errors.Clean() {
			return
		}
	}
}

/*
This is a junk statement
It does nothing, but hold source locations
*/
type DummyStatement struct {
	Location errors.SourceLocation
}

var _ Statement = &DummyStatement{}

func (ds *DummyStatement) Check(ctx *CheckContext, scope *Scope) {
	ctx.Errors.SetCurrentLocation(ds.Location)
}

type BreakStatement struct {
}

var _ Statement = &BreakStatement{}

func (bs *BreakStatement) Check(ctx *CheckContext, scope *Scope) {

}

type WhileStatement struct {
	Condition Expression
	Block     *StatementBlock
}

var _ Statement = &WhileStatement{}

func (ws *WhileStatement) Check(ctx *CheckContext, scope *Scope) {
	ws.Condition.Check(ctx, scope)
	ws.Block.Check(ctx)
}

type ReturnStatement struct {
	// If nil, this is a void return
	ReturnedValue Expression
}

var _ Statement = &ReturnStatement{}

func (rs *ReturnStatement) Check(ctx *CheckContext, scope *Scope) {
	if rs.ReturnedValue != nil {
		rs.ReturnedValue.Check(ctx, scope)
	}

	functionReturnType, inFunction := ctx.CurrentReturnType()
	if !inFunction {
		ctx.Errors.Errorf("Trying to return when not in a function")
	}

	if rs.ReturnedValue == nil {
		if functionReturnType.Selector != KTypeVoid {
			ctx.Errors.Errorf("Mismatched return types cannot return void in a function returning '%v'", functionReturnType.String())
		}
	} else {
		returnedType := rs.ReturnedValue.Type()

		switch ctx.CurrentPass() {
		case KPassSetTypes:
			if !returnedType.CanAssignTo(functionReturnType) {
				ctx.Errors.Errorf("Mismatched return types '%v' != '%v'", returnedType.String(), functionReturnType.String())
			}

		case KPassMutate:
			if functionReturnType.Selector == KTypeTuple && !returnedType.Equal(functionReturnType) {
				returnedTuple, ok := rs.ReturnedValue.(*TupleExpression)
				if !ok {
					panic("Currently this only supports re-casting direct tuple expressions - TODO fix this")
				}

				// We need to repack the tuple, we can use conversion operators for this
				for rti, _ := range returnedTuple.Expressions {
					e := returnedTuple.Expressions[rti]
					ce := &CastExpression{
						Casted:  e,
						NewType: functionReturnType.Types[rti],
						CheckCastable: false,
					}
					returnedTuple.Expressions[rti] = ce
				}
			}
		}
	}
}

type SendPipeStatement struct {
	Pipe  Expression
	Value Expression
}

var _ Statement = &SendPipeStatement{}

func (sps *SendPipeStatement) Check(ctx *CheckContext, scope *Scope) {
	ctx.NoteCpuRequired("send pipe")

	sps.Pipe.Check(ctx, scope)
	sps.Value.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	if sps.Pipe.Type().Selector != KTypeWorker {
		ctx.Errors.Errorf("Trying to send to non-worker type: %v", sps.Pipe.Type().String())
		return
	}

	if ctx.CurrentPass() == KPassSetTypes {
		pipeSendType := sps.Pipe.Type().Types[0]
		sentValueType := sps.Value.Type()

		// CONTINUE we need to unwrap th epointer to vector to i64 or whatever
		if sentValueType.Selector == KTypePointer {
			sentValueType = sentValueType.Types[0]
			if sentValueType.Selector == KTypeVector {
				sentValueType = sentValueType.Types[0]
			} else {
				ctx.Errors.Errorf("Sent value type %v was not a vector", sentValueType.String())
				return
			}
		} else {
			ctx.Errors.Errorf("Sent value type %v was not a pointer", sentValueType.String())
			return
		}

		if !sentValueType.CanAssignTo(pipeSendType) {
			ctx.Errors.Errorf("Sent value type %v does not match the send type of the pipe: %v", sentValueType.String(), pipeSendType.String())
			return
		}
	}
}

type ForType int

const (
	// foreach (regular)
	KForEach ForType = iota

	// for iterating over a range
	KForRange
)

type ForeachStatement struct {
	// The name of the temporary variable
	TemporaryVariableName string

	// That which we are iterating over
	Iterable Expression

	IteratedType Type

	Body *StatementBlock

	Variant ForType

	// set in the nonrange case
	StartName, EndName, StepName string
}

var _ Statement = &ForeachStatement{}

func (fs *ForeachStatement) Check(ctx *CheckContext, scope *Scope) {
	switch fs.Variant {
	case KForEach:
		switch ctx.CurrentPass() {
		case KPassSetTypes:
			fs.Iterable.Check(ctx, scope)
			if !ctx.Errors.Clean() {
				return
			}

			fst := fs.Iterable.Type().Unwrapped()
			if fst.Selector != KTypeVector {
				ctx.Errors.Errorf("Attempting to iterate over something that is not a vector: %v", fst)
			}
			fs.IteratedType = fst.Types[0]

			// Set the iterated type on the body
			fs.Body.Context.SetVariable(fs.TemporaryVariableName, fs.IteratedType, true)

		case KPassMutate:
			fs.Iterable.Check(ctx, scope)

			rexp, isRange := fs.Iterable.(*RangeExpression)
			if isRange {
				fs.EndName = ctx.GetTemporaryName()
				ctx.InsertStatementBefore(&AssignStatement{
					Lhs: &IdentifierLValue{
						Name: fs.EndName,
					},
					PinPointers: true,
					NewType:     rexp.Count.Type(),
					Rhs:         rexp.Count,
					Type:        KAssignLet,
				})

				fs.StartName = ctx.GetTemporaryName()
				ctx.InsertStatementBefore(&AssignStatement{
					Lhs: &IdentifierLValue{
						Name: fs.StartName,
					},
					PinPointers: true,
					NewType:     rexp.Start.Type(),
					Rhs:         rexp.Start,
					Type:        KAssignLet,
				})

				fs.StepName = ctx.GetTemporaryName()
				ctx.InsertStatementBefore(&AssignStatement{
					Lhs: &IdentifierLValue{
						Name: fs.StepName,
					},
					PinPointers: true,
					NewType:     rexp.Step.Type(),
					Rhs:         rexp.Step,
					Type:        KAssignLet,
				})
				fs.Variant = KForRange
			}
		}

	case KForRange:

	}

	fs.Body.Check(ctx)
}

type ModifyOperator int

const (
	KModifyPlus ModifyOperator = iota
	KModifyMinus
	KModifyTimes
	KModifyDivide
)

type ModifyInPlaceStatement struct {
	Operator   ModifyOperator
	Modified   LValue
	Expression Expression
}

var _ Statement = &ModifyInPlaceStatement{}

func (ms *ModifyInPlaceStatement) Check(ctx *CheckContext, scope *Scope) {
	assignable := ms.Modified.CheckAssignable(ctx, scope)
	if ctx.CurrentPass() == KPassCheckTypes {
		if !assignable {
			ctx.Errors.Errorf("Unable to reassign %v", ms.Modified)
		}
	}
	ms.Expression.Check(ctx, scope)
}

type ClosureArgDeclarationStatement struct {
	Name string

	// the various args to be written in
	Args []string

	// If true it'll take the address of the vars passed in
	AddressOf bool
}

var _ Statement = &ClosureArgDeclarationStatement{}

func (ms *ClosureArgDeclarationStatement) Check(ctx *CheckContext, scope *Scope) {
	// do nothing, we don't really care
}
