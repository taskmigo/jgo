package gots

import "testing"

func TestLexerLocations(t *testing.T) {
	ts, e := lex("let x = 1;\n@")
	if e == nil {
		t.Fatal("expected invalid character")
	}
	se := e.(*SyntaxError)
	if se.Span.Start.Line != 2 || se.Span.Start.Column != 1 {
		t.Fatalf("%+v", se.Span)
	}
	if ts != nil {
		t.Fatal("tokens returned with error")
	}
}
func TestUnterminatedString(t *testing.T) {
	_, e := lex("'oops")
	if e == nil {
		t.Fatal("expected error")
	}
}

func TestLexerRecognizesUpdateOperatorsAsSingleTokens(t *testing.T) {
	tokens, err := lex("++value; --value")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokPlusPlus || tokens[3].Type != TokMinusMinus {
		t.Fatalf("update tokens = %q, %q", tokens[0].Type, tokens[3].Type)
	}
}

func TestLexerPreservesTokenSpansAcrossTrivia(t *testing.T) {
	tokens, err := lex("/* note */\n  let value=12;")
	if err != nil {
		t.Fatal(err)
	}
	want := []Token{
		{Type: TokLet, Literal: "let", Span: Span{Start: Position{Offset: 13, Line: 2, Column: 3}, End: Position{Offset: 16, Line: 2, Column: 6}}},
		{Type: TokIdent, Literal: "value", Span: Span{Start: Position{Offset: 17, Line: 2, Column: 7}, End: Position{Offset: 22, Line: 2, Column: 12}}},
		{Type: TokAssign, Literal: "=", Span: Span{Start: Position{Offset: 22, Line: 2, Column: 12}, End: Position{Offset: 23, Line: 2, Column: 13}}},
		{Type: TokNumber, Literal: "12", Span: Span{Start: Position{Offset: 23, Line: 2, Column: 13}, End: Position{Offset: 25, Line: 2, Column: 15}}},
		{Type: TokSemi, Literal: ";", Span: Span{Start: Position{Offset: 25, Line: 2, Column: 15}, End: Position{Offset: 26, Line: 2, Column: 16}}},
		{Type: TokEOF, Span: Span{Start: Position{Offset: 26, Line: 2, Column: 16}, End: Position{Offset: 26, Line: 2, Column: 16}}},
	}
	if len(tokens) != len(want) {
		t.Fatalf("token count = %d, want %d", len(tokens), len(want))
	}
	for index := range want {
		if tokens[index] != want[index] {
			t.Fatalf("token %d = %+v, want %+v", index, tokens[index], want[index])
		}
	}
}
