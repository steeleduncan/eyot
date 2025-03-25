package ast

import (
	"fmt"
	"strings"
)

type ModuleId []string

func (mid ModuleId) Key() string {
	return strings.Join(mid, "::")
}

func (mid ModuleId) Blank() bool {
	if mid == nil {
		return true
	}

	if len(mid) == 0 {
		return true
	}

	return false
}

func (mid ModuleId) DisplayName() string {
	return mid.Key()
}

func (mid ModuleId) Namespace() string {
	if len(mid) == 0 {
		panic("empty id")
	}
	return strings.ReplaceAll(strings.Join(mid, "__"), "-", "_")
}

// A module id that refers to the C namespace (generally ffi)
func BuiltinModuleId() ModuleId {
	return []string{"______builtin______"}
}

// Return true when this is the builtin id, ie one that should be passed on directly without namespacing
func (mid ModuleId) IsBuiltin() bool {
	return len(mid) == 1 && mid[0] == "______builtin______"
}

func (lhs ModuleId) IsEqual(rhs ModuleId) bool {
	if len(lhs) != len(rhs) {
		return false
	}

	for i, l := range lhs {
		if l != rhs[i] {
			return false
		}
	}

	return true
}

// A fully resolved function identifier
type FunctionId struct {
	// the module that owns this
	Module ModuleId

	// The struct/etc within the module that owns this
	// This is blank if it is unbound to a struct
	Struct StructId

	// the user facing name
	Name string
}

func (fi FunctionId) String() string {
	return fmt.Sprintf("FunctionId(%v, %v, %v)", fi.Module.Key(), fi.Struct, fi.Name)
}

func (lhs FunctionId) IsEqual(rhs FunctionId) bool {
	if lhs.Name != rhs.Name {
		return false
	}

	if !lhs.Struct.IsEqual(rhs.Struct) {
		return false
	}

	if !lhs.Module.IsEqual(rhs.Module) {
		return false
	}

	return true
}

// A fully resolved function identifier
type StructId struct {
	// the module that owns this
	Module ModuleId

	// the user facing name
	Name string
}

func BlankStructId() StructId {
	return StructId{}
}

func (si StructId) Blank() bool {
	return si.Name == ""
}

func (si StructId) String() string {
	return fmt.Sprintf("StructId(%v::%v)", si.Module.Key(), si.Name)
}

func (si StructId) Key() string {
	return si.String()
}

func (si StructId) IsEqual(rhs StructId) bool {
	if si.Name != rhs.Name {
		return false
	}

	if !si.Module.IsEqual(rhs.Module) {
		return false
	}

	return true
}
