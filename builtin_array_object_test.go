package gots

import (
	"context"
	"strings"
	"testing"
)

func TestArrayStringAndObjectBuiltins(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `let arrayLike={length:3}; arrayLike[0]="a"; arrayLike[2]="c"; Array.of(1,2,3).at(-1) === 3 && Array.from("ab").includes("b") && [1,null,undefined,4].join("-") === "1---4" && Array.prototype.join.call(arrayLike) === "a,,c" && "  gots  ".trimStart().trimEnd().padStart(6,"-").replaceAll("-","+") === "++gots" && Object.hasOwn({x:1},"x") && globalThis.globalThis === globalThis`)
	if err != nil || !result.Value.ToBoolean() {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
}

func TestObjectIs(t *testing.T) {
	runtime := New(Config{})
	source := `
		let object = {};
		Object.is(NaN, NaN) && !Object.is(+0, -0) && Object.is(-0, -0) &&
		Object.is() && Object.is(undefined) && Object.is(null, null) &&
		!Object.is(undefined, null) && Object.is("x", "x") && !Object.is("x", "y") &&
		Object.is(object, object) && !Object.is({}, {}) && Object.is(1, 1, 2, 3) &&
		Object.is.call(null, NaN, NaN) && !Object.is.call({}, +0, -0) &&
		!Object.is(Symbol(), Symbol()) &&
		Object.is.length === 2 && Object.is.name === "is"`
	result, err := runtime.EvaluateString(context.Background(), source)
	if err != nil || !result.Value.ToBoolean() {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
	if _, err := runtime.EvaluateString(context.Background(), `new Object.is()`); err == nil {
		t.Fatal("Object.is was constructible")
	}
	result, err = runtime.EvaluateString(context.Background(), `Object.getOwnPropertyDescriptor(Object,"is").value === Object.is`)
	if err != nil || !result.Value.ToBoolean() {
		t.Fatalf("Object.getOwnPropertyDescriptor failed: value=%v error=%v", result.Value, err)
	}
}

func TestPropertyDescriptorsAndPrototypeChain(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let prototype={inherited:4}; let object=Object.create(prototype);
		Object.defineProperty(object,"fixed",{value:3,writable:false,enumerable:false,configurable:false});
		let descriptor=Object.getOwnPropertyDescriptor(object,"fixed");
		object.inherited === 4 && Object.hasOwn(object,"fixed") && descriptor.value === 3 &&
		!descriptor.writable && !descriptor.enumerable && !descriptor.configurable &&
		Object.getPrototypeOf(object) === prototype`)
	if !value.ToBoolean() {
		t.Fatalf("descriptor/prototype expression = %s", value.Inspect())
	}
}

func TestEqualityRegression(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `!(NaN === NaN) && +0 === -0 && [NaN].includes(NaN) && [+0].includes(-0)`)
	if err != nil || !result.Value.ToBoolean() {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
}

func TestBuiltinsPreserveMissingArgumentBehavior(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `Object.is() && "x".at() === "x" && "x".repeat() === ""`)
	if err != nil || !result.Value.ToBoolean() {
		t.Fatalf("value=%v error=%v", result.Value, err)
	}
	if _, err := New(Config{}).EvaluateString(context.Background(), `Array.from(undefined)`); err == nil || !strings.Contains(err.Error(), "source is not iterable") {
		t.Fatalf("unexpected Array.from(undefined) error: %v", err)
	}

	_, err = New(Config{}).EvaluateString(context.Background(), `let get = new WeakMap().get; get({})`)
	if err == nil || !strings.Contains(err.Error(), "WeakMap method called on incompatible receiver") {
		t.Fatalf("unexpected WeakMap receiver error: %v", err)
	}
}
