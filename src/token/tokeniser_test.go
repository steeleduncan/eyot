package token

import (
	"testing"
)

/*
   Untested

   - line numbers in errors
*/

func TestTokenise(t *testing.T) {
	src := `fn main() {
	print_ln("Hello World!");
} == != <= >= a.b`

	tts := []TokenType{
		Function,
		Identifier,
		OpenCurved,
		CloseCurved,
		OpenCurly,
		Identifier,
		OpenCurved,
		String,
		CloseCurved,
		Semicolon,
		CloseCurly,
		Equality,
		Inequality,
		LessThanOrEqual,
		GreaterThanOrEqual,
		Identifier,
		Dot,
		Identifier,
		Eof,
	}

	tkns, err := Tokenise(src)
	if err != nil {
		t.Fatalf("Tokenise failed with error: %v", err)
	}

	if len(tkns) != len(tts) {
		t.Fatalf("Tokenise returned the wrong number of tokens: %v", tkns)
	}

	for i, _ := range tkns {
		if tkns[i].Type != tts[i] {
			t.Fatalf("Token %v of wrong type: %v", i, tkns)
		}
	}
}

func TestInsertSemicolons(t *testing.T) {
	src := `fn main() {
		print_ln("Hello World!")
	}
`

	tts := []TokenType{
		Function,
		Identifier,
		OpenCurved,
		CloseCurved,
		OpenCurly,
		Identifier,
		OpenCurved,
		String,
		CloseCurved,
		Semicolon,
		CloseCurly,
		Semicolon,
		Eof,
	}

	tkns, err := Tokenise(src)
	if err != nil {
		t.Fatalf("Tokenise failed with error: %v", err)
	}

	if len(tkns) != len(tts) {
		t.Fatalf("Tokenise returned the wrong number of tokens: %v", tkns)
	}

	for i, _ := range tkns {
		if tkns[i].Type != tts[i] {
			t.Fatalf("Token %v of wrong type: %v", i, tkns)
		}
	}
}

func TestTokeniseIntegers(t *testing.T) {
	for _, src := range []string{"12 345", "12 // hello \n345", "12 /* junk \n */ 345"} {
		rawTkns, err := Tokenise(src)
		if err != nil {
			t.Fatalf("Tokenise failed with error %v on '%v'", err, src)
		}

		tkns := []Token{}
		for _, t := range rawTkns {
			if t.Type != Semicolon {
				tkns = append(tkns, t)
			}
		}

		if len(tkns) != 3 {
			t.Fatalf("Tokenise returned the wrong number of tokens: %v", tkns)
		}

		t1, t2 := tkns[0], tkns[1]
		if t1.Type != Integer {
			t.Fatalf("wrong type for t1")
		}
		if t2.Type != Integer {
			t.Fatalf("wrong type for t2")
		}
		if tkns[2].Type != Eof {
			t.Fatalf("wrong type for t3")
		}
		if t1.Ival != 12 {
			t.Fatalf("wrong value for t1: %v", t1.Ival)
		}
		if t2.Ival != 345 {
			t.Fatalf("wrong value for t2: %v", t2.Ival)
		}
	}
}

func TestTokeniseFloats(t *testing.T) {
	for srci, src := range []string{"1.0 23.45", "1f 23.45f"} {
		tkns, err := Tokenise(src)
		if err != nil {
			t.Fatalf("Tokenise failed with error %v on '%v'", err, src)
		}

		if len(tkns) != 3 {
			t.Fatalf("Tokenise returned the wrong number of tokens: %v", tkns)
		}

		t1, t2 := tkns[0], tkns[1]
		if srci == 0 {
			if t1.Type != Float64 {
				t.Fatalf("wrong type for t1")
			}
			if t2.Type != Float64 {
				t.Fatalf("wrong type for t2")
			}
		} else {
			if t1.Type != Float32 {
				t.Fatalf("wrong type for t1")
			}
			if t2.Type != Float32 {
				t.Fatalf("wrong type for t2")
			}
		}
		if tkns[2].Type != Eof {
			t.Fatalf("wrong type for t3")
		}
		if t1.Ival != 1 || t1.Fval != 0 {
			t.Fatalf("wrong value for t1: %v, %v", t1.Ival, t1.Fval)
		}
		if srci == 0 {
			if t1.FvalZeros != 1 {
				t.Fatalf("wrong value for t1/1: %v", t1.FvalZeros)
			}
		} else {
			if t1.FvalZeros != 0 {
				t.Fatalf("wrong value for t1/2: %v", t1.FvalZeros)
			}
		}
		if t2.Ival != 23 || t2.Fval != 45 || t2.FvalZeros != 0 {
			t.Fatalf("wrong value for t2: %v, %v, %v", t2.Ival, t2.Fval, t2.FvalZeros)
		}
	}
}

func TestTokeniseFloatA(t *testing.T) {
	tkns, err := Tokenise("0.007")
	if err != nil {
		t.Fatalf("Tokenise failed with error %v", err)
	}

	if len(tkns) != 2 {
		t.Fatalf("Tokenise returned the wrong number of tokens: %v", tkns)
	}

	t1 := tkns[0]
	if t1.Type != Float64 {
		t.Fatalf("wrong type for t1")
	}
	if t1.Ival != 0 || t1.Fval != 7 {
		t.Fatalf("wrong value for t1: %v, %v", t1.Ival, t1.Fval)
	}
	if t1.FvalZeros != 2 {
		t.Fatalf("wrong number leading zero")
	}
}
