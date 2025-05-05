package cwriter

import (
	"bytes"
	"eyot/output/textwriter"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"eyot/ast"
	"eyot/program"
)

type CWriterScope struct {
	SavedPointers []ast.LValue
}

type CWriter struct {
	writers    []*textwriter.W
	tempCount  int
	writingGpu bool

	scopes []*CWriterScope
}

const closureIdFieldName string = "fn_id"

func DumpRuntime(path string, env *program.Environment) []string {
	runtimeFiles := []string{}

	getSrc := func(name string) string {
		path := filepath.Join(env.RuntimeRoot(), name)
		blob, err := os.ReadFile(path)
		if err != nil {
			panic("Unable to find runtime source: " + path)
		}
		return string(blob)
	}

	cFiles := []string{
		"eyot-runtime-closures.c",
		"eyot-runtime-strings.c",
		"eyot-runtime-vectors.c",
		"eyot-runtime-entry-point.c",
		"eyot-runtime-cpu-worker.c",
		"eyot-runtime-cpu-pipeline.c",
		"eyot-runtime-pipes.c",
		"eyot-runtime-gc.c",
		"eyot-runtime-opencl.c",
	}
	for _, src := range cFiles {
		runtimeFiles = append(runtimeFiles, src)
		dotC := filepath.Join(path, src)
		os.WriteFile(dotC, []byte(getSrc(src)), 0664)
	}

	hFiles := []string{
		"eyot-runtime-cpu.h",
		"eyot-runtime-pipe.h",
		"eyot-runtime-common.h",
	}
	for _, src := range hFiles {
		dotH := filepath.Join(path, src)
		os.WriteFile(dotH, []byte(getSrc(src)), 0664)
	}

	return runtimeFiles
}

// Create a new cwriter (automatically writing the runtime)
func NewCWriter(w *textwriter.W) *CWriter {
	cw := &CWriter{
		writers:    []*textwriter.W{w},
		tempCount:  0,
		writingGpu: false,
		scopes:     []*CWriterScope{},
	}

	return cw
}

func (cw *CWriter) StartScope() {
	cw.scopes = append(cw.scopes, &CWriterScope{SavedPointers: []ast.LValue{}})
}

func (cw *CWriter) EndScope() {
	cw.scopes = cw.scopes[:(len(cw.scopes) - 1)]
}

func (cw *CWriter) CurrentStackScope() *CWriterScope {
	return cw.scopes[len(cw.scopes)-1]
}

func (cw *CWriter) exitScope(scope *CWriterScope) {
	if cw.WritingGpu() {
		return
	}

	for _, lv := range scope.SavedPointers {
		cw.w().AddComponents("ey_runtime_gc_forget_root_pointer", "(", "ey_runtime_gc", "(", namespaceExecutionContext(), ")", ",", "&")
		cw.WriteLValue(lv)
		cw.w().AddComponents(")", ";")
		cw.w().EndLine()
	}
}

/*
Leaving just this scope, write any relevant close statements
*/
func (cw *CWriter) AboutToExitThisScope() {
	cw.exitScope(cw.CurrentStackScope())
}

/*
Returning from a function, and exiting all scopes
*/
func (cw *CWriter) AboutToExitAllScopes() {
	for _, scope := range cw.scopes {
		cw.exitScope(scope)
	}
}

func (cw *CWriter) w() *textwriter.W {
	return cw.writers[len(cw.writers)-1]
}

func (cw *CWriter) GetTemporaryName() string {
	cw.tempCount += 1
	return fmt.Sprintf("ey_tempb_%v", cw.tempCount)
}

func convertedBinaryOperator(binop ast.BinaryOperator) string {
	switch binop {
	case ast.KOperatorAdd:
		return "+"
	case ast.KOperatorSubtract:
		return "-"
	case ast.KOperatorMultiply:
		return "*"
	case ast.KOperatorDivide:
		return "/"

	case ast.KOperatorEquality:
		return "=="
	case ast.KOperatorInequality:
		return "!="

	case ast.KOperatorLT:
		return "<"
	case ast.KOperatorLTE:
		return "<="
	case ast.KOperatorGT:
		return ">"
	case ast.KOperatorGTE:
		return ">="

	case ast.KOperatorAnd:
		return "&&"
	case ast.KOperatorOr:
		return "||"

	case ast.KOperatorMod:
		return "%"
	}

	panic("convertedBinaryOperator()")
	return ""
}

func convertedUnaryOperator(op ast.UnaryOperator) string {
	switch op {
	case ast.KOperatorNot:
		return "!"

	case ast.KOperatorAddressOf:
		return "&"

	case ast.KOperatorNegate:
		return "-"
	}

	panic("exhausted cases in convertedUnaryOperator()")
	return ""
}

func (cw *CWriter) CanWriteRequirement(req ast.FunctionLocation) bool {
	if cw.WritingGpu() && req == ast.KLocationCpu {
		return false
	}

	if !cw.WritingGpu() && req == ast.KLocationGpu {
		return false
	}

	return true
}

/*
Write an expression that appears on the rhs of a x=
This is an override point for copying literals to the heap if need be
*/
func (cw *CWriter) WriteAssignedExpression(e ast.Expression) {
	if e.Type().Selector == ast.KTypeString {
		cw.w().AddComponents(
			namespaceUseStringLiteral(), "(",
			namespaceExecutionContext(), ",",
		)
		cw.WriteExpression(e)
		cw.w().AddComponents(")")
	} else {
		cw.WriteExpression(e)
	}
}

func (cw *CWriter) WriteExpression(re ast.Expression) {
	switch e := re.(type) {
	case *ast.NullLiteral:
		cw.w().AddComponentf(`0`)

	case *ast.CastExpression:
		cw.w().AddComponents("(")
		cw.WriteType(e.NewType)
		cw.w().AddComponents(")")
		cw.WriteExpression(e.Casted)

	case *ast.SelfTerminal:
		cw.w().AddComponentf(`ey_self`)

	case *ast.DereferenceExpression:
		cw.w().AddComponent(`*`)
		cw.w().SuppressNextSpace()
		cw.WriteExpression(e.Pointer)

	case *ast.SizeofExpression:
		cw.w().AddComponent(`sizeof`)
		cw.w().AddComponentNoSpace(`(`)
		cw.w().SuppressNextSpace()
		cw.WriteType(e.SizedType)
		cw.w().AddComponentNoSpace(`)`)

	case *ast.IntegerTerminal:
		cw.w().AddComponentf(`%v`, e.Value)

	case *ast.CharacterTerminal:
		cw.w().AddComponents(fmt.Sprintf("%v", e.CodePoint))

	case *ast.StringTerminal:
		cw.w().AddComponents(
			namespaceStringPoolGet(), "(",
			namespaceExecutionContext(), ",",
			fmt.Sprintf("%v", e.Id),
			")",
		)

	case *ast.FloatTerminal:
		zeros := ""
		for i := int64(0); i < e.Zeros; i += 1 {
			zeros += "0"
		}
		cw.w().AddComponentf(`%v.%v%v`, e.LValue, zeros, e.RValue)

	case *ast.TupleExpression:
		ty := e.Type()

		tupid := ty.TupleIdentifier()
		cw.w().AddComponent("(")
		cw.w().AddComponentNoSpace(tupid)
		cw.w().AddComponentNoSpace(")")
		cw.w().AddComponent("{")
		for ei, ee := range e.Expressions {
			if ei > 0 {
				cw.w().AddComponentNoSpace(",")
			}
			cw.w().AddComponentf(".f%v", ei)
			cw.w().AddComponent("=")
			cw.WriteAssignedExpression(ee)
		}
		cw.w().AddComponent("}")

	case *ast.IdentifierTerminal:
		ty := e.Type()
		if e.Fid != nil {
			cw.w().AddComponent(namespaceFunctionId(*e.Fid))
		} else if ty.Selector == ast.KTypeFunction && !e.DontNamespace {
			// TODO this will always panic, so maybe we should just do it
			panic("problem case " + e.Name)
		} else {
			cw.w().AddComponent(e.Name)
		}

	case *ast.BooleanTerminal:
		if e.Value {
			cw.w().AddComponentf("k_true")
		} else {
			cw.w().AddComponentf("k_false")
		}

	case *ast.RangeExpression:
		cw.w().AddComponents("ey_runtime_range", "(", namespaceExecutionContext(), ",")
		cw.WriteExpression(e.Start)
		cw.w().AddComponentNoSpace(",")
		cw.WriteExpression(e.Count)
		cw.w().AddComponentNoSpace(",")
		cw.WriteExpression(e.Step)
		cw.w().AddComponentNoSpace(")")

	case *ast.BinaryExpression:
		// the excess parens are not pretty, but they are precise
		cw.w().AddComponents("(")
		switch e.Lhs.Type().Selector {
		case ast.KTypeString:
			switch e.Operator {
			case ast.KOperatorAdd:
				cw.w().AddComponents("ey_runtime_string_join")

			case ast.KOperatorEquality:
				cw.w().AddComponents("ey_runtime_string_equality")

			case ast.KOperatorInequality:
				cw.w().AddComponents("!", "ey_runtime_string_equality")

			default:
				panic(fmt.Errorf("Do not understand binary operator %v", e.Operator))
			}

			cw.w().AddComponents(
				"(",
				namespaceExecutionContext(), ",",
			)
			cw.WriteExpression(e.Lhs)
			cw.w().AddComponents(",")
			cw.WriteExpression(e.Rhs)
			cw.w().AddComponents(")")

		default:
			cw.WriteExpression(e.Lhs)
			cw.w().AddComponents(
				")",
				convertedBinaryOperator(e.Operator),
				"(",
			)
			cw.WriteExpression(e.Rhs)
		}
		cw.w().AddComponent(")")

	case *ast.AccessExpression:
		cw.WriteExpression(e.Accessed)

		ty := e.Accessed.Type()

		if ty.Selector == ast.KTypePointer || ty.Selector == ast.KTypeWorker {
			cw.w().AddComponentNoSpace("->")
		} else {
			cw.w().AddComponentNoSpace(".")
		}

		cw.w().AddComponentNoSpace(e.Identifier)

	case *ast.NewExpression:
		// New is passthrough
		cw.WriteExpression(e.Replacement)

	case *ast.ClosureExpression:
		cw.w().AddComponents(
			"ey_closure_create", "(",
			namespaceFunctionEnumId(e.CalledFunctionId),
			",",
			e.ArgumentArrayName,
			")",
		)

	case *ast.CallExpression:
		called := e.CalledExpression
		arguments := e.Arguments

		if !e.SkipExecutionContext {
			iargs := []ast.Expression{
				&ast.IdentifierTerminal{Name: namespaceExecutionContext()},
			}
			arguments = append(iargs, arguments...)
		}

		if e.StackedResultVariableName != "" {
			cw.w().AddComponent(e.StackedResultVariableName)
		} else {
			cw.WriteExpression(called)
			cw.w().AddComponentNoSpace("(")
			cw.w().SuppressNextSpace()
			for i, arg := range arguments {
				if i > 0 {
					cw.w().AddComponentNoSpace(",")
				}

				cw.WriteExpression(arg)
			}

			cw.w().AddComponentNoSpace(")")
		}

	case *ast.StructLiteralExpression:
		cw.w().AddComponents(
			"(",
			namespaceStruct(e.Id),
			")",
			"{",
		)
		for pi, pair := range e.Pairs {
			if pi > 0 {
				cw.w().AddComponent(",")
			}

			cw.w().AddComponent("." + pair.FieldName)
			cw.w().AddComponent("=")
			cw.WriteExpression(pair.Value)
		}
		cw.w().AddComponent("}")

	case *ast.UnaryExpression:
		cw.w().AddComponent(convertedUnaryOperator(e.Operator))
		cw.w().SuppressNextSpace()
		cw.WriteExpression(e.Rhs)

	case *ast.VectorLiteralExpression:
		cw.WriteExpression(e.Replacement)

	case *ast.IndexExpression:
		switch e.AccessedType {
		case ast.KTypeVector:
			// indirect
			cw.w().AddComponent("*")

			// case to appropriate pointer
			cw.w().AddComponentNoSpace("(")
			cw.w().SuppressNextSpace()
			cw.WriteType(e.Type())
			cw.w().AddComponentNoSpace("*")
			cw.w().AddComponentNoSpace(")")

			// access
			cw.w().SuppressNextSpace()
			cw.w().AddComponents(
				"ey_vector_access", "(", namespaceExecutionContext(), ",",
			)
			cw.WriteExpression(e.Indexed)
			cw.w().AddComponentNoSpace(",")
			cw.WriteExpression(e.Index)
			cw.w().AddComponentNoSpace(")")

		case ast.KTypeString:
			cw.w().AddComponents(
				"ey_runtime_string_get_character", "(",
				namespaceExecutionContext(), ",",
			)
			cw.WriteExpression(e.Indexed)
			cw.w().AddComponentNoSpace(",")
			cw.WriteExpression(e.Index)
			cw.w().AddComponents(")")
		}

	case *ast.CreatePipelineExpression:
		cw.w().AddComponents("ey_worker_create_pipeline", "(")
		cw.WriteExpression(e.LhsWorker)
		cw.w().AddComponents(",")
		cw.WriteExpression(e.RhsWorker)
		cw.w().AddComponents(")")

	case *ast.CreateWorkerExpression:
		switch e.Destination {
		case ast.KDestinationGpu:
			cw.w().AddComponents(
				"ey_worker_create_opencl", "(",
				`"`+namespaceFunctionId(e.KernelId)+`"`, ",",
				"sizeof(",
			)
			cw.WriteType(e.SendType)
			cw.w().AddComponents(
				")", ",", "sizeof(",
			)
			cw.WriteType(e.ReceiveType)
			cw.w().AddComponents(
				")",
				",",
			)
			if e.ClosureVariable != "" {
				cw.w().AddComponents(
					e.ClosureVariable,
					",",
					"ey_closure_size(",
					e.ClosureVariable,
					")",
				)
			} else {
				cw.w().AddComponents(
					"0",
					",",
					"0",
				)
			}

			cw.w().AddComponentNoSpace(")")

		case ast.KDestinationCpu:
			cw.w().AddComponents(
				"ey_worker_create_cpu", "(",
				// casting this argument is a bit of a hack to make something that only accepts void* to accept something with an execution context as first arg
				"(", namespaceWorkerFunction(), ")",
				namespaceFunctionId(e.WrapperId),
				",", "sizeof", "(",
			)

			cw.WriteType(e.SendType)
			cw.w().AddComponentNoSpace(")")
			cw.w().AddComponentNoSpace(",")

			if e.ReceiveType.Selector == ast.KTypeVoid {
				cw.w().AddComponent("0")
			} else {
				cw.w().AddComponent("sizeof")
				cw.w().AddComponentNoSpace("(")
				cw.WriteType(e.ReceiveType)
				cw.w().AddComponentNoSpace(")")
			}
			if e.ClosureVariable == "" {
				cw.w().AddComponentNoSpace(", 0, 0)")
			} else {
				cw.w().AddComponentNoSpace(fmt.Sprintf(", %v, ey_closure_size(%v))", e.ClosureVariable, e.ClosureVariable))
			}
		}

	case *ast.ReceiveWorkerExpression:
		cw.WriteExpression(e.Received)

	default:
		panic(fmt.Sprintf("WriteExpression: Do not recognise expression %v", re))
	}
}

func (cw *CWriter) WriteLValue(rlv ast.LValue) {
	switch lv := rlv.(type) {
	case *ast.AccessorLValue:
		cw.WriteLValue(lv.Inner)

		ty := lv.Inner.Type()

		if ty.Selector == ast.KTypePointer {
			cw.w().AddComponentNoSpace("->")
		} else {
			cw.w().AddComponentNoSpace(".")
		}

		cw.w().AddComponentNoSpace(lv.FieldName)

	case *ast.IndexLValue:
		it := lv.Indexed.Type()
		if it.Selector == ast.KTypePointer {
			it = it.Types[0]
		}

		switch it.Selector {
		case ast.KTypeVector:
			// indirect
			cw.w().AddComponent("*")

			// cast to appropriate pointer
			cw.w().AddComponentNoSpace("(")
			cw.w().SuppressNextSpace()
			cw.WriteType(lv.Type())
			cw.w().AddComponentNoSpace("*")
			cw.w().AddComponentNoSpace(")")

			// access
			cw.w().SuppressNextSpace()
			cw.w().AddComponents("ey_vector_access", "(", namespaceExecutionContext(), ",")
			cw.w().SuppressNextSpace()
			cw.WriteLValue(lv.Indexed)
			cw.w().AddComponentNoSpace(",")
			cw.WriteExpression(lv.Index)
			cw.w().AddComponentNoSpace(")")

		case ast.KTypeString:
			cw.w().AddComponents(
				"(", "(", "EyCharacter", "*", ")",
			)
			cw.WriteLValue(lv.Indexed)
			cw.w().AddComponents(
				"->", "ptr", ")", "[",
			)
			cw.WriteExpression(lv.Index)
			cw.w().AddComponents("]")
		}

	case *ast.IdentifierLValue:
		cw.w().AddComponent(lv.Name)

	case *ast.DerefLValue:
		cw.w().AddComponent("*")
		cw.w().SuppressNextSpace()
		cw.WriteLValue(lv.Inner)

	case *ast.SelfLValue:
		cw.w().AddComponent("ey_self")

	case *ast.MultipleLValue:
		panic("cannot directly write a multiple lvalue")

	default:
		panic(fmt.Sprintf("WriteLValue: Do not recognise expression %v", rlv))
	}
}

func (cw *CWriter) RememberLValue(ty ast.Type, lv ast.LValue) {
	if cw.WritingGpu() {
		return
	}

	if ty.Selector == ast.KTypePointer {
		cw.w().AddComponents("ey_runtime_gc_remember_root_pointer", "(", "ey_runtime_gc", "(", namespaceExecutionContext(), ")", ",", "&")
		cw.WriteLValue(lv)
		cw.w().AddComponents(")", ";")
		cw.w().EndLine()

		scope := cw.scopes[len(cw.scopes)-1]
		scope.SavedPointers = append(scope.SavedPointers, lv)
	}
}

func (cw *CWriter) WriteAssign(st *ast.AssignStatement) {
	mlv, ok := st.Lhs.(*ast.MultipleLValue)
	if ok {
		// TODO this type of statement rewriting should be done in the mutate phase of the checker now

		// in all cases we need to unpack the struct somewhere
		tempVarName := cw.GetTemporaryName()
		cw.w().AddComponent("const")
		cw.WriteType(st.NewType)
		cw.w().AddComponent(tempVarName)
		cw.w().AddComponent("=")
		cw.WriteAssignedExpression(st.Rhs)
		cw.w().AddComponentNoSpace(";")
		cw.w().EndLine()

		// break the individual assigns into separate statements
		for lvi, lv := range mlv.LValues {
			mty := st.NewType.Types[lvi]

			letType := false
			switch st.Type {
			case ast.KAssignLet, ast.KAssignConst:
				// write the type first to declare the variable
				cw.WriteType(mty)
				letType = true

			case ast.KAssignNormal:
				// nothing, the variable, should exist and not be const
			}

			cw.WriteLValue(lv)
			cw.w().AddComponent("=")
			cw.w().AddComponentf("%v.f%v", tempVarName, lvi)
			cw.w().AddComponentNoSpace(";")
			cw.w().EndLine()

			if letType && st.PinPointers {
				cw.RememberLValue(mty, lv)
			}
		}
	} else {
		letType := false
		switch st.Type {
		case ast.KAssignLet, ast.KAssignConst:
			// write the type first to declare the variable
			cw.WriteType(st.NewType)
			letType = true

		case ast.KAssignNormal:
			// nothing, the variable, should exist and not be const
		}

		cw.WriteLValue(st.Lhs)
		if st.Rhs != nil {
			cw.w().AddComponent("=")
			cw.WriteAssignedExpression(st.Rhs)
		}
		cw.w().AddComponentNoSpace(";")
		cw.w().EndLine()

		if letType && st.PinPointers {
			cw.RememberLValue(st.NewType, st.Lhs)
		}
	}
}

func (cw *CWriter) WriteStatement(rst ast.Statement) {
	switch st := rst.(type) {
	case *ast.ModifyInPlaceStatement:
		cw.WriteLValue(st.Modified)
		switch st.Operator {
		case ast.KModifyPlus:
			cw.w().AddComponent("+=")

		case ast.KModifyMinus:
			cw.w().AddComponent("-=")

		case ast.KModifyTimes:
			cw.w().AddComponent("*=")

		case ast.KModifyDivide:
			cw.w().AddComponent("/=")
		}
		cw.WriteExpression(st.Expression)
		cw.w().AddComponentNoSpace(";")

	case *ast.AssignStatement:
		cw.WriteAssign(st)

	case *ast.BreakStatement:
		cw.w().AddComponents("break", ";")
		cw.w().EndLine()

	case *ast.DummyStatement:
		// do nothing, it is just here to hold source locations

	case *ast.WhileStatement:
		cw.w().AddComponent("while")
		cw.w().AddComponent("(")
		cw.w().SuppressNextSpace()
		cw.WriteExpression(st.Condition)
		cw.w().AddComponentNoSpace(")")
		cw.WriteStatementBlock(st.Block, false)

	case *ast.ForeachStatement:
		switch st.Variant {
		case ast.KForEach:
			vect := cw.GetTemporaryName()
			// temp vector
			cw.w().AddComponent("EyVector")
			cw.w().AddComponent("*")
			cw.w().AddComponentNoSpace(vect)
			cw.w().AddComponent("=")
			cw.WriteExpression(st.Iterable)
			cw.w().AddComponentNoSpace(";")
			cw.w().EndLine()

			// iterator index into vector
			index := cw.GetTemporaryName()
			cw.w().AddComponent("int")
			cw.w().AddComponent(index)
			cw.w().AddComponent("=")
			cw.w().AddComponent("0")
			cw.w().AddComponentNoSpace(";")
			cw.w().EndLine()

			cw.w().AddComponents(
				"while", "(", index, "<",
				"ey_vector_length", "(", namespaceExecutionContext(), ",", vect, ")",
				")", "{",
			)
			cw.w().EndLine()
			cw.w().Indent()

			// iterator
			cw.WriteType(st.IteratedType)
			cw.w().AddComponent(st.TemporaryVariableName)
			cw.w().AddComponent("=")
			cw.w().AddComponent("*")
			cw.w().AddComponentNoSpace("(")
			cw.WriteType(st.IteratedType)
			cw.w().AddComponent("*")
			cw.w().AddComponentNoSpace(")")
			cw.w().AddComponents("ey_vector_access", "(", namespaceExecutionContext(), ",", vect, ",", index, ")")
			cw.w().AddComponentNoSpace(";")
			cw.w().EndLine()

			cw.WriteStatementBlock(st.Body, false)

			cw.w().AddComponent(index)
			cw.w().AddComponent("++")
			cw.w().AddComponent(";")
			cw.w().EndLine()

			cw.w().Unindent()
			cw.w().AddComponent("}")
			cw.w().EndLine()

		case ast.KForRange:
			cw.w().AddComponent("for")
			cw.w().AddComponent("(")

			cw.w().AddComponent("EyInteger")
			cw.w().AddComponent(st.TemporaryVariableName)
			cw.w().AddComponent("=")
			cw.w().AddComponent(st.StartName)
			cw.w().AddComponent(";")

			cw.w().AddComponent("ey_runtime_continue_iterating(")
			cw.w().AddComponent(st.StepName)
			cw.w().AddComponent(",")
			cw.w().AddComponent(st.TemporaryVariableName)
			cw.w().AddComponent(",")
			cw.w().AddComponent(st.EndName)
			cw.w().AddComponent(");")

			cw.w().AddComponent(st.TemporaryVariableName)
			cw.w().AddComponent("+=")
			cw.w().AddComponent(st.StepName)
			cw.w().AddComponent(")")
			cw.w().AddComponent("{")
			cw.w().EndLine()

			cw.WriteStatementBlock(st.Body, false)
			cw.w().Indent()

			cw.w().Unindent()
			cw.w().AddComponent("}")
			cw.w().EndLine()
		}

	case *ast.ClosureArgDeclarationStatement:
		cw.w().AddComponents(
			"void",
			"*",
			st.Name,
			"[",
			"]",
			"=",
			"{",
		)

		for argi, arg := range st.Args {
			if argi > 0 {
				cw.w().AddComponent(",")
			}

			if arg == "" {
				cw.w().AddComponent("0")
			} else {
				if st.AddressOf {
					cw.w().AddComponents("&", arg)
				} else {
					cw.w().AddComponents(arg)
				}
			}
		}

		cw.w().AddComponents(
			"}",
			";",
		)

	case *ast.ReturnStatement:
		cw.AboutToExitAllScopes()
		cw.w().AddComponent("return")
		if st.ReturnedValue != nil {
			cw.WriteExpression(st.ReturnedValue)
		}
		cw.w().AddComponentNoSpace(";")

	case *ast.ExpressionStatement:
		cw.WriteExpression(st.Expression)
		cw.w().AddComponentNoSpace(";")

	case *ast.SendPipeStatement:
		cw.WriteExpression(st.Pipe)
		cw.w().AddComponentNoSpace("->")
		cw.w().AddComponentNoSpace("send")
		cw.w().AddComponentNoSpace("(")
		cw.w().SuppressNextSpace()
		cw.WriteExpression(st.Pipe)
		cw.w().AddComponentNoSpace(",")
		cw.WriteExpression(st.Value)
		cw.w().SuppressNextSpace()
		cw.w().AddComponentNoSpace(")")
		cw.w().AddComponentNoSpace(";")

	case *ast.IfStatement:
		for segi, seg := range st.Segments {
			if seg.Condition == nil {
				cw.w().AddComponent("else")
				cw.WriteStatementBlock(seg.Block, false)
			} else {
				ty := seg.Condition.Type()

				// checked in verification too
				if ty.Selector != ast.KTypeBoolean {
					panic(fmt.Sprint("TODO if condition not of boolean type ", seg.Condition))
				}

				if segi > 0 {
					cw.w().AddComponent("else")
					cw.w().AddComponent("if")
					cw.w().AddComponent("(")
					cw.WriteExpression(seg.Condition)
					cw.w().AddComponentNoSpace(")")
				} else {
					cw.w().AddComponent("if")
					cw.w().AddComponent("(")
					cw.WriteExpression(seg.Condition)
					cw.w().AddComponentNoSpace(")")
				}

				cw.WriteStatementBlock(seg.Block, false)
			}
		}

	default:
		panic(fmt.Sprintf("WriteStatement: Do not recognise statement %v", rst))
	}
}

func (cw *CWriter) WriteStatementBlock(ss *ast.StatementBlock, withGcScope bool) {
	cw.w().AddComponent("{")
	cw.w().EndLine()
	cw.w().Indent()

	cw.StartScope()

	for _, sc := range ss.Statements {
		cw.WriteStatement(sc.Statement)
		cw.w().EndLine()
	}

	cw.AboutToExitThisScope()
	cw.EndScope()

	cw.w().Unindent()
	cw.w().AddComponent("}")
	cw.w().EndLine()
}

func (cw *CWriter) WriteType(ty ast.Type) {
	switch ty.Selector {
	case ast.KTypeNull:
		panic("CWriter.WriteType: should never be asked to write null")

	case ast.KTypeInteger:
		cw.w().AddComponent("EyInteger")

	case ast.KTypeFloat:
		cw.w().AddComponent(fmt.Sprintf("EyFloat%v", ty.Width))

	case ast.KTypeString:
		cw.w().AddComponent("EyString")

	case ast.KTypeCharacter:
		cw.w().AddComponent("EyCharacter")

	case ast.KTypeBoolean:
		cw.w().AddComponent("EyBoolean")

	case ast.KTypeVoid:
		cw.w().AddComponent("void")

	case ast.KTypeTuple:
		cw.w().AddComponent(ty.TupleIdentifier())

	case ast.KTypeStruct:
		cw.w().AddComponent(namespaceStruct(ty.StructId))

	case ast.KTypePointer:
		cw.WriteType(ty.Types[0])
		cw.w().AddComponentNoSpace("*")

	case ast.KTypeVector:
		cw.w().AddComponent("EyVector")

	case ast.KTypeWorker:
		cw.w().AddComponent("EyWorker")
		cw.w().AddComponentNoSpace("*")

	case ast.KTypeFunction:
		panic("Cannot write function types right now")

	case ast.KTypeClosure:
		cw.w().AddComponent("EyClosure")

	default:
		panic(fmt.Sprintf("exhausted cases in Type.Write %v", ty.String()))
	}
}

func (cw *CWriter) WriteTopLevelElement(rtle ast.TopLevelElement, pool []string) {
	switch tle := rtle.(type) {
	case *ast.StructDefinitionStatement:
		// this is handled elsewhere

	case *ast.DummyTle:
		// this is an internal marker

	case *ast.ImportElement:
		// nothing to do at this level

	case *ast.FunctionDefinitionTle:
		cw.WriteFunction(tle.Definition)

	case *ast.ConstTle:
		cw.WriteAssign(tle.Assign)

	case *ast.GpuKernelTle:
		if !cw.WritingGpu() {
			return
		}

		const closureBufferName string = "closure_block"
		const stackClosureVarName string = "closure"
		cw.w().AddComponents(
			"__kernel",
			"void",
			namespaceFunctionId(tle.KernelId),
			"(",

			// arg 0
			"__global",
		)
		cw.WriteType(tle.Input)
		cw.w().AddComponents(
			"*",
			"global_input",
			",",

			// arg 1
			"__global",
		)
		cw.WriteType(tle.Output)
		cw.w().AddComponents(
			"*", "global_output", ",",

			// arg 2
			"const unsigned int count",
			",",

			// arg 3
			"__global", "EyWorkerShared", "*", "shared",
		)

		// arg4
		if tle.IsClosureWorker {
			cw.w().AddComponents(
				",",

				"__global", "void", "*", "raw_closure",
			)
		}

		cw.w().AddComponents(")", "{")
		cw.w().EndLine()

		cw.w().Indent()

		cw.WriteStringPool(pool)
		cw.w().EndLine()

		cw.w().AddComponents("int", "i", "=", "get_global_id", "(", "0", ")", ";")
		cw.w().EndLine()

		cw.w().AddComponents("EyExecutionContext", namespaceExecutionContext(), "=", "{")
		cw.w().EndLine()
		cw.w().Indent()
		cw.w().AddComponents(".shared", "=", "shared", "+", "get_local_id", "(", "0", ")", ",")
		cw.w().EndLine()
		cw.w().AddComponents(".strings", "=", namespaceStringPoolName(), ",")
		cw.w().EndLine()
		cw.w().Unindent()
		cw.w().AddComponents("}", ";")
		cw.w().EndLine()

		cw.w().AddComponent("if (i < count) {")
		cw.w().EndLine()
		cw.w().Indent()

		if !tle.IsClosureWorker {
			if tle.Output.Selector != ast.KTypeVoid {
				cw.w().AddComponents(
					"global_output[i]",
					"=",
				)
			}

			cw.w().AddComponents(
				namespaceFunctionId(tle.WorkerId),
				"(",
				"&", namespaceExecutionContext(), ",",
				"global_input[i]",
				")",
				";",
			)
			cw.w().EndLine()
		} else {
			cw.w().AddComponents("unsigned", "char", "closure_buffer", "[", "EYOT_RUNTIME_MAX_CLOSURE_SIZE", "]", ";")
			cw.w().EndLine()

			cw.w().AddComponents("ey_runtime_closure_copy", "(", "closure_buffer", ",", "raw_closure", ")", ";")
			cw.w().EndLine()

			cw.WriteType(tle.Input)
			cw.w().AddComponents("input", "=", "global_input", "[", "i", "]", ";")
			cw.w().EndLine()

			if tle.Output.Selector != ast.KTypeVoid {
				cw.WriteType(tle.Output)
				cw.w().AddComponents("output", ";")
				cw.w().EndLine()
			}

			cw.w().AddComponents(
				"void", "*", "args", "[", "]",
				"=",
				"{", "&", "input", "}",
				";",
			)
			cw.w().EndLine()

			cw.w().AddComponents(
				"ey_closure_call", "(",
				"&", namespaceExecutionContext(), ",",
				"(", "EyClosure", ")", "closure_buffer", ",",
			)
			if tle.Output.Selector == ast.KTypeVoid {
				cw.w().AddComponents("0", ",")
			} else {
				cw.w().AddComponents("&", "output", ",")
			}
			cw.w().AddComponents(
				"args", ")",
				";",
			)
			cw.w().EndLine()

			if tle.Output.Selector != ast.KTypeVoid {
				cw.w().AddComponents("global_output", "[", "i", "]", "=", "output", ";")
				cw.w().EndLine()
			}
		}

		cw.w().Unindent()
		cw.w().AddComponent("}")
		cw.w().EndLine()

		cw.w().Unindent()
		cw.w().AddComponent("}")
		cw.w().EndLine()

	default:
		panic(fmt.Sprintf("WriteTopLevelElement: Do not recognise tle %v", rtle))
	}
}

func (cw *CWriter) ExecutionContextType() ast.Type {
	sid := ast.StructId{
		Module: ast.BuiltinModuleId(),
		Name:   "EyExecutionContext",
	}
	return ast.MakePointer(ast.Type{Selector: ast.KTypeStruct, StructId: sid})
}

func (cw *CWriter) WriteFunctionPrototypeRawName(sig ast.FunctionSignature, rawName string) {
	cw.WriteType(sig.Return)
	cw.w().ForceSpace()
	cw.w().AddComponent(rawName)
	cw.w().AddComponentNoSpace("(")
	cw.w().SuppressNextSpace()

	tys := append([]ast.Type{cw.ExecutionContextType()}, sig.Types...)

	for pi, ty := range tys {
		if pi > 0 {
			cw.w().AddComponentNoSpace(",")
		}

		cw.WriteType(ty)
	}

	cw.w().AddComponentNoSpace(")")
}

func (cw *CWriter) WriteFunctionPrototype(sig ast.FunctionSignature, fid ast.FunctionId) {
	cw.WriteFunctionPrototypeRawName(sig, namespaceFunctionId(fid))
}

func (cw *CWriter) WriteFunction(fd *ast.FunctionDefinition) {
	if !cw.CanWriteRequirement(fd.Location) {
		return
	}

	cw.WriteType(fd.Return)
	cw.w().ForceSpace()
	cw.w().AddComponent(namespaceFunctionId(fd.Id))

	cw.w().AddComponentNoSpace("(")
	cw.w().SuppressNextSpace()

	ecParameter := ast.FunctionParameter{
		Name: namespaceExecutionContext(),
		Type: cw.ExecutionContextType(),
	}

	for pi, param := range fd.EffectiveParameters(ecParameter) {
		if pi > 0 {
			cw.w().AddComponentNoSpace(",")
		}

		cw.WriteType(param.Type)
		cw.w().AddComponent(param.Name)
	}

	cw.w().AddComponentNoSpace(")")
	cw.WriteStatementBlock(fd.Block, true)

	// wrapper
	sig := fd.Signature()

	cw.w().AddComponents(
		"void",
		namespaceFunctionCallerId(fd.Id),
		"(",
	)
	cw.w().AddComponents("EyExecutionContext", "*", namespaceExecutionContext(), ",")
	cw.w().AddComponents(
		"void", "*", "result", ",",
		"void", "*", "*", "args",
		")",
		"{",
	)
	cw.w().EndLine()
	cw.w().Indent()

	if sig.Return.Selector != ast.KTypeVoid {
		cw.w().AddComponents("*", "(")
		cw.WriteType(fd.Return)
		cw.w().AddComponents("*", ")", "result", "=")
	}

	cw.w().AddComponents(
		namespaceFunctionId(fd.Id),
		"(",
	)
	cw.w().AddComponents(namespaceExecutionContext())

	for argi, argt := range sig.Types {
		cw.w().AddComponents(",")

		cw.w().AddComponents("*", "(", "(")

		cw.WriteType(argt)
		cw.w().AddComponents("*", ")", "args", "[", fmt.Sprintf("%v", argi), "]", ")")
	}
	cw.w().AddComponents(
		")",
		";",
	)
	cw.w().EndLine()

	cw.w().Unindent()
	cw.w().AddComponents(
		"}",
	)
	cw.w().EndLine()
}

func (cw *CWriter) WriteFile(f *ast.Module, consts bool, pool []string) {
	for _, tle := range f.TopLevelElements {
		_, isConst := tle.TopLevelElement.(*ast.ConstTle)
		if isConst != consts {
			continue
		}

		cw.WriteTopLevelElement(tle.TopLevelElement, pool)
		cw.w().EndLine()
	}
}

func (cw *CWriter) PushWriter(w *textwriter.W) {
	cw.writers = append(cw.writers, w)
}

func (cw *CWriter) PopWriter() {
	cw.writers = cw.writers[:len(cw.writers)-1]
}

/*
Return true when we are writing gpu code
*/
func (cw *CWriter) WritingGpu() bool {
	return cw.writingGpu
}

func escapeString(s string) string {
	rs := []rune{}

	lines := strings.SplitN(s, "\n", -1)
	for _, line := range lines {
		rs = append(rs, ' ', ' ', '"')
		for _, r := range line {
			switch r {
			case '\n':
				panic("Shouldn't happen")

			case '\\':
				rs = append(rs, '\\', '\\')

			case '"':
				rs = append(rs, '\\', '"')

			default:
				rs = append(rs, r)
			}
		}
		rs = append(rs, '\\', 'n', '"', '\n')
	}

	return string(rs)
}

func (cw *CWriter) WriteMain(p *program.Program) {
	cw.w().AddComponents("void", "ey_generated_main", "(", "EyExecutionContext", "*", "ctx", ")", "{")
	cw.w().EndLine()
	cw.w().Indent()

	mainFid := ast.FunctionId{
		Module: p.RootModuleId,
		Struct: ast.BlankStructId(),
		Name:   "main",
	}
	cw.w().AddComponents(namespaceFunctionId(mainFid), "(", "ctx", ")", ";")
	cw.w().EndLine()

	cw.w().Unindent()
	cw.w().AddComponents("}")
	cw.w().EndLine()
}

func (cw *CWriter) WriteProgram(p *program.Program) {
	// calculate the ocl appropriate variant
	cw.w().AddComponent("const")
	cw.w().AddComponent("char")
	cw.w().AddComponent("*")
	cw.w().AddComponent("ey_runtime_cl_src")
	cw.w().AddComponent("=")

	if p.GpuRequired {
		buf := bytes.NewBuffer([]byte{})
		oclWriter := textwriter.NewWriter(buf)
		cw.writingGpu = true
		cw.PushWriter(oclWriter)
		cw.writeProgram(p)
		cw.PopWriter()
		cw.writingGpu = false

		cw.w().EndLine()
		cw.w().WriteRaw(escapeString(buf.String()))
	} else {
		cw.w().AddComponent("0")
	}

	cw.w().AddComponent(";")
	cw.w().EndLine()

	cw.writeProgram(p)
}

func (cw *CWriter) WriteArgCountFunction(p *program.Program) {
	cw.w().AddComponents(
		"int",
		"ey_generated_arg_count",
		"(",
		"int",
		"fid",
		")",
		"{",
	)
	cw.w().EndLine()
	cw.w().Indent()

	cw.w().AddComponents(
		"switch",
		"(",
		"(", namespaceEnumFunctionListNew(), ")",
		"fid",
		")",
		"{",
	)
	cw.w().EndLine()

	for _, fs := range p.Functions.Functions {
		for loc, ids := range fs.AllIds {
			if !cw.CanWriteRequirement(loc) {
				continue
			}
			for _, fid := range ids {
				cw.w().AddComponents(
					"case",
					namespaceFunctionEnumId(fid),
					":",
				)
				cw.w().EndLine()
				cw.w().Indent()
				
				cw.w().AddComponents(
					"return",
					fmt.Sprintf("%v", len(fs.Signature.Types)),
					";",
				)
				cw.w().EndLine()
				
				cw.w().Unindent()
			}
		}
	}

	// end switch
	cw.w().AddComponents(
		"}",
	)
	cw.w().EndLine()

	// end fn
	cw.w().Unindent()
	cw.w().AddComponents(
		"}",
	)
	cw.w().EndLine()
}

func (cw *CWriter) WriteStringPool(pool []string) {
	cw.w().AddComponent("// String pool")
	cw.w().EndLine()

	if len(pool) == 0 {
		/*
		   Open CL is not happy with empty arrays
		   A null pouinter is "safe" as it would crash to use it anyhow
		*/
		cw.w().AddComponents("EyStringS", "*", namespaceStringPoolName(), "=", "0", ";")
		return
	}

	counts := []int{}
	for si, s := range pool {
		cw.w().AddComponents(
			"EyCharacter", namespaceStringPoolStringUtf32(si), "[", "]", "=", "{",
		)

		l := 0

		// NB golang uses utf8 as default
		for _, c := range s {
			cw.w().AddComponents(
				fmt.Sprintf("%v", int(c)),
				",",
			)
			l += 1
		}

		counts = append(counts, l)

		cw.w().AddComponents("0", "}", ";")
		cw.w().EndLine()
	}

	tempName := namespaceStringPoolName() + "_temp"

	cw.w().AddComponents("EyStringS", tempName, "[", "]", "=", "{")
	cw.w().EndLine()

	cw.w().Indent()
	for si, _ := range pool {
		cw.w().AddComponents(
			"{",
			// NB data length, so 4 * usv count
			".length", "=", fmt.Sprintf("%v", 4*counts[si]), ",",
			".ptr", "=", namespaceStringPoolStringUtf32(si), ",",
			".static_lifetime", "=", "k_true",
			"}", ",",
		)
		cw.w().EndLine()
	}
	cw.w().Unindent()

	cw.w().AddComponents("}", ";")
	cw.w().EndLine()

	cw.w().AddComponents("EyStringS", "*", namespaceStringPoolName(), "=", tempName, ";")
	cw.w().EndLine()
}

func (cw *CWriter) WriteNewFunctionEnum(p *program.Program) {
	cw.w().AddComponents("typedef", "enum", "{")
	cw.w().EndLine()
	cw.w().Indent()

	for _, fe := range p.Functions.FunctionEntries() {
		if cw.CanWriteRequirement(fe.Location) {
			cw.w().AddComponents(
				namespaceFunctionEnumId(fe.Fid),
				"=", fmt.Sprintf("%v", fe.Id),
				",",
			)
			cw.w().EndLine()
		}
	}

	cw.w().Unindent()
	cw.w().AddComponents(
		"}",
		namespaceEnumFunctionListNew(),
		"/* should be unuique */",
		";",
	)
	cw.w().EndLine()

	cw.w().EndLine()
}

func (cw *CWriter) WriteFunctionArgSize(p *program.Program) {
	cw.w().AddComponents(
		"int",
		namespaceClosureArgSize(),
		"(",
		"int", "fid", ",",
		"int", "arg",
		")", "{",
	)
	cw.w().EndLine()
	cw.w().Indent()

	cw.w().AddComponents(
		"switch",
		"(",
		"(", namespaceEnumFunctionListNew(), ")",
		"fid",
		")",
		"{",
	)
	cw.w().EndLine()
	cw.w().Indent()

	// cases
	for _, fs := range p.Functions.Functions {
		if len(fs.Signature.Types) == 0 {
			continue
		}

		for loc, ids := range fs.AllIds {
			if !cw.CanWriteRequirement(loc) {
				continue
			}
			for _, fid := range ids {
				cw.w().AddComponents(
					"case",
					namespaceFunctionEnumId(fid),
					":",
				)
				cw.w().EndLine()
				cw.w().Indent()

				if cw.CanWriteRequirement(fs.Signature.Location) {
					cw.w().AddComponents(
						"switch",
						"(",
						"arg",
						")",
						"{",
					)
					cw.w().EndLine()
					cw.w().Indent()

					for tyi, ty := range fs.Signature.Types {
						cw.w().AddComponents(
							"case",
							fmt.Sprintf("%v", tyi),
							":",
						)
						cw.w().EndLine()
						cw.w().Indent()

						cw.w().AddComponents(
							"return", "sizeof", "(",
						)
						cw.WriteType(ty)
						cw.w().AddComponents(
							")", ";",
						)
						cw.w().EndLine()
						cw.w().Unindent()
					}

					cw.w().Unindent()
					cw.w().AddComponents(
						"}",
					)
					cw.w().EndLine()
				} else {
					cw.w().AddComponents(
						"return", "0", ";", "// not available on gpu",
					)
				}
				cw.w().EndLine()
				cw.w().Unindent()
			}
		}
	}

	// handle functions with no args
	cw.w().AddComponents(
		"default",
		":",
	)
	cw.w().EndLine()
	cw.w().Indent()
	cw.w().AddComponents(
		"return", "0", ";",
	)
	cw.w().EndLine()
	cw.w().Unindent()

	cw.w().Unindent()
	cw.w().AddComponents(
		"}",
	)
	cw.w().EndLine()

	cw.w().Unindent()
	cw.w().AddComponents(
		"}",
	)
	cw.w().EndLine()
}

func (cw *CWriter) WriteFunctionCaller(p *program.Program) {
	cw.w().AddComponents(
		"void",
		namespaceCentralFunctionCaller(),
		"(",
	)
	cw.w().AddComponents("EyExecutionContext", "*", namespaceExecutionContext(), ",")
	cw.w().AddComponents(
		"int", "fid", ",",
		"void", "*", "result", ",",
		"void", "*", "*", "args",
		")", "{",
	)
	cw.w().EndLine()
	cw.w().Indent()

	cw.w().AddComponents(
		"switch",
		"(",
		"(", namespaceEnumFunctionListNew(), ")",
		"fid",
		")",
		"{",
	)
	cw.w().EndLine()
	cw.w().Indent()

	// cases
	for _, fs := range p.Functions.Functions {
		if len(fs.Signature.Types) == 0 {
			continue
		}

		for loc, ids := range fs.AllIds {
			if !cw.CanWriteRequirement(loc) {
				continue
			}
			for _, fid := range ids {
				cw.w().AddComponents(
					"case",
					namespaceFunctionEnumId(fid),
					":",
				)
				cw.w().EndLine()
				cw.w().Indent()

				if cw.CanWriteRequirement(fs.Signature.Location) {
					cw.w().AddComponents(
						namespaceFunctionCallerId(fid),
						"(",
					)
					cw.w().AddComponents(
						namespaceExecutionContext(), ",",
						"result", ",",
						"args",
						")",
						";",
					)
				} else {
					// this keeps the cl compiler happy
					cw.w().AddComponent("// function not available on GPU")
				}
				cw.w().EndLine()

				cw.w().AddComponents(
					"break",
					";",
				)
				cw.w().EndLine()
				cw.w().Unindent()
			}
		}
	}

	cw.w().Unindent()
	cw.w().AddComponents(
		"}",
	)
	cw.w().EndLine()

	cw.w().Unindent()
	cw.w().AddComponents(
		"}",
	)
	cw.w().EndLine()
}

// Ideally there wouldn't be CRLFs in there, but occasionally they slip in and GCC doesn't like it
func cleanupCode(code string) string {
	return strings.ReplaceAll(code, "\r", "")
}

func (cw *CWriter) writeProgram(p *program.Program) {
	if cw.WritingGpu() {
		cw.w().AddComponents(
			"#define",
			"EYOT_RUNTIME_MAX_ARGS",
			fmt.Sprintf("%v", p.Functions.MaxArgCount()),
		)
		cw.w().EndLine()

		cw.w().AddComponents(
			"#define",
			"EYOT_RUNTIME_MAX_CLOSURE_SIZE",
			fmt.Sprintf("%v", p.MaximumClosureSize),
		)
		cw.w().EndLine()

		cw.w().AddComponents(
			"#define",
			"EYOT_RUNTIME_GPU",
		)
		cw.w().EndLine()

		crhPath := filepath.Join(p.Env.RuntimeRoot(), "eyot-runtime-common.h")
		blob, err := os.ReadFile(crhPath)
		if err != nil {
			panic("Unable to read the common runtime header")
		}

		// not sure if, how (or should) we include files with cl
		cw.w().WriteRaw(string(blob))
	} else {
		cw.w().WriteRaw(`#include "eyot-runtime-cpu.h"`)
	}
	cw.w().EndLine()
	cw.w().EndLine()

	if !cw.WritingGpu() {
		cw.WriteStringPool(p.GetStringPool())
		cw.w().EndLine()
	}

	cw.WriteNewFunctionEnum(p)
	cw.WriteArgCountFunction(p)
	cw.w().EndLine()

	cw.w().AddComponent("// Forward struct definitions")
	cw.w().EndLine()
	for _, m := range p.Modules {
		for si, s := range m.Structs {
			if si > 0 {
				cw.w().EndLine()
			}
			cw.w().AddComponents("typedef", "struct", namespaceStruct(s.Id), namespaceStruct(s.Id), ";")
			cw.w().EndLine()
		}
	}

	cw.w().AddComponent("// Struct definitions")
	cw.w().EndLine()
	for _, m := range p.Modules {
		for si, s := range m.Structs {
			namespaced := s.Id.Name
			if !s.GeneratedForTuple {
				namespaced = namespaceStruct(s.Id)
			}

			if si > 0 {
				cw.w().EndLine()
			}
			cw.w().AddComponents(
				"typedef",
				"struct",
				namespaced,
				"{",
			)
			cw.w().EndLine()

			cw.w().Indent()
			for _, field := range s.Definition.Fields {
				cw.WriteType(field.Type)
				cw.w().AddComponents(
					field.Name,
					";",
				)
				cw.w().EndLine()
			}

			cw.w().Unindent()
			cw.w().AddComponents(
				"}",
				namespaced,
				";",
			)
			cw.w().EndLine()
		}
	}
	cw.w().EndLine()

	cw.w().AddComponent("// Forward decls for all functions")
	cw.w().EndLine()
	for _, fs := range p.Functions.Functions {
		if len(fs.Signature.Types) == 0 {
			continue
		}

		for loc, ids := range fs.AllIds {
			if !cw.CanWriteRequirement(loc) {
				continue
			}
			for _, fid := range ids {
				cw.WriteFunctionPrototype(fs.Signature, fid)
				cw.w().AddComponentNoSpace(";")
				cw.w().EndLine()
			}
		}
	}
	cw.w().EndLine()
	cw.w().AddComponent("// Forward decls for ffi")
	cw.w().EndLine()
	for _, m := range p.Modules {
		if m.Ffid == nil {
			continue
		}

		for _, cfn := range m.Ffid.Functions {
			sig := ast.FunctionSignature{
				Location: ast.KLocationCpu,
				Return:   cfn.ReturnType,
				Types:    cfn.ArgumentTypes,
			}

			cw.WriteFunctionPrototypeRawName(sig, cfn.Name)
			cw.w().AddComponent(";")
			cw.w().EndLine()
		}
	}

	pool := p.GetStringPool()

	cw.w().AddComponent("// Consts")
	cw.w().EndLine()
	for _, m := range p.Modules {
		cw.WriteFile(m, true, pool)
	}

	cw.w().AddComponent("// Struct functions")
	cw.w().EndLine()
	for _, m := range p.Modules {
		for _, s := range m.Structs {
			for _, fn := range s.Definition.Functions {
				cw.WriteFunction(fn)
				cw.w().EndLine()
			}
		}
	}
	cw.w().EndLine()

	cw.w().AddComponent("// Non-struct code")
	cw.w().EndLine()
	for _, m := range p.Modules {
		cw.WriteFile(m, false, pool)
	}

	cw.w().AddComponent("// Function shims")
	cw.w().EndLine()
	cw.WriteFunctionArgSize(p)
	cw.WriteFunctionCaller(p)

	if !cw.WritingGpu() {
		cw.w().AddComponent("// Main function")
		cw.w().EndLine()
		cw.WriteMain(p)
	}
}
