package token

import (
	"fmt"
	"strings"
)

type tokeniserState struct {
	Line     int
	Position int
}

type tokeniser struct {
	tkns         []Token
	src          []rune
	runeMap      map[rune]TokenType
	multicharMap map[string]TokenType
	keywordMap   map[string]TokenType
	state        tokeniserState

	pendingToken     *Token
	pendingSemicolon bool

	// eof for "no token" or the token type
	lastTokenType TokenType
}

func IsIdentifierStart(r rune) bool {
	return strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_", r)
}

func IsZero(r rune) bool {
	return r == '0'
}

func IsDigit(r rune) bool {
	return strings.ContainsRune("0123456789", r)
}

func IsIdentifier(r rune) bool {
	return IsIdentifierStart(r) || IsDigit(r)
}

func NotEndString(r rune) bool {
	return r != '"'
}

// get a rune, and advance
func (t *tokeniser) getNextRune() (rune, bool) {
	if t.state.Position >= len(t.src) {
		return 0, false
	}

	r := t.src[t.state.Position]
	t.state.Position += 1
	if r == '\n' {
		t.state.Line += 1
	}
	return r, true
}

// return the rune (or 0 if none)
func (t *tokeniser) peekNextRune() rune {
	sp := t.getSavePoint()
	r, ok := t.getNextRune()
	if !ok {
		return 0
	}
	t.restoreSavePoint(sp)

	return r
}

func (t *tokeniser) getSavePoint() tokeniserState {
	return t.state
}

func (t *tokeniser) restoreSavePoint(s tokeniserState) {
	t.state = s
}

// reject a rune, back up one
func (t *tokeniser) backup() {
	t.state.Position -= 1
	if t.peekNextRune() == '\n' {
		t.state.Line -= 1
	}
}

func (t *tokeniser) gather(rs []rune, test func(rune) bool) []rune {
	for {
		r, found := t.getNextRune()
		if !found {
			break
		}

		if !test(r) {
			t.backup()
			break
		}

		if rs != nil {
			rs = append(rs, r)
		}
	}

	return rs
}

func (t *tokeniser) eat2(test func(rune, rune) bool) {
	for {
		r1, found := t.getNextRune()
		if !found {
			break
		}

		r2, found := t.getNextRune()
		if !found {
			t.backup()
			break
		}

		if test(r1, r2) {
			return
		}

		t.backup()
	}
}

func convertNumber(rs []rune) int64 {
	var val int64 = 0

	for _, r := range rs {
		digitValue := int64(r - '0')
		val = val*10 + digitValue
	}

	return val
}

func IsNotEol(r rune) bool {
	return r != '\n' && r != '\r'
}

/*
Should we (based on previous token) insert a semicolon at this newline?
*/
func (t *tokeniser) shouldInsertSemicolon() bool {
	switch t.lastTokenType {
	case Integer, Float64, Float32, Identifier, Character, String, CloseCurly, CloseCurved, Colon, True, False, IntegerKeyword, Float32Keyword, Float64Keyword, BoolKeyword, CharKeyword, StringKeyword, Self:
		return true

	default:
		return false
	}
}

/*
This both eats whitespace and inserts semicolons
*/
func (t *tokeniser) eatWhitespace() {
	for {
		r, found := t.getNextRune()
		if !found {
			break
		}

		if strings.ContainsRune(" \t", r) {
			// regular whitespace, just keep going
		} else if strings.ContainsRune("\r\n", r) {
			// newline, potentially insert a semicolon
			if t.shouldInsertSemicolon() {
				t.pendingSemicolon = true
			}
		} else {
			t.backup()
			break
		}
	}
}

func (t *tokeniser) eatJunk() {
	for {
		t.eatWhitespace()

		savePoint := t.getSavePoint()

		r1, found := t.getNextRune()
		if !found {
			t.restoreSavePoint(savePoint)
			break
		}

		r2, found := t.getNextRune()
		if !found {
			t.restoreSavePoint(savePoint)
			break
		}

		if r1 == '/' && r2 == '/' {
			t.gather(nil, IsNotEol)
			continue
		}

		if r1 == '/' && r2 == '*' {
			t.eat2(func(r1, r2 rune) bool {
				return r1 == '*' && r2 == '/'
			})
			continue
		}

		t.restoreSavePoint(savePoint)
		break
	}
}

func (t *tokeniser) getNextInner() (Token, error) {
	t.eatJunk()

	r, ok := t.getNextRune()
	if !ok {
		return Token{Type: Eof}, nil
	}

	nextRune := t.peekNextRune()
	for str, token := range t.multicharMap {
		runes := []rune(str)
		if runes[0] == r && runes[1] == nextRune {
			t.getNextRune()
			return Token{Type: token}, nil
		}
	}

	tt, found := t.runeMap[r]
	if found {
		return Token{Type: tt}, nil
	}

	if IsDigit(r) {
		rs := t.gather([]rune{r}, IsDigit)
		nr := t.peekNextRune()
		if nr == '.' {
			t.getNextRune()
			zs := t.gather([]rune{}, IsZero)
			nrs := t.gather([]rune{}, IsDigit)

			var ty TokenType = Float64
			if t.peekNextRune() == 'f' {
				t.getNextRune()
				ty = Float32
			}

			return Token{
				Type:      ty,
				Ival:      convertNumber(rs),
				FvalZeros: int64(len(zs)),
				Fval:      convertNumber(nrs),
			}, nil
		} else if nr == 'f' {
			t.getNextRune()
			return Token{
				Type:      Float32,
				Ival:      convertNumber(rs),
				FvalZeros: 0,
				Fval:      0,
			}, nil
		} else {
			return Token{
				Type:      Integer,
				FvalZeros: 0,
				Ival:      convertNumber(rs),
			}, nil
		}
	} else if IsIdentifierStart(r) {
		ident := string(t.gather([]rune{r}, IsIdentifier))
		kw, found := t.keywordMap[ident]
		if found {
			return Token{Type: kw}, nil
		} else {
			return Token{
				Type: Identifier,
				Tval: ident,
			}, nil
		}
	} else if r == '\'' {
		chr, foundRune := t.getNextRune()
		if !foundRune {
			return Token{}, fmt.Errorf("Found nothing after an open character literal")
		}
		ichr := int64(chr)
		if chr == '\\' {
			esc, ok := t.getNextRune()
			if !ok {
				return Token{}, fmt.Errorf("No escape found in character literal")
			}

			switch esc {
			case 'n':
				ichr = 10

			case 'r':
				ichr = 13

			case 't':
				ichr = 9

			default:
				return Token{}, fmt.Errorf("Do not recognise escape sequence '\\%v'", esc)
			}
		}

		close, found := t.getNextRune()
		if !found {
			return Token{}, fmt.Errorf("Did not find end of character rune")
		}

		if close != '\'' {
			return Token{}, fmt.Errorf("Did not find end of character")
		}

		return Token{
			Type: Character,
			Ival: ichr,
		}, nil
	} else if r == '"' {
		contents := string(t.gather([]rune{}, NotEndString))
		_, found := t.getNextRune()
		if !found {
			// only option is for it to be an end string here
			return Token{}, fmt.Errorf("Unexpected end of file in string")
		}

		return Token{
			Type: String,
			Tval: contents,
		}, nil
	}

	lead, c, tail := t.currentSurround()
	return Token{}, fmt.Errorf("Unable to tokenise '%v|%v|%v'", lead, c, tail)
}

func (t *tokeniser) getNext() (Token, error) {
	if t.pendingToken != nil {
		tk := *t.pendingToken
		t.pendingToken = nil
		return tk, nil
	}

	tk, err := t.getNextInner()
	if err != nil {
		return tk, err
	}

	if t.pendingSemicolon {
		t.pendingToken = &tk
		t.pendingSemicolon = false
		return Token{Type: Semicolon}, nil
	}

	t.lastTokenType = tk.Type
	return tk, nil
}

func (t *tokeniser) currentSurround() (string, string, string) {
	surroundSize := 10

	la := t.state.Position - surroundSize
	lb := t.state.Position
	ra := t.state.Position + 1
	rb := t.state.Position + 1 + surroundSize

	if la < 0 {
		la = 0
	}
	if ra >= len(t.src) {
		ra = len(t.src) - 1
	}
	if rb >= len(t.src) {
		rb = len(t.src) - 1
	}

	lead := t.src[la:lb]
	c := t.src[lb:ra]
	tail := t.src[ra:rb]

	return string(lead), string(c), string(tail)
}

func Tokenise(text string) ([]Token, error) {
	t := tokeniser{
		lastTokenType: Eof,
		tkns:          []Token{},
		src:           []rune(text),
		state: tokeniserState{
			Position: 0,
			Line:     1,
		},
		keywordMap: map[string]TokenType{
			"partial":  Partial,
			"_":        Placeholder,
			"struct":   Struct,
			"self":     Self,
			"as":       As,
			"new":      New,
			"fn":       Function,
			"null":     Null,
			"break":    Break,
			"range":    Range,
			"let":      Let,
			"const":    Const,
			"true":     True,
			"false":    False,
			"i64":      IntegerKeyword,
			"f32":      Float32Keyword,
			"f64":      Float64Keyword,
			"bool":     BoolKeyword,
			"char":     CharKeyword,
			"string":   StringKeyword,
			"return":   Return,
			"if":       If,
			"else":     Else,
			"elseif":   ElseIf,
			"while":    While,
			"and":      And,
			"not":      Not,
			"or":       Or,
			"send":     Send,
			"receive":  Receive,
			"pipeline": Pipeline,
			"cpu":      Cpu,
			"gpu":      Gpu,
			"worker":   Worker,
			"drain":    Drain,
			"import":   Import,
			"for":      Foreach,
		},
		multicharMap: map[string]TokenType{
			"==": Equality,
			"!=": Inequality,
			"<=": LessThanOrEqual,
			">=": GreaterThanOrEqual,
			"+=": PlusEquals,
			"-=": MinusEquals,
			"*=": TimesEquals,
			"/=": DivideEquals,
			"::": ScopeResolution,
		},
		runeMap: map[rune]TokenType{
			'=': Equals,
			'(': OpenCurved,
			')': CloseCurved,
			'[': OpenSquare,
			']': CloseSquare,
			'{': OpenCurly,
			'}': CloseCurly,
			',': Comma,
			';': Semicolon,
			'+': Plus,
			'-': Minus,
			'*': Multiply,
			'/': Divide,
			'<': LessThan,
			'>': GreaterThan,
			':': Colon,
			'.': Dot,
			'%': Percent,
		},
	}

	for {
		tkn, err := t.getNext()
		if err != nil {
			return nil, fmt.Errorf("Error in tokenise: %v", err)
		}
		tkn.Line = t.state.Line

		t.tkns = append(t.tkns, tkn)

		if tkn.Type == Eof {
			break
		}
	}

	return t.tkns, nil
}
