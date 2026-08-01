package gots

import (
	"runtime"
	"strconv"
	"weak"
)

type weakMapKey struct{ identity weak.Pointer[weakIdentity] }
type weakMapData struct{ entries map[weakMapKey]Value }

func (r *Runtime) installWeakMapBuiltin() {
	r.global.define("WeakMap", newWeakMapConstructor(), false)
}

func newWeakMapConstructor() Value {
	constructor := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		weakMap := newWeakMapValue()
		iterable := argument(args, 0)
		if iterable.IsUndefined() || iterable.Kind() == KindNull {
			return weakMap, nil
		}
		if iterable.k != KindObject || !iterable.o.array {
			return Undefined(), &RuntimeError{Message: "WeakMap iterable must be an array"}
		}

		length := int(number(iterable.o.props["length"]))
		for index := 0; index < length; index++ {
			entry, _ := property(iterable, strconv.Itoa(index))
			if entry.k != KindObject || !entry.o.array || int(number(entry.o.props["length"])) < 2 {
				return Undefined(), &RuntimeError{Message: "WeakMap entry must be a key-value pair"}
			}
			if _, err := weakMap.o.weakmap.set(entry.o.props["0"], entry.o.props["1"]); err != nil {
				return Undefined(), err
			}
		}
		return weakMap, nil
	})
	constructor.f.constructOnly = true
	return constructor
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
		return Undefined(), &RuntimeError{Message: "WeakMap key must be an object"}
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

func newWeakMapValue() Value {
	weakMap := NewObject()
	weakMap.o.weakmap = &weakMapData{entries: map[weakMapKey]Value{}}
	installWeakMethods(weakMap)
	return weakMap
}

func installWeakMethods(weakMap Value) {
	weakMap.o.props["set"] = weakMapMethod(func(_ *Runtime, data *weakMapData, receiver Value, args []Value) (Value, error) {
		if len(args) < 2 {
			return Undefined(), &RuntimeError{Message: "WeakMap.set requires two arguments"}
		}
		if _, err := data.set(args[0], args[1]); err != nil {
			return Undefined(), err
		}
		return receiver, nil
	})
	weakMap.o.props["get"] = weakMapMethod(func(_ *Runtime, data *weakMapData, _ Value, args []Value) (Value, error) {
		if len(args) == 0 {
			return Undefined(), nil
		}
		value, _ := data.get(args[0])
		return value, nil
	})
	weakMap.o.props["has"] = weakMapMethod(func(_ *Runtime, data *weakMapData, _ Value, args []Value) (Value, error) {
		if len(args) == 0 {
			return Boolean(false), nil
		}
		_, found := data.get(args[0])
		return Boolean(found), nil
	})
	weakMap.o.props["delete"] = weakMapMethod(func(_ *Runtime, data *weakMapData, _ Value, args []Value) (Value, error) {
		if len(args) == 0 {
			return Boolean(false), nil
		}
		return Boolean(data.delete(args[0])), nil
	})
	weakMap.o.props["getOrInsert"] = weakMapMethod(func(_ *Runtime, data *weakMapData, _ Value, args []Value) (Value, error) {
		if len(args) < 2 {
			return Undefined(), &RuntimeError{Message: "WeakMap.getOrInsert requires two arguments"}
		}
		if existing, found := data.get(args[0]); found {
			return existing, nil
		}
		if _, err := data.set(args[0], args[1]); err != nil {
			return Undefined(), err
		}
		return args[1], nil
	})
	weakMap.o.props["getOrInsertComputed"] = weakMapMethod(func(runtime *Runtime, data *weakMapData, _ Value, args []Value) (Value, error) {
		if len(args) < 2 || args[1].Kind() != KindFunction {
			return Undefined(), &RuntimeError{Message: "WeakMap.getOrInsertComputed callback must be callable"}
		}
		if existing, found := data.get(args[0]); found {
			return existing, nil
		}
		value, err := runtime.Call(args[1], Undefined(), args[0])
		if err != nil {
			return Undefined(), err
		}
		if _, err = data.set(args[0], value); err != nil {
			return Undefined(), err
		}
		return value, nil
	})
}

type weakMapMethodBody func(*Runtime, *weakMapData, Value, []Value) (Value, error)

func weakMapMethod(body weakMapMethodBody) Value {
	return nativeValue(func(runtime *Runtime, receiver Value, args []Value) (Value, error) {
		data, err := weakReceiver(receiver)
		if err != nil {
			return Undefined(), err
		}
		return body(runtime, data, receiver, args)
	})
}
func weakReceiver(receiver Value) (*weakMapData, error) {
	if receiver.k != KindObject || receiver.o.weakmap == nil {
		return nil, &RuntimeError{Message: "WeakMap method called on incompatible receiver"}
	}
	return receiver.o.weakmap, nil
}
