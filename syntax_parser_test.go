package gots

import "testing"

func TestParserErrorLocations(t *testing.T) {
	tests := []struct {
		name   string
		source string
		line   int
		column int
	}{
		{name: "missing const initializer", source: "\nconst value;", line: 2, column: 7},
		{name: "duplicate lexical name", source: "let first, first;", line: 1, column: 12},
		{name: "missing loop separator", source: "for (let i=0; i<2 i=i+1) {}", line: 1, column: 19},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().Compile(test.source)
			syntaxError, ok := err.(*SyntaxError)
			if !ok {
				t.Fatalf("error = %T %v, want *SyntaxError", err, err)
			}
			if syntaxError.Span.Start.Line != test.line || syntaxError.Span.Start.Column != test.column {
				t.Fatalf("position = %d:%d, want %d:%d", syntaxError.Span.Start.Line, syntaxError.Span.Start.Column, test.line, test.column)
			}
		})
	}
}
