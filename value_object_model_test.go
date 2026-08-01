package gots

import (
	"context"
	"reflect"
	"testing"
)

func TestPropertyDescriptorPresenceAndConversion(t *testing.T) {
	runtime := New(Config{})
	object := runtime.newOrdinaryObject()
	key := StringKey("property")

	if err := definePropertyWithRuntime(runtime, object, key, PropertyDescriptor{Value: Number(1), HasValue: true}); err != nil {
		t.Fatal(err)
	}
	descriptor, found := ownProperty(object, key)
	if !found || !descriptor.HasValue || !descriptor.HasWritable || descriptor.Writable || !descriptor.HasEnumerable || descriptor.Enumerable || !descriptor.HasConfigurable || descriptor.Configurable {
		t.Fatalf("new descriptor was not completed with ECMAScript defaults: %+v", descriptor)
	}

	if err := definePropertyWithRuntime(runtime, object, key, PropertyDescriptor{Writable: true, HasWritable: true}); err == nil {
		t.Fatal("non-configurable, non-writable property became writable")
	}
}

func TestAccessorUsesOriginalReceiver(t *testing.T) {
	runtime := New(Config{})
	prototype := runtime.newOrdinaryObject()
	receiver := runtime.newOrdinaryObject()
	receiver.o.prototype = prototype.o
	var getterThis, setterThis, setterValue Value
	getter := nativeValue(func(_ *Runtime, this Value, _ []Value) (Value, error) {
		getterThis = this
		return Number(7), nil
	})
	setter := nativeValue(func(_ *Runtime, this Value, arguments []Value) (Value, error) {
		setterThis = this
		setterValue = argument(arguments, 0)
		return Undefined(), nil
	})
	descriptor := PropertyDescriptor{
		Get: getter, Set: setter, HasGet: true, HasSet: true,
		Configurable: true, HasConfigurable: true,
	}
	if err := definePropertyWithRuntime(runtime, prototype, StringKey("value"), descriptor); err != nil {
		t.Fatal(err)
	}

	value, found, err := runtime.GetProperty(receiver, String("value"))
	if err != nil || !found || !SameValue(value, Number(7)) || !SameValue(getterThis, receiver) {
		t.Fatalf("getter result=%s found=%v this=%s err=%v", value.Inspect(), found, getterThis.Inspect(), err)
	}
	if err := runtime.SetProperty(receiver, String("value"), Number(9)); err != nil {
		t.Fatal(err)
	}
	if !SameValue(setterThis, receiver) || !SameValue(setterValue, Number(9)) {
		t.Fatalf("setter this=%s value=%s", setterThis.Inspect(), setterValue.Inspect())
	}
}

func TestOwnPropertyKeyOrdering(t *testing.T) {
	runtime := New(Config{})
	object := runtime.newOrdinaryObject()
	symbol := newSymbol(jsString(String("key").s), false)
	for _, key := range []PropertyKey{StringKey("b"), StringKey("2"), StringKey("1"), StringKey("a")} {
		if err := defineProperty(object, key, defaultProperty(Undefined())); err != nil {
			t.Fatal(err)
		}
	}
	symbolKey, _ := SymbolKey(symbol)
	if err := defineProperty(object, symbolKey, defaultProperty(Undefined())); err != nil {
		t.Fatal(err)
	}
	keys := ordinaryOwnPropertyKeys(object.o)
	want := []PropertyKey{StringKey("1"), StringKey("2"), StringKey("b"), StringKey("a"), symbolKey}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
	if !ordinaryDelete(object.o, StringKey("b")) {
		t.Fatal("delete failed")
	}
	if err := defineProperty(object, StringKey("b"), defaultProperty(Undefined())); err != nil {
		t.Fatal(err)
	}
	keys = ordinaryOwnPropertyKeys(object.o)
	want = []PropertyKey{StringKey("1"), StringKey("2"), StringKey("a"), StringKey("b"), symbolKey}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys after re-creation = %#v, want %#v", keys, want)
	}
}

func TestPropertyKeysPreserveLoneSurrogates(t *testing.T) {
	runtime := New(Config{})
	object := runtime.newOrdinaryObject()
	loneSurrogate := StringUTF16([]uint16{0xd800})
	replacementCharacter := String("\ufffd")
	if err := runtime.SetProperty(object, loneSurrogate, Number(1)); err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetProperty(object, replacementCharacter, Number(2)); err != nil {
		t.Fatal(err)
	}
	first, _, err := runtime.GetProperty(object, loneSurrogate)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := runtime.GetProperty(object, replacementCharacter)
	if err != nil {
		t.Fatal(err)
	}
	if !SameValue(first, Number(1)) || !SameValue(second, Number(2)) {
		t.Fatalf("property keys collided: first=%s second=%s", first.Inspect(), second.Inspect())
	}
}

func TestAccessorDescriptorThroughObjectDefineProperty(t *testing.T) {
	runtime := New(Config{})
	result, err := runtime.EvaluateString(context.Background(), `
		let object = {};
		Object.defineProperty(object, "answer", { get: function() { return 42; } });
		object.answer;
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !SameValue(result.Value, Number(42)) {
		t.Fatalf("result = %s", result.Value.Inspect())
	}
}

func TestPrimitiveBoxingUsesRealmPrototypes(t *testing.T) {
	runtime := New(Config{})
	tests := []struct {
		source string
		want   Value
	}{
		{`Object(true).valueOf();`, Boolean(true)},
		{`Object(12).valueOf();`, Number(12)},
		{`Object("text").valueOf();`, String("text")},
		{`true.toString();`, String("true")},
		{`(12).toString();`, String("12")},
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
