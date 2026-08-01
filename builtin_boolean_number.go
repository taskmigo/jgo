package gots

func (runtime *Runtime) installBooleanBuiltin() {
	constructor := nativeValue(func(_ *Runtime, _ Value, arguments []Value) (Value, error) {
		return Boolean(toBoolean(argument(arguments, 0))), nil
	})
	constructor.f.object.prototype = runtime.intrinsics.functionPrototype
	constructor.f.construct = func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		object := newObject(runtime.intrinsics.booleanPrototype)
		object.boxed = Boolean(toBoolean(argument(arguments, 0)))
		return Value{k: KindObject, o: object}, nil
	}
	linkConstructor(constructor, runtime.intrinsics.booleanPrototype)
	defineBuiltin(runtime.intrinsics.booleanPrototype, "valueOf", defineFunctionMetadata(nativeValue(booleanValueOf), "valueOf", 0))
	defineBuiltin(runtime.intrinsics.booleanPrototype, "toString", defineFunctionMetadata(nativeValue(booleanToString), "toString", 0))
	runtime.global.createMutableBinding("Boolean", defineFunctionMetadata(constructor, "Boolean", 1))
}

func booleanValueOf(_ *Runtime, receiver Value, _ []Value) (Value, error) {
	if receiver.k == KindBoolean {
		return receiver, nil
	}
	if receiver.k == KindObject && receiver.o.boxed.k == KindBoolean {
		return receiver.o.boxed, nil
	}
	return Undefined(), typeError("Boolean.prototype.valueOf called on incompatible receiver")
}

func booleanToString(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	value, err := booleanValueOf(runtime, receiver, arguments)
	if err != nil {
		return Undefined(), err
	}
	if value.b {
		return String("true"), nil
	}
	return String("false"), nil
}

func (runtime *Runtime) installNumberBuiltin() {
	constructor := nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		if len(arguments) == 0 {
			return Number(0), nil
		}
		number, err := runtime.toNumber(arguments[0])
		return Number(number), err
	})
	constructor.f.object.prototype = runtime.intrinsics.functionPrototype
	constructor.f.construct = func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		primitive, err := constructor.f.call(runtime, Undefined(), arguments)
		if err != nil {
			return Undefined(), err
		}
		object := newObject(runtime.intrinsics.numberPrototype)
		object.boxed = primitive
		return Value{k: KindObject, o: object}, nil
	}
	linkConstructor(constructor, runtime.intrinsics.numberPrototype)
	storeProperty(constructor.f.object, StringKey("MAX_VALUE"), dataProperty(Number(1.7976931348623157e308), false, false, false))
	defineBuiltin(runtime.intrinsics.numberPrototype, "valueOf", defineFunctionMetadata(nativeValue(numberValueOf), "valueOf", 0))
	defineBuiltin(runtime.intrinsics.numberPrototype, "toString", defineFunctionMetadata(nativeValue(numberPrototypeToString), "toString", 1))
	runtime.global.createMutableBinding("Number", defineFunctionMetadata(constructor, "Number", 1))
}

func numberValueOf(_ *Runtime, receiver Value, _ []Value) (Value, error) {
	if receiver.k == KindNumber {
		return receiver, nil
	}
	if receiver.k == KindObject && receiver.o.boxed.k == KindNumber {
		return receiver.o.boxed, nil
	}
	return Undefined(), typeError("Number.prototype.valueOf called on incompatible receiver")
}

func numberPrototypeToString(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	value, err := numberValueOf(runtime, receiver, arguments)
	if err != nil {
		return Undefined(), err
	}
	return String(numberToString(value.n)), nil
}
