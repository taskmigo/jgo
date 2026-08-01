package gots

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeLanguage(t *testing.T) {
	runtime := New()
	value, err := runtime.RunString(`function fib(n){ if(n < 2) return n; return fib(n-1)+fib(n-2); } fib(8);`)
	if err != nil || value.Float64() != 21 {
		t.Fatalf("%v %v", value, err)
	}
	program, err := runtime.Compile(`let x=1; x+2`)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		value, err = runtime.Run(program)
		if err != nil || value.Float64() != 3 {
			t.Fatal(value, err)
		}
	}
}

func TestTypeofAndNumericGlobals(t *testing.T) {
	value, err := New().RunString(`typeof missing + "," + typeof function(){} + "," + typeof NaN + "," + Infinity`)
	if err != nil || value.String() != "undefined,function,number,Infinity" {
		t.Fatalf("value=%q error=%v", value.String(), err)
	}
}

func TestMultipleLexicalDeclarations(t *testing.T) {
	runtime := New()
	value, err := runtime.RunString(`let a = 1, b = a + 2; const c = b + 3; a + b + c`)
	if err != nil || value.Float64() != 10 {
		t.Fatalf("value=%v error=%v", value, err)
	}
	if _, err := runtime.Compile(`const missing;`); err == nil {
		t.Fatal("const without initializer compiled")
	}
	if _, err := runtime.Compile(`let duplicate, duplicate;`); err == nil {
		t.Fatal("duplicate let declaration compiled")
	}
}

func TestForLoopLexicalIterationsAndComma(t *testing.T) {
	value, err := New().RunString(`var first, second; for (let x=1; x<3; x=x+1) { if (x===1) first=function(){return x;}; else second=function(){return x;}; } (first(), second())`)
	if err != nil || value.Float64() != 2 {
		t.Fatalf("value=%v error=%v", value, err)
	}
}

func TestLimitsAndCancellation(t *testing.T) {
	_, err := New(WithMaxSteps(20)).RunString(`while(true){}`)
	if !errors.Is(err, ErrStepLimit) {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New().RunStringContext(ctx, "1")
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("%v", err)
	}
	if _, err = New().Run(nil); !errors.Is(err, ErrNilProgram) {
		t.Fatal(err)
	}
}
