package ast

import (
	"fmt"
)

// runtime provided functions
type CFunction struct {
	// Name of the builtin (identical to the C name)
	Name string

	// The return type of the builtin
	ReturnType Type

	// The types of arguments
	ArgumentTypes []Type
}

type VariableBinding struct {
	VariableType Type
	Assignable   bool
}

type Scope struct {
	// This is either the parent frame, or nil if it is the top
	Parent *Scope

	// A map from variable names to variable types
	VariableBindings map[string]*VariableBinding

	// a map from namespace name to module id
	ModuleBindings map[string]*Module

	// a map from struct names to definitions
	StructBindings map[string]StructDefinition
}

func (s *Scope) log(level int) {
	for name, bound := range s.VariableBindings {
		for i := 0; i < level; i += 1 {
			fmt.Print(" ")
		}
		if !bound.Assignable {
			fmt.Print("const ")
		}
		fmt.Println(name, "->", bound.VariableType)
	}

	if s.Parent != nil {
		s.Parent.log(level + 1)
	}
}

func (s *Scope) Log() {
	fmt.Println("Log scope")
	s.log(1)
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		Parent:           parent,
		VariableBindings: map[string]*VariableBinding{},
		ModuleBindings:   map[string]*Module{},
		StructBindings:   map[string]StructDefinition{},
	}
}

/*
Is this type ok to use on the GPU

For now nothing with heap allocs, e.g.
- vectors
- pointers

This returns true/false along with the type that can't be passed
*/
func (s *Scope) CanPassToGpu(ty Type) (bool, Type) {
	switch ty.Selector {
	// Depends on what is in there
	case KTypeTuple:
		for _, t := range ty.Types {
			if ok, ty := s.CanPassToGpu(t); !ok {
				return false, ty
			}
		}
		return true, Type{}

	case KTypeStruct:
		sd, ok := s.LookupStructDefinition(ty.StructId)
		if !ok {
			// I think this error should have been squashed a while back
			panic("Unable to lookup struct definition: '" + ty.StructId.String() + "'")
		}

		for _, field := range sd.Fields {
			if ok, ty := s.CanPassToGpu(field.Type); !ok {
				return false, ty
			}
		}

		return true, Type{}

	case KTypeFloat:
		return ty.Width == 32, ty

	// easy answers
	case KTypeInteger, KTypeString, KTypeBoolean, KTypeVoid:
		return true, Type{}

	case KTypeClosure, KTypeFunction, KTypePointer, KTypeVector, KTypeWorker:
		return false, ty
	}

	panic("exhausted cases")
	return false, Type{}
}

func (s *Scope) LookupModule(ident string) (*Module, bool) {
	mod, fnd := s.ModuleBindings[ident]
	if fnd {
		return mod, true
	}

	if s.Parent == nil {
		return nil, false
	}

	return s.Parent.LookupModule(ident)
}

// Return if it cannot be defined at this level (ie if it has been once already)
func (s *Scope) CannotBeDefinedAtThisLevel(ident string) bool {
	_, fnd := s.VariableBindings[ident]
	return fnd
}

// Return type, assignable, found
func (s *Scope) LookupVariableType(ident string) (Type, bool, bool) {
	b, fnd := s.VariableBindings[ident]
	if fnd {
		return b.VariableType, b.Assignable, true
	}

	if s.Parent == nil {
		return Type{}, false, false
	}

	return s.Parent.LookupVariableType(ident)
}

func (s *Scope) LookupStructDefinition(ident StructId) (StructDefinition, bool) {
	sd, fnd := s.StructBindings[ident.Key()]
	if fnd {
		return sd, true
	}

	if s.Parent == nil {
		return StructDefinition{}, false
	}

	return s.Parent.LookupStructDefinition(ident)
}

func (s *Scope) AddCFunction(builtin CFunction) {
	ty := Type{
		Selector: KTypeFunction,
		Return:   &builtin.ReturnType,
		Types:    builtin.ArgumentTypes,
		Builtin:  true,
	}

	s.SetVariable(builtin.Name, ty, false)
}

func (s *Scope) AddCFunctions(cfs []CFunction) {
	for _, cf := range cfs {
		s.AddCFunction(cf)
	}
}

func (s *Scope) SetVariable(ident string, ty Type, assignable bool) {
	if ty.IsCallable() {
		binding, fnd := s.VariableBindings[ident]
		if fnd {
			if binding.VariableType.IsCallable() {
				// idk if the right approach
				// but when we dual assign a cpu and gpu function, we pretend it is a call any
				if binding.VariableType.Location == KLocationCpu && ty.Location == KLocationGpu {
					binding.VariableType.Location = KLocationAnywhere
					return
				} else if binding.VariableType.Location == KLocationGpu && ty.Location == KLocationCpu {
					binding.VariableType.Location = KLocationAnywhere
					return
				}
			} else {
				// maybe this should be an error?
				panic("Incompatible type for variable assignment " + ident)
			}
		}
	}

	s.VariableBindings[ident] = &VariableBinding{
		VariableType: ty,
		Assignable:   assignable,
	}
}

// Set the map from a module ident to module id
func (s *Scope) SetModule(ident string, mod *Module) {
	s.ModuleBindings[ident] = mod
}

func (s *Scope) SetStruct(id StructId, sd StructDefinition) {
	s.StructBindings[id.Key()] = sd
}
