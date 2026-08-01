package gots

func (runtime *Runtime) installSymbolBuiltin() {
	constructor := nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		description := jsString(nil)
		if !argument(arguments, 0).IsUndefined() {
			converted, err := runtime.toString(argument(arguments, 0))
			if err != nil {
				return Undefined(), err
			}
			description = converted
		}
		return newSymbol(description, false), nil
	})
	constructor.f.object.prototype = runtime.intrinsics.functionPrototype
	linkConstructor(constructor, runtime.intrinsics.symbolPrototype)
	defineBuiltin(runtime.intrinsics.symbolPrototype, "valueOf", defineFunctionMetadata(nativeValue(symbolValueOf), "valueOf", 0))
	defineBuiltin(runtime.intrinsics.symbolPrototype, "toString", defineFunctionMetadata(nativeValue(symbolToString), "toString", 0))
	defineBuiltin(constructor.f.object, "for", defineFunctionMetadata(nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		key, err := runtime.toString(argument(arguments, 0))
		if err != nil {
			return Undefined(), err
		}
		registryKey := symbolRegistryKey(key)
		if symbol, found := runtime.symbols[registryKey]; found {
			return Value{k: KindSymbol, sy: symbol}, nil
		}
		symbol := newSymbol(key, true)
		runtime.symbols[registryKey] = symbol.sy
		return symbol, nil
	}), "for", 1))
	defineBuiltin(constructor.f.object, "iterator", Value{k: KindSymbol, sy: runtime.iteratorSymbol})
	defineBuiltin(constructor.f.object, "hasInstance", Value{k: KindSymbol, sy: runtime.hasInstanceSymbol})
	runtime.global.createMutableBinding("Symbol", defineFunctionMetadata(constructor, "Symbol", 0))
}

func symbolValueOf(_ *Runtime, receiver Value, _ []Value) (Value, error) {
	if receiver.k == KindSymbol {
		return receiver, nil
	}
	if receiver.k == KindObject && receiver.o.boxed.k == KindSymbol {
		return receiver.o.boxed, nil
	}
	return Undefined(), typeError("Symbol.prototype.valueOf called on incompatible receiver")
}

func symbolToString(_ *Runtime, receiver Value, _ []Value) (Value, error) {
	symbol, err := symbolValueOf(nil, receiver, nil)
	if err != nil {
		return Undefined(), err
	}
	return String("Symbol(" + symbol.sy.description.goString() + ")"), nil
}

func symbolRegistryKey(key jsString) string {
	encoded := make([]byte, len(key)*2)
	for index, codeUnit := range key {
		encoded[index*2] = byte(codeUnit)
		encoded[index*2+1] = byte(codeUnit >> 8)
	}
	return string(encoded)
}

func newSymbol(description jsString, registered bool) Value {
	return Value{k: KindSymbol, sy: &symbolValue{identity: newIdentity(), description: description.clone(), registered: registered}}
}

func (runtime *Runtime) installObjectBuiltin() {
	constructor := nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		value := argument(arguments, 0)
		if objectRecord(value) != nil {
			return value, nil
		}
		if value.k != KindNull && value.k != KindUndefined {
			prototype := runtime.intrinsics.objectPrototype
			if value.k == KindSymbol {
				prototype = runtime.intrinsics.symbolPrototype
			}
			object := newObject(prototype)
			object.boxed = value
			return Value{k: KindObject, o: object}, nil
		}
		return runtime.newOrdinaryObject(), nil
	})
	constructor.f.construct = constructor.f.call
	constructor.f.object.prototype = runtime.intrinsics.functionPrototype
	linkConstructor(constructor, runtime.intrinsics.objectPrototype)

	defineBuiltin(constructor.f.object, "hasOwn", defineFunctionMetadata(nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		object := argument(arguments, 0)
		if object.k == KindNull || object.k == KindUndefined {
			return Undefined(), typeError("Object.hasOwn called on null or undefined")
		}
		key, err := runtime.toPropertyKey(argument(arguments, 1))
		if err != nil {
			return Undefined(), err
		}
		_, found := ownProperty(object, key)
		if object.k == KindString && !key.isSymbol() {
			_, found = stringOwnProperty(object, key.name)
		}
		return Boolean(found), nil
	}), "hasOwn", 2))
	defineBuiltin(constructor.f.object, "is", defineFunctionMetadata(nativeValue(func(_ *Runtime, _ Value, arguments []Value) (Value, error) {
		return Boolean(SameValue(argument(arguments, 0), argument(arguments, 1))), nil
	}), "is", 2))
	defineBuiltin(constructor.f.object, "defineProperty", defineFunctionMetadata(nativeValue(objectDefineProperty), "defineProperty", 3))
	defineBuiltin(constructor.f.object, "getOwnPropertyDescriptor", defineFunctionMetadata(nativeValue(objectGetOwnPropertyDescriptor), "getOwnPropertyDescriptor", 2))
	defineBuiltin(constructor.f.object, "create", defineFunctionMetadata(nativeValue(objectCreate), "create", 2))
	defineBuiltin(constructor.f.object, "getPrototypeOf", defineFunctionMetadata(nativeValue(objectGetPrototypeOf), "getPrototypeOf", 1))
	defineBuiltin(constructor.f.object, "setPrototypeOf", defineFunctionMetadata(nativeValue(objectSetPrototypeOf), "setPrototypeOf", 2))
	defineBuiltin(constructor.f.object, "preventExtensions", defineFunctionMetadata(nativeValue(objectPreventExtensions), "preventExtensions", 1))
	defineBuiltin(constructor.f.object, "isExtensible", defineFunctionMetadata(nativeValue(objectIsExtensible), "isExtensible", 1))
	runtime.global.createMutableBinding("Object", defineFunctionMetadata(constructor, "Object", 1))
}

func objectPreventExtensions(_ *Runtime, _ Value, arguments []Value) (Value, error) {
	target := argument(arguments, 0)
	object := objectRecord(target)
	if object == nil {
		return target, nil
	}
	object.extensible = false
	return target, nil
}

func objectIsExtensible(_ *Runtime, _ Value, arguments []Value) (Value, error) {
	object := objectRecord(argument(arguments, 0))
	return Boolean(object != nil && object.extensible), nil
}

func objectDefineProperty(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
	target := argument(arguments, 0)
	if objectRecord(target) == nil {
		return Undefined(), typeError("Object.defineProperty called on non-object")
	}
	key, err := runtime.toPropertyKey(argument(arguments, 1))
	if err != nil {
		return Undefined(), err
	}
	descriptorObject := argument(arguments, 2)
	if objectRecord(descriptorObject) == nil {
		return Undefined(), typeError("property descriptor must be an object")
	}
	descriptor := PropertyDescriptor{}
	if current, found := ownProperty(target, key); found {
		descriptor = current
	}
	if value, found, err := getProperty(runtime, descriptorObject, StringKey("value")); err != nil {
		return Undefined(), err
	} else if found {
		descriptor.Value = value
	}
	_, hasGetter, err := getProperty(runtime, descriptorObject, StringKey("get"))
	if err != nil {
		return Undefined(), err
	}
	_, hasSetter, err := getProperty(runtime, descriptorObject, StringKey("set"))
	if err != nil {
		return Undefined(), err
	}
	if hasGetter || hasSetter {
		return Undefined(), typeError("accessor property descriptors are not supported")
	}
	for name, destination := range map[string]*bool{"writable": &descriptor.Writable, "enumerable": &descriptor.Enumerable, "configurable": &descriptor.Configurable} {
		value, found, err := getProperty(runtime, descriptorObject, StringKey(name))
		if err != nil {
			return Undefined(), err
		}
		if found {
			*destination = toBoolean(value)
		}
	}
	if err := defineProperty(target, key, descriptor); err != nil {
		return Undefined(), err
	}
	return target, nil
}

func objectGetOwnPropertyDescriptor(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
	target := argument(arguments, 0)
	if objectRecord(target) == nil && target.k != KindString {
		return Undefined(), typeError("Object.getOwnPropertyDescriptor called on non-object")
	}
	key, err := runtime.toPropertyKey(argument(arguments, 1))
	if err != nil {
		return Undefined(), err
	}
	descriptor, found := ownProperty(target, key)
	if !found && target.k == KindString && !key.isSymbol() {
		if value, stringFound := stringOwnProperty(target, key.name); stringFound {
			descriptor = PropertyDescriptor{Value: value, Enumerable: true}
			found = true
		}
	}
	if !found {
		return Undefined(), nil
	}
	result := runtime.newOrdinaryObject()
	for name, value := range map[string]Value{
		"value": descriptor.Value, "writable": Boolean(descriptor.Writable),
		"enumerable": Boolean(descriptor.Enumerable), "configurable": Boolean(descriptor.Configurable),
	} {
		_ = defineProperty(result, StringKey(name), defaultProperty(value))
	}
	return result, nil
}

func objectCreate(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
	prototypeValue := argument(arguments, 0)
	var prototype *Object
	if prototypeValue.k != KindNull {
		prototype = objectRecord(prototypeValue)
		if prototype == nil {
			return Undefined(), typeError("Object prototype may only be an object or null")
		}
	}
	created := Value{k: KindObject, o: newObject(prototype)}
	properties := argument(arguments, 1)
	if properties.IsUndefined() {
		return created, nil
	}
	if properties.k == KindNull {
		return Undefined(), typeError("Object.create properties cannot be null")
	}
	if properties.k == KindString {
		for index := range len(properties.s) {
			descriptorValue, _, err := getProperty(runtime, properties, StringKey(fmtInt(index)))
			if err != nil {
				return Undefined(), err
			}
			if _, err := objectDefineProperty(runtime, Undefined(), []Value{created, String(fmtInt(index)), descriptorValue}); err != nil {
				return Undefined(), err
			}
		}
		return created, nil
	}
	propertiesObject := objectRecord(properties)
	if propertiesObject == nil {
		return created, nil
	}
	for key, property := range propertiesObject.properties {
		if !property.Enumerable {
			continue
		}
		keyValue := String(key.name)
		if key.isSymbol() {
			keyValue = Value{k: KindSymbol, sy: key.symbol}
		}
		if _, err := objectDefineProperty(runtime, Undefined(), []Value{created, keyValue, property.Value}); err != nil {
			return Undefined(), err
		}
	}
	return created, nil
}

func objectGetPrototypeOf(_ *Runtime, _ Value, arguments []Value) (Value, error) {
	object := objectRecord(argument(arguments, 0))
	if object == nil {
		return Undefined(), typeError("Object.getPrototypeOf called on non-object")
	}
	if object.prototype == nil {
		return Null(), nil
	}
	return Value{k: KindObject, o: object.prototype}, nil
}

func objectSetPrototypeOf(_ *Runtime, _ Value, arguments []Value) (Value, error) {
	target := argument(arguments, 0)
	object := objectRecord(target)
	if object == nil {
		return Undefined(), typeError("Object.setPrototypeOf called on non-object")
	}
	prototypeValue := argument(arguments, 1)
	var prototype *Object
	if prototypeValue.k != KindNull {
		prototype = objectRecord(prototypeValue)
		if prototype == nil {
			return Undefined(), typeError("Object prototype may only be an object or null")
		}
	}
	for candidate := prototype; candidate != nil; candidate = candidate.prototype {
		if candidate == object {
			return Undefined(), typeError("cyclic prototype value")
		}
	}
	object.prototype = prototype
	return target, nil
}
