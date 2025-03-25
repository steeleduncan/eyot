package ast

import ()

type StructField struct {
	Name string
	Type Type
}

type StructDefinition struct {
	// Variables in the struct
	Fields []StructField

	// Functions in the struct
	Functions []*FunctionDefinition

	Scope *Scope
}

func (sd *StructDefinition) Check(ctx *CheckContext) {
	for _, fn := range sd.Functions {
		fn.Check(ctx, sd.Scope)
	}
}

func (sd *StructDefinition) GetField(name string) (StructField, bool) {
	for _, f := range sd.Fields {
		if f.Name == name {
			return f, true
		}
	}

	for _, fn := range sd.Functions {
		if fn.Id.Name == name {
			args := []Type{}
			for _, p := range fn.Parameters {
				args = append(args, p.Type)
			}

			return StructField{
				Name: name,
				Type: Type{
					Selector:        KTypeFunction,
					BoundStructName: name,
					Return:          &fn.Return,
					Requirement:     fn.Requirement,
					Types:           args,
				},
			}, true
		}
	}

	return StructField{}, false
}
