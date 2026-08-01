package gots

import (
	"context"
	"strings"
	"testing"
)

func TestRuntimeStatisticsAndCallValidation(t *testing.T) {
	runtime := New(Config{})
	result, err := runtime.EvaluateString(context.Background(), `function outer(){ return function(){ return 1; }(); } outer()`)
	if err != nil {
		t.Fatal(err)
	}
	stats := result.Stats
	if stats.Steps == 0 || stats.MaxCallDepth != 2 || stats.Duration <= 0 {
		t.Fatalf("unexpected runtime statistics: %+v", stats)
	}
	if _, err := runtime.Call(context.Background(), Number(1), Undefined()); err == nil || !strings.Contains(err.Error(), "value is not callable") {
		t.Fatalf("unexpected Call error: %v", err)
	}
}
