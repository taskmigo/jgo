package gots

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

type lexer struct {
	source         string
	off, line, col int
}

func lex(source string) ([]Token, error) {
	l := &lexer{source: source, line: 1}
	var out []Token
	for {
		t, e := l.next()
		if e != nil {
			return nil, e
		}
		out = append(out, t)
		if t.Type == TokEOF {
			return out, nil
		}
	}
}

// columnHack exists only to keep construction explicit while column starts at one.
func (l *lexer) next() (Token, error) {
	for l.off < len(l.source) {
		c := l.source[l.off]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			l.advance()
			continue
		}
		if c == '/' && l.peek(1) == '/' {
			for l.off < len(l.source) && l.source[l.off] != '\n' {
				l.advance()
			}
			continue
		}
		if c == '/' && l.peek(1) == '*' {
			s := l.pos()
			l.advance()
			l.advance()
			for l.off < len(l.source) && !(l.source[l.off] == '*' && l.peek(1) == '/') {
				l.advance()
			}
			if l.off >= len(l.source) {
				t := Token{Literal: "/*", Span: Span{Start: s, End: l.pos()}}
				return t, &SyntaxError{t.Span, t, "unterminated comment"}
			}
			l.advance()
			l.advance()
			continue
		}
		break
	}
	s := l.pos()
	if l.off >= len(l.source) {
		return Token{Type: TokEOF, Span: Span{Start: s, End: s}}, nil
	}
	c := l.source[l.off]
	if isIdentStart(c) {
		l.advance()
		for l.off < len(l.source) && isIdentPart(l.source[l.off]) {
			l.advance()
		}
		lit := l.source[s.Offset:l.off]
		typ := keywords[lit]
		if typ == "" {
			typ = TokIdent
		}
		return l.token(typ, lit, s), nil
	}
	if c >= '0' && c <= '9' {
		l.advance()
		for l.off < len(l.source) && ((l.source[l.off] >= '0' && l.source[l.off] <= '9') || l.source[l.off] == '.') {
			l.advance()
		}
		lit := l.source[s.Offset:l.off]
		if _, e := strconv.ParseFloat(lit, 64); e != nil {
			t := l.token(TokNumber, lit, s)
			return t, &SyntaxError{t.Span, t, "invalid number"}
		}
		return l.token(TokNumber, lit, s), nil
	}
	if c == '\'' || c == '"' {
		q := c
		l.advance()
		var b []byte
		for l.off < len(l.source) && l.source[l.off] != q {
			if l.source[l.off] == '\n' {
				break
			}
			if l.source[l.off] == '\\' {
				l.advance()
				if l.off >= len(l.source) {
					break
				}
				switch l.source[l.off] {
				case 'n':
					b = append(b, '\n')
				case 't':
					b = append(b, '\t')
				default:
					b = append(b, l.source[l.off])
				}
				l.advance()
			} else {
				b = append(b, l.source[l.off])
				l.advance()
			}
		}
		if l.off >= len(l.source) || l.source[l.off] != q {
			t := l.token(TokString, string(b), s)
			return t, &SyntaxError{t.Span, t, "unterminated string"}
		}
		l.advance()
		return l.token(TokString, string(b), s), nil
	}
	for _, x := range []struct {
		s string
		t TokenType
	}{{"===", TokStrictEQ}, {"!==", TokStrictNE}, {"==", TokEQ}, {"!=", TokNE}, {"<=", TokLE}, {">=", TokGE}, {"&&", TokAnd}, {"||", TokOr}} {
		if len(l.source)-l.off >= len(x.s) && l.source[l.off:l.off+len(x.s)] == x.s {
			for range x.s {
				l.advance()
			}
			return l.token(x.t, x.s, s), nil
		}
	}
	one := map[byte]TokenType{'(': TokLParen, ')': TokRParen, '{': TokLBrace, '}': TokRBrace, '[': TokLBracket, ']': TokRBracket, ',': TokComma, ';': TokSemi, '.': TokDot, ':': TokColon, '=': TokAssign, '+': TokPlus, '-': TokMinus, '*': TokStar, '/': TokSlash, '!': TokBang, '<': TokLT, '>': TokGT}
	if typ := one[c]; typ != "" {
		l.advance()
		return l.token(typ, string(c), s), nil
	}
	l.advance()
	t := l.token("invalid", string(c), s)
	return t, &SyntaxError{t.Span, t, "invalid character"}
}

var keywords = map[string]TokenType{"let": TokLet, "const": TokConst, "function": TokFunction, "return": TokReturn, "if": TokIf, "else": TokElse, "while": TokWhile, "true": TokTrue, "false": TokFalse, "null": TokNull, "undefined": TokUndefined, "new": TokNew}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= utf8.RuneSelf || unicode.IsLetter(rune(c))
}
func isIdentPart(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }
func (l *lexer) peek(n int) byte {
	if l.off+n >= len(l.source) {
		return 0
	}
	return l.source[l.off+n]
}
func (l *lexer) pos() Position { return Position{l.off, l.line, l.col + 1} }
func (l *lexer) advance() {
	if l.source[l.off] == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
	l.off++
}
func (l *lexer) token(t TokenType, v string, s Position) Token { return Token{t, v, Span{s, l.pos()}} }
