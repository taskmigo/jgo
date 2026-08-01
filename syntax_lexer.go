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

func (l *lexer) next() (Token, error) {
	if err := l.skipTrivia(); err != nil {
		return Token{}, err
	}
	start := l.pos()
	if l.off >= len(l.source) {
		return Token{Type: TokEOF, Span: Span{Start: start, End: start}}, nil
	}

	character := l.source[l.off]
	if isIdentStart(character) {
		return l.scanIdentifier(start), nil
	}
	if isDigit(character) {
		return l.scanNumber(start)
	}
	if character == '\'' || character == '"' {
		return l.scanString(start)
	}
	if token, found := l.scanMultiCharacterOperator(start); found {
		return token, nil
	}
	if tokenType := singleCharacterTokens[character]; tokenType != "" {
		l.advance()
		return l.token(tokenType, string(character), start), nil
	}

	l.advance()
	token := l.token("invalid", string(character), start)
	return token, &SyntaxError{token.Span, token, "invalid character"}
}

func (l *lexer) skipTrivia() error {
	for l.off < len(l.source) {
		character := l.source[l.off]
		if isWhitespace(character) {
			l.advance()
			continue
		}
		if character == '/' && l.peek(1) == '/' {
			for l.off < len(l.source) && l.source[l.off] != '\n' {
				l.advance()
			}
			continue
		}
		if character == '/' && l.peek(1) == '*' {
			start := l.pos()
			l.advance()
			l.advance()
			for l.off < len(l.source) && !(l.source[l.off] == '*' && l.peek(1) == '/') {
				l.advance()
			}
			if l.off >= len(l.source) {
				token := Token{Literal: "/*", Span: Span{Start: start, End: l.pos()}}
				return &SyntaxError{token.Span, token, "unterminated comment"}
			}
			l.advance()
			l.advance()
			continue
		}
		break
	}
	return nil
}

func (l *lexer) scanIdentifier(start Position) Token {
	l.advance()
	for l.off < len(l.source) && isIdentPart(l.source[l.off]) {
		l.advance()
	}
	literal := l.source[start.Offset:l.off]
	tokenType := keywords[literal]
	if tokenType == "" {
		tokenType = TokIdent
	}
	return l.token(tokenType, literal, start)
}

func (l *lexer) scanNumber(start Position) (Token, error) {
	l.advance()
	for l.off < len(l.source) && (isDigit(l.source[l.off]) || l.source[l.off] == '.') {
		l.advance()
	}
	literal := l.source[start.Offset:l.off]
	token := l.token(TokNumber, literal, start)
	if _, err := strconv.ParseFloat(literal, 64); err != nil {
		return token, &SyntaxError{token.Span, token, "invalid number"}
	}
	return token, nil
}

func (l *lexer) scanString(start Position) (Token, error) {
	quote := l.source[l.off]
	l.advance()
	var value []byte
	for l.off < len(l.source) && l.source[l.off] != quote {
		if l.source[l.off] == '\n' {
			break
		}
		if l.source[l.off] != '\\' {
			value = append(value, l.source[l.off])
			l.advance()
			continue
		}
		l.advance()
		if l.off >= len(l.source) {
			break
		}
		switch l.source[l.off] {
		case 'n':
			value = append(value, '\n')
		case 't':
			value = append(value, '\t')
		default:
			value = append(value, l.source[l.off])
		}
		l.advance()
	}
	token := l.token(TokString, string(value), start)
	if l.off >= len(l.source) || l.source[l.off] != quote {
		return token, &SyntaxError{token.Span, token, "unterminated string"}
	}
	l.advance()
	return l.token(TokString, string(value), start), nil
}

func (l *lexer) scanMultiCharacterOperator(start Position) (Token, bool) {
	for _, operator := range multiCharacterTokens {
		if len(l.source)-l.off < len(operator.literal) || l.source[l.off:l.off+len(operator.literal)] != operator.literal {
			continue
		}
		for range operator.literal {
			l.advance()
		}
		return l.token(operator.tokenType, operator.literal, start), true
	}
	return Token{}, false
}

var keywords = map[string]TokenType{
	"let": TokLet, "var": TokVar, "const": TokConst, "function": TokFunction,
	"return": TokReturn, "if": TokIf, "else": TokElse, "while": TokWhile,
	"for": TokFor, "true": TokTrue, "false": TokFalse, "null": TokNull,
	"undefined": TokUndefined, "new": TokNew, "typeof": TokTypeof,
}

var multiCharacterTokens = []struct {
	literal   string
	tokenType TokenType
}{
	{"===", TokStrictEQ}, {"!==", TokStrictNE}, {"==", TokEQ}, {"!=", TokNE},
	{"<=", TokLE}, {">=", TokGE}, {"&&", TokAnd}, {"||", TokOr},
}

var singleCharacterTokens = map[byte]TokenType{
	'(': TokLParen, ')': TokRParen, '{': TokLBrace, '}': TokRBrace,
	'[': TokLBracket, ']': TokRBracket, ',': TokComma, ';': TokSemi,
	'.': TokDot, ':': TokColon, '=': TokAssign, '+': TokPlus,
	'-': TokMinus, '*': TokStar, '/': TokSlash, '!': TokBang,
	'<': TokLT, '>': TokGT,
}

func isWhitespace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n'
}

func isDigit(character byte) bool { return character >= '0' && character <= '9' }

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
