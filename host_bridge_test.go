package gots

import (
	"errors"
	"strings"
	"testing"
)

func TestReflectedHostFunctions(t *testing.T) {
	runtime := New()
	if err := runtime.Set("join", func(prefix string, values ...int) string {
		return prefix + Number(float64(values[0])).String()
	}); err != nil {
		t.Fatal(err)
	}
	value, err := runtime.RunString(`join("value=", 2)`)
	if err != nil || value.String() != "value=2" {
		t.Fatalf("value=%q error=%v", value.String(), err)
	}
	if _, err := runtime.RunString(`join("sum=", 2, 3)`); err == nil || !strings.Contains(err.Error(), "host panic") {
		t.Fatalf("unexpected extra variadic argument behavior: %v", err)
	}

	if err := runtime.Set("unsupported", func(complex64) {}); err != nil {
		t.Fatal(err)
	}
	_, err = runtime.RunString(`unsupported(1)`)
	var runtimeError *RuntimeError
	if !errors.As(err, &runtimeError) || runtimeError.Message != "unsupported host argument type" {
		t.Fatalf("unexpected host bridge error: %v", err)
	}
}
