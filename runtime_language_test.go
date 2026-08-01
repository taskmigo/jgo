package gots

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeLanguage(t *testing.T) {
	runtime := New(Config{})
	result, err := runtime.EvaluateString(context.Background(), `function fib(n){ if(n < 2) return n; return fib(n-1)+fib(n-2); } fib(8);`)
	if number, _ := result.Value.ToNumber(); err != nil || number != 21 {
		t.Fatalf("%v %v", result.Value, err)
	}
	program, err := runtime.Compile(`let x=1; x+2`)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		result, err = New(Config{}).Evaluate(context.Background(), program)
		if number, _ := result.Value.ToNumber(); err != nil || number != 3 {
			t.Fatal(result.Value, err)
		}
	}
}

func TestTypeofAndNumericGlobals(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `typeof missing + "," + typeof function(){} + "," + typeof NaN + "," + Infinity`)
	if err != nil || result.Value.Inspect() != "undefined,function,number,Infinity" {
		t.Fatalf("value=%q error=%v", result.Value.Inspect(), err)
	}
}

func TestMultipleLexicalDeclarations(t *testing.T) {
	runtime := New(Config{})
	result, err := runtime.EvaluateString(context.Background(), `let a = 1, b = a + 2; const c = b + 3; a + b + c`)
	if number, _ := result.Value.ToNumber(); err != nil || number != 10 {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
	if _, err := runtime.Compile(`const missing;`); err == nil {
		t.Fatal("const without initializer compiled")
	}
	if _, err := runtime.Compile(`let duplicate, duplicate;`); err == nil {
		t.Fatal("duplicate let declaration compiled")
	}
}

func TestForLoopLexicalIterationsAndComma(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `var first, second; for (let x=1; x<3; x=x+1) { if (x===1) first=function(){return x;}; else second=function(){return x;}; } (first(), second())`)
	if number, _ := result.Value.ToNumber(); err != nil || number != 2 {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
}

func TestPrefixUpdateAndPerIterationBindings(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let closures=[];
		for(let index=0; index<3; ++index){closures.push(function(){return index;});}
		closures[0]()===0 && closures[1]()===1 && closures[2]()===2`)
	if !value.ToBoolean() {
		t.Fatalf("prefix update expression = %s", value.Inspect())
	}
}

func TestLimitsAndCancellation(t *testing.T) {
	_, err := New(Config{MaxSteps: 20}).EvaluateString(context.Background(), `while(true){}`)
	if !errors.Is(err, ErrStepLimit) {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New(Config{}).EvaluateString(ctx, "1")
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("%v", err)
	}
	if _, err = New(Config{}).Evaluate(context.Background(), nil); !errors.Is(err, ErrNilProgram) {
		t.Fatal(err)
	}
}
