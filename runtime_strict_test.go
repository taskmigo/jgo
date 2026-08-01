package gots

import (
	"context"
	"errors"
	"testing"
)

func TestUseStrictDirectiveRecognition(t *testing.T) {
	tests := []struct {
		name   string
		source string
		strict bool
	}{
		{name: "double quoted", source: `"use strict";`, strict: true},
		{name: "single quoted", source: `'use strict';`, strict: true},
		{name: "after another directive", source: `"another directive"; "use strict";`, strict: true},
		{name: "escaped", source: `"use\x20strict";`, strict: false},
		{name: "parenthesized", source: `("use strict");`, strict: false},
		{name: "after prologue", source: `0; "use strict";`, strict: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program, err := New(Config{}).Compile(test.source)
			if err != nil {
				t.Fatal(err)
			}
			if program.strict != test.strict {
				t.Fatalf("strict = %v, want %v", program.strict, test.strict)
			}
		})
	}
}

func TestStrictnessControlsThisBinding(t *testing.T) {
	runtime := New(Config{})
	tests := []struct {
		source string
		want   Value
	}{
		{`function f() { return this; } f() === globalThis;`, Boolean(true)},
		{`"use strict"; function f() { return this; } f() === undefined;`, Boolean(true)},
		{`function f() { "use strict"; return this; } f() === undefined;`, Boolean(true)},
		{`"use\x20strict"; function f() { return this; } f() === globalThis;`, Boolean(true)},
		{`function f() { return typeof this; } f.call(3);`, String("object")},
		{`function f() { "use strict"; return typeof this; } f.call(3);`, String("number")},
	}
	for _, test := range tests {
		result, err := runtime.EvaluateString(context.Background(), test.source)
		if err != nil {
			t.Fatalf("%s: %v", test.source, err)
		}
		if !SameValue(result.Value, test.want) {
			t.Fatalf("%s = %s, want %s", test.source, result.Value.Inspect(), test.want.Inspect())
		}
	}
}

func TestStrictAssignmentFailures(t *testing.T) {
	runtime := New(Config{})
	result, err := runtime.EvaluateString(context.Background(), `sloppyGlobal = 3; sloppyGlobal;`)
	if err != nil || !SameValue(result.Value, Number(3)) {
		t.Fatalf("sloppy assignment result=%s err=%v", result.Value.Inspect(), err)
	}

	for _, source := range []string{
		`"use strict"; strictGlobal = 3;`,
		`"use strict"; let object = {}; Object.defineProperty(object, "x", {value: 1}); object.x = 2;`,
	} {
		_, err := New(Config{}).EvaluateString(context.Background(), source)
		var exception *Exception
		if !errors.As(err, &exception) || (exception.Name != "ReferenceError" && exception.Name != "TypeError") {
			t.Fatalf("%s: error = %v", source, err)
		}
	}

	result, err = New(Config{}).EvaluateString(context.Background(), `let object = {}; Object.defineProperty(object, "x", {value: 1}); object.x = 2; object.x;`)
	if err != nil || !SameValue(result.Value, Number(1)) {
		t.Fatalf("sloppy read-only assignment result=%s err=%v", result.Value.Inspect(), err)
	}
}

func TestDeleteUsesReferenceStrictness(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `let object = {value: 1}; delete object.value; typeof object.value;`)
	if err != nil || !SameValue(result.Value, String("undefined")) {
		t.Fatalf("configurable delete result=%s err=%v", result.Value.Inspect(), err)
	}

	result, err = New(Config{}).EvaluateString(context.Background(), `let object = {}; Object.defineProperty(object, "value", {value: 1}); delete object.value;`)
	if err != nil || !SameValue(result.Value, Boolean(false)) {
		t.Fatalf("sloppy non-configurable delete result=%s err=%v", result.Value.Inspect(), err)
	}

	_, err = New(Config{}).EvaluateString(context.Background(), `"use strict"; let object = {}; Object.defineProperty(object, "value", {value: 1}); delete object.value;`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Name != "TypeError" {
		t.Fatalf("strict non-configurable delete error=%v", err)
	}

	if _, err := New(Config{}).Compile(`"use strict"; delete binding;`); err == nil {
		t.Fatal("strict unqualified delete compiled")
	}
}

func TestSloppyWithUsesAnObjectEnvironment(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `let object = {value: 2}; let result = 0; with (object) { result = value; value = 3; } result + object.value;`)
	if err != nil || !SameValue(result.Value, Number(5)) {
		t.Fatalf("with result=%s err=%v", result.Value.Inspect(), err)
	}

	result, err = New(Config{}).EvaluateString(context.Background(), `let value = 4; with ({}) { value = value + 1; } value;`)
	if err != nil || !SameValue(result.Value, Number(5)) {
		t.Fatalf("with outer lookup result=%s err=%v", result.Value.Inspect(), err)
	}

	for _, source := range []string{
		`"use strict"; with ({}) {}`,
		`function strictFunction() { "use strict"; with ({}) {} }`,
	} {
		if _, err := New(Config{}).Compile(source); err == nil {
			t.Fatalf("strict with compiled: %s", source)
		}
	}
}

func TestDirectAndIndirectEvalEnvironments(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   Value
	}{
		{
			name:   "direct eval reads caller lexical binding",
			source: `function read() { let local = 4; return eval("local + 1"); } read();`,
			want:   Number(5),
		},
		{
			name:   "sloppy direct eval var reaches caller variable environment",
			source: `function declare() { eval("var local = 6;"); return local; } declare();`,
			want:   Number(6),
		},
		{
			name:   "eval lexical declarations never escape",
			source: `eval("let local = 1;"); typeof local;`,
			want:   String("undefined"),
		},
		{
			name:   "indirect eval uses global environment",
			source: `let local = 2; function read() { let local = 3; let indirect = eval; return indirect("typeof local"); } read();`,
			want:   String("number"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New(Config{}).EvaluateString(context.Background(), test.source)
			if err != nil || !SameValue(result.Value, test.want) {
				t.Fatalf("result=%s want=%s err=%v", result.Value.Inspect(), test.want.Inspect(), err)
			}
		})
	}

	_, err := New(Config{}).EvaluateString(context.Background(), `function strictEval() { "use strict"; eval("undeclared = 1;"); } strictEval();`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Name != "ReferenceError" {
		t.Fatalf("strict direct eval error=%v", err)
	}
}

func TestEvalReplacesAnEmptyCompletionWithUndefined(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `(0, eval)("var value = 1");`)
	if err != nil || !result.Value.IsUndefined() {
		t.Fatalf("eval var completion=%s err=%v", result.Value.Inspect(), err)
	}
}

func TestStrictDeleteOfNonConfigurableGlobalPropertyThrows(t *testing.T) {
	_, err := New(Config{}).EvaluateString(context.Background(), `"use strict"; delete globalThis.NaN;`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Name != "TypeError" {
		t.Fatalf("delete globalThis.NaN error=%v", err)
	}
}

func TestFunctionConstructorDeterminesItsOwnStrictness(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `"use strict"; Function("return this;")() === globalThis;`)
	if err != nil || !SameValue(result.Value, Boolean(true)) {
		t.Fatalf("sloppy constructed function result=%s err=%v", result.Value.Inspect(), err)
	}

	result, err = New(Config{}).EvaluateString(context.Background(), `Function("\"use strict\"; return this;")() === undefined;`)
	if err != nil || !SameValue(result.Value, Boolean(true)) {
		t.Fatalf("strict constructed function result=%s err=%v", result.Value.Inspect(), err)
	}

	result, err = New(Config{}).EvaluateString(context.Background(), `Function("left", "right", "return left + right;")(2, 3);`)
	if err != nil || !SameValue(result.Value, Number(5)) {
		t.Fatalf("constructed parameters result=%s err=%v", result.Value.Inspect(), err)
	}
}

func TestMappedAndUnmappedArgumentsObjects(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   Value
	}{
		{
			name:   "sloppy indexed assignment updates parameter",
			source: `function update(value) { arguments[0] = 3; return value; } update(1);`,
			want:   Number(3),
		},
		{
			name:   "sloppy parameter assignment updates index",
			source: `function update(value) { value = 4; return arguments[0]; } update(1);`,
			want:   Number(4),
		},
		{
			name:   "strict arguments are unmapped",
			source: `function update(value) { "use strict"; arguments[0] = 3; return value; } update(1);`,
			want:   Number(1),
		},
		{
			name:   "deleting an index removes its mapping",
			source: `function update(value) { delete arguments[0]; value = 3; return typeof arguments[0]; } update(1);`,
			want:   String("undefined"),
		},
		{
			name:   "duplicate parameter maps only its last index",
			source: `function update(value, value) { arguments[0] = 3; return value; } update(1, 2);`,
			want:   Number(2),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New(Config{}).EvaluateString(context.Background(), test.source)
			if err != nil || !SameValue(result.Value, test.want) {
				t.Fatalf("result=%s want=%s err=%v", result.Value.Inspect(), test.want.Inspect(), err)
			}
		})
	}

	for _, source := range []string{
		`function strictFunction() { "use strict"; return arguments.callee; } strictFunction();`,
		`function strictFunction() { "use strict"; } strictFunction.caller;`,
		`function strictFunction() { "use strict"; } strictFunction.arguments;`,
	} {
		_, err := New(Config{}).EvaluateString(context.Background(), source)
		var exception *Exception
		if !errors.As(err, &exception) || exception.Name != "TypeError" {
			t.Fatalf("restricted property source=%s error=%v", source, err)
		}
	}
}

func TestStrictModeEarlyErrors(t *testing.T) {
	tests := []string{
		`"use strict"; function f(a, a) {}`,
		`function f(eval) { "use strict"; }`,
		`"use strict"; let arguments = 1;`,
		`"use strict"; let yield = 1;`,
		`"use strict"; 010;`,
		`"use strict"; "\1";`,
		`"use strict"; eval = 1;`,
		`"use strict"; yield: 1;`,
		`"use strict"; if (true) function declaration() {}`,
		`"use strict"; { function duplicate() {} function duplicate() {} }`,
		`"use strict"; try {} catch (arguments) {}`,
		`"use strict"; void { set value(eval) {} };`,
	}
	for _, source := range tests {
		if _, err := New(Config{}).Compile(source); err == nil {
			t.Fatalf("strict early error accepted: %s", source)
		}
	}
	if _, err := New(Config{}).Compile(`function f(a, a) { return a; }`); err != nil {
		t.Fatalf("sloppy duplicate parameters rejected: %v", err)
	}
}

func TestStrictWritesToStandardReadOnlyPropertiesThrow(t *testing.T) {
	for _, source := range []string{
		`"use strict"; Number.MAX_VALUE = 1;`,
		`"use strict"; globalThis.undefined = 1;`,
	} {
		_, err := New(Config{}).EvaluateString(context.Background(), source)
		var exception *Exception
		if !errors.As(err, &exception) || exception.Name != "TypeError" {
			t.Fatalf("source=%s error=%v", source, err)
		}
	}
}

func TestStrictCallbacksKeepUndefinedThis(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `"use strict"; Array.from([1], function() { return this === undefined; })[0];`)
	if err != nil || !SameValue(result.Value, Boolean(true)) {
		t.Fatalf("Array.from strict callback result=%s err=%v", result.Value.Inspect(), err)
	}
}

func TestArrayToLocaleStringInvokesPrimitiveMethodsWithPrimitiveThis(t *testing.T) {
	result, err := New(Config{}).EvaluateString(context.Background(), `"use strict"; Boolean.prototype.toString = function() { return typeof this; }; [true, false].toLocaleString();`)
	if err != nil || !SameValue(result.Value, String("boolean,boolean")) {
		t.Fatalf("Array#toLocaleString result=%s err=%v", result.Value.Inspect(), err)
	}
}

func TestNestedEvaluationIsRejectedWithoutBreakingReentrantCalls(t *testing.T) {
	runtime := New(Config{})
	if err := runtime.Define("nestedEvaluate", NativeFunction(func(runtime *Runtime, _ Value, _ []Value) (Value, error) {
		_, err := runtime.EvaluateString(context.Background(), `1;`)
		return Undefined(), err
	})); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.EvaluateString(context.Background(), `nestedEvaluate();`)
	if !errors.Is(err, ErrRuntimeBusy) {
		t.Fatalf("nested Evaluate error = %v", err)
	}

	if err := runtime.Define("callAgain", NativeFunction(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		result, err := runtime.Call(context.Background(), argument(arguments, 0), Undefined())
		return result.Value, err
	})); err != nil {
		t.Fatal(err)
	}
	result, err := runtime.EvaluateString(context.Background(), `callAgain(function() { return 7; });`)
	if err != nil || !SameValue(result.Value, Number(7)) {
		t.Fatalf("reentrant Call result=%s err=%v", result.Value.Inspect(), err)
	}
}
