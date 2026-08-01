package gots

import (
	"runtime"
	"weak"
)

type weakMapKey struct{ identity weak.Pointer[weakIdentity] }
type weakMapData struct{ entries map[weakMapKey]Value }

func (interpreter *Runtime) installWeakMapBuiltin() {
	constructor := nativeValue(nil)
	constructor.f.object.prototype = interpreter.intrinsics.functionPrototype
	constructor.f.construct = func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		weakMap := runtime.newWeakMap()
		iterable := argument(arguments, 0)
		if iterable.k == KindUndefined || iterable.k == KindNull {
			return weakMap, nil
		}
		entries, err := iterableEntries(runtime, iterable)
		if err != nil {
			return Undefined(), err
		}
		for _, entry := range entries {
			if _, err := weakMap.o.weakmap.set(entry[0], entry[1]); err != nil {
				return Undefined(), err
			}
		}
		return weakMap, nil
	}

	for name, method := range map[string]NativeFunction{
		"set": weakMapSet, "get": weakMapGet, "has": weakMapHas, "delete": weakMapDelete,
		"getOrInsert": weakMapGetOrInsert, "getOrInsertComputed": weakMapGetOrInsertComputed,
	} {
		length := 1
		if name == "set" || name == "getOrInsert" || name == "getOrInsertComputed" {
			length = 2
		}
		defineBuiltin(interpreter.intrinsics.weakMapPrototype, name, defineFunctionMetadata(nativeValue(method), name, length))
	}
	interpreter.global.createMutableBinding("WeakMap", defineFunctionMetadata(constructor, "WeakMap", 0))
}

func (interpreter *Runtime) newWeakMap() Value {
	return Value{k: KindObject, o: &Object{
		identity: newIdentity(), properties: make(map[PropertyKey]PropertyDescriptor),
		prototype: interpreter.intrinsics.weakMapPrototype,
		weakmap:   &weakMapData{entries: make(map[weakMapKey]Value)},
	}}
}

func iterableEntries(runtime *Runtime, iterable Value) ([][2]Value, error) {
	values, found, err := iteratorToList(runtime, iterable)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, typeError("WeakMap iterable is not iterable")
	}
	entries := make([][2]Value, 0, len(values))
	for _, entryValue := range values {
		entry := objectRecord(entryValue)
		if entry == nil || !entry.array {
			return nil, typeError("WeakMap iterator value is not an object")
		}
		key, _, err := getProperty(runtime, entryValue, StringKey("0"))
		if err != nil {
			return nil, err
		}
		value, _, err := getProperty(runtime, entryValue, StringKey("1"))
		if err != nil {
			return nil, err
		}
		entries = append(entries, [2]Value{key, value})
	}
	return entries, nil
}

func (data *weakMapData) cleanup() {
	for key := range data.entries {
		if key.identity.Value() == nil {
			delete(data.entries, key)
		}
	}
}

func (data *weakMapData) set(key, value Value) (Value, error) {
	data.cleanup()
	pointer, strongIdentity, valid := identity(key)
	if !valid {
		return Undefined(), typeError("WeakMap key must be an object or non-registered symbol")
	}
	data.entries[weakMapKey{pointer}] = value
	runtime.KeepAlive(strongIdentity)
	return value, nil
}

func (data *weakMapData) get(key Value) (Value, bool) {
	data.cleanup()
	pointer, strongIdentity, valid := identity(key)
	if !valid {
		return Undefined(), false
	}
	value, found := data.entries[weakMapKey{pointer}]
	runtime.KeepAlive(strongIdentity)
	return value, found
}

func (data *weakMapData) delete(key Value) bool {
	data.cleanup()
	pointer, strongIdentity, valid := identity(key)
	if !valid {
		return false
	}
	weakKey := weakMapKey{pointer}
	_, found := data.entries[weakKey]
	delete(data.entries, weakKey)
	runtime.KeepAlive(strongIdentity)
	return found
}

func weakMapReceiver(receiver Value) (*weakMapData, error) {
	if receiver.k != KindObject || receiver.o.weakmap == nil {
		return nil, typeError("WeakMap method called on incompatible receiver")
	}
	return receiver.o.weakmap, nil
}

func weakMapSet(_ *Runtime, receiver Value, arguments []Value) (Value, error) {
	data, err := weakMapReceiver(receiver)
	if err != nil {
		return Undefined(), err
	}
	if _, err = data.set(argument(arguments, 0), argument(arguments, 1)); err != nil {
		return Undefined(), err
	}
	return receiver, nil
}
func weakMapGet(_ *Runtime, receiver Value, arguments []Value) (Value, error) {
	data, err := weakMapReceiver(receiver)
	if err != nil {
		return Undefined(), err
	}
	value, _ := data.get(argument(arguments, 0))
	return value, nil
}
func weakMapHas(_ *Runtime, receiver Value, arguments []Value) (Value, error) {
	data, err := weakMapReceiver(receiver)
	if err != nil {
		return Undefined(), err
	}
	_, found := data.get(argument(arguments, 0))
	return Boolean(found), nil
}
func weakMapDelete(_ *Runtime, receiver Value, arguments []Value) (Value, error) {
	data, err := weakMapReceiver(receiver)
	if err != nil {
		return Undefined(), err
	}
	return Boolean(data.delete(argument(arguments, 0))), nil
}
func weakMapGetOrInsert(_ *Runtime, receiver Value, arguments []Value) (Value, error) {
	data, err := weakMapReceiver(receiver)
	if err != nil {
		return Undefined(), err
	}
	key := argument(arguments, 0)
	if value, found := data.get(key); found {
		return value, nil
	}
	value := argument(arguments, 1)
	_, err = data.set(key, value)
	return value, err
}
func weakMapGetOrInsertComputed(interpreter *Runtime, receiver Value, arguments []Value) (Value, error) {
	data, err := weakMapReceiver(receiver)
	if err != nil {
		return Undefined(), err
	}
	key := argument(arguments, 0)
	if _, _, valid := identity(key); !valid {
		return Undefined(), typeError("WeakMap key must be an object or non-registered symbol")
	}
	callback := argument(arguments, 1)
	if callback.k != KindFunction || callback.f.call == nil {
		return Undefined(), typeError("WeakMap.getOrInsertComputed callback must be callable")
	}
	if value, found := data.get(key); found {
		return value, nil
	}
	value, err := interpreter.call(callback, Undefined(), []Value{key}, Span{})
	if err != nil {
		return Undefined(), err
	}
	_, err = data.set(key, value)
	return value, err
}
