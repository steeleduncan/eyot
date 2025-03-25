package ast

type FfiDefinitions struct {
	// the C source
	Src         string
	Functions   []CFunction
	LinkerFlags []string
}

type Module struct {
	TopLevelElements []TopLevelElementContainer
	Structs          []*RequiredStruct
	Scope            *Scope
	Id               ModuleId
	Ffid             *FfiDefinitions
}

func (f *Module) LookupFunction(name string) (*FunctionDefinition, bool) {
	for _, tlec := range f.TopLevelElements {
		fdtle, ok := tlec.TopLevelElement.(*FunctionDefinitionTle)
		if ok && fdtle.Definition.Id.Name == name {
			return fdtle.Definition, true
		}
	}

	return nil, false
}

func (f *Module) Check(ctx *CheckContext) {
	newTlecs := []TopLevelElementContainer{}

	for _, tlec := range f.TopLevelElements {
		tlec.Check(ctx)
		if !ctx.Errors.Clean() {
			return
		}

		els := ctx.GetElementsForInsert()

		if els != nil {
			for _, el := range els {
				newTlecs = append(newTlecs, TopLevelElementContainer{
					TopLevelElement: el,
					Context:         f.Scope,
				})
			}
		}

		newTlecs = append(newTlecs, tlec)
	}

	f.TopLevelElements = newTlecs
}
