package gots

import (
	"math"
	"strconv"
)

func objectRecord(value Value) *Object {
	switch value.k {
	case KindObject:
		return value.o
	case KindFunction:
		return value.f.object
	default:
		return nil
	}
}

func getProperty(runtime *Runtime, value Value, key PropertyKey) (Value, bool, error) {
	if value.k == KindNull || value.k == KindUndefined {
		return Undefined(), false, typeError("cannot read property of null or undefined")
	}
	if value.k == KindString && !key.isSymbol() {
		if property, found := stringOwnProperty(value, key.name); found {
			return property, true, nil
		}
	}
	object := objectRecord(value)
	if object == nil && runtime != nil && value.k == KindString {
		object = runtime.intrinsics.stringPrototype
	}
	usedFunctionFallback := false
	for object != nil {
		if descriptor, found := object.properties[key]; found {
			return descriptor.Value, true, nil
		}
		if object.prototype == nil && runtime != nil && value.k == KindFunction && !usedFunctionFallback && object != runtime.intrinsics.functionPrototype {
			object = runtime.intrinsics.functionPrototype
			usedFunctionFallback = true
		} else {
			object = object.prototype
		}
	}
	return Undefined(), false, nil
}

func defineProperty(value Value, key PropertyKey, descriptor PropertyDescriptor) error {
	object := objectRecord(value)
	if object == nil {
		return typeError("value is not an object")
	}
	if current, found := object.properties[key]; found && !current.Configurable {
		if descriptor.Configurable || descriptor.Enumerable != current.Enumerable || (!current.Writable && descriptor.Writable) {
			return typeError("cannot redefine non-configurable property")
		}
		if !current.Writable && !SameValue(current.Value, descriptor.Value) {
			return typeError("cannot assign to read-only property")
		}
	}
	if object.array && !key.isSymbol() && key.name == "length" {
		return defineArrayLength(object, descriptor)
	}
	if object.array && !key.isSymbol() {
		if index, ok := arrayIndex(key.name); ok {
			length := arrayLength(object)
			if index >= length {
				object.properties[StringKey("length")] = PropertyDescriptor{Value: Number(float64(index + 1)), Writable: true}
			}
		}
	}
	object.properties[key] = descriptor
	return nil
}

func setProperty(value Value, key PropertyKey, propertyValue Value) error {
	object := objectRecord(value)
	if object == nil {
		return typeError("value is not an object")
	}
	if current, found := object.properties[key]; found {
		if !current.Writable {
			return typeError("cannot assign to read-only property")
		}
		current.Value = propertyValue
		if object.array && !key.isSymbol() && key.name == "length" {
			return defineArrayLength(object, current)
		}
		object.properties[key] = current
		return nil
	}
	for prototype := object.prototype; prototype != nil; prototype = prototype.prototype {
		if inherited, found := prototype.properties[key]; found && !inherited.Writable {
			return typeError("cannot assign to read-only property")
		}
	}
	return defineProperty(value, key, defaultProperty(propertyValue))
}

func defineArrayLength(object *Object, descriptor PropertyDescriptor) error {
	number, err := toNumber(descriptor.Value)
	if err != nil || number < 0 || number > math.MaxUint32 || number != math.Trunc(number) {
		return &Exception{Name: "RangeError", Message: "invalid array length"}
	}
	newLength := int(number)
	oldLength := arrayLength(object)
	if newLength < oldLength {
		for propertyKey := range object.properties {
			if propertyKey.isSymbol() {
				continue
			}
			if index, isIndex := arrayIndex(propertyKey.name); isIndex && index >= newLength {
				delete(object.properties, propertyKey)
			}
		}
	}
	descriptor.Value = Number(number)
	descriptor.Enumerable = false
	descriptor.Configurable = false
	object.properties[StringKey("length")] = descriptor
	return nil
}

func (runtime *Runtime) GetProperty(value, key Value) (Value, bool, error) {
	propertyKey, err := runtime.toPropertyKey(key)
	if err != nil {
		return Undefined(), false, err
	}
	return getProperty(runtime, value, propertyKey)
}

func (runtime *Runtime) SetProperty(value, key Value, propertyValue Value) error {
	propertyKey, err := runtime.toPropertyKey(key)
	if err != nil {
		return err
	}
	return setProperty(value, propertyKey, propertyValue)
}

func (runtime *Runtime) DefineProperty(value, key Value, descriptor PropertyDescriptor) error {
	propertyKey, err := runtime.toPropertyKey(key)
	if err != nil {
		return err
	}
	return defineProperty(value, propertyKey, descriptor)
}

func ownProperty(value Value, key PropertyKey) (PropertyDescriptor, bool) {
	object := objectRecord(value)
	if object == nil {
		return PropertyDescriptor{}, false
	}
	descriptor, found := object.properties[key]
	return descriptor, found
}

func arrayIndex(name string) (int, bool) {
	index, err := strconv.Atoi(name)
	return index, err == nil && index >= 0 && strconv.Itoa(index) == name
}

func arrayLength(object *Object) int {
	descriptor, found := object.properties[StringKey("length")]
	if !found || descriptor.Value.k != KindNumber {
		return 0
	}
	return int(descriptor.Value.n)
}

func stringOwnProperty(value Value, name string) (Value, bool) {
	if name == "length" {
		return Number(float64(len(value.s))), true
	}
	index, ok := arrayIndex(name)
	if !ok || index >= len(value.s) {
		return Undefined(), false
	}
	return StringUTF16([]uint16{value.s[index]}), true
}
