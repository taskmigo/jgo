package gots

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestRuntimeLanguage(t *testing.T) {
	r := New()
	v, e := r.RunString(`function fib(n){ if(n < 2) return n; return fib(n-1)+fib(n-2); } fib(8);`)
	if e != nil || v.Float64() != 21 {
		t.Fatalf("%v %v", v, e)
	}
	p, e := r.Compile(`let x=1; x+2`)
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		v, e = r.Run(p)
		if e != nil || v.Float64() != 3 {
			t.Fatal(v, e)
		}
	}
}

func TestTypeofAndNumericGlobals(t *testing.T) {
	r := New()
	v, err := r.RunString(`typeof missing + "," + typeof function(){} + "," + typeof NaN + "," + Infinity`)
	if err != nil || v.String() != "undefined,function,number,Infinity" {
		t.Fatalf("value=%q error=%v", v.String(), err)
	}
}

func TestMultipleLexicalDeclarations(t *testing.T) {
	r := New()
	v, err := r.RunString(`let a = 1, b = a + 2; const c = b + 3; a + b + c`)
	if err != nil || v.Float64() != 10 {
		t.Fatalf("value=%v error=%v", v, err)
	}
	if _, err := r.Compile(`const missing;`); err == nil {
		t.Fatal("const without initializer compiled")
	}
	if _, err := r.Compile(`let duplicate, duplicate;`); err == nil {
		t.Fatal("duplicate let declaration compiled")
	}
}
func TestForLoopLexicalIterationsAndComma(t *testing.T) {
	r := New()
	v, err := r.RunString(`var first, second; for (let x=1; x<3; x=x+1) { if (x===1) first=function(){return x;}; else second=function(){return x;}; } (first(), second())`)
	if err != nil || v.Float64() != 2 {
		t.Fatalf("value=%v error=%v", v, err)
	}
}
func TestLimitsAndCancellation(t *testing.T) {
	_, e := New(WithMaxSteps(20)).RunString(`while(true){}`)
	if !errors.Is(e, ErrStepLimit) {
		t.Fatalf("%v", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = New().RunStringContext(ctx, "1")
	if !errors.Is(e, ErrCancelled) {
		t.Fatalf("%v", e)
	}
	if _, e = New().Run(nil); !errors.Is(e, ErrNilProgram) {
		t.Fatal(e)
	}
}
func TestWeakMapSemantics(t *testing.T) {
	r := New()
	v, e := r.RunString(`let k={}; let m=new WeakMap(); m.set(k, undefined); m.has(k) && m.get(k) === undefined;`)
	if e != nil || !v.Bool() {
		t.Fatal(v, e)
	}
	_, e = r.RunString(`new WeakMap().set(1,2)`)
	if e == nil {
		t.Fatal("primitive key accepted")
	}
	v, e = r.RunString(`let a={}; let b={}; let m=new WeakMap([[a,1],[b,2]]); m.get(a)+m.get(b)`)
	if e != nil || v.Float64() != 3 {
		t.Fatal(v, e)
	}
}

func TestWeakMapConstructorAndVarSemantics(t *testing.T) {
	r := New()
	v, err := r.RunString(`var a = new WeakMap(null); var k = {}; a.set(k, 7); a.get(k);`)
	if err != nil || v.Float64() != 7 {
		t.Fatalf("value=%v error=%v", v, err)
	}
	if _, err := r.RunString(`WeakMap()`); err == nil {
		t.Fatal("WeakMap call without new succeeded")
	}
}
func TestWeakMapSymbolAndGetOrInsert(t *testing.T) {
	r := New()
	v, err := r.RunString(`var m=new WeakMap(); var s=Symbol("key"); m.set(s, 1); m.getOrInsert(s, 2) + m.getOrInsert(Symbol(), 3)`)
	if err != nil || v.Float64() != 4 {
		t.Fatalf("value=%v error=%v", v, err)
	}
	if _, err := r.RunString(`new WeakMap().set(Symbol.for("registered"), 1)`); err == nil {
		t.Fatal("registered symbol accepted as weak key")
	}
}
func TestWeakMapDoesNotKeepKeyAlive(t *testing.T) {
	m := newWeakMapValue()
	done := make(chan struct{})
	func() {
		k := NewObject()
		runtime.AddCleanup(k.o.identity, func(ch chan struct{}) { close(ch) }, done)
		if _, e := m.o.weakmap.set(k, Number(1)); e != nil {
			t.Fatal(e)
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		m.o.weakmap.cleanup()
		select {
		case <-done:
			if len(m.o.weakmap.entries) != 0 {
				t.Fatal("dead entry retained")
			}
			runtime.KeepAlive(m)
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatal("key was retained")
}
