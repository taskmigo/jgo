package gots

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestWeakMapSemantics(t *testing.T) {
	interpreter := New(Config{})
	result, err := interpreter.EvaluateString(context.Background(), `let k={}; let m=new WeakMap(); m.set(k, undefined); m.has(k) && m.get(k) === undefined;`)
	if err != nil || !result.Value.ToBoolean() {
		t.Fatal(result.Value, err)
	}
	_, err = interpreter.EvaluateString(context.Background(), `new WeakMap().set(1,2)`)
	if err == nil {
		t.Fatal("primitive key accepted")
	}
	result, err = New(Config{}).EvaluateString(context.Background(), `let a={}; let b={}; let m=new WeakMap([[a,1],[b,2]]); m.get(a)+m.get(b)`)
	if number, _ := result.Value.ToNumber(); err != nil || number != 3 {
		t.Fatal(result.Value, err)
	}
}

func TestWeakMapConstructorAndVarSemantics(t *testing.T) {
	interpreter := New(Config{})
	result, err := interpreter.EvaluateString(context.Background(), `var a = new WeakMap(null); var k = {}; a.set(k, 7); a.get(k);`)
	if number, _ := result.Value.ToNumber(); err != nil || number != 7 {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
	if _, err := interpreter.EvaluateString(context.Background(), `WeakMap()`); err == nil {
		t.Fatal("WeakMap call without new succeeded")
	}
}

func TestWeakMapSymbolAndGetOrInsert(t *testing.T) {
	interpreter := New(Config{})
	result, err := interpreter.EvaluateString(context.Background(), `var m=new WeakMap(); var s=Symbol("key"); m.set(s, 1); m.getOrInsert(s, 2) + m.getOrInsert(Symbol(), 3)`)
	if number, _ := result.Value.ToNumber(); err != nil || number != 4 {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
	if _, err := interpreter.EvaluateString(context.Background(), `new WeakMap().set(Symbol.for("registered"), 1)`); err == nil {
		t.Fatal("registered symbol accepted as weak key")
	}
}

func TestWeakMapDoesNotKeepKeyAlive(t *testing.T) {
	weakMap := New(Config{}).newWeakMap()
	done := make(chan struct{})
	func() {
		key := NewObject()
		runtime.AddCleanup(key.o.identity, func(channel chan struct{}) { close(channel) }, done)
		if _, err := weakMap.o.weakmap.set(key, Number(1)); err != nil {
			t.Fatal(err)
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		weakMap.o.weakmap.cleanup()
		select {
		case <-done:
			if len(weakMap.o.weakmap.entries) != 0 {
				t.Fatal("dead entry retained")
			}
			runtime.KeepAlive(weakMap)
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatal("key was retained")
}
