package gots

import (
	"strings"
	"testing"
)

func TestRuntimeStatisticsAndCallValidation(t *testing.T) {
	runtime := New()
	if _, err := runtime.RunString(`function outer(){ return function(){ return 1; }(); } outer()`); err != nil {
		t.Fatal(err)
	}
	stats := runtime.LastStats()
	if stats.Steps == 0 || stats.MaxCallDepth != 2 || stats.Duration <= 0 {
		t.Fatalf("unexpected runtime statistics: %+v", stats)
	}
	if _, err := runtime.Call(Number(1), Undefined()); err == nil || !strings.Contains(err.Error(), "value is not callable") {
		t.Fatalf("unexpected Call error: %v", err)
	}
}
