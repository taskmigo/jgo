package gots

import (
	"runtime"
	"testing"
	"time"
)

func TestWeakMapSemantics(t *testing.T) {
	runtime := New()
	value, err := runtime.RunString(`let k={}; let m=new WeakMap(); m.set(k, undefined); m.has(k) && m.get(k) === undefined;`)
	if err != nil || !value.Bool() {
		t.Fatal(value, err)
	}
	_, err = runtime.RunString(`new WeakMap().set(1,2)`)
	if err == nil {
		t.Fatal("primitive key accepted")
	}
	value, err = runtime.RunString(`let a={}; let b={}; let m=new WeakMap([[a,1],[b,2]]); m.get(a)+m.get(b)`)
	if err != nil || value.Float64() != 3 {
		t.Fatal(value, err)
	}
}

func TestWeakMapConstructorAndVarSemantics(t *testing.T) {
	runtime := New()
	value, err := runtime.RunString(`var a = new WeakMap(null); var k = {}; a.set(k, 7); a.get(k);`)
	if err != nil || value.Float64() != 7 {
		t.Fatalf("value=%v error=%v", value, err)
	}
	if _, err := runtime.RunString(`WeakMap()`); err == nil {
		t.Fatal("WeakMap call without new succeeded")
	}
}

func TestWeakMapSymbolAndGetOrInsert(t *testing.T) {
	runtime := New()
	value, err := runtime.RunString(`var m=new WeakMap(); var s=Symbol("key"); m.set(s, 1); m.getOrInsert(s, 2) + m.getOrInsert(Symbol(), 3)`)
	if err != nil || value.Float64() != 4 {
		t.Fatalf("value=%v error=%v", value, err)
	}
	if _, err := runtime.RunString(`new WeakMap().set(Symbol.for("registered"), 1)`); err == nil {
		t.Fatal("registered symbol accepted as weak key")
	}
}

func TestWeakMapDoesNotKeepKeyAlive(t *testing.T) {
	weakMap := newWeakMapValue()
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
