package gots

import (
	"strconv"
	"unicode"
	"unicode/utf16"
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
	base := 10
	if l.source[l.off] == '0' && l.off+1 < len(l.source) {
		switch l.source[l.off+1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
	}
	if base != 10 {
		l.advance()
		l.advance()
		digitStart := l.off
		for l.off < len(l.source) && validDigitForBase(l.source[l.off], base) {
			l.advance()
		}
		literal := l.source[start.Offset:l.off]
		token := l.token(TokNumber, literal, start)
		if l.off == digitStart || (l.off < len(l.source) && isIdentPart(l.source[l.off])) {
			return token, &SyntaxError{token.Span, token, "invalid number"}
		}
		integer, err := strconv.ParseUint(l.source[digitStart:l.off], base, 64)
		if err != nil {
			return token, &SyntaxError{token.Span, token, "invalid number"}
		}
		value := float64(integer)
		token.numberValue = &value
		return token, nil
	}

	for l.off < len(l.source) && isDigit(l.source[l.off]) {
		l.advance()
	}
	if l.off < len(l.source) && l.source[l.off] == '.' {
		l.advance()
		for l.off < len(l.source) && isDigit(l.source[l.off]) {
			l.advance()
		}
	}
	if l.off < len(l.source) && (l.source[l.off] == 'e' || l.source[l.off] == 'E') {
		l.advance()
		if l.off < len(l.source) && (l.source[l.off] == '+' || l.source[l.off] == '-') {
			l.advance()
		}
		exponentStart := l.off
		for l.off < len(l.source) && isDigit(l.source[l.off]) {
			l.advance()
		}
		if exponentStart == l.off {
			token := l.token(TokNumber, l.source[start.Offset:l.off], start)
			return token, &SyntaxError{token.Span, token, "invalid number"}
		}
	}
	literal := l.source[start.Offset:l.off]
	token := l.token(TokNumber, literal, start)
	_, err := strconv.ParseFloat(literal, 64)
	if err != nil || (l.off < len(l.source) && isIdentPart(l.source[l.off])) {
		return token, &SyntaxError{token.Span, token, "invalid number"}
	}
	return token, nil
}

func validDigitForBase(character byte, base int) bool {
	switch {
	case character >= '0' && character <= '9':
		return int(character-'0') < base
	case character >= 'a' && character <= 'f':
		return base == 16
	case character >= 'A' && character <= 'F':
		return base == 16
	default:
		return false
	}
}

func (l *lexer) scanString(start Position) (Token, error) {
	quote := l.source[l.off]
	l.advance()
	var units []uint16
	for l.off < len(l.source) && l.source[l.off] != quote {
		if l.source[l.off] == '\n' {
			break
		}
		if l.source[l.off] != '\\' {
			character, width := utf8.DecodeRuneInString(l.source[l.off:])
			if character == utf8.RuneError && width == 1 {
				token := l.token(TokString, string(utf16.Decode(units)), start)
				return token, &SyntaxError{token.Span, token, "invalid UTF-8 in string literal"}
			}
			units = append(units, utf16.Encode([]rune{character})...)
			for range width {
				l.advance()
			}
			continue
		}
		l.advance()
		if l.off >= len(l.source) {
			break
		}
		escape := l.source[l.off]
		switch escape {
		case '\n':
			l.advance()
			continue
		case 'b':
			units = append(units, '\b')
		case 'f':
			units = append(units, '\f')
		case 'n':
			units = append(units, '\n')
		case 'r':
			units = append(units, '\r')
		case 't':
			units = append(units, '\t')
		case 'v':
			units = append(units, '\v')
		case '0':
			units = append(units, 0)
		case 'x':
			l.advance()
			codeUnit, ok := l.scanHexDigits(2)
			if !ok {
				token := l.token(TokString, string(utf16.Decode(units)), start)
				return token, &SyntaxError{token.Span, token, "invalid hexadecimal escape"}
			}
			units = append(units, codeUnit)
			continue
		case 'u':
			l.advance()
			if l.off < len(l.source) && l.source[l.off] == '{' {
				l.advance()
				codePointStart := l.off
				for l.off < len(l.source) && validDigitForBase(l.source[l.off], 16) {
					l.advance()
				}
				if codePointStart == l.off || l.off >= len(l.source) || l.source[l.off] != '}' {
					token := l.token(TokString, string(utf16.Decode(units)), start)
					return token, &SyntaxError{token.Span, token, "invalid Unicode escape"}
				}
				codePoint, parseErr := strconv.ParseUint(l.source[codePointStart:l.off], 16, 32)
				l.advance()
				if parseErr != nil || codePoint > utf8.MaxRune || codePoint >= 0xd800 && codePoint <= 0xdfff {
					token := l.token(TokString, string(utf16.Decode(units)), start)
					return token, &SyntaxError{token.Span, token, "invalid Unicode escape"}
				}
				units = append(units, utf16.Encode([]rune{rune(codePoint)})...)
				continue
			}
			codeUnit, ok := l.scanHexDigits(4)
			if !ok {
				token := l.token(TokString, string(utf16.Decode(units)), start)
				return token, &SyntaxError{token.Span, token, "invalid Unicode escape"}
			}
			units = append(units, codeUnit)
			continue
		default:
			units = append(units, uint16(escape))
		}
		l.advance()
	}
	literal := string(utf16.Decode(units))
	token := l.token(TokString, literal, start)
	tokenUnits := jsString(units).clone()
	token.stringValue = &tokenUnits
	if l.off >= len(l.source) || l.source[l.off] != quote {
		return token, &SyntaxError{token.Span, token, "unterminated string"}
	}
	l.advance()
	token = l.token(TokString, literal, start)
	tokenUnits = jsString(units).clone()
	token.stringValue = &tokenUnits
	return token, nil
}

func (l *lexer) scanHexDigits(count int) (uint16, bool) {
	if len(l.source)-l.off < count {
		return 0, false
	}
	var value uint16
	for range count {
		digit := l.source[l.off]
		var numeric byte
		switch {
		case digit >= '0' && digit <= '9':
			numeric = digit - '0'
		case digit >= 'a' && digit <= 'f':
			numeric = digit - 'a' + 10
		case digit >= 'A' && digit <= 'F':
			numeric = digit - 'A' + 10
		default:
			return 0, false
		}
		value = value*16 + uint16(numeric)
		l.advance()
	}
	return value, true
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
func (l *lexer) token(t TokenType, v string, s Position) Token {
	return Token{Type: t, Literal: v, Span: Span{Start: s, End: l.pos()}}
}
