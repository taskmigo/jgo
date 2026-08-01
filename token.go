package gots

import "fmt"

type Position struct{ Offset, Line, Column int }
type Span struct{ Start, End Position }
type Token struct {
	Type    TokenType
	Literal string
	Span    Span
}
type TokenType string

const (
	TokEOF       TokenType = "eof"
	TokIdent     TokenType = "identifier"
	TokNumber    TokenType = "number"
	TokString    TokenType = "string"
	TokLet       TokenType = "let"
	TokConst     TokenType = "const"
	TokFunction  TokenType = "function"
	TokReturn    TokenType = "return"
	TokIf        TokenType = "if"
	TokElse      TokenType = "else"
	TokWhile     TokenType = "while"
	TokTrue      TokenType = "true"
	TokFalse     TokenType = "false"
	TokNull      TokenType = "null"
	TokUndefined TokenType = "undefined"
	TokNew       TokenType = "new"
	TokLParen    TokenType = "("
	TokRParen    TokenType = ")"
	TokLBrace    TokenType = "{"
	TokRBrace    TokenType = "}"
	TokLBracket  TokenType = "["
	TokRBracket  TokenType = "]"
	TokComma     TokenType = ","
	TokSemi      TokenType = ";"
	TokDot       TokenType = "."
	TokColon     TokenType = ":"
	TokAssign    TokenType = "="
	TokPlus      TokenType = "+"
	TokMinus     TokenType = "-"
	TokStar      TokenType = "*"
	TokSlash     TokenType = "/"
	TokBang      TokenType = "!"
	TokLT        TokenType = "<"
	TokGT        TokenType = ">"
	TokLE        TokenType = "<="
	TokGE        TokenType = ">="
	TokEQ        TokenType = "=="
	TokNE        TokenType = "!="
	TokStrictEQ  TokenType = "==="
	TokStrictNE  TokenType = "!=="
	TokAnd       TokenType = "&&"
	TokOr        TokenType = "||"
)

type SyntaxError struct {
	Span    Span
	Token   Token
	Message string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("syntax error at %d:%d: %s (got %q)", e.Span.Start.Line, e.Span.Start.Column, e.Message, e.Token.Literal)
}
