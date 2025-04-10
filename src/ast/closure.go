package ast

import (
	"bytes"
	"fmt"
)

type FunctionSignature struct {
	Location FunctionLocation
	Return      Type
	Types       []Type
}

func (fs FunctionSignature) MapKey() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf, fs.Return.RawIdentifier())
	switch fs.Location {
	case KLocationCpu:
		fmt.Fprintf(buf, "__cpu")

	case KLocationGpu:
		fmt.Fprintf(buf, "__gpu")

	case KLocationAnywhere:

	default:
		panic("missing case")
	}
	fmt.Fprintf(buf, "__")
	for tyi, ty := range fs.Types {
		if tyi > 0 {
			fmt.Fprintf(buf, "_")
		}
		fmt.Fprintf(buf, ty.RawIdentifier())
	}
	return buf.String()
}

// Represent the creation of a closure
type ClosureExpression struct {
	// The root function this builds upon
	CalledExpression Expression

	// The name of the function this calls (NB this is temporary)
	CalledFunctionId FunctionId

	// Pre-supplied arguments to the root function
	SuppliedArguments []Expression

	// varnames for supplied arguments when "frozen"
	ArgumentVariables []string

	// The name of the args array variable
	ArgumentArrayName string
}

var _ Expression = &ClosureExpression{}

func (ce *ClosureExpression) Type() Type {
	cet := ce.CalledExpression.Type()
	tys := []Type{}
	for si, se := range ce.SuppliedArguments {
		if se == nil {
			tys = append(tys, cet.Types[si])
		} else {
			// in this case the argument has been supplied by the closure
		}
	}

	prov := make([]bool, len(ce.SuppliedArguments))
	for argi, arg := range ce.SuppliedArguments {
		prov[argi] = arg != nil
	}
	ty := Type{
		Selector: KTypeClosure,
		Types:    tys,
		Return:   cet.Return,
	}

	return ty
}

func (ce *ClosureExpression) String() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf, "ClosureExpression(%v", ce.CalledExpression.String())
	for _, arg := range ce.SuppliedArguments {
		if arg == nil {
			fmt.Fprint(buf, ", <placeholder>")
		} else {
			fmt.Fprintf(buf, ", %v", arg)
		}
	}
	fmt.Fprint(buf, ")")
	return buf.String()
}

func (ce *ClosureExpression) Check(ctx *CheckContext, scope *Scope) {
	ce.CalledExpression.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	cet := ce.CalledExpression.Type()

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		// do this check early, it will cause crashes if false and unchecked
		if len(cet.Types) != len(ce.SuppliedArguments) {
			ctx.Errors.Errorf("Cannot partially apply %v arguments to a function of %v arguments", len(ce.SuppliedArguments), len(cet.Types))
			return
		}

		ce.ArgumentArrayName = ctx.GetTemporaryName()

	case KPassMutate:
		// freeze the supplied arguments into variables
		ce.ArgumentVariables = make([]string, len(ce.SuppliedArguments))
		for ei, e := range ce.SuppliedArguments {
			if e == nil {
				continue
			}

			ce.ArgumentVariables[ei] = ctx.GetTemporaryName()
			ctx.InsertStatementBefore(&AssignStatement{
				Lhs: &IdentifierLValue{
					Name: ce.ArgumentVariables[ei],
				},
				PinPointers: false,
				NewType:     e.Type(),
				Rhs:         e,
				Type:        KAssignLet,
			})

		}
		ctx.InsertStatementBefore(&ClosureArgDeclarationStatement{
			Name:      ce.ArgumentArrayName,
			Args:      ce.ArgumentVariables,
			AddressOf: true,
		})

	case KPassCheckTypes:
		sizeEstimate := 8
		for _, ty := range cet.Types {
			sizeEstimate += ty.EstimateCSize(scope)
			sizeEstimate += 8
		}
		ctx.RequireClosureSize(sizeEstimate)

		if !cet.IsCallable() {
			ctx.Errors.Errorf("Called expression in partial is not callable")
			return
		}

		if cet.Selector == KTypeFunction {
			// this is creating from a raw function (ok)
			it, ok := ce.CalledExpression.(*IdentifierTerminal)
			if !ok {
				ctx.Errors.Errorf(".CalledExpression not an identifier")
				return
			}

			ce.CalledFunctionId = *it.Fid
		} else {
			ctx.Errors.Errorf("Cannot create from closure yet")
			return
		}
	}
}
