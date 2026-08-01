package gots

import "math"

type intrinsics struct {
	objectPrototype   *Object
	functionPrototype *Object
	arrayPrototype    *Object
	stringPrototype   *Object
	weakMapPrototype  *Object
}

func (runtime *Runtime) installGlobalBuiltins() {
	runtime.intrinsics.objectPrototype = newObject(nil)
	runtime.intrinsics.functionPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.arrayPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.stringPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.weakMapPrototype = newObject(runtime.intrinsics.objectPrototype)
	defineBuiltin(runtime.intrinsics.objectPrototype, "valueOf", defineFunctionMetadata(nativeValue(func(_ *Runtime, receiver Value, _ []Value) (Value, error) {
		if objectRecord(receiver) == nil {
			return Undefined(), typeError("Object.prototype.valueOf called on incompatible receiver")
		}
		return receiver, nil
	}), "valueOf", 0))
	defineBuiltin(runtime.intrinsics.objectPrototype, "toString", defineFunctionMetadata(nativeValue(func(_ *Runtime, receiver Value, _ []Value) (Value, error) {
		if objectRecord(receiver) == nil {
			return Undefined(), typeError("Object.prototype.toString called on incompatible receiver")
		}
		if receiver.k == KindObject && receiver.o.array {
			return String("[object Array]"), nil
		}
		return String("[object Object]"), nil
	}), "toString", 0))
	defineBuiltin(runtime.intrinsics.functionPrototype, "call", defineFunctionMetadata(nativeValue(func(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
		if receiver.k != KindFunction || receiver.f.call == nil {
			return Undefined(), typeError("Function.prototype.call called on incompatible receiver")
		}
		return runtime.call(receiver, argument(arguments, 0), arguments[min(1, len(arguments)):], Span{})
	}), "call", 1))

	runtime.installObjectBuiltin()
	runtime.installSymbolBuiltin()
	runtime.installArrayBuiltin()
	runtime.installStringBuiltin()
	runtime.installWeakMapBuiltin()
	runtime.global.createMutableBinding("NaN", Number(math.NaN()))
	runtime.global.createMutableBinding("Infinity", Number(math.Inf(1)))
	runtime.installGlobalThis()
}

func (runtime *Runtime) newOrdinaryObject() Value {
	return Value{k: KindObject, o: newObject(runtime.intrinsics.objectPrototype)}
}

func (runtime *Runtime) newArray(values ...Value) Value {
	array := NewArray(values...)
	array.o.prototype = runtime.intrinsics.arrayPrototype
	return array
}

func (runtime *Runtime) installGlobalThis() {
	globalThis := runtime.newOrdinaryObject()
	_ = defineProperty(globalThis, StringKey("globalThis"), defaultProperty(globalThis))
	runtime.global.createMutableBinding("globalThis", globalThis)
}

func argument(arguments []Value, index int) Value {
	if index >= len(arguments) {
		return Undefined()
	}
	return arguments[index]
}

func defineBuiltin(object *Object, name string, value Value) {
	object.properties[StringKey(name)] = PropertyDescriptor{Value: value, Writable: true, Configurable: true}
}

func defineFunctionMetadata(functionValue Value, name string, length int) Value {
	defineBuiltin(functionValue.f.object, "name", String(name))
	functionValue.f.object.properties[StringKey("length")] = PropertyDescriptor{Value: Number(float64(length)), Configurable: true}
	return functionValue
}
