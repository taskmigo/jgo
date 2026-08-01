package gots

import (
	"context"
	"testing"
)

func TestLoopBreakAndContinueCompletions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   float64
	}{
		{
			name:   "while break",
			source: `let value = 0; while (true) { value = value + 1; if (value === 3) break; } value;`,
			want:   3,
		},
		{
			name:   "for continue still updates",
			source: `let total = 0; for (let index = 0; index < 5; index = index + 1) { if (index === 2) continue; total = total + index; } total;`,
			want:   8,
		},
		{
			name: "labelled continue",
			source: `let count = 0;
				outer: for (let left = 0; left < 3; left = left + 1) {
					for (let right = 0; right < 3; right = right + 1) {
						if (right === 1) continue outer;
						count = count + 1;
					}
				}
				count;`,
			want: 3,
		},
		{
			name:   "labelled break",
			source: `let value = 0; target: { value = 1; break target; value = 2; } value;`,
			want:   1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New(Config{}).EvaluateString(context.Background(), test.source)
			if err != nil {
				t.Fatal(err)
			}
			number, err := result.Value.ToNumber()
			if err != nil || number != test.want {
				t.Fatalf("result = %s, want %v", result.Value.Inspect(), test.want)
			}
		})
	}
}

func TestControlFlowEarlyErrors(t *testing.T) {
	tests := []string{
		`break;`,
		`continue;`,
		`while (true) { break missing; }`,
		`label: { continue label; }`,
		`label: label: while (true) { break; }`,
		`while (true) { function nested() { break; } }`,
		`return 1;`,
		`label: function declaration() {}`,
	}
	for _, source := range tests {
		if _, err := New(Config{}).Compile(source); err == nil {
			t.Fatalf("invalid control flow compiled: %s", source)
		}
	}
}

func TestContinueLineTerminatorPreventsLabelAttachment(t *testing.T) {
	program, err := New(Config{}).Compile("outer: while (true) { continue\ninner: while (false) {} }")
	if err != nil {
		t.Fatal(err)
	}
	outer := program.body[0].(*labelledStmt)
	loop := outer.body.(*whileStmt)
	body := loop.body.(*blockStmt)
	continuation := body.body[0].(*continueStmt)
	if continuation.target != "" {
		t.Fatalf("continue target = %q, want empty", continuation.target)
	}
}
