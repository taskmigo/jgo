package gots

import "runtime"
import "weak"

type weakMapKey struct{ identity weak.Pointer[weakIdentity] }
type weakMapData struct{ entries map[weakMapKey]Value }

func (d *weakMapData) cleanup() {
	for k := range d.entries {
		if k.identity.Value() == nil {
			delete(d.entries, k)
		}
	}
}
func (d *weakMapData) set(k, v Value) (Value, error) {
	d.cleanup()
	p, id, ok := identity(k)
	if !ok {
		return Undefined(), &RuntimeError{Message: "WeakMap key must be an object"}
	}
	d.entries[weakMapKey{p}] = v
	runtime.KeepAlive(id)
	return v, nil
}
func (d *weakMapData) get(k Value) (Value, bool) {
	d.cleanup()
	p, id, ok := identity(k)
	if !ok {
		return Undefined(), false
	}
	v, found := d.entries[weakMapKey{p}]
	runtime.KeepAlive(id)
	return v, found
}
func (d *weakMapData) delete(k Value) bool {
	d.cleanup()
	p, id, ok := identity(k)
	if !ok {
		return false
	}
	key := weakMapKey{p}
	_, found := d.entries[key]
	delete(d.entries, key)
	runtime.KeepAlive(id)
	return found
}
func newWeakMapValue() Value {
	o := NewObject()
	o.o.weakmap = &weakMapData{entries: map[weakMapKey]Value{}}
	installWeakMethods(o)
	return o
}
func installWeakMethods(v Value) {
	v.o.props["set"] = nativeValue(func(_ *Runtime, this Value, args []Value) (Value, error) {
		d, e := weakReceiver(this)
		if e != nil {
			return Undefined(), e
		}
		if len(args) < 2 {
			return Undefined(), &RuntimeError{Message: "WeakMap.set requires two arguments"}
		}
		_, e = d.set(args[0], args[1])
		if e != nil {
			return Undefined(), e
		}
		return this, nil
	})
	v.o.props["get"] = nativeValue(func(_ *Runtime, this Value, args []Value) (Value, error) {
		d, e := weakReceiver(this)
		if e != nil {
			return Undefined(), e
		}
		if len(args) == 0 {
			return Undefined(), nil
		}
		x, _ := d.get(args[0])
		return x, nil
	})
	v.o.props["has"] = nativeValue(func(_ *Runtime, this Value, args []Value) (Value, error) {
		d, e := weakReceiver(this)
		if e != nil {
			return Undefined(), e
		}
		if len(args) == 0 {
			return Boolean(false), nil
		}
		_, ok := d.get(args[0])
		return Boolean(ok), nil
	})
	v.o.props["delete"] = nativeValue(func(_ *Runtime, this Value, args []Value) (Value, error) {
		d, e := weakReceiver(this)
		if e != nil {
			return Undefined(), e
		}
		if len(args) == 0 {
			return Boolean(false), nil
		}
		return Boolean(d.delete(args[0])), nil
	})
	v.o.props["getOrInsert"] = nativeValue(func(_ *Runtime, this Value, args []Value) (Value, error) {
		d, e := weakReceiver(this)
		if e != nil {
			return Undefined(), e
		}
		if len(args) < 2 {
			return Undefined(), &RuntimeError{Message: "WeakMap.getOrInsert requires two arguments"}
		}
		if old, ok := d.get(args[0]); ok {
			return old, nil
		}
		if _, e = d.set(args[0], args[1]); e != nil {
			return Undefined(), e
		}
		return args[1], nil
	})
	v.o.props["getOrInsertComputed"] = nativeValue(func(rt *Runtime, this Value, args []Value) (Value, error) {
		d, e := weakReceiver(this)
		if e != nil {
			return Undefined(), e
		}
		if len(args) < 2 || args[1].Kind() != KindFunction {
			return Undefined(), &RuntimeError{Message: "WeakMap.getOrInsertComputed callback must be callable"}
		}
		if old, ok := d.get(args[0]); ok {
			return old, nil
		}
		value, e := rt.Call(args[1], Undefined(), args[0])
		if e != nil {
			return Undefined(), e
		}
		if _, e = d.set(args[0], value); e != nil {
			return Undefined(), e
		}
		return value, nil
	})
}
func weakReceiver(v Value) (*weakMapData, error) {
	if v.k != KindObject || v.o.weakmap == nil {
		return nil, &RuntimeError{Message: "WeakMap method called on incompatible receiver"}
	}
	return v.o.weakmap, nil
}
