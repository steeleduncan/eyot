package ast

import (
	"bytes"
	"fmt"
)

// An assignable "l-value"
type LValue interface {
	/*
	  This is called with a type and the parent scope in the let case

	   It allows the LValue to update both its cached type, and the scope
	*/
	UpdateScope(scope *Scope, ty Type, assignable bool) error

	Type() Type

	// Perform the check phase, returning true if it is assignable
	CheckAssignable(ctx *CheckContext, scope *Scope) bool

	String() string
}

// Simple identifier on lhs
type IdentifierLValue struct {
	Name       string
	cachedType Type
}

var _ LValue = &IdentifierLValue{}

func (ilv *IdentifierLValue) String() string {
	return fmt.Sprintf("IdentifierLValue(%v)", ilv.Name)
}

func (ilv *IdentifierLValue) CheckAssignable(ctx *CheckContext, scope *Scope) bool {
	ty, assignable, ok := scope.LookupVariableType(ilv.Name)
	if !ok {
		ctx.Errors.Errorf("IdentifierLValue.Type Could not find type for '%v': %v", ilv.Name, ilv)
		return false
	}
	ilv.cachedType = ty
	return assignable
}

func (ilv *IdentifierLValue) Type() Type {
	return ilv.cachedType
}

/*
LValue representing the object this is called inside
*/
type SelfLValue struct {
	cachedType Type
}

var _ LValue = &SelfLValue{}

func (ilv *SelfLValue) String() string {
	return "SelfLValue()"
}
func (slv *SelfLValue) Type() Type {
	return slv.cachedType
}
func (slv *SelfLValue) CheckAssignable(ctx *CheckContext, scope *Scope) bool {
	ty, _, ok := scope.LookupVariableType("__self__")
	if !ok {
		ctx.Errors.Errorf("SelfLValue.Type Could not find type")
		return false
	}
	slv.cachedType = ty
	return false
}

// dereference, ie *a
type DerefLValue struct {
	// The pointer to dereference
	Inner LValue
}

var _ LValue = &DerefLValue{}

func (dl *DerefLValue) CheckAssignable(ctx *CheckContext, scope *Scope) bool {
	assignable := dl.Inner.CheckAssignable(ctx, scope)

	if dl.Inner.Type().Selector != KTypePointer {
		// this is never user generated, so no need for an error
		ctx.Errors.Errorf("DerefLValue is not dereferencing a pointer")
	}

	return assignable
}

func (dl *DerefLValue) Type() Type {
	// we've checked it is a pointer during the Check() phase
	innerType := dl.Inner.Type()
	return innerType.Types[0]
}

func (dl *DerefLValue) String() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprint(buf, "DerefLValue(")
	fmt.Fprint(buf, dl.Inner.String())
	fmt.Fprint(buf, ")")
	return buf.String()
}

func (dl DerefLValue) UpdateScope(scope *Scope, ty Type, assignable bool) error {
	// because you can't let *a = 5, that doesn't make sense
	return fmt.Errorf("Accessor LValue should not be written to scope")
}

// a.b
type AccessorLValue struct {
	Inner      LValue
	FieldName  string
	cachedType Type
}

var _ LValue = &AccessorLValue{}

func (alv *AccessorLValue) CheckAssignable(ctx *CheckContext, scope *Scope) bool {
	assignable := alv.Inner.CheckAssignable(ctx, scope)
	if !ctx.Errors.Clean() {
		return false
	}

	if _, isSelf := alv.Inner.(*SelfLValue); isSelf {
		assignable = true
	}

	it := alv.Inner.Type().Unwrapped()

	if it.Selector != KTypeStruct {
		// vectors have fields too, but they are not assignable
		ctx.Errors.Errorf("Cannot assign to a field of a non-struct type: %v", it.String())
		return false
	}

	sdef, fnd := scope.LookupStructDefinition(it.StructId)
	if !fnd {
		ctx.Errors.Errorf("Could not find struct of type %v", it.StructId)
		return false
	}

	field, fnd := sdef.GetField(alv.FieldName)
	if !fnd {
		ctx.Errors.Errorf("Could not find field named %v", alv.FieldName)
		return false
	}

	alv.cachedType = field.Type
	return assignable
}
func (alv *AccessorLValue) String() string {
	return fmt.Sprintf("AccessorLValue(%v, %v)", alv.Inner.String(), alv.FieldName)
}
func (alv *AccessorLValue) Type() Type {
	return alv.cachedType
}
func (al AccessorLValue) UpdateScope(scope *Scope, ty Type, assignable bool) error {
	// because you can't let a.x = 5, that doesn't make sense
	return fmt.Errorf("Accessor LValue should not be written to scope")
}

// a[b]
type IndexLValue struct {
	Indexed    LValue
	Index      Expression
	cachedType Type
}

var _ LValue = &IndexLValue{}

func (ilv *IndexLValue) CheckAssignable(ctx *CheckContext, scope *Scope) bool {
	ilv.Index.Check(ctx, scope)
	assignable := ilv.Indexed.CheckAssignable(ctx, scope)
	if !ctx.Errors.Clean() {
		return false
	}

	ity := ilv.Indexed.Type()
	if ity.Selector == KTypePointer {
		ity = ity.Types[0]
	}

	switch ity.Selector {
	case KTypeVector:
		ilv.cachedType = ity.Types[0]

	case KTypeString:
		ilv.cachedType = Type{Selector: KTypeCharacter}

	default:
		ctx.Errors.Errorf("Can only index lvalue vectors (%v, %v)", ity, ilv.Indexed)
	}

	return assignable
}

func (ilv *IndexLValue) String() string {
	return fmt.Sprintf("IndexLValue(%v, %v)", ilv.Indexed.String(), ilv.Index.String())
}

func (ilv *IndexLValue) Type() Type {
	return ilv.cachedType
}

func (al IndexLValue) UpdateScope(scope *Scope, ty Type, assignable bool) error {
	// because you can't let a.x = 5, that doesn't make sense
	return fmt.Errorf("Index LValue should not be written to scope")
}

func (il IdentifierLValue) UpdateScope(scope *Scope, ty Type, assignable bool) error {
	// type is irrelevant, no multiple lets
	if scope.CannotBeDefinedAtThisLevel(il.Name) {
		return fmt.Errorf("'%v' has already been defined in this scope and cannot be redefined", il.Name)
	}

	scope.SetVariable(il.Name, ty, assignable)
	il.cachedType = ty
	return nil
}

func (sl SelfLValue) UpdateScope(scope *Scope, ty Type, assignable bool) error {
	scope.SetVariable("__self__", Type{
		Selector: KTypePointer,
		Types:    []Type{ty},
	}, assignable)
	return nil
}

// a, b, c
type MultipleLValue struct {
	LValues []LValue
}

var _ LValue = &MultipleLValue{}

func (mlv *MultipleLValue) String() string {
	s := "MultipleLValue("
	for i, lv := range mlv.LValues {
		if i > 0 {
			s += ", "
		}
		s += lv.String()
	}
	s += ")"
	return s
}

func (mlv *MultipleLValue) CheckAssignable(ctx *CheckContext, scope *Scope) bool {
	assignable := true

	for _, lv := range mlv.LValues {
		if !lv.CheckAssignable(ctx, scope) {
			assignable = false
		}
	}

	return assignable
}

func (mlv *MultipleLValue) Type() Type {
	ret := Type{
		Selector: KTypeTuple,
		Types:    []Type{},
	}

	for _, lv := range mlv.LValues {
		ty := lv.Type()
		ret.Types = append(ret.Types, ty)
	}

	return ret
}

func (ml MultipleLValue) UpdateScope(scope *Scope, ty Type, assignable bool) error {
	if ty.Selector != KTypeTuple {
		return fmt.Errorf("type assigned to multiple lvalues must b ea tuple")
	}

	if len(ml.LValues) != len(ty.Types) {
		return fmt.Errorf("Wrong number of expressions on LHS of multiple assign")
	}

	for ei, tty := range ty.Types {
		lv := ml.LValues[ei]
		err := lv.UpdateScope(scope, tty, assignable)
		if err != nil {
			return err
		}
	}

	return nil
}
