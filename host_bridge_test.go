package gots

import (
	"context"
	"errors"
	"testing"
)

func TestReflectedHostFunctions(t *testing.T) {
	runtime := New(Config{})
	if err := runtime.Define("join", func(prefix string, values ...int) string {
		return prefix + Number(float64(values[0])).Inspect()
	}); err != nil {
		t.Fatal(err)
	}
	result, err := runtime.EvaluateString(context.Background(), `join("value=", 2)`)
	if err != nil || result.Value.Inspect() != "value=2" {
		t.Fatalf("value=%q error=%v", result.Value.Inspect(), err)
	}
	if result, err := runtime.EvaluateString(context.Background(), `join("sum=", 2, 3)`); err != nil || result.Value.Inspect() != "sum=2" {
		t.Fatalf("unexpected variadic call result=%v error=%v", result.Value, err)
	}

	if err := runtime.Define("unsupported", func(complex64) {}); err != nil {
		t.Fatal(err)
	}
	_, err = runtime.EvaluateString(context.Background(), `unsupported(1)`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Message != "unsupported host argument type" {
		t.Fatalf("unexpected host bridge error: %v", err)
	}
}
