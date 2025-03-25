package ast

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

type TypeSelector int

const (
	KTypeInteger TypeSelector = iota
	KTypeString
	KTypeBoolean
	KTypeCharacter
	KTypeFloat
	KTypeVoid

	// special type for pointers
	KTypeNull

	// Types[] are the parameters, and return type in Return
	KTypeFunction // "real" function
	KTypeClosure  // a closure object

	// Types[] are the types specified by this
	KTypeTuple
	KTypeStruct

	// Types[0] is what it points to
	KTypePointer

	// Types[0] is what it points to
	KTypeVector

	// Types[0] is the type sent to the channel, Types[1] is the type received
	KTypeWorker
)

type Type struct {
	// Base type
	Selector TypeSelector

	// In the case of tuples, these are the constituent types
	// or for a pointer type, [0] will be the type it points to
	Types []Type

	// The typename (used for structs)
	StructId StructId

	// In the case this is callable, this is the return type
	// TODO swap this to a .Signature instead, but then we need to be sure no lambdas are using .Types
	Return *Type

	// Callables only
	// True means it is a builtin function
	Builtin bool

	// Callables only
	// If this is non-empty, then the function is bound to this struct
	BoundStructName string

	// Callables only
	// This is the function requirements for the function
	Requirement FunctionRequirement

	// Floats only. Generally 32 or 64
	Width int
}

func (ty Type) Signature() FunctionSignature {
	if !ty.IsCallable() {
		panic("Type.Signature() can only be called on callable types")
	}
	if ty.Return == nil {
		panic("Type.Signature() can only be called on callable types with the Return value not nil")
	}

	return FunctionSignature{
		Requirement: ty.Requirement,
		Return:      *ty.Return,
		Types:       ty.Types,
	}
}

func (ty Type) Unwrapped() Type {
	r := ty
	if r.Selector == KTypePointer {
		r = r.Types[0]
	}
	return r
}


func MakePointer(ty Type) Type {
	return Type{
		Selector: KTypePointer,
		Types:    []Type{ty},
	}
}

func MakeVector(ty Type) Type {
	return MakePointer(Type{
		Selector: KTypeVector,
		Types:    []Type{ty},
	})
}

func (ty Type) IsNumeric() bool {
	switch ty.Selector {
	case KTypeInteger, KTypeFloat:
		return true

	default:
		return false
	}
}

func RoughTypeName(ts TypeSelector) string {
	switch ts {
	case KTypeNull:
		return "null"

	case KTypeInteger:
		return "integer"

	case KTypeString:
		return "string"

	case KTypeBoolean:
		return "boolean"

	case KTypeCharacter:
		return "character"

	case KTypeFloat:
		return "float"

	case KTypeVoid:
		return "void"

	case KTypeFunction:
		return "function"
	case KTypeClosure:
		return "closure"
	case KTypeStruct:
		return "struct"
	case KTypeVector:
		return "vector"
	case KTypeWorker:
		return "worker"
	case KTypePointer:
		return "pointer"
	case KTypeTuple:
		return "tuple"
	default:
		panic("writeId(): exhausted cases")
	}
}

func (ty Type) writeId(w io.Writer) {
	switch ty.Selector {
	case KTypeNull:
		fmt.Fprintf(w, "l")

	case KTypeInteger:
		fmt.Fprintf(w, "i")

	case KTypeString:
		fmt.Fprintf(w, "s")

	case KTypeBoolean:
		fmt.Fprintf(w, "b")

	case KTypeCharacter:
		fmt.Fprintf(w, "a")

	case KTypeFloat:
		if ty.Width == 64 {
			fmt.Fprintf(w, "F")
		} else {
			fmt.Fprintf(w, "f")
		}

	case KTypeVoid:
		// not sure how this would happen
		fmt.Fprintf(w, "v")

	case KTypeFunction:
		fmt.Fprintf(w, "n")
		ty.Return.writeId(w)
		fmt.Fprintf(w, "N")

	case KTypeClosure:
		fmt.Fprintf(w, "b")
		ty.Return.writeId(w)
		fmt.Fprintf(w, "B")

	case KTypeStruct:
		fmt.Fprintf(w, "s_")
		fmt.Fprint(w, ty.StructId.Name)
		for i, cpt := range ty.StructId.Module {
			if i > 0 {
				fmt.Fprint(w, "_")
			} else {
				fmt.Fprint(w, "__")
			}
			fmt.Fprint(w, cpt)
		}
		fmt.Fprintf(w, "_S")

	case KTypeVector:
		fmt.Fprintf(w, "v")
		ty.Types[0].writeId(w)
		fmt.Fprintf(w, "V")

	case KTypeWorker:
		fmt.Fprintf(w, "c")
		ty.Types[0].writeId(w)
		fmt.Fprintf(w, "c")
		ty.Types[1].writeId(w)
		fmt.Fprintf(w, "C")

	case KTypePointer:
		fmt.Fprintf(w, "p")
		ty.Types[0].writeId(w)
		fmt.Fprintf(w, "P")

	case KTypeTuple:
		fmt.Fprint(w, "_")
		for _, ty := range ty.Types {
			ty.writeId(w)
		}
		fmt.Fprint(w, "_")

	default:
		panic("writeId(): exhausted cases")
	}
}

func (ty Type) namespacedIdentifier(ns string) string {
	buf := bytes.NewBuffer([]byte{})
	if ns != "" {
		fmt.Fprintf(buf, "ey_"+ns)
	}
	ty.writeId(buf)
	bs := buf.String()
	if strings.HasSuffix(bs, "_") {
		bs = bs[:len(bs)-1]
	}
	return bs
}

func (ty Type) TupleIdentifier() string {
	return ty.namespacedIdentifier("tuple")
}

func (ty Type) RawIdentifier() string {
	return ty.namespacedIdentifier("")
}

// Check that we can assign the given lhs type to the rhs type
// ie lhs -> rhs
func (lhs Type) CanAssignTo(rhs Type) bool {
	if lhs.NumericallyCompatible(rhs) {
		// clearly this is always possible
		return true
	}

	if lhs.Selector == KTypeTuple && rhs.Selector == KTypeTuple {
		if len(lhs.Types) != len(rhs.Types) {
			return false
		}

		for ity, lty := range lhs.Types {
			rty := rhs.Types[ity]
			if !lty.CanAssignTo(rty) {
				return false
			}
		}

		return true
	}

	if lhs.Selector == KTypeInteger && rhs.Selector == KTypeFloat {
		// no reason not to allow this
		return true
	}

	if lhs.Selector == KTypeFloat && rhs.Selector == KTypeInteger {
		// there is good reason not to allow this, but for now I'll leave it ok for convenience
		return true
	}

	if lhs.Selector == KTypePointer && rhs.Selector == KTypeNull {
		// assigning pointers always ok
		return true
	}

	return false
}

// true if this is a callable type
func (ty Type) IsCallable() bool {
	switch ty.Selector {
	case KTypeFunction, KTypeClosure:
		return true
	}
	return false
}

func (lhs Type) NumericallyCompatible(rhs Type) bool {
	if lhs.Equal(rhs) {
		return true
	}

	if lhs.Selector == KTypeFloat && rhs.Selector == KTypeFloat {
		// the backend will figure out the 32/64 differences
		return true
	}

	return false
}

// Check for precise equality between types
func (lhs Type) Equal(rhs Type) bool {
	if lhs.Selector != rhs.Selector {
		return false
	}

	switch lhs.Selector {
	case KTypeFloat:
		return lhs.Width == rhs.Width

	case KTypeInteger, KTypeBoolean, KTypeString, KTypeCharacter, KTypeVoid:
		return true

	case KTypeVector, KTypePointer:
		return rhs.Types[0].Equal(lhs.Types[0])

	case KTypeWorker:
		return rhs.Types[0].Equal(lhs.Types[0]) && rhs.Types[1].Equal(lhs.Types[1])

	// NB should closure check the descriptor too?
	case KTypeFunction, KTypeClosure:
		if rhs.Selector != lhs.Selector {
			return false
		}
		if !lhs.Return.Equal(*rhs.Return) {
			return false
		}
		if lhs.BoundStructName != rhs.BoundStructName {
			return false
		}
		if len(lhs.Types) != len(rhs.Types) {
			return false
		}
		for tyi, ty := range lhs.Types {
			if !ty.Equal(rhs.Types[tyi]) {
				return false
			}
		}

		return true

	case KTypeStruct:
		return rhs.StructId.IsEqual(lhs.StructId)

	case KTypeTuple:
		if rhs.Selector != KTypeTuple {
			return false
		}
		if len(lhs.Types) != len(rhs.Types) {
			return false
		}
		for i, l := range lhs.Types {
			r := rhs.Types[i]
			if !l.Equal(r) {
				return false
			}
		}
		return true
	}

	panic(fmt.Sprintf("Cases exhausted '%v' and '%v'", lhs.String(), rhs.String()))
	return false
}

func (ty Type) EstimateCSize(scope *Scope) int {
	switch ty.Selector {
	case KTypeFloat:
		if ty.Width == 64 {
			return 8
		} else {
			return 4
		}

	case KTypeInteger, KTypeBoolean, KTypeCharacter:
		return 8

	case KTypeVoid:
		return 0

	case KTypeTuple:
		r := 0
		for _, ty := range ty.Types {
			r += ty.EstimateCSize(scope)
		}
		return r

	case KTypeStruct:
		sd, fnd := scope.LookupStructDefinition(ty.StructId)
		if !fnd {
			panic(fmt.Sprintf("Failed to find struct definition for '%v'", ty.StructId))
		}

		r := 0
		for _, field := range sd.Fields {
			r += field.Type.EstimateCSize(scope)
		}
		return r
	}

	panic(fmt.Sprintf("EstimateCSize: exhausted type %v", ty))
	return 0
}

func (ty Type) writeCType(w io.Writer) {
	switch ty.Selector {
	case KTypeInteger:
		fmt.Fprintf(w, "i64")

	case KTypeString:
		fmt.Fprintf(w, "string")

	case KTypeBoolean:
		fmt.Fprintf(w, "boolean")

	case KTypeCharacter:
		fmt.Fprintf(w, "character")

	case KTypeFloat:
		fmt.Fprintf(w, "f%v", ty.Width)

	case KTypeVoid:
		fmt.Fprintf(w, "void")

	case KTypePointer:
		ty.Types[0].writeCType(w)
		fmt.Fprintf(w, "*")

	case KTypeClosure:
		ty.Return.writeCType(w)
		fmt.Fprintf(w, "(closure)")

	case KTypeFunction:
		ty.Return.writeCType(w)
		fmt.Fprintf(w, "(function)")

	case KTypeTuple:
		fmt.Fprint(w, "(")
		for i, ty := range ty.Types {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprint(w, ty.String())
		}
		fmt.Fprint(w, ")")

	case KTypeVector:
		fmt.Fprintf(w, "EyVector")

	case KTypeWorker:
		fmt.Fprintf(w, "EyWorker")

	case KTypeStruct:
		fmt.Fprint(w, ty.StructId.String())

	default:
		panic("cases exhausted for Type.write")
	}
}

func (ty Type) String() string {
	switch ty.Selector {
	case KTypeInteger:
		return "i64"

	case KTypeString:
		return "string"

	case KTypeBoolean:
		return "boolean"

	case KTypeCharacter:
		return "character"

	case KTypeFloat:
		return fmt.Sprintf("f%v", ty.Width)

	case KTypeVoid:
		return "void"

	case KTypePointer:
		return "*" + ty.Types[0].String()

	case KTypeClosure, KTypeFunction:
		llr := ty.Return.String()
		if ty.Selector == KTypeClosure {
			llr += "(closure)"
		}
		llr += "("
		for ii, ity := range ty.Types {
			if ii > 0 {
				llr += ", "
			}
			llr += ity.String()
		}
		llr += ")"
		return llr

	case KTypeTuple:
		w := bytes.NewBuffer([]byte{})
		fmt.Fprint(w, "(")
		for i, ty := range ty.Types {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprint(w, ty.String())
		}
		fmt.Fprint(w, ")")
		return w.String()

	case KTypeVector:
		return "[" + ty.Types[0].String() + "]"

	case KTypeWorker:
		return "worker(" + ty.Types[0].String() + ")" + ty.Types[1].String()

	case KTypeStruct:
		return fmt.Sprintf("struct(%v, %v)", ty.StructId.Module.Key(), ty.StructId.Name)

	case KTypeNull:
		return "null"

	default:
		panic("cases exhausted for Type.String()")
		return ""
	}
}

func (ty Type) DefaultValueExpression(scope *Scope) (Expression, bool) {
	switch ty.Selector {
	case KTypeInteger:
		return &IntegerTerminal{ Value: 0 }, true

	case KTypeString:
		return &StringTerminal{ Value: "" }, true

	case KTypeBoolean:
		return &BooleanTerminal{ Value: false }, true

	case KTypeCharacter:
		return &CharacterTerminal{ CodePoint: 0 }, true

	case KTypeFloat:
		return &FloatTerminal{ LValue: 0, Zeros: 1, RValue: 0 }, true

	// this admits null pointers, but to remove them we need a solution to recursive structs
	case KTypePointer, KTypeNull:
		return &NullLiteral {}, true
		
	case KTypeStruct:
		sd, fnd := scope.LookupStructDefinition(ty.StructId)
		if fnd {
			sle := &StructLiteralExpression {
				Id: ty.StructId,
				Pairs: []StructLiteralPair {},
			}

			for _, field := range sd.Fields {
				dve, ok := field.Type.DefaultValueExpression(scope)
				if !ok {
					return nil, false
				}
				sle.Pairs = append(sle.Pairs, StructLiteralPair {
					FieldName: field.Name,
					Value: dve,
				})
			}

			return sle, true
		} else {
			return nil, false
		}

	// contraversial, but for now i'm requiring these
	case KTypeClosure, KTypeFunction, KTypeWorker, KTypeVector:
		return nil, false

	default:
		panic("Type.DefaultValueExpression: no default value for " + ty.String())
		return nil, false
	}
}
