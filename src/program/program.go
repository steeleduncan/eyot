package program

import (
	"fmt"
	"os"

	"eyot/ast"
	"eyot/errors"
	"eyot/parser"
	"eyot/token"
)

// the whole program
// NB a bunch of this stuff is per module that could be moved to a struct and merged
// e.g. GpuRequired, MaximumClosureSize, Vectors, Strings (NB FunctionGroup already has such a merge method)
type Program struct {
	// true when gpu is required for something in this program
	GpuRequired bool

	// The largest memory required for a closure
	MaximumClosureSize int

	// map from function type to a vector of functions
	Functions *ast.FunctionGroup

	Env *Environment

	Modules map[string]*ast.Module

	RootModuleId ast.ModuleId

	Strings map[string]int

	// all vector types found in the program (that must be later 
	Vectors map[string]ast.Type

	es *errors.Errors
}

var _ parser.ModuleProvider = &Program{}

func NewProgram(e *Environment, es *errors.Errors) *Program {
	return &Program{
		Modules:            map[string]*ast.Module{},
		Vectors:            map[string]ast.Type {},
		GpuRequired:        false,
		Env:                e,
		es:                 es,
		Functions:          ast.NewFunctionGroup(),
		MaximumClosureSize: 0,
		Strings:            map[string]int{},
	}
}

func (p *Program) FfiFlags() map[string]bool {
	flags := map[string]bool{}

	for _, m := range p.Modules {
		if m.Ffid == nil {
			continue
		}
		for _, flag := range m.Ffid.LinkerFlags {
			flags[flag] = true
		}
	}

	return flags
}

func (p *Program) innerParse(id ast.ModuleId, disallowedIds map[string]bool) *ast.Module {
	path := p.Env.FindModule(id)
	if path == "" {
		return nil
	}

	ffiPath := path[:len(path)-2] + "json"

	ffi, err := FfiAt(ffiPath)
	if err != nil {
		p.es.LogInternalError(fmt.Errorf("Failed to load ffi information %v: %v", ffiPath, err))
		return nil
	}

	blob, err := os.ReadFile(path)
	if err != nil {
		p.es.LogInternalError(fmt.Errorf("Failed to read file %v", path))
		return nil
	}

	tkns, err := token.Tokenise(string(blob))
	if err != nil {
		p.es.LogInternalError(fmt.Errorf("Tokenise failed with error: %v", err))
		return nil
	}

	m := parser.NewParser(p, id, tkns, p.es, disallowedIds, ffi).Module()
	if !p.es.Clean() {
		return nil
	}

	m.Id = id
	p.Modules[id.Key()] = m
	return m
}

func (p *Program) GetModule(id ast.ModuleId, disallowedIds map[string]bool) *ast.Module {
	m, fnd := p.Modules[id.Key()]
	if fnd {
		return m
	}

	newIds := map[string]bool{}
	for k, _ := range disallowedIds {
		newIds[k] = true
	}
	newIds[id.Key()] = true

	mod := p.innerParse(id, newIds)
	p.CheckModule(mod)
	return mod
}

func (p *Program) ParseRoot(moduleName string) {
	p.RootModuleId = []string{moduleName}
	rootModule := p.innerParse(p.RootModuleId, map[string]bool{})
	if rootModule == nil {
		p.es.Errorf("file not found")
	}
	if !p.es.Clean() {
		return
	}
	p.CheckModule(rootModule)

	// add the synthesized main function
	mainFd, fnd := rootModule.LookupFunction("main")
	if !fnd {
		p.es.Errorf("No main function found")
		return
	}
	if len(mainFd.Parameters) != 0 {
		p.es.Errorf("Main function (%v) should not take arguments %v", mainFd.Id, mainFd.Parameters)
		return
	}
}

// Top level type checking
//
// This fills in a bunch of data that is needed for output
func (p *Program) CheckModule(m *ast.Module) {
	if m == nil {
		return
	}

	ctx := ast.NewCheckContext(p.es, p.Strings)

	ctx.Errors.SetActivity("Set top level types")
	ctx.Pass = ast.KPassSetTleTypes
	ctx.PrepareForPass(m)
	m.Check(ctx)

	ctx.Errors.SetActivity("Set types")
	ctx.Pass = ast.KPassSetTypes
	ctx.PrepareForPass(m)
	m.Check(ctx)

	// the manually expanded loop is because the Definition.Check can add to the end of the structs array
	// TODO remove this and rerun tests, it is probably fine without
	i := 0
	for i < len(ctx.Structs) {
		rstr := ctx.Structs[i]
		if rstr.Id.Module.IsEqual(m.Id) {
			m.Structs = append(m.Structs, rstr)
		}

		i += 1
	}

	for vecId, vec := range ctx.Vectors {
		// it is ok to overwrite
		p.Vectors[vecId] = vec
	}

	ctx.Errors.SetActivity("Mutate tree")
	ctx.Pass = ast.KPassMutate
	ctx.PrepareForPass(m)
	m.Check(ctx)
	if !p.es.Clean() {
		return
	}

	ctx.Errors.SetActivity("Check types")
	ctx.Pass = ast.KPassCheckTypes
	ctx.PrepareForPass(m)
	m.Check(ctx)

	if !p.es.Clean() {
		return
	}

	ctx.Errors.SetActivity("")

	if !p.GpuRequired {
		p.GpuRequired = ctx.GpuRequired()
	}
	p.Functions.MergeIn(ctx.Functions)
	if ctx.MaximumClosureSize() > p.MaximumClosureSize {
		p.MaximumClosureSize = ctx.MaximumClosureSize()
	}
}

func (p *Program) GetStringPool() []string {
	pool := make([]string, len(p.Strings))

	for s, id := range p.Strings {
		pool[id] = s
	}

	return pool
}

