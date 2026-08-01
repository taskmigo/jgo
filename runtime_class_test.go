package gots

import (
	"context"
	"errors"
	"testing"
)

func TestBaseClassConstructionAndMethods(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		class Counter {
			constructor(value) { this.value = value; }
			read() { return this.value; }
			static create(value) { return new Counter(value); }
		}
		Counter.create(4).read();`)
	if !SameValue(value, Number(4)) {
		t.Fatalf("class result = %s", value.Inspect())
	}
}

func TestClassAccessorsAndComputedMethodNames(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let methodName = "read";
		class Box {
			constructor(value) { this.stored = value; }
			get value() { return this.stored; }
			set value(next) { this.stored = next; }
			[methodName]() { return this.value; }
		}
		let box = new Box(2); box.value = 5; box.read();`)
	if !SameValue(value, Number(5)) {
		t.Fatalf("class accessor result = %s", value.Inspect())
	}
}

func TestClassCodeIsStrictAndClassMethodsAreNotConstructors(t *testing.T) {
	for _, source := range []string{
		`class Strict { method() { undeclared = 1; } } new Strict().method();`,
		`class Methods { method() {} } new (new Methods().method)();`,
		`class Constructor {} Constructor();`,
	} {
		_, err := New(Config{}).EvaluateString(context.Background(), source)
		var exception *Exception
		if !errors.As(err, &exception) {
			t.Fatalf("source=%s error=%v", source, err)
		}
	}
}

func TestClassEarlyErrorsAndTDZ(t *testing.T) {
	for _, source := range []string{
		`class Duplicate { constructor() {} constructor() {} }`,
		`class Invalid { get constructor() {} }`,
		`class Invalid { static prototype() {} }`,
		`class DuplicatePrivate { #value; #value; }`,
		`class InvalidPrivate { #constructor; }`,
		`class UndeclaredPrivate { read() { return this.#missing; } }`,
	} {
		if _, err := New(Config{}).Compile(source); err == nil {
			t.Fatalf("invalid class compiled: %s", source)
		}
	}

	_, err := New(Config{}).EvaluateString(context.Background(), `ClassName; class ClassName {}`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Name != "ReferenceError" {
		t.Fatalf("class TDZ error = %v", err)
	}
}

func TestPrivateClassElementsAndBrands(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		class Counter {
			#value = 1;
			#increment() { this.#value = this.#value + 1; }
			get #current() { return this.#value; }
			set #current(value) { this.#value = value; }
			update(value) { this.#current = value; this.#increment(); return this.#current; }
		}
		new Counter().update(4);`)
	if !SameValue(value, Number(5)) {
		t.Fatalf("private class result = %s", value.Inspect())
	}

	_, err := New(Config{}).EvaluateString(context.Background(), `
		class Owner { #value = 1; read(value) { return value.#value; } }
		new Owner().read({});`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Name != "TypeError" {
		t.Fatalf("private brand error = %v", err)
	}
}

func TestStaticPrivateElementsAndStaticBlocks(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		class Counter {
			static #value = 2;
			static #read() { return this.#value; }
			static { this.answer = this.#read() + 3; }
		}
		Counter.answer;`)
	if !SameValue(value, Number(5)) {
		t.Fatalf("static class result = %s", value.Inspect())
	}
}

func TestClassNumericKeysNamedEvaluationAndFieldInExpressions(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let inferred;
		let values = [42];
		let C = class {
			get 1E+9() { return 1; }
			get 0.0000001() { return 2; }
			contains = 0 in values
			static capture = (inferred = this.name);
		};
		let instance = new C();
		instance["1000000000"] + instance["1e-7"] + instance.contains + (inferred === "C");`)
	if !SameValue(value, Number(5)) {
		t.Fatalf("class key/name/in result = %s", value.Inspect())
	}
}

func TestClassFieldEvalRestrictionsAndEmptyStatementCompletion(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `eval("class C {}{ label: 42 };");`)
	if !SameValue(value, Number(42)) {
		t.Fatalf("eval completion = %s", value.Inspect())
	}

	_, err := New(Config{}).EvaluateString(context.Background(), `
		class C { field = eval("arguments"); }
		new C();`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Name != "SyntaxError" {
		t.Fatalf("class field eval error = %v", err)
	}
}

func TestPublicClassFieldsUseDefinitionAndInitializationOrder(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let key = "value";
		class Fields {
			[key] = 2;
			doubled = this.value * 2;
			static answer = 6;
		}
		let instance = new Fields(); instance.doubled + Fields.answer;`)
	if !SameValue(value, Number(10)) {
		t.Fatalf("class fields result = %s", value.Inspect())
	}
}

func TestDerivedConstructorsSuperPropertiesAndNewTarget(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		class Base {
			constructor(value) { this.value = value; this.target = new.target; }
			read() { return this.value; }
			static label() { return 1; }
		}
		class Derived extends Base {
			constructor(value) { super(value + 1); }
			read() { return super.read() + 1; }
			static label() { return super.label() + 1; }
		}
		let instance = new Derived(2);
		instance.read() + Derived.label() + (instance.target === Derived);`)
	if !SameValue(value, Number(7)) {
		t.Fatalf("derived class result = %s", value.Inspect())
	}
}

func TestDefaultDerivedConstructorForwardsArguments(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		class Base { constructor(value) { this.value = value; } }
		class Derived extends Base {}
		new Derived(8).value;`)
	if !SameValue(value, Number(8)) {
		t.Fatalf("default derived constructor result = %s", value.Inspect())
	}
}

func TestDerivedFieldsInitializeAfterSuper(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		class Base { constructor() { this.base = 2; } }
		class Derived extends Base {
			field = this.base + 1;
			constructor() { super(); this.seen = this.field; }
		}
		new Derived().seen;`)
	if !SameValue(value, Number(3)) {
		t.Fatalf("derived field result = %s", value.Inspect())
	}
}

func TestDerivedThisBindingFailures(t *testing.T) {
	for _, source := range []string{
		`class Base {} class Derived extends Base { constructor() { this.value = 1; super(); } } new Derived();`,
		`class Base {} class Derived extends Base { constructor() {} } new Derived();`,
		`class Base {} class Derived extends Base { constructor() { super(); super(); } } new Derived();`,
	} {
		_, err := New(Config{}).EvaluateString(context.Background(), source)
		var exception *Exception
		if !errors.As(err, &exception) {
			t.Fatalf("source=%s error=%v", source, err)
		}
	}
}
