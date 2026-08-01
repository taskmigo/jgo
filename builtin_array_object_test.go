package gots

import (
	"strings"
	"testing"
)

func TestArrayStringAndObjectBuiltins(t *testing.T) {
	value, err := New().RunString(`Array.of(1,2,3).at(-1) === 3 && Array.from("ab").includes("b") && "  gots  ".trimStart().trimEnd().padStart(6,"-").replaceAll("-","+") === "++gots" && Object.hasOwn({x:1},"x") && globalThis.globalThis === globalThis`)
	if err != nil || !value.Bool() {
		t.Fatalf("value=%v error=%v", value, err)
	}
}

func TestObjectIs(t *testing.T) {
	runtime := New()
	source := `
		let object = {};
		Object.is(NaN, NaN) && !Object.is(+0, -0) && Object.is(-0, -0) &&
		Object.is() && Object.is(undefined) && Object.is(null, null) &&
		!Object.is(undefined, null) && Object.is("x", "x") && !Object.is("x", "y") &&
		Object.is(object, object) && !Object.is({}, {}) && Object.is(1, 1, 2, 3) &&
		Object.is.call(null, NaN, NaN) && !Object.is.call({}, +0, -0) &&
		!Object.is(Symbol(), Symbol()) &&
		Object.is.length === 2 && Object.is.name === "is"`
	value, err := runtime.RunString(source)
	if err != nil || !value.Bool() {
		t.Fatalf("value=%v error=%v", value, err)
	}
	if _, err := runtime.RunString(`new Object.is()`); err == nil {
		t.Fatal("Object.is was constructible")
	}
	if value, err := runtime.RunString(`Object.getOwnPropertyDescriptor`); err != nil || !value.IsUndefined() {
		t.Fatalf("partial Object.getOwnPropertyDescriptor leaked into the runtime: value=%v error=%v", value, err)
	}
}

func TestEqualityRegression(t *testing.T) {
	value, err := New().RunString(`!(NaN === NaN) && +0 === -0 && [NaN].includes(NaN) && [+0].includes(-0)`)
	if err != nil || !value.Bool() {
		t.Fatalf("value=%v error=%v", value, err)
	}
}

func TestBuiltinsPreserveMissingArgumentBehavior(t *testing.T) {
	value, err := New().RunString(`Object.is() && Array.from().length === 0 && "x".at() === "x" && "x".repeat() === ""`)
	if err != nil || !value.Bool() {
		t.Fatalf("value=%v error=%v", value, err)
	}
	if _, err := New().RunString(`Array.from(undefined)`); err == nil || !strings.Contains(err.Error(), "source is not iterable") {
		t.Fatalf("unexpected Array.from(undefined) error: %v", err)
	}

	_, err = New().RunString(`let get = new WeakMap().get; get({})`)
	if err == nil || !strings.Contains(err.Error(), "WeakMap method called on incompatible receiver") {
		t.Fatalf("unexpected WeakMap receiver error: %v", err)
	}
}
