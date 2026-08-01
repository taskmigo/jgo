package gots

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func evaluateForTest(t *testing.T, runtime *Runtime, source string) Value {
	t.Helper()
	result, err := runtime.EvaluateString(context.Background(), source)
	if err != nil {
		t.Fatalf("EvaluateString(%q): %v", source, err)
	}
	return result.Value
}

func TestECMAScriptCoercionAndEquality(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		!NaN && !(+"invalid" === +"invalid") && +"" === 0 && +"  0x10  " === 16 &&
		0x10 === 16 && 0o10 === 8 && 0b10 === 2 && 1e2 === 100 &&
		null == undefined && "1" == 1 && true == 1 && !(null === undefined)`)
	if !value.ToBoolean() {
		t.Fatalf("coercion expression = %s", value.Inspect())
	}

	number, err := String("invalid").ToNumber()
	if err != nil || !math.IsNaN(number) {
		t.Fatalf("ToNumber(invalid) = %v, %v", number, err)
	}
	if _, err := (Value{k: KindSymbol, sy: &symbolValue{identity: newIdentity()}}).ToNumber(); err == nil {
		t.Fatal("Symbol ToNumber did not throw")
	}
}

func TestLogicalOperatorsReturnOperandsWithoutCoercion(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `let symbol=Symbol(); symbol && true`)
	if !value.ToBoolean() {
		t.Fatalf("symbol && true = %s", value.Inspect())
	}
	value = evaluateForTest(t, New(Config{}), `let object={}; false || object`)
	if value.Kind() != KindObject {
		t.Fatalf("false || object = %s", value.Inspect())
	}
}

func TestObjectToPrimitiveHooks(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let numeric={valueOf:function(){return 7;}};
		let textual={toString:function(){return "field";}};
		let object={}; object[textual]=9;
		numeric+1 === 8 && String(textual) === "field" && object.field === 9 && !("2" < "10")`)
	if !value.ToBoolean() {
		t.Fatalf("object coercion expression = %s", value.Inspect())
	}
}

func TestReferenceEvaluatesMemberBaseOnce(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let calls=0; let object={method:function(){return 7;}};
		function getObject(){calls=calls+1; return object;}
		getObject().method() + calls`)
	number, _ := value.ToNumber()
	if number != 8 {
		t.Fatalf("result = %v, member base was evaluated more than once", number)
	}
}

func TestTemporalDeadZone(t *testing.T) {
	_, err := New(Config{}).EvaluateString(context.Background(), `value; let value=1`)
	var exception *Exception
	if !errors.As(err, &exception) || exception.Name != "ReferenceError" {
		t.Fatalf("error = %T %v", err, err)
	}
}

func TestUTF16StringSemantics(t *testing.T) {
	runtime := New(Config{})
	value := evaluateForTest(t, runtime, `"😀".length === 2 && "😀" === "\u{1F600}" && "😀"[0] === "\uD83D" && "😀"[1] === "\uDE00" && Array.from("😀").length === 1`)
	if !value.ToBoolean() {
		t.Fatalf("UTF-16 expression = %s", value.Inspect())
	}

	lone := evaluateForTest(t, runtime, `"\uD800"`)
	units, ok := lone.UTF16()
	if !ok || !reflect.DeepEqual(units, []uint16{0xd800}) {
		t.Fatalf("lone surrogate = %#v", units)
	}
	if value := evaluateForTest(t, New(Config{MaxSteps: 1_000}), `"".repeat(2147483647)`); value.Inspect() != "" {
		t.Fatalf("large empty repeat = %q", value.Inspect())
	}
	if value := evaluateForTest(t, New(Config{}), `"А" === "\А"`); !value.ToBoolean() {
		t.Fatal("non-ASCII NonEscapeSequence was not preserved")
	}
}

func TestDuplicateFunctionDeclarationUsesLastDeclaration(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `function f(){return 1;} function f(){return 2;} f()`)
	number, _ := value.ToNumber()
	if number != 2 {
		t.Fatalf("duplicate function result = %v", number)
	}
}

func TestSymbolRegistryAndSymbolPropertyKeys(t *testing.T) {
	runtime := New(Config{})
	if value := evaluateForTest(t, runtime, `Symbol.for("key") === Symbol.for("key")`); !value.ToBoolean() {
		t.Fatal("Symbol.for did not preserve identity")
	}
	symbol := evaluateForTest(t, runtime, `Symbol("property")`)
	object := runtime.newOrdinaryObject()
	if err := runtime.SetProperty(object, symbol, Number(42)); err != nil {
		t.Fatal(err)
	}
	value, found, err := runtime.GetProperty(object, symbol)
	number, _ := value.ToNumber()
	if err != nil || !found || number != 42 {
		t.Fatalf("symbol property = %v, %t, %v", number, found, err)
	}
}

func TestStringSymbolConstructionAndSymbolHasInstance(t *testing.T) {
	runtime := New(Config{})
	if _, err := runtime.EvaluateString(context.Background(), `new String(Symbol())`); err == nil {
		t.Fatal("new String(Symbol()) did not throw")
	}
	value := evaluateForTest(t, runtime, `
		function Constructor(){} let instance=new Constructor();
		Constructor[Symbol.hasInstance](instance)`)
	if !value.ToBoolean() {
		t.Fatal("Symbol.hasInstance did not follow the prototype chain")
	}
}

func TestArrayLengthTracksIndexedProperties(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `let values=[]; values[2]=3; let before=values.length; values.length=1; before === 3 && values[2] === undefined`)
	if !value.ToBoolean() {
		t.Fatalf("array length expression = %s", value.Inspect())
	}
}

func TestSynchronousIteratorProtocol(t *testing.T) {
	value := evaluateForTest(t, New(Config{}), `
		let iterable={};
		iterable[Symbol.iterator]=function(){
			let index=0;
			return {next:function(){index=index+1; if(index<3)return {value:index,done:false}; return {done:true};}};
		};
		let values=Array.from(iterable);
		values.length === 2 && values[0] === 1 && values[1] === 2`)
	if !value.ToBoolean() {
		t.Fatalf("custom iterator expression = %s", value.Inspect())
	}

	value = evaluateForTest(t, New(Config{}), `
		let key={}; let iterable={};
		iterable[Symbol.iterator]=function(){
			let done=false;
			return {next:function(){if(done)return {done:true}; done=true; return {value:[key,9],done:false};}};
		};
		let map=new WeakMap(iterable); map.get(key) === 9`)
	if !value.ToBoolean() {
		t.Fatalf("WeakMap iterator expression = %s", value.Inspect())
	}
}
