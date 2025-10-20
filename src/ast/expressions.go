package ast

import (
	"bytes"
	"fmt"
)

type Expression interface {
	Type() Type
	String() string
	Check(*CheckContext, *Scope)
}

type NullLiteral struct {
}

var _ Expression = &NullLiteral{}

func (st NullLiteral) Type() Type {
	return Type{Selector: KTypeNull}
}

func (st NullLiteral) String() string {
	return fmt.Sprintf("NullLiteral()")
}

func (st *NullLiteral) Check(ctx *CheckContext, scope *Scope) {

}

type CastExpression struct {
	NewType Type
	Casted  Expression
	CheckCastable bool
}

var _ Expression = &CastExpression{}

func (ce CastExpression) Type() Type {
	return ce.NewType
}

func (ce CastExpression) String() string {
	return fmt.Sprintf("CastExpression(%v, %v)", ce.Casted, ce.NewType)
}

func (ce *CastExpression) Check(ctx *CheckContext, scope *Scope) {
	ce.Casted.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		ctx.RequireType(ce.NewType, scope)

	case KPassCheckTypes:
		if ce.CheckCastable && !ce.Casted.Type().CanAssignTo(ce.NewType) {
			ctx.Errors.Errorf("cannot cast %v to %v", ce.Casted.Type().String(), ce.NewType.String())
			return
		}
	}
}

type SelfTerminal struct {
	cachedType Type
}

var _ Expression = &SelfTerminal{}

func (st SelfTerminal) Type() Type {
	return st.cachedType
}

func (st SelfTerminal) String() string {
	return fmt.Sprintf("SelfTerminal()")
}

func (st *SelfTerminal) Check(ctx *CheckContext, scope *Scope) {
	if ctx.CurrentPass() == KPassSetTypes {
		var ok bool
		st.cachedType, _, ok = scope.LookupVariableType("__self__")
		if !ok {
			ctx.Errors.Errorf("SelfTerminal: Could not find a type for self")
		}
		ctx.RequireType(st.cachedType, scope)
	}
}

type BooleanTerminal struct {
	Value bool
}

var _ Expression = &BooleanTerminal{}

func (bt BooleanTerminal) Type() Type {
	return Type{Selector: KTypeBoolean}
}

func (bt BooleanTerminal) String() string {
	return fmt.Sprintf("BooleanTerminal(%v)", bt.Value)
}

func (bt *BooleanTerminal) Check(ctx *CheckContext, scope *Scope) {
	ctx.RequireType(bt.Type(), scope)
}

type CharacterTerminal struct {
	CodePoint int64
}

var _ Expression = &CharacterTerminal{}

func (bt CharacterTerminal) Type() Type {
	return Type{Selector: KTypeCharacter}
}

func (bt CharacterTerminal) String() string {
	return fmt.Sprintf("CharacterTerminal(%v)", bt.CodePoint)
}

func (bt *CharacterTerminal) Check(ctx *CheckContext, scope *Scope) {
	ctx.RequireType(bt.Type(), scope)
}

type StringTerminal struct {
	// The raw value of the terminal
	Value string

	// The id of this string in the preallocated pool
	Id int
}

var _ Expression = &StringTerminal{}

func (st StringTerminal) Type() Type {
	return Type{Selector: KTypeString}
}

func (st StringTerminal) String() string {
	return fmt.Sprintf("StringTerminal(%v)", st.Value)
}

func (st *StringTerminal) Check(ctx *CheckContext, scope *Scope) {
	if ctx.CurrentPass() == KPassCheckTypes {
		st.Id = ctx.GetStringId(st.Value)
	}
	ctx.RequireType(st.Type(), scope)
}

type GpuBuiltinTerminal struct {
	Name string
}
var _ Expression = &GpuBuiltinTerminal{}

func (gt GpuBuiltinTerminal) String() string {
	return fmt.Sprintf("GpuBuiltinTerminal(%v)", gt.Name)
}

func (gt GpuBuiltinTerminal) calculateType() (Type, bool) {
	rty := Type {
		Selector: KTypeFunction,
		Location: KLocationGpu,
	}

	switch (gt.Name) {
	case "sqrt":
		rty.Types = []Type { Type { Selector: KTypeFloat, Width: 32 } }
		rty.Return = &Type { Selector: KTypeFloat, Width: 32 }
		return rty, true

	default:
		return rty, false
	}
}

func (gt GpuBuiltinTerminal) Type() Type {
	ty, _ := gt.calculateType()
	return ty
}

func (gt GpuBuiltinTerminal) Check(ctx *CheckContext, scope *Scope) {
	_, fnd := gt.calculateType()
	if !fnd {
		ctx.Errors.Errorf("No such gpu builtin %v", gt.Name)
	}
	ctx.NoteGpuRequired("gpu builtin")
}

// A literal identifier
type IdentifierTerminal struct {
	// The symbol of this identifier
	Name string

	// Set true when it shouldn't be namespaced on output
	DontNamespace bool

	// in the case that this is a function call this will be set with all information needed to call it
	Fid *FunctionId

	CachedType     Type
	TypeSetInParse bool
}

var _ Expression = &IdentifierTerminal{}

func (it IdentifierTerminal) Type() Type {
	return it.CachedType
}
func (it IdentifierTerminal) String() string {
	return fmt.Sprintf("IdentifierTerminal(%v)", it.Name)
}
func (it *IdentifierTerminal) Check(ctx *CheckContext, scope *Scope) {
	if it.TypeSetInParse {
		return
	}

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		var ok bool
		it.CachedType, _, ok = scope.LookupVariableType(it.Name)
		if !ok {
			ctx.Errors.Errorf("Failed to find variable type %v", it.Name)
		}
		ctx.RequireType(it.CachedType, scope)

	case KPassMutate:
		if it.Type().Selector == KTypeFunction {
			if it.Type().Builtin {
				it.DontNamespace = true
			} else {
				// we can't do this in pass 1 in case the fn is below us
				def, ok := ctx.CurrentModule().LookupFunction(it.Name)
				if !ok {
					ctx.Errors.Errorf("Failed to find function %v in current module", it.Name)
					return
				}
				fid := def.Id
				it.Fid = &fid
			}
		}
	}
}

type StructLiteralPair struct {
	FieldName string
	Value     Expression
}

type StructLiteralExpression struct {
	Id    StructId
	Pairs []StructLiteralPair
}

var _ Expression = &StructLiteralExpression{}

func (sle StructLiteralExpression) Type() Type {
	return Type{
		Selector: KTypeStruct,
		StructId: sle.Id,
	}
}
func (sle StructLiteralExpression) String() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf, "StructLiteralExpression(%v", sle.Id)
	for _, pr := range sle.Pairs {
		fmt.Fprintf(buf, ", %v = %v", pr.FieldName, pr.Value.String())
	}
	fmt.Fprint(buf, ")")
	return buf.String()
}

func (sle *StructLiteralExpression) Check(ctx *CheckContext, scope *Scope) {
	for _, pr := range sle.Pairs {
		pr.Value.Check(ctx, scope)
		if !ctx.Errors.Clean() {
			return
		}
	}

	if ctx.CurrentPass() == KPassMutate {
		existingKeys := map[string]bool{}
		for _, pr := range sle.Pairs {
			existingKeys[pr.FieldName] = true
		}

		sd, fnd := scope.LookupStructDefinition(sle.Type().StructId)
		if fnd {
			for _, field := range sd.Fields {
				if _, ok := existingKeys[field.Name]; !ok {
					dv, ok := field.Type.DefaultValueExpression(scope)
					if !ok {
						ctx.Errors.Errorf("No default value for '%v' on type %v", field.Name, sle.Type())
						return
					}

					sle.Pairs = append(sle.Pairs, StructLiteralPair{
						FieldName: field.Name,
						Value:     dv,
					})
				}
			}
		}
	}

	ctx.RequireType(sle.Type(), scope)
}

type TupleExpression struct {
	Expressions []Expression
}

var _ Expression = &TupleExpression{}

func (te TupleExpression) Type() Type {
	ty := Type{
		Selector: KTypeTuple,
		Types:    []Type{},
	}

	for _, e := range te.Expressions {
		ty.Types = append(ty.Types, e.Type())
	}

	return ty
}

func (te TupleExpression) String() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprint(buf, "TupleExpression(")
	for ei, e := range te.Expressions {
		if ei > 0 {
			fmt.Fprintf(buf, ", ")
		}
		fmt.Fprint(buf, e.String())
	}
	fmt.Fprint(buf, ")")
	return buf.String()
}

func (te *TupleExpression) Check(ctx *CheckContext, scope *Scope) {
	for _, e := range te.Expressions {
		e.Check(ctx, scope)

		if !ctx.Errors.Clean() {
			return
		}
	}

	ctx.RequireType(te.Type(), scope)
}

type AccessExpression struct {
	Accessed   Expression
	Identifier string

	// when true this is a raw C access expression, just let it through with no namespacing
	AllowRaw bool

	cachedType Type
}

var _ Expression = &AccessExpression{}

func (ae AccessExpression) Type() Type {
	return ae.cachedType
}

func (ae AccessExpression) String() string {
	return fmt.Sprintf("AccessExpression(%v, %v)", ae.Accessed, ae.Identifier)
}

func (ae *AccessExpression) Check(ctx *CheckContext, scope *Scope) {
	ae.Accessed.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	ty := ae.Accessed.Type().Unwrapped()
	switch ctx.CurrentPass() {
	case KPassSetTypes:
		logNotFound := func() {
			ctx.Errors.Errorf("Do not recognise field '%v' on type %v", ae.Identifier, ae.Accessed.Type())
		}

		switch ty.Selector {
		case KTypeStruct:
			sd, fnd := scope.LookupStructDefinition(ty.StructId)
			if !fnd {
				ctx.Errors.Errorf("Could not find struct named %v", ty.StructId)
				return
			}

			field, ok := sd.GetField(ae.Identifier)
			if !ok {
				logNotFound()
				return
			}

			ae.cachedType = field.Type
			ctx.RequireType(ae.cachedType, scope)

		case KTypeString:
			switch ae.Identifier {
			case "resize":
				ae.cachedType = Type{Selector: KTypeFunction, Return: &Type{Selector: KTypeVoid}, Location: KLocationCpu}

			case "length":
				ae.cachedType = Type{Selector: KTypeFunction, Return: &Type{Selector: KTypeInteger}, Location: KLocationAnywhere}

			default:
				logNotFound()
				return
			}

		case KTypeVector:
			switch ae.Identifier {
			case "append", "resize", "erase":
				ae.cachedType = Type{Selector: KTypeFunction, Return: &Type{Selector: KTypeVoid}, Location: KLocationCpu}

			case "length":
				ae.cachedType = Type{Selector: KTypeFunction, Return: &Type{Selector: KTypeInteger}, Location: KLocationCpu}

			default:
				logNotFound()
				return
			}

		default:
			ctx.Errors.Errorf("Tried to take a field value of a non-struct type in access expression: " + ty.String())
			return
		}

	case KPassMutate:
	}
}

type BinaryOperator int

const (
	KOperatorAdd BinaryOperator = iota
	KOperatorSubtract
	KOperatorMultiply
	KOperatorDivide

	KOperatorEquality
	KOperatorInequality

	KOperatorLT
	KOperatorLTE
	KOperatorGT
	KOperatorGTE

	KOperatorAnd
	KOperatorOr

	KOperatorMod
)

type BinaryExpression struct {
	Operator   BinaryOperator
	Lhs, Rhs   Expression
	cachedType Type
}

func (be BinaryExpression) String() string {
	return fmt.Sprintf("BinaryExpression(%v, %v, %v)", be.Lhs.String(), be.Operator, be.Rhs.String())
}

var _ Expression = &BinaryExpression{}

func (be BinaryExpression) Type() Type {
	return be.cachedType
}

func arithmeticTypeCombine(lhs, rhs Type) Type {
	if lhs.Selector == KTypeFloat && rhs.Selector == KTypeFloat {
		if lhs.Width > rhs.Width {
			return lhs
		} else {
			return rhs
		}
	} else {
		return lhs
	}
}

func (be *BinaryExpression) Check(ctx *CheckContext, scope *Scope) {
	be.Lhs.Check(ctx, scope)
	be.Rhs.Check(ctx, scope)

	if !ctx.Errors.Clean() {
		return
	}

	if ctx.CurrentPass() == KPassSetTypes {
		lt := be.Lhs.Type()
		rt := be.Rhs.Type()

		emsg := fmt.Sprintf("Mismatched types in binary operator '%v' vs '%v'", lt, rt)

		switch be.Operator {
		case KOperatorAdd, KOperatorSubtract, KOperatorMultiply, KOperatorDivide:
			if !lt.NumericallyCompatible(rt) {
				ctx.Errors.Errorf(emsg)
				return
			}
			be.cachedType = arithmeticTypeCombine(lt, rt)

		case KOperatorMod:
			if lt.Selector != KTypeInteger {
				ctx.Errors.Errorf("Left hand side of '%%' must be integer")
				return
			}
			if rt.Selector != KTypeInteger {
				ctx.Errors.Errorf("Right hand side of '%%' must be integer")
				return
			}
			be.cachedType = lt

		case KOperatorEquality, KOperatorInequality:
			if rt.Selector == KTypeNull && lt.Selector == KTypePointer {
				// always ok to compare pointer and null
			} else if lt.Selector == KTypeNull && rt.Selector == KTypePointer {
				// always ok to compare pointer and null
			} else if !lt.Equal(rt) {
				ctx.Errors.Errorf(emsg)
				return
			}
			be.cachedType = Type{Selector: KTypeBoolean}

		case KOperatorLT, KOperatorLTE, KOperatorGT, KOperatorGTE, KOperatorAnd, KOperatorOr:
			if !lt.Equal(rt) {
				ctx.Errors.Errorf(emsg)
				return
			}
			be.cachedType = Type{Selector: KTypeBoolean}

		default:
			be.cachedType = Type{Selector: KTypeVoid}
		}
		ctx.RequireType(be.cachedType, scope)
	}
}

type UnaryOperator int

const (
	KOperatorNot UnaryOperator = iota
	KOperatorAddressOf
	KOperatorNegate
)

type UnaryExpression struct {
	Operator   UnaryOperator
	Rhs        Expression
	cachedType Type
}

var _ Expression = &UnaryExpression{}

func (ue UnaryExpression) String() string {
	return fmt.Sprintf("UnaryExpression(%v, %v)", ue.Operator, ue.Rhs.String())
}
func (ue UnaryExpression) Type() Type {
	return ue.cachedType
}

func (ue *UnaryExpression) Check(ctx *CheckContext, scope *Scope) {
	ue.Rhs.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	if ctx.CurrentPass() == KPassSetTypes {
		ty := ue.Rhs.Type()
		switch ue.Operator {
		case KOperatorNot:
			if ty.Selector != KTypeBoolean {
				ctx.Errors.Errorf("Not operator cannot be applied to non-boolean type")
				return
			}
			ue.cachedType = Type{Selector: KTypeBoolean}

		case KOperatorAddressOf:
			ue.cachedType = Type{Selector: KTypePointer, Types: []Type{ty}}

		case KOperatorNegate:
			if !ty.IsNumeric() {
				ctx.Errors.Errorf("Negation operator cannot be applied to non-numeric type")
				return
			}
			ue.cachedType = ty

		default:
			panic("UnaryExpression.Type exhausted cases")
		}

		ctx.RequireType(ue.cachedType, scope)
	}
}

// access a vector
type IndexExpression struct {
	Indexed      Expression
	Index        Expression
	cachedType   Type
	AccessedType TypeSelector
}

var _ Expression = &IndexExpression{}

func (ie IndexExpression) String() string {
	return fmt.Sprintf("AccessExpression(%v, %v)", ie.Indexed.String(), ie.Index.String())
}

func (ie IndexExpression) Type() Type {
	return ie.cachedType
}

func (ae *IndexExpression) Check(ctx *CheckContext, scope *Scope) {
	ae.Indexed.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	ae.Index.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	if ctx.CurrentPass() == KPassSetTypes {
		at := ae.Indexed.Type().Unwrapped()
		switch at.Selector {
		case KTypeVector:
			ae.cachedType = at.Types[0]
			ae.AccessedType = KTypeVector

		case KTypeString:
			ae.cachedType.Selector = KTypeCharacter
			ae.AccessedType = KTypeString

		default:
			ctx.Errors.Errorf("Attempting to access a non-vector type %v", at)
			return
		}

		it := ae.Index.Type()
		if it.Selector != KTypeInteger {
			ctx.Errors.Errorf("Attempting to access a vector using non-integer %v", it)
			return
		}

		ctx.RequireType(ae.cachedType, scope)
	}
}

type IntegerTerminal struct {
	Value int64
}

var _ Expression = &IntegerTerminal{}

func (it IntegerTerminal) Type() Type {
	return Type{Selector: KTypeInteger}
}
func (it IntegerTerminal) String() string {
	return fmt.Sprintf("IntegerTerminal(%v)", it.Value)
}

func (it *IntegerTerminal) Check(ctx *CheckContext, scope *Scope) {
	ctx.RequireType(it.Type(), scope)
}

type FloatTerminal struct {
	LValue, Zeros, RValue int64
	Width                 int
}

var _ Expression = &FloatTerminal{}

func (ft *FloatTerminal) Check(ctx *CheckContext, scope *Scope) {
	ctx.RequireType(ft.Type(), scope)
}

func (it *FloatTerminal) Type() Type {
	return Type{Selector: KTypeFloat, Width: it.Width}
}

func (ft *FloatTerminal) String() string {
	return fmt.Sprintf("FloatTerminal(%v, %v)", ft.LValue, ft.RValue)
}

type CallExpression struct {
	// The expression being called
	CalledExpression Expression

	// The parameters to pass to the call expression
	Arguments []Expression

	// Set to true to ignore type checking phase
	IgnoreTypeChecks bool

	// When this != "", it will be used in place of this
	StackedResultVariableName string

	// if true skip the exe context on this call
	SkipExecutionContext bool

	cachedType Type
}

var _ Expression = &CallExpression{}

func (ce *CallExpression) Type() Type {
	return ce.cachedType
}

func (ce *CallExpression) String() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprint(buf, "CallExpression(")
	fmt.Fprint(buf, ce.CalledExpression.String())
	for _, e := range ce.Arguments {
		fmt.Fprintf(buf, ", ")
		fmt.Fprint(buf, e.String())
	}
	fmt.Fprint(buf, ")")
	return buf.String()
}

func (ce *CallExpression) IsPrintLn() (bool, bool) {
	if it, ok := ce.CalledExpression.(*IdentifierTerminal); ok {
		if it.Name == "print_ln" {
			return true, true
		} else if it.Name == "print" {
			return true, false
		}
	}
	return false, false
}

func (ce *CallExpression) Check(ctx *CheckContext, scope *Scope) {
	switch ctx.CurrentPass() {
	case KPassSetTypes:
		if ok, _ := ce.IsPrintLn(); ok {
			ce.IgnoreTypeChecks = true

			it, _ := ce.CalledExpression.(*IdentifierTerminal)
			// this is a bit redundant
			it.CachedType = voidFunction()
			it.CachedType.Location = KLocationAnywhere
			ctx.RequireType(it.CachedType, scope)
		} else {
			ce.CalledExpression.Check(ctx, scope)
			if !ctx.Errors.Clean() {
				return
			}

			if ae, calledIsAccess := ce.CalledExpression.(*AccessExpression); calledIsAccess {
				ty := ae.Accessed.Type().Unwrapped()
				if ty.Selector == KTypeVector {
					ctx.RequireVector(ty.Types[0], scope)
				}
			}
		}

	case KPassMutate:
		if _, isGpuBuiltin := ce.CalledExpression.(*GpuBuiltinTerminal); isGpuBuiltin {
			ce.SkipExecutionContext = true
		} else if ae, calledIsAccess := ce.CalledExpression.(*AccessExpression); calledIsAccess {
			ae.Check(ctx, scope)
			if !ctx.Errors.Clean() {
				return
			}

			at := ae.Accessed.Type().Unwrapped()

			switch at.Selector {
			case KTypeStruct:
				/*
				   This is the case we are calling an accessed type
				     e.g. a_rectangle.area()
				   We convert this to a fully qualified (struct included) at this point
				*/
				at := ae.Accessed.Type()

				alreadyPointer := false
				if at.Selector == KTypePointer {
					at = at.Types[0]
					alreadyPointer = true
				}

				fid := FunctionId{
					Struct: at.StructId,
					// this is wrong, we should look up the struct module
					Module: ctx.CurrentModule().Id,
					Name:   ae.Identifier,
				}

				var aexp Expression = ae.Accessed
				if !alreadyPointer {
					aexp = &UnaryExpression{
						Operator: KOperatorAddressOf,
						Rhs:      aexp,
					}
				}

				ce.Arguments = append([]Expression{aexp}, ce.Arguments...)
				calledType := Type{
					Selector: KTypeFunction,
					Types:    []Type{},
					Return:   ae.Type().Return,
				}
				for _, arg := range ce.Arguments {
					calledType.Types = append(calledType.Types, arg.Type())
				}
				ce.CalledExpression = &IdentifierTerminal{
					Name:       "junk",
					Fid:        &fid,
					CachedType: calledType,
				}

			case KTypeString:
				switch ae.Identifier {
				case "length":
					ce.IgnoreTypeChecks = true
					ce.CalledExpression = &IdentifierTerminal{
						Name:          "ey_runtime_string_character_length",
						DontNamespace: true,
						CachedType:    functionReturning(Type{Selector: KTypeInteger}),
					}
					ce.Arguments = append([]Expression{ae.Accessed}, ce.Arguments...)

				case "resize":
					ce.IgnoreTypeChecks = true
					ce.CalledExpression = &IdentifierTerminal{
						Name:          "ey_runtime_string_resize",
						DontNamespace: true,
						CachedType:    voidFunction(),
					}
					if len(ce.Arguments) != 1 {
						ctx.Errors.Errorf("Currently string resize only supports a single argument")
						return
					}
					if ce.Arguments[0].Type().Selector != KTypeInteger {
						ctx.Errors.Errorf("String.resize() takes a single integer argument")
						return
					}
					ce.Arguments = append([]Expression{ae.Accessed, ce.Arguments[0]})
				}

			case KTypeVector:
				switch ae.Identifier {
				case "erase":
					ce.IgnoreTypeChecks = true
					ce.CalledExpression = &IdentifierTerminal{
						Name:          "ey_vector_erase",
						DontNamespace: true,
						CachedType:    voidFunction(),
					}
					if len(ce.Arguments) < 1 || len(ce.Arguments) > 2 {
						ctx.Errors.Errorf("Vector erase takes one or two arguments")
						return
					}
					if ce.Arguments[0].Type().Selector != KTypeInteger {
						ctx.Errors.Errorf("Vector.erase's first argument should be an integer")
						return
					}

					// NB we need to fill in the compiler pass
					// This is a symptom of the hacky implementation of these methods
					position := ce.Arguments[0]
					position.Check(ctx, scope)
					if !ctx.Errors.Clean() {
						return
					}

					if len(ce.Arguments) > 1 {
						if ce.Arguments[1].Type().Selector != KTypeInteger {
							ctx.Errors.Errorf("Vector.erase's second argument should be an integer")
							return
						}

						deleteLength := ce.Arguments[1]
						deleteLength.Check(ctx, scope)
						if !ctx.Errors.Clean() {
							return
						}

						ce.Arguments = []Expression{
							ae.Accessed,
							position,
							deleteLength,
						}
					} else {
						ce.Arguments = []Expression{
							ae.Accessed,
							position,
							&IntegerTerminal{
								Value: 1,
							},
						}
					}

				case "append":
					vt := MakeVoid()
					it := at.Types[0]
					newCalled := &IdentifierTerminal{
						Name:          it.VectorAddName(),
						DontNamespace: true,
						CachedType:    	Type{
							Selector: KTypeFunction,
							Types: []Type { ae.Accessed.Type(), it },
							Return:   &vt,
							Builtin: true,
						},
					}
					newCalled.Check(ctx, scope)
					if !ctx.Errors.Clean() {
						return
					}
					ce.Arguments = append([]Expression { ae.Accessed }, ce.Arguments...)
					ce.CalledExpression = newCalled

				case "resize":
					ce.IgnoreTypeChecks = true
					ce.CalledExpression = &IdentifierTerminal{
						Name:          "ey_vector_resize",
						DontNamespace: true,
						CachedType:    voidFunction(),
					}
					if len(ce.Arguments) != 1 {
						ctx.Errors.Errorf("Currently vector append only supports a single argument")
						return
					}
					if ce.Arguments[0].Type().Selector != KTypeInteger {
						ctx.Errors.Errorf("Vector.resize() takes a single integer argument")
						return
					}

					newSize := ce.Arguments[0]
					newSize.Check(ctx, scope)
					if !ctx.Errors.Clean() {
						return
					}

					ce.Arguments = append([]Expression{ae.Accessed, newSize})

				case "length":
					ce.IgnoreTypeChecks = true
					ce.CalledExpression = &IdentifierTerminal{
						Name:          "ey_vector_length",
						DontNamespace: true,
						CachedType:    functionReturning(Type{Selector: KTypeInteger}),
					}
					ce.Arguments = append([]Expression{ae.Accessed}, ce.Arguments...)
				}
			}
		} else if ok, withNl := ce.IsPrintLn(); ok {
			/*
				     	First we preserve the values
						This ensures that any side effects from called statements happen first
			*/

			argValues := []Expression{}

			for _, arg := range ce.Arguments {
				arg.Check(ctx, scope)
				if !ctx.Errors.Clean() {
					return
				}

				valName := ctx.GetTemporaryName()
				at := arg.Type()
				ctx.InsertStatementBefore(&AssignStatement{
					Lhs: &IdentifierLValue{
						Name:       valName,
						cachedType: at,
					},
					PinPointers: true,
					NewType:     at,
					Rhs:         arg,
					Type:        KAssignLet,
				})

				argValues = append(argValues, &IdentifierTerminal{
					Name:          valName,
					DontNamespace: true,
				})
			}

			// format string is the first argument to printf
			for argi, arg := range ce.Arguments {
				name := "unknown"

				ty := arg.Type()
				switch ty.Selector {
				case KTypeInteger:
					name = "ey_print_int"

				case KTypeFloat:
					if ty.Width == 32 {
						name = "ey_print_float32"
					} else {
						name = "ey_print_float64"
					}

				case KTypeString:
					name = "ey_print_string"

				case KTypeBoolean:
					name = "ey_print_boolean"

				case KTypeCharacter:
					name = "ey_print_character"

				default:
					ctx.Errors.Errorf("print_ln can't handle type '%v' (yet)", ty)
				}

				called := &CallExpression{
					IgnoreTypeChecks: true,
					CalledExpression: &IdentifierTerminal{
						Name:          name,
						DontNamespace: true,
					},
					Arguments: []Expression{
						argValues[argi],
					},
					cachedType: Type{Selector: KTypeVoid},
				}
				/*
						NB this will only do the mutate pass
					    That should be fine assuming all the operations are in the right place
				*/
				ctx.InsertStatementBefore(&ExpressionStatement{
					Expression: called,
				})
			}

			/*
			   All that remains is the newline, so gut this expression and handle that
			*/
			ce.IgnoreTypeChecks = true
			if withNl {
				ce.CalledExpression = &IdentifierTerminal{
					Name:          "ey_print_nl",
					DontNamespace: true,
					CachedType:    voidFunction(),
				}
			} else {
				// not efficient ofc
				ce.CalledExpression = &IdentifierTerminal{
					Name:          "ey_noop",
					DontNamespace: true,
					CachedType:    voidFunction(),
				}
			}
			ce.Arguments = []Expression{}
		} else {
			ce.CalledExpression.Check(ctx, scope)
		}

		if ce.CalledExpression.Type().Selector == KTypeClosure {
			/*
			   First we create a bunch of variables for calling the closure
			*/
			closureArgName := ctx.GetTemporaryName()

			ads := &ClosureArgDeclarationStatement{
				Name:      closureArgName,
				Args:      []string{},
				AddressOf: true,
			}

			for _, arg := range ce.Arguments {
				name := ctx.GetTemporaryName()
				ads.Args = append(ads.Args, name)

				ct := arg.Type()
				// place each arg on the stack
				ctx.InsertStatementBefore(&AssignStatement{
					Lhs: &IdentifierLValue{
						Name:       name,
						cachedType: ct,
					},
					PinPointers: true,
					NewType:     ct,
					Rhs:         arg,
					Type:        KAssignLet,
				})
			}
			ctx.InsertStatementBefore(ads)

			/*
				In the non void case, split out the closure
			*/
			returnType := *ce.CalledExpression.Type().Return
			if returnType.Selector != KTypeVoid {
				ce.StackedResultVariableName = ctx.GetTemporaryName()

				// declare the return
				ctx.InsertStatementBefore(&AssignStatement{
					Lhs: &IdentifierLValue{
						Name:       ce.StackedResultVariableName,
						cachedType: returnType,
					},
					PinPointers: true,
					NewType:     returnType,
					Rhs:         nil,
					Type:        KAssignLet,
				})

				// call with that return
				ctx.InsertStatementBefore(&ExpressionStatement{
					Expression: &CallExpression{
						IgnoreTypeChecks: true,
						CalledExpression: &IdentifierTerminal{
							Name:          "ey_closure_call",
							DontNamespace: true,
						},
						Arguments: []Expression{
							ce.CalledExpression,
							&UnaryExpression{
								Operator: KOperatorAddressOf,
								Rhs: &IdentifierTerminal{
									Name: ce.StackedResultVariableName,
								},
							},
							&IdentifierTerminal{
								Name:          closureArgName,
								DontNamespace: true,
							},
						},
						cachedType: Type{Selector: KTypeVoid},
					},
				})
			}
		}

	case KPassCheckTypes:
		if !ce.IgnoreTypeChecks {
			ty := ce.CalledExpression.Type()
			if !ty.IsCallable() {
				ctx.Errors.Errorf("Expression of type '%v' not callable", ty)
				return
			}

			if len(ty.Types) != len(ce.Arguments) {
				ctx.Errors.Errorf("Wrong number of arguments in call expression, have %v, expecting %v", len(ce.Arguments), len(ty.Types))
				return
			}

			for i, lhsTy := range ty.Types {
				rhsTy := ce.Arguments[i].Type()

				if !lhsTy.CanAssignTo(rhsTy) {
					ctx.Errors.Errorf("Wrong argument type in call expression, have %v, expecting %v", rhsTy, lhsTy)
				}
			}
		}
	}

	if !ctx.Errors.Clean() {
		return
	}

	for _, e := range ce.Arguments {
		e.Check(ctx, scope)
		if !ctx.Errors.Clean() {
			return
		}
	}

	// could this be moved up?
	if ctx.CurrentPass() == KPassSetTypes {
		ty := ce.CalledExpression.Type()
		// this is duped above
		if !ty.IsCallable() {
			ctx.Errors.Errorf("Expression not callable: is of type '%v'", ty)
			return
		}

		if ty.Location == KLocationCpu {
			ctx.NoteCpuRequired("function call")
		}
		ce.cachedType = *ty.Return
	}
}

type NewExpression struct {
	// The initialiser, set during the original creation of this
	Initialiser Expression

	// The replacement for this expression (set during mutate)
	Replacement Expression
}

var _ Expression = &NewExpression{}

func (ce *NewExpression) Type() Type {
	return MakePointer(ce.Initialiser.Type())
}

func (ce *NewExpression) String() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprint(buf, "NewExpression(")
	fmt.Fprint(buf, ce.Initialiser.String())
	fmt.Fprint(buf, ")")
	return buf.String()
}

func (be *NewExpression) Check(ctx *CheckContext, scope *Scope) {
	ctx.NoteCpuRequired("new expression")

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		be.Initialiser.Check(ctx, scope)
		if !ctx.Errors.Clean() {
			return
		}

		ctx.RequireType(be.Initialiser.Type(), scope)

	case KPassMutate:
		// allow the core expression to mutate if it needs to
		be.Initialiser.Check(ctx, scope)

		tempName := ctx.GetTemporaryName()
		ty := be.Initialiser.Type()

		/*
		   Create a malloc expression
		   T * blah = malloc(sizeof(T))
		*/
		ct := Type{
			Selector: KTypePointer,
			Types:    []Type{ty},
		}
		ctx.InsertStatementBefore(&AssignStatement{
			Lhs: &IdentifierLValue{
				Name: tempName,
			},
			PinPointers: false,
			NewType:     ct,
			Rhs: &CallExpression{
				IgnoreTypeChecks: true,
				CalledExpression: &IdentifierTerminal{
					Name:          "ey_runtime_gc_alloc",
					DontNamespace: true,
				},
				SkipExecutionContext: true,
				Arguments: []Expression{
					&CallExpression{
						IgnoreTypeChecks: true,
						CalledExpression: &IdentifierTerminal{
							Name:          "ey_runtime_gc",
							DontNamespace: true,
						},
						Arguments: []Expression{},
					},
					&SizeofExpression{SizedType: ty},
					&IntegerTerminal{Value: 0},
				},
				cachedType: ct,
			},
			Type: KAssignLet,
		})
		scope.SetVariable(tempName, ct, true)

		/*
		   Assign the new variable
		   *blah = initialiser
		*/
		ctx.InsertStatementBefore(&AssignStatement{
			// TODO need a deref here
			Lhs: &DerefLValue{
				Inner: &IdentifierLValue{
					Name: tempName,
				},
			},
			PinPointers: false,
			NewType:     be.Initialiser.Type(),
			Rhs:         be.Initialiser,
			Type:        KAssignNormal,
		})

		/*
		   Update ourself to use the (initialised) temp variable as the value
		*/
		be.Replacement = &IdentifierTerminal{
			Name:          tempName,
			DontNamespace: true,
			CachedType:    be.Initialiser.Type(),
		}
	}
}

type SizeofExpression struct {
	SizedType Type
}

var _ Expression = &SizeofExpression{}

func (se SizeofExpression) Type() Type {
	return Type{Selector: KTypeInteger}
}
func (se SizeofExpression) String() string {
	return fmt.Sprintf("SizeofExpression(%v)", se.SizedType)
}

func (se *SizeofExpression) Check(ctx *CheckContext, scope *Scope) {

}

type DereferenceExpression struct {
	Pointer Expression
}

var _ Expression = &DereferenceExpression{}

func (de *DereferenceExpression) Type() Type {
	return de.Pointer.Type().Types[0]
}
func (de *DereferenceExpression) String() string {
	return fmt.Sprintf("DereferenceExpression(%v)", de.Pointer)
}

func (de *DereferenceExpression) Check(ctx *CheckContext, scope *Scope) {
	de.Pointer.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	if de.Pointer.Type().Selector != KTypePointer {
		ctx.Errors.Errorf("Attempting to deference something that is not a pointer")
	}
}

type VectorLiteralExpression struct {
	ElementType  Type
	Initialisers []Expression

	Replacement Expression
}

var _ Expression = &VectorLiteralExpression{}

func (vl *VectorLiteralExpression) Type() Type {
	return MakePointer(Type{
		Selector: KTypeVector,
		Types:    []Type{vl.ElementType},
	})
}

func (vl VectorLiteralExpression) String() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprint(buf, "VectorLiteralExpression(")
	for i, vi := range vl.Initialisers {
		if i > 0 {
			fmt.Fprint(buf, ", ")
		}
		fmt.Fprint(buf, vi.String())
	}
	fmt.Fprint(buf, ")")
	return buf.String()
}

func (vl *VectorLiteralExpression) Check(ctx *CheckContext, scope *Scope) {
	ctx.NoteCpuRequired("vector literal")

	for _, e := range vl.Initialisers {
		e.Check(ctx, scope)

		if !e.Type().CanAssignTo(vl.ElementType) {
			ctx.Errors.Errorf("Bad type in vector literal. Have %v, expecting %v", e.Type(), vl.ElementType)
			return
		}
	}

	if ctx.CurrentPass() == KPassMutate {
		vectorName := ctx.GetTemporaryName()

		/*
		   Malloc the vector
		   vec * blah = malloc(sizeof(T))
		*/
		ctx.InsertStatementBefore(&AssignStatement{
			Lhs: &IdentifierLValue{
				Name: vectorName,
			},
			PinPointers: false,
			NewType:     vl.Type(),
			Rhs: &CallExpression{
				IgnoreTypeChecks: true,
				CalledExpression: &IdentifierTerminal{
					Name:          "ey_vector_create",
					DontNamespace: true,
				},
				Arguments: []Expression{
					&SizeofExpression{SizedType: vl.ElementType},
				},
				cachedType: vl.Type(),
			},
			Type: KAssignLet,
		})

		for _, e := range vl.Initialisers {
			tempInitialiser := ctx.GetTemporaryName()

			// assign an initialiser statement
			ctx.InsertStatementBefore(&AssignStatement{
				Lhs: &IdentifierLValue{
					Name: tempInitialiser,
				},
				PinPointers: false,
				Rhs:         e,
				// NB we may need C to coerce the type across for us
				NewType: vl.ElementType,
				Type:    KAssignLet,
			})

			// append that initialiser
			ctx.InsertStatementBefore(&ExpressionStatement{
				Expression: &CallExpression{
					IgnoreTypeChecks: true,
					CalledExpression: &IdentifierTerminal{
						Name:          "ey_vector_append",
						DontNamespace: true,
					},
					Arguments: []Expression{
						&IdentifierTerminal{
							Name: vectorName,
						},
						&UnaryExpression{
							Operator: KOperatorAddressOf,
							Rhs: &IdentifierTerminal{
								Name: tempInitialiser,
							},
						},
					},
					cachedType: Type{Selector: KTypeVoid},
				},
			})
		}

		vl.Replacement = &IdentifierTerminal{
			Name:          vectorName,
			DontNamespace: true,
			CachedType:    vl.Type(),
		}
	}
}

type RangeExpression struct {
	Count, Start, Step Expression
}

var _ Expression = &RangeExpression{}

func (re *RangeExpression) Type() Type {
	return MakeVector(Type{Selector: KTypeInteger})
}

func (re *RangeExpression) String() string {
	return fmt.Sprintf("RangeExpression(%v, %v, %v)", re.Start, re.Count, re.Step)
}

func (re *RangeExpression) Check(ctx *CheckContext, scope *Scope) {
	// no real point checking
	re.Count.Check(ctx, scope)
	re.Start.Check(ctx, scope)
	re.Step.Check(ctx, scope)

	// NB do this after mutate phase when it will be gone
	switch ctx.CurrentPass() {
	case KPassSetTypes:
		// needs to happen before this is optimised away
		ctx.AssertType(re.Count, KTypeInteger)
		ctx.AssertType(re.Start, KTypeInteger)
		ctx.AssertType(re.Step, KTypeInteger)

	case KPassCheckTypes:
		ctx.NoteCpuRequired("create range expression")
	}
}
