package token

import (
	"bytes"
	"fmt"
)

type TokenType int

const (
	Integer = iota
	Float32
	Float64
	Identifier
	String
	Character
	Eof

	// one char tokens
	Equals
	Semicolon
	Comma
	OpenCurly
	CloseCurly
	OpenCurved
	CloseCurved
	OpenSquare
	CloseSquare
	Plus
	Minus
	Multiply
	Divide
	LessThan
	GreaterThan
	Colon
	Dot
	Percent

	// two char tokens
	Equality
	Inequality
	LessThanOrEqual
	GreaterThanOrEqual
	PlusEquals
	MinusEquals
	TimesEquals
	DivideEquals
	ScopeResolution

	// keyword tokens (technically some two char tokens in here, but grouping as they are words)
	Null
	Function
	GpuBuiltin
	Break
	Pipeline
	As
	Range
	True
	False
	Let
	Const
	IntegerKeyword
	Float32Keyword
	Float64Keyword
	BoolKeyword
	CharKeyword
	StringKeyword
	Struct
	Return
	If
	Else
	ElseIf
	While
	And
	Or
	Not
	Self
	New
	Send
	Receive
	Drain
	Foreach
	Cpu
	Gpu
	Worker
	Partial
	Placeholder
	Import
)

type Token struct {
	Type TokenType
	Tval string

	// the integral val
	Ival int64

	// the leading zeros on the floating val
	FvalZeros int64

	// the floating val
	Fval int64

	Line int
}

func (t Token) String() string {
	buf := bytes.NewBuffer([]byte{})

	switch t.Type {
	case Integer:
		fmt.Fprintf(buf, "Integer(%v)", t.Ival)

	case Partial:
		fmt.Fprintf(buf, "Partial")

	case Placeholder:
		fmt.Fprintf(buf, "Placeholder")

	case Float32:
		fmt.Fprintf(buf, "Float32(%v.%v)", t.Ival, t.Fval)

	case Float64:
		fmt.Fprintf(buf, "Float64(%v.%v)", t.Ival, t.Fval)

	case Identifier:
		fmt.Fprintf(buf, "Identifier(%v)", t.Tval)

	case Character:
		fmt.Fprintf(buf, "Character(%v)", t.Ival)

	case String:
		fmt.Fprintf(buf, "String(%v)", t.Tval)

	case Pipeline:
		fmt.Fprintf(buf, "Pipeline")

	case Eof:
		fmt.Fprintf(buf, "Eof")

	case Struct:
		fmt.Fprintf(buf, "Struct")

	case Return:
		fmt.Fprintf(buf, "Return")

	case Semicolon:
		fmt.Fprintf(buf, "Semicolon")

	case Const:
		fmt.Fprintf(buf, "Const")

	case Let:
		fmt.Fprintf(buf, "Let")

	case Equals:
		fmt.Fprintf(buf, "Equals")

	case Comma:
		fmt.Fprintf(buf, "Comma")

	case OpenCurly:
		fmt.Fprintf(buf, "OpenCurly")

	case CloseCurly:
		fmt.Fprintf(buf, "CloseCurly")

	case OpenCurved:
		fmt.Fprintf(buf, "OpenCurved")

	case CloseCurved:
		fmt.Fprintf(buf, "CloseCurved")

	case As:
		fmt.Fprintf(buf, "As")

	case GpuBuiltin:
		fmt.Fprintf(buf, "GpuBuiltin")

	case Function:
		fmt.Fprintf(buf, "Function")

	case Null:
		fmt.Fprintf(buf, "Null")

	case Range:
		fmt.Fprintf(buf, "Range")

	case Equality:
		fmt.Fprintf(buf, "Equality")

	case Inequality:
		fmt.Fprintf(buf, "Inequality")

	case LessThan:
		fmt.Fprintf(buf, "LessThan")

	case GreaterThan:
		fmt.Fprintf(buf, "GreaterThan")

	case LessThanOrEqual:
		fmt.Fprintf(buf, "LessThanOrEqual")

	case GreaterThanOrEqual:
		fmt.Fprintf(buf, "GreaterThanOrEqual")

	case PlusEquals:
		fmt.Fprintf(buf, "PlusEquals")

	case MinusEquals:
		fmt.Fprintf(buf, "MinusEquals")

	case TimesEquals:
		fmt.Fprintf(buf, "TimesEquals")

	case DivideEquals:
		fmt.Fprintf(buf, "DivideEquals")

	case ScopeResolution:
		fmt.Fprintf(buf, "ScopeResolution")

	case True:
		fmt.Fprintf(buf, "True")

	case False:
		fmt.Fprintf(buf, "False")

	case Send:
		fmt.Fprintf(buf, "Send")

	case Receive:
		fmt.Fprintf(buf, "Receive")

	case Cpu:
		fmt.Fprintf(buf, "Cpu")

	case Gpu:
		fmt.Fprintf(buf, "Gpu")

	case Worker:
		fmt.Fprintf(buf, "Worker")

	default:
		fmt.Fprintf(buf, "Unknown(%v)", t.Type)
	}

	fmt.Fprintf(buf, "[%v]", t.Line)

	return buf.String()
}
