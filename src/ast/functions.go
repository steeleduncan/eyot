package ast

import (
	"fmt"
)

type FunctionParameter struct {
	Name string
	Type Type
}

type FunctionLocation int

const (
	/*
	   This needs to run on the CPU

	   Eventually this should probably be split into actual requirements
	   Likely some cpu functions are available on gpu, and some will become so eventually
	*/
	KLocationCpu FunctionLocation = iota

	/*
	   Pure code that can run anywhere
	*/
	KLocationAnywhere

	/*
	   This needs to run on the GPU
	*/
	KLocationGpu
)

type FunctionDefinition struct {
	Id              FunctionId
	Return          Type
	AvoidCheckPhase bool
	Location     FunctionLocation

	// scope including function parameters
	Scope *Scope

	Block      *StatementBlock
	Parameters []FunctionParameter
}

func (fd *FunctionDefinition) String() string {
	return fmt.Sprintf("FunctionDefinition(%v)", fd.Id)
}

func (fd *FunctionDefinition) Signature() FunctionSignature {
	types := []Type{}

	if !fd.Id.Struct.Blank() {
		types = append(types, MakePointer(Type{Selector: KTypeStruct, StructId: fd.Id.Struct}))
	}

	for _, fp := range fd.Parameters {
		types = append(types, fp.Type)
	}

	return FunctionSignature{
		Return:      fd.Return,
		Types:       types,
		Location: fd.Location,
	}
}

// the type of this function when viewed as a variable
func (fd *FunctionDefinition) OurType() Type {
	ftype := Type{
		Return:      &fd.Return,
		Selector:    KTypeFunction,
		Types:       []Type{},
		Location: fd.Location,
	}

	for _, arg := range fd.Parameters {
		ftype.Types = append(ftype.Types, arg.Type)
	}

	return ftype
}

func CheckStatementBlockEndsWithReturn(sb *StatementBlock) bool {
	if len(sb.Statements) == 0 {
		return false
	} else {
		last := sb.Statements[len(sb.Statements)-1].Statement

		switch s := last.(type) {
		case *ReturnStatement:
			return true

		case *IfStatement:
			for _, seg := range s.Segments {
				if !CheckStatementBlockEndsWithReturn(seg.Block) {
					return false
				}
			}
			return true

		default:
			return false
		}
	}
}

func (fd *FunctionDefinition) Check(ctx *CheckContext, externalScope *Scope) {
	if fd.Id.Module.Blank() {
		panic("blank definition " + fd.Id.Name)
	}

	switch fd.Location {
	case KLocationGpu:
		ctx.EnterGpuBlock()
		defer ctx.LeaveGpuBlock()

	case KLocationCpu:
		ctx.EnterCpuBlock()
		defer ctx.LeaveCpuBlock()
	}

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		if fd.Return.Selector != KTypeVoid {
			if !CheckStatementBlockEndsWithReturn(fd.Block) {
				ctx.Errors.Errorf("A non-void function must end with a return")
				return
			}
		}

		for _, arg := range fd.Parameters {
			ctx.RequireType(arg.Type, externalScope)
		}

		// struct functions should not be readily accessible in the local namespace
		if fd.Id.Struct.Blank() {
			externalScope.SetVariable(fd.Id.Name, fd.OurType(), false)
		}

		ctx.RequireType(fd.Return, externalScope)
		if !ctx.Errors.Clean() {
			return
		}
		ctx.AddFunction(fd.Id, fd.Signature())

	case KPassCheckTypes:
		if fd.AvoidCheckPhase {
			return
		}
	}

	ctx.PushReturnType(fd.Return)
	defer ctx.PopReturnType()
	fd.Block.Check(ctx)
}

// Return the effective parameters of a function, NB this takes into account whether or not it is bound to a struct
func (fd FunctionDefinition) EffectiveParameters(executionContextParameter FunctionParameter) []FunctionParameter {
	prms := fd.Parameters

	if !fd.Id.Struct.Blank() {
		iprms := []FunctionParameter{
			FunctionParameter{
				Name: "ey_self",
				Type: Type{
					Selector: KTypePointer,
					Types: []Type{
						Type{
							Selector: KTypeStruct,
							StructId: fd.Id.Struct,
						},
					},
				},
			},
		}
		prms = append(iprms, prms...)
	}

	iprms := []FunctionParameter{
		executionContextParameter,
	}
	prms = append(iprms, prms...)

	return prms
}
