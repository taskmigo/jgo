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
