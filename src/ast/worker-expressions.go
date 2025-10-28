package ast

import (
	"fmt"
)

type PipeDestination int

const (
	KDestinationCpu PipeDestination = iota
	KDestinationGpu
)

type CreateWorkerExpression struct {
	Worker                Expression
	SendType, ReceiveType Type
	Destination           PipeDestination

	// In the case of creating a closure, the variable name, or blank
	ClosureVariable string

	// name of the wrapper function (and kernel)
	WrapperId, KernelId FunctionId
}

var _ Expression = &CreateWorkerExpression{}

func (cce *CreateWorkerExpression) Type() Type {
	return Type{
		Selector: KTypeWorker,
		Types:    []Type{cce.SendType, cce.ReceiveType},
	}
}

func (cce *CreateWorkerExpression) String() string {
	return fmt.Sprintf("CreateWorkerExpression(%v, %v)", cce.Destination, cce.Worker.String())
}

func (cce *CreateWorkerExpression) Check(ctx *CheckContext, scope *Scope) {
	ctx.NoteCpuRequired("create worker")

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		cce.Worker.Check(ctx, scope)
		if !ctx.Errors.Clean() {
			return
		}
		if cce.Destination == KDestinationGpu {
			ctx.SetGpuRequired()
		}

		// need early checks
		lty := cce.Worker.Type()
		if !lty.IsCallable() {
			ctx.Errors.Errorf("A create channel expression must be passed something callable")
			return
		}
		if len(lty.Types) != 1 {
			ctx.Errors.Errorf("A create channel expression must be passed a function with a single parameter")
			return
		}
		if lty.Selector == KTypeFunction {
			// ok case
		} else {
			cce.ClosureVariable = ctx.GetTemporaryName()
		}

		cce.SendType = lty.Types[0]
		cce.ReceiveType = *lty.Return

	case KPassMutate:
		cce.Worker.Check(ctx, scope)
		cce.WrapperId = FunctionId{
			Module: ctx.CurrentModule().Id,
			Struct: BlankStructId(),
			Name:   fmt.Sprintf("generated_wrapper_%v", ctx.GetUniqueId()),
		}

		// this requires the structs to be set (earlier on)
		if cce.Destination == KDestinationGpu {
			for _, ty := range cce.Worker.Type().Types {
				if ok, problemType := scope.CanPassToGpu(ty); !ok {
					if ty.Equal(problemType) {
						ctx.Errors.Errorf("Worker creation uses type that cannot be passed to GPU '" + ty.String() + "'")
					} else {
						ctx.Errors.Errorf("Worker creation uses type that cannot be passed to GPU '" + problemType.String() + "' embedded in '" + ty.String() + "'")
					}
				}
			}
		}

		if cce.ClosureVariable != "" {
			ct := cce.Worker.Type()
			// place a copy of the closure on the stack
			ctx.InsertStatementBefore(&AssignStatement{
				Lhs: &IdentifierLValue{
					Name: cce.ClosureVariable,
					// is this true?
					cachedType: ct,
				},
				PinPointers: true,
				NewType:     ct,
				Rhs:         cce.Worker,
				Type:        KAssignLet,
			})
		}

		switch cce.Destination {
		case KDestinationGpu:
			if cce.Worker.Type().Selector == KTypeClosure {
				cce.KernelId = FunctionId{
					Module: ctx.CurrentModule().Id,
					Struct: BlankStructId(),
					Name:   fmt.Sprintf("ey_generated_kernel_%v", ctx.GetUniqueId()),
				}

				gkt := &GpuKernelTle{
					KernelId:        cce.KernelId,
					IsClosureWorker: true,
					Input:           cce.Worker.Type().Types[0],
					Output:          *cce.Worker.Type().Return,
				}
				ctx.InsertElementBefore(gkt)
			} else {
				it, ok := cce.Worker.(*IdentifierTerminal)
				if ok {
					cce.KernelId = FunctionId{
						Module: ctx.CurrentModule().Id,
						Struct: BlankStructId(),
						Name:   fmt.Sprintf("ey_generated_kernel_%v", ctx.GetUniqueId()),
					}

					if it.Fid == nil {
						panic("Function id nil for gpu worker")
					}

					gkt := &GpuKernelTle{
						KernelId:        cce.KernelId,
						WorkerId:        *it.Fid,
						IsClosureWorker: false,
						Input:           cce.Worker.Type().Types[0],
						Output:          *cce.Worker.Type().Return,
					}
					ctx.InsertElementBefore(gkt)
				} else {
					// Eventually it would be nice if this were more flexible
					// However we need to understand this at C generation time
					ctx.Errors.Errorf("A create worker expression must be passed a function name (for now)")
					return
				}
			}

		case KDestinationCpu:
			inName := "input"
			outName := "output"
			clsName := "ctx"
			typedInputName := "typed_input"
			typedOutputName := "typed_output"

			called := cce.Worker

			castInputPointerType := MakePointer(cce.Worker.Type().Types[0])
			castOutputPointerType := MakePointer(*cce.Worker.Type().Return)

			// TODO this and typed_output aren't always needed
			// EyInteger* typed_input = input;
			typedInputStatement := &AssignStatement{
				Lhs: &IdentifierLValue{
					Name:       typedInputName,
					cachedType: castInputPointerType,
				},
				PinPointers: true,
				NewType:     castInputPointerType,
				Rhs: &IdentifierTerminal{
					Name:          inName,
					DontNamespace: true,
					CachedType:    castInputPointerType,
				},
				Type: KAssignLet,
			}

			// EyInteger* typed_output = output;
			typedOutputStatement := &AssignStatement{
				Lhs: &IdentifierLValue{
					Name:       typedOutputName,
					cachedType: castOutputPointerType,
				},
				PinPointers: true,
				NewType:     castOutputPointerType,
				Rhs: &IdentifierTerminal{
					Name:          outName,
					DontNamespace: true,
					CachedType:    castOutputPointerType,
				},
				Type: KAssignLet,
			}

			callExpression := &CallExpression{
				CalledExpression: called,
				Arguments: []Expression{
					&DereferenceExpression{
						Pointer: &IdentifierTerminal{
							Name:          typedInputName,
							DontNamespace: true,
							CachedType:    castInputPointerType,
						},
					},
				},
				cachedType: Type{Selector: KTypeVoid},
			}

			stmts := []StatementContainer{
				StatementContainer{
					Statement: typedInputStatement,
					Context:   scope, // TODO bad?
				},
				StatementContainer{
					Statement: typedOutputStatement,
					Context:   scope,
				},
			}

			var callStatement Statement
			if cce.ClosureVariable == "" {
				if cce.Worker.Type().Return.Selector == KTypeVoid {
					// no output required
					callStatement = &ExpressionStatement{
						Expression: callExpression,
					}
				} else {
					// we need the output
					callStatement = &AssignStatement{
						Lhs: &DerefLValue{
							Inner: &IdentifierLValue{
								Name:       typedOutputName,
								cachedType: castOutputPointerType,
							},
						},
						PinPointers: false,
						Type:        KAssignNormal,
						NewType:     castOutputPointerType,
						Rhs:         callExpression,
					}
				}
			} else {
				// const char *args[] = { input };
				stmts = append(stmts, StatementContainer{
					Statement: &ClosureArgDeclarationStatement{
						Name:      "args",
						Args:      []string{"input"},
						AddressOf: false,
					},
					Context: scope,
				})

				// ey_closure_call((*typed_closure)->fid, output, args);
				callStatement = &ExpressionStatement{
					Expression: &CallExpression{
						IgnoreTypeChecks: true,
						CalledExpression: &IdentifierTerminal{
							Name:          "ey_closure_call",
							DontNamespace: true,
						},
						Arguments: []Expression{
							&IdentifierTerminal{
								Name: "ctx",
								// a hack to get a ->
								CachedType: MakePointer(MakePointer(Type{Selector: KTypeVoid})),
							},
							&IdentifierTerminal{
								Name:          "output",
								DontNamespace: true,
							},
							&IdentifierTerminal{
								Name:          "args",
								DontNamespace: true,
							},
						},
						cachedType: Type{Selector: KTypeVoid},
					},
				}
			}

			stmts = append(stmts, StatementContainer{
				Statement: callStatement,
				Context:   scope,
			})

			// nil contexts will probably cause a crash one day
			fd := &FunctionDefinition{
				Id: cce.WrapperId,

				Return:   Type{Selector: KTypeVoid},
				Scope:    nil,
				Location: KLocationCpu,

				// don't typecheck stuff we've generated
				AvoidCheckPhase: true,

				Block: &StatementBlock{
					Statements: stmts,
				},
				// we've checked earlier that worker is a single param lambda
				Parameters: []FunctionParameter{
					FunctionParameter{
						Name: inName,
						Type: MakePointer(Type{Selector: KTypeVoid}),
					},
					FunctionParameter{
						Name: outName,
						Type: MakePointer(Type{Selector: KTypeVoid}),
					},
					FunctionParameter{
						Name: clsName,
						Type: MakePointer(Type{Selector: KTypeVoid}),
					},
				},
			}

			// this adds to the function group (which would normally be done in an earlier check pass)
			fd.AddToContext(ctx)

			// generate a wrapper function for this that can be passed to the worker
			ctx.InsertElementBefore(&FunctionDefinitionTle{
				Definition: fd,
			})
		}
	}
}

type ReceiveWorkerExpression struct {
	// The worker in question
	Worker Expression

	// the received variable (set in a previous statement)
	Received Expression

	// When true this drains the pipe (close and receive a vector)
	All bool
}

var _ Expression = &ReceiveWorkerExpression{}

func (rpe *ReceiveWorkerExpression) Type() Type {
	ty := rpe.Worker.Type().Types[1]

	if ty.Selector == KTypeVoid {
		// a non-returning function leads to a void receive pipe expression
		return Type{Selector: KTypeVoid}
	}

	if rpe.All {
		return MakeVector(ty)
	} else {
		return ty
	}
}
func (rpe *ReceiveWorkerExpression) String() string {
	v := "one"
	if rpe.All {
		v = "drain"
	}
	return fmt.Sprintf("ReceiveWorkerExpression(%v, %v)", rpe.Worker, v)
}

func (rpe *ReceiveWorkerExpression) Check(ctx *CheckContext, scope *Scope) {
	ctx.NoteCpuRequired("receive from worker")

	rpe.Worker.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		pty := rpe.Worker.Type()
		if pty.Selector != KTypeWorker {
			ctx.Errors.Errorf("Expected a pipe after 'receive'")
		}

	case KPassMutate:
		receivedVarName := ctx.GetTemporaryName()
		receivedType := rpe.Worker.Type().Types[1]

		if rpe.All {
			// no need for a separate receive step as we always get a vector back
			// EyVector *vec = ey_worker_drain(w);
			rpe.Received = &CallExpression{
				IgnoreTypeChecks: true,
				CalledExpression: &AccessExpression{
					Accessed:   rpe.Worker,
					AllowRaw:   true,
					Identifier: "drain",
				},
				SkipExecutionContext: true,
				Arguments: []Expression{
					rpe.Worker,
				},
				// this should calc correctly as vector
				cachedType: rpe.Type(),
			}
		} else {
			// declare receiving variable
			ctx.InsertStatementBefore(&AssignStatement{
				Lhs: &IdentifierLValue{
					Name:       receivedVarName,
					cachedType: receivedType,
				},
				PinPointers: false,
				NewType:     receivedType,
				Rhs:         nil,
				Type:        KAssignLet,
			})
			// receive into that variable
			ctx.InsertStatementBefore(&ExpressionStatement{
				Expression: &CallExpression{
					IgnoreTypeChecks: true,
					CalledExpression: &AccessExpression{
						Accessed:   rpe.Worker,
						AllowRaw:   true,
						Identifier: "receive",
					},
					SkipExecutionContext: true,
					Arguments: []Expression{
						rpe.Worker,
						&UnaryExpression{
							Operator: KOperatorAddressOf,
							Rhs: &IdentifierTerminal{
								Name: receivedVarName,
							},
						},
					},
					cachedType: Type{Selector: KTypeVoid},
				},
			})
			// set the received to that name
			rpe.Received = &IdentifierTerminal{
				Name:          receivedVarName,
				DontNamespace: true,
			}
		}

	}
}

type CreatePipelineExpression struct {
	// the input and output types of this as a whole
	SendType, ReceiveType Type

	// lhs is the first worker that feeds to the second worker
	LhsWorker, RhsWorker Expression

	// The type that is transferred internally by this expression
	IntermediateType Type
}

var _ Expression = &CreatePipelineExpression{}

func (cpe *CreatePipelineExpression) Type() Type {
	return Type{
		Selector: KTypeWorker,
		Types:    []Type{cpe.SendType, cpe.ReceiveType},
	}
}

func (cpe *CreatePipelineExpression) String() string {
	return fmt.Sprintf("CreatePipelineExpression(%v, %v)", cpe.LhsWorker.String(), cpe.RhsWorker.String())
}

func (cpe *CreatePipelineExpression) Check(ctx *CheckContext, scope *Scope) {
	ctx.NoteCpuRequired("create pipeline")

	cpe.LhsWorker.Check(ctx, scope)
	cpe.RhsWorker.Check(ctx, scope)
	if !ctx.Errors.Clean() {
		return
	}

	switch ctx.CurrentPass() {
	case KPassSetTypes:
		lty := cpe.LhsWorker.Type()
		if lty.Selector != KTypeWorker {
			ctx.Errors.Errorf("First argument to pipeline keyword must be a worker expression")
			return
		}

		rty := cpe.RhsWorker.Type()
		if rty.Selector != KTypeWorker {
			ctx.Errors.Errorf("Second argument to pipeline keyword must be a worker expression")
			return
		}

		cpe.SendType = lty.Types[0]
		cpe.IntermediateType = lty.Types[1]
		cpe.ReceiveType = rty.Types[1]

		if !rty.Types[0].Equal(cpe.IntermediateType) {
			ctx.Errors.Errorf("Output from first argument to pipeline must be the same as the input to the second")
			return
		}

	}
}
