package gots

import "math"

type intrinsics struct {
	objectPrototype   *Object
	functionPrototype *Object
	arrayPrototype    *Object
	stringPrototype   *Object
	booleanPrototype  *Object
	numberPrototype   *Object
	symbolPrototype   *Object
	weakMapPrototype  *Object
	evalFunction      Value
	throwTypeError    Value
}

func (runtime *Runtime) installGlobalBuiltins() {
	runtime.intrinsics.objectPrototype = newObject(nil)
	runtime.intrinsics.functionPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.arrayPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.stringPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.booleanPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.numberPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.symbolPrototype = newObject(runtime.intrinsics.objectPrototype)
	runtime.intrinsics.stringPrototype.boxed = String("")
	runtime.intrinsics.booleanPrototype.boxed = Boolean(false)
	runtime.intrinsics.numberPrototype.boxed = Number(0)
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
		if receiver.k == KindObject && receiver.o.boxed.k == KindBoolean {
			return String("[object Boolean]"), nil
		}
		if receiver.k == KindObject && receiver.o.boxed.k == KindNumber {
			return String("[object Number]"), nil
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
			_, found = stringOwnProperty(receiver, key.goString())
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
			_, found = stringOwnProperty(receiver, key.goString())
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
	runtime.intrinsics.throwTypeError = defineFunctionMetadata(nativeValue(func(_ *Runtime, _ Value, _ []Value) (Value, error) {
		return Undefined(), typeError("restricted function property")
	}), "", 0)
	for _, name := range []string{"caller", "arguments"} {
		storeProperty(runtime.intrinsics.functionPrototype, StringKey(name), completePropertyDescriptor(PropertyDescriptor{
			Get: runtime.intrinsics.throwTypeError, Set: runtime.intrinsics.throwTypeError,
			HasGet: true, HasSet: true, Enumerable: false, Configurable: false,
			HasEnumerable: true, HasConfigurable: true,
		}))
	}
	defineBuiltin(runtime.intrinsics.functionPrototype, "toString", defineFunctionMetadata(nativeValue(func(_ *Runtime, receiver Value, _ []Value) (Value, error) {
		if receiver.k != KindFunction || receiver.f.call == nil {
			return Undefined(), typeError("Function.prototype.toString called on incompatible receiver")
		}
		if receiver.f.body != nil {
			return String("function " + receiver.f.name + "() { [ECMAScript code] }"), nil
		}
		return String("function () { [native code] }"), nil
	}), "toString", 0))
	storeProperty(runtime.intrinsics.functionPrototype, PropertyKey{symbol: runtime.hasInstanceSymbol}, dataProperty(
		defineFunctionMetadata(nativeValue(functionHasInstance), "[Symbol.hasInstance]", 1), false, false, false,
	))

	runtime.installObjectBuiltin()
	runtime.installBooleanBuiltin()
	runtime.installNumberBuiltin()
	runtime.installSymbolBuiltin()
	runtime.installArrayBuiltin()
	runtime.installStringBuiltin()
	runtime.installWeakMapBuiltin()
	runtime.installEvalAndFunctionBuiltins()
	runtime.global.bindings["NaN"] = binding{value: Number(math.NaN()), mutable: true, initialized: true, silentReadOnly: true}
	runtime.global.bindings["Infinity"] = binding{value: Number(math.Inf(1)), mutable: true, initialized: true, silentReadOnly: true}
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
	_ = defineProperty(globalThis, StringKey("NaN"), dataProperty(Number(math.NaN()), false, false, false))
	_ = defineProperty(globalThis, StringKey("Infinity"), dataProperty(Number(math.Inf(1)), false, false, false))
	_ = defineProperty(globalThis, StringKey("undefined"), dataProperty(Undefined(), false, false, false))
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
	storeProperty(object, StringKey(name), dataProperty(value, true, false, true))
}

func defineFunctionMetadata(functionValue Value, name string, length int) Value {
	defineBuiltin(functionValue.f.object, "name", String(name))
	storeProperty(functionValue.f.object, StringKey("length"), dataProperty(Number(float64(length)), false, false, true))
	return functionValue
}

func linkConstructor(constructor Value, prototype *Object) {
	storeProperty(constructor.f.object, StringKey("prototype"), dataProperty(Value{k: KindObject, o: prototype}, false, false, false))
	storeProperty(prototype, StringKey("constructor"), dataProperty(constructor, true, false, true))
}
