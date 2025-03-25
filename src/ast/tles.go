package ast

import (
	"bytes"
	"eyot/errors"
	"fmt"
)

type TopLevelElement interface {
	Check(*CheckContext, *Scope)
	String() string
}

type TopLevelElementContainer struct {
	TopLevelElement TopLevelElement
	Context         *Scope
}

func (tlec *TopLevelElementContainer) Check(ctx *CheckContext) {
	tlec.TopLevelElement.Check(ctx, tlec.Context)
}

type FunctionDefinitionTle struct {
	Definition *FunctionDefinition
}

var _ TopLevelElement = &FunctionDefinitionTle{}

func (ftle *FunctionDefinitionTle) String() string {
	return fmt.Sprintf("FunctionDefinitionTle(%v)", ftle.Definition.String())
}

func (ftle *FunctionDefinitionTle) Check(ctx *CheckContext, scope *Scope) {
	ftle.Definition.Check(ctx, scope)
}

type GpuKernelTle struct {
	// the name of the kernel itself
	KernelId FunctionId

	// the closure that is used for work
	IsClosureWorker bool

	// the function called by the kernel (used if .IsClosureWorker is false)
	WorkerId      FunctionId
	Input, Output Type
}

var _ TopLevelElement = &GpuKernelTle{}

var _ TopLevelElement = &FunctionDefinitionTle{}

func (gkt *GpuKernelTle) String() string {
	return "GpuKernelTle(TODO)"
}

func (gkt *GpuKernelTle) Check(ctx *CheckContext, scope *Scope) {
	// passover, we aren't really interested in doing anything specific here
}

/*
This is a junk tle
It does nothing, but hold source locations
*/
type DummyTle struct {
	Location errors.SourceLocation
}

var _ TopLevelElement = &DummyTle{}

func (dt *DummyTle) String() string {
	return fmt.Sprintf("DummyTle(%v)", dt.Location)
}

func (dt *DummyTle) Check(ctx *CheckContext, scope *Scope) {
	ctx.Errors.SetCurrentLocation(dt.Location)
}

type ImportElement struct {
	Names    []string
	ImportAs string
	Mod      *Module
}

var _ TopLevelElement = &ImportElement{}

func (ie *ImportElement) ImportedId() ModuleId {
	return ie.Names
}

func (ie *ImportElement) String() string {
	buf := bytes.NewBuffer([]byte{})

	fmt.Fprint(buf, "ImportElement(")

	for nmi, nm := range ie.Names {
		if nmi > 0 {
			fmt.Fprint(buf, ", ")
		}
		fmt.Print(buf, nm)
	}
	fmt.Fprint(buf, ")")

	return buf.String()
}

func (ie *ImportElement) Check(ctx *CheckContext, scope *Scope) {
	ident := ie.Names[len(ie.Names)-1]
	scope.SetModule(ident, ie.Mod)
}

type ConstTle struct {
	Assign *AssignStatement
}

var _ TopLevelElement = &ConstTle{}

func (ce *ConstTle) String() string {
	return fmt.Sprintf("ConstTle(%v)", ce.Assign)
}

func (ce *ConstTle) Check(ctx *CheckContext, scope *Scope) {
	// seems the easiest way to do this for now
	ce.Assign.Check(ctx, scope)
}
