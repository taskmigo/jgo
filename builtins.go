package gots

import "math"

type intrinsics struct {
	objectPrototype   *Object
	functionPrototype *Object
	arrayPrototype    *Object
	stringPrototype   *Object
	symbolPrototype   *Object
	weakMapPrototype  *Object
}

func (runtime *Runtime) installGlobalBuiltins() {
	runtime.intrinsics.objectPrototype = newObject(nil)
	runtime.intrinsics.functionPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.arrayPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.stringPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.symbolPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.stringPrototype.boxed = String("")
	runtime.intrinsics.weakMapPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.iteratorSymbol = newSymbol(jsString(String("Symbol.iterator").s), false).sy
	runtime.hasInstanceSymbol = newSymbol(jsString(String("Symbol.hasInstance").s), false).sy
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
		if receiver.k == KindObject && receiver.o.boxed.k == KindString {
			return String("[object String]"), nil
		}
		if receiver.k == KindObject && receiver.o.boxed.k == KindSymbol {
			return String("[object Symbol]"), nil
		}
		return String("[object Object]"), nil
	}), "toString", 0))
	defineBuiltin(runtime.intrinsics.objectPrototype, "hasOwnProperty", defineFunctionMetadata(nativeValue(func(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
		if receiver.k == KindNull || receiver.k == KindUndefined {
			return Undefined(), typeError("Object.prototype.hasOwnProperty called on null or undefined")
		}
		key, err := runtime.toPropertyKey(argument(arguments, 0))
		if err != nil {
			return Undefined(), err
		}
		_, found := ownProperty(receiver, key)
		if receiver.k == KindString && !key.isSymbol() {
			_, found = stringOwnProperty(receiver, key.name)
		}
		return Boolean(found), nil
	}), "hasOwnProperty", 1))
	defineBuiltin(runtime.intrinsics.objectPrototype, "propertyIsEnumerable", defineFunctionMetadata(nativeValue(func(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
		if receiver.k == KindNull || receiver.k == KindUndefined {
			return Undefined(), typeError("Object.prototype.propertyIsEnumerable called on null or undefined")
		}
		key, err := runtime.toPropertyKey(argument(arguments, 0))
		if err != nil {
			return Undefined(), err
		}
		descriptor, found := ownProperty(receiver, key)
		if !found && receiver.k == KindString && !key.isSymbol() {
			_, found = stringOwnProperty(receiver, key.name)
			descriptor.Enumerable = found
		}
		return Boolean(found && descriptor.Enumerable), nil
	}), "propertyIsEnumerable", 1))
	defineBuiltin(runtime.intrinsics.objectPrototype, "toLocaleString", defineFunctionMetadata(nativeValue(func(runtime *Runtime, receiver Value, _ []Value) (Value, error) {
		method, _, err := getProperty(runtime, receiver, StringKey("toString"))
		if err != nil {
			return Undefined(), err
		}
		if method.k != KindFunction || method.f.call == nil {
			return Undefined(), typeError("toString is not callable")
		}
		return runtime.call(method, receiver, nil, Span{})
	}), "toLocaleString", 0))
	defineBuiltin(runtime.intrinsics.functionPrototype, "call", defineFunctionMetadata(nativeValue(func(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
		if receiver.k != KindFunction || receiver.f.call == nil {
			return Undefined(), typeError("Function.prototype.call called on incompatible receiver")
		}
		return runtime.call(receiver, argument(arguments, 0), arguments[min(1, len(arguments)):], Span{})
	}), "call", 1))
	runtime.intrinsics.functionPrototype.properties[PropertyKey{symbol: runtime.hasInstanceSymbol}] = PropertyDescriptor{
		Value: defineFunctionMetadata(nativeValue(functionHasInstance), "[Symbol.hasInstance]", 1), Configurable: false,
	}

	runtime.installObjectBuiltin()
	runtime.installSymbolBuiltin()
	runtime.installArrayBuiltin()
	runtime.installStringBuiltin()
	runtime.installWeakMapBuiltin()
	runtime.global.createMutableBinding("NaN", Number(math.NaN()))
	runtime.global.createMutableBinding("Infinity", Number(math.Inf(1)))
	runtime.installGlobalThis()
}

func functionHasInstance(runtime *Runtime, constructor Value, arguments []Value) (Value, error) {
	if constructor.k != KindFunction || constructor.f.call == nil {
		return Boolean(false), nil
	}
	candidate := argument(arguments, 0)
	object := objectRecord(candidate)
	if object == nil {
		return Boolean(false), nil
	}
	prototypeValue, _, err := getProperty(runtime, constructor, StringKey("prototype"))
	if err != nil {
		return Undefined(), err
	}
	prototype := objectRecord(prototypeValue)
	if prototype == nil {
		return Undefined(), typeError("function has non-object prototype in instanceof check")
	}
	for current := object.prototype; current != nil; current = current.prototype {
		if current == prototype {
			return Boolean(true), nil
		}
	}
	return Boolean(false), nil
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
	runtime.global.createMutableBinding("this", globalThis)
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

func linkConstructor(constructor Value, prototype *Object) {
	constructor.f.object.properties[StringKey("prototype")] = PropertyDescriptor{Value: Value{k: KindObject, o: prototype}}
	prototype.properties[StringKey("constructor")] = PropertyDescriptor{Value: constructor, Writable: true, Configurable: true}
}
