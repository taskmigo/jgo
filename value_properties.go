package gots

import (
	"context"
	"math"
	"sort"
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

func isAccessorDescriptor(descriptor PropertyDescriptor) bool {
	return descriptor.HasGet || descriptor.HasSet
}

func isDataDescriptor(descriptor PropertyDescriptor) bool {
	return descriptor.HasValue || descriptor.HasWritable
}

func isGenericDescriptor(descriptor PropertyDescriptor) bool {
	return !isAccessorDescriptor(descriptor) && !isDataDescriptor(descriptor)
}

func completePropertyDescriptor(descriptor PropertyDescriptor) PropertyDescriptor {
	if isAccessorDescriptor(descriptor) {
		if !descriptor.HasGet {
			descriptor.Get, descriptor.HasGet = Undefined(), true
		}
		if !descriptor.HasSet {
			descriptor.Set, descriptor.HasSet = Undefined(), true
		}
	} else {
		if !descriptor.HasValue {
			descriptor.Value, descriptor.HasValue = Undefined(), true
		}
		if !descriptor.HasWritable {
			descriptor.Writable, descriptor.HasWritable = false, true
		}
	}
	if !descriptor.HasEnumerable {
		descriptor.Enumerable, descriptor.HasEnumerable = false, true
	}
	if !descriptor.HasConfigurable {
		descriptor.Configurable, descriptor.HasConfigurable = false, true
	}
	return descriptor
}

func ordinaryGetOwnProperty(object *Object, key PropertyKey) (PropertyDescriptor, bool) {
	descriptor, found := object.properties[key]
	if found && object.argumentsEnv != nil {
		if parameterName, mapped := object.parameterMap[key]; mapped {
			if value, err := object.argumentsEnv.getBindingValue(parameterName); err == nil {
				descriptor.Value = value
				descriptor.HasValue = true
			}
		}
	}
	return descriptor, found
}

func ordinaryGetPrototypeOf(object *Object) *Object { return object.prototype }

func ordinarySetPrototypeOf(object, prototype *Object) bool {
	if object.prototype == prototype {
		return true
	}
	if !object.extensible {
		return false
	}
	for candidate := prototype; candidate != nil; candidate = ordinaryGetPrototypeOf(candidate) {
		if candidate == object {
			return false
		}
	}
	object.prototype = prototype
	return true
}

func ordinaryIsExtensible(object *Object) bool { return object.extensible }

func ordinaryPreventExtensions(object *Object) bool {
	object.extensible = false
	return true
}

func ordinaryHasProperty(object *Object, key PropertyKey) bool {
	if _, found := ordinaryGetOwnProperty(object, key); found {
		return true
	}
	prototype := ordinaryGetPrototypeOf(object)
	return prototype != nil && ordinaryHasProperty(prototype, key)
}

func ordinaryDefineOwnProperty(object *Object, key PropertyKey, descriptor PropertyDescriptor) bool {
	current, found := ordinaryGetOwnProperty(object, key)
	if !validateAndApplyPropertyDescriptor(object, key, object.extensible, descriptor, current, found) {
		return false
	}
	if object.argumentsEnv != nil {
		if parameterName, mapped := object.parameterMap[key]; mapped {
			if isAccessorDescriptor(descriptor) {
				delete(object.parameterMap, key)
			} else {
				if descriptor.HasValue {
					_ = object.argumentsEnv.setMutableBinding(parameterName, descriptor.Value)
				}
				if descriptor.HasWritable && !descriptor.Writable {
					delete(object.parameterMap, key)
				}
			}
		}
	}
	return true
}

func validateAndApplyPropertyDescriptor(object *Object, key PropertyKey, extensible bool, descriptor, current PropertyDescriptor, currentFound bool) bool {
	if isAccessorDescriptor(descriptor) && isDataDescriptor(descriptor) {
		return false
	}
	if !currentFound {
		if !extensible {
			return false
		}
		if object != nil {
			storeProperty(object, key, completePropertyDescriptor(descriptor))
		}
		return true
	}
	if !descriptor.HasValue && !descriptor.HasWritable && !descriptor.HasGet && !descriptor.HasSet && !descriptor.HasEnumerable && !descriptor.HasConfigurable {
		return true
	}
	if !current.Configurable {
		if descriptor.HasConfigurable && descriptor.Configurable {
			return false
		}
		if descriptor.HasEnumerable && descriptor.Enumerable != current.Enumerable {
			return false
		}
		if !isGenericDescriptor(descriptor) && isAccessorDescriptor(descriptor) != isAccessorDescriptor(current) {
			return false
		}
		if isAccessorDescriptor(current) {
			if descriptor.HasGet && !SameValue(descriptor.Get, current.Get) {
				return false
			}
			if descriptor.HasSet && !SameValue(descriptor.Set, current.Set) {
				return false
			}
		} else if !current.Writable {
			if descriptor.HasWritable && descriptor.Writable {
				return false
			}
			if descriptor.HasValue && !SameValue(descriptor.Value, current.Value) {
				return false
			}
		}
	}
	if object == nil {
		return true
	}
	if isDataDescriptor(current) && isAccessorDescriptor(descriptor) {
		current = completePropertyDescriptor(PropertyDescriptor{
			Get: descriptor.Get, Set: descriptor.Set,
			HasGet: descriptor.HasGet, HasSet: descriptor.HasSet,
			Enumerable: current.Enumerable, Configurable: current.Configurable,
			HasEnumerable: true, HasConfigurable: true,
		})
	} else if isAccessorDescriptor(current) && isDataDescriptor(descriptor) {
		current = completePropertyDescriptor(PropertyDescriptor{
			Value: descriptor.Value, Writable: descriptor.Writable,
			HasValue: descriptor.HasValue, HasWritable: descriptor.HasWritable,
			Enumerable: current.Enumerable, Configurable: current.Configurable,
			HasEnumerable: true, HasConfigurable: true,
		})
	}
	applyDescriptorFields(&current, descriptor)
	storeProperty(object, key, current)
	return true
}

func applyDescriptorFields(target *PropertyDescriptor, source PropertyDescriptor) {
	if source.HasValue {
		target.Value, target.HasValue = source.Value, true
	}
	if source.HasWritable {
		target.Writable, target.HasWritable = source.Writable, true
	}
	if source.HasGet {
		target.Get, target.HasGet = source.Get, true
	}
	if source.HasSet {
		target.Set, target.HasSet = source.Set, true
	}
	if source.HasEnumerable {
		target.Enumerable, target.HasEnumerable = source.Enumerable, true
	}
	if source.HasConfigurable {
		target.Configurable, target.HasConfigurable = source.Configurable, true
	}
}

func getProperty(runtime *Runtime, value Value, key PropertyKey) (Value, bool, error) {
	return getPropertyWithReceiver(runtime, value, key, value)
}

func getPropertyWithReceiver(runtime *Runtime, value Value, key PropertyKey, receiver Value) (Value, bool, error) {
	if value.k == KindNull || value.k == KindUndefined {
		return Undefined(), false, typeError("cannot read property of null or undefined")
	}
	if value.k == KindString && !key.isSymbol() {
		if property, found := stringOwnProperty(value, key.goString()); found {
			return property, true, nil
		}
	}
	object := objectRecord(value)
	if object == nil && runtime != nil {
		switch value.k {
		case KindBoolean:
			object = runtime.intrinsics.booleanPrototype
		case KindNumber:
			object = runtime.intrinsics.numberPrototype
		case KindString:
			object = runtime.intrinsics.stringPrototype
		case KindSymbol:
			object = runtime.intrinsics.symbolPrototype
		}
	}
	usedFunctionFallback := false
	for object != nil {
		if descriptor, found := ordinaryGetOwnProperty(object, key); found {
			if isDataDescriptor(descriptor) {
				return descriptor.Value, true, nil
			}
			if descriptor.Get.IsUndefined() {
				return Undefined(), true, nil
			}
			result, err := callPropertyAccessor(runtime, descriptor.Get, receiver)
			return result, true, err
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

func callPropertyAccessor(runtime *Runtime, callable, this Value, arguments ...Value) (Value, error) {
	if runtime == nil {
		return Undefined(), typeError("accessor requires a runtime")
	}
	if runtime.exec != nil {
		return runtime.call(callable, this, arguments, Span{})
	}
	result, err := runtime.Call(context.Background(), callable, this, arguments...)
	return result.Value, err
}

func defineProperty(value Value, key PropertyKey, descriptor PropertyDescriptor) error {
	return definePropertyWithRuntime(nil, value, key, descriptor)
}

func definePropertyWithRuntime(runtime *Runtime, value Value, key PropertyKey, descriptor PropertyDescriptor) error {
	object := objectRecord(value)
	if object == nil {
		return typeError("value is not an object")
	}
	if object.array {
		succeeded, err := arrayDefineOwnProperty(runtime, object, key, descriptor)
		if err != nil {
			return err
		}
		if !succeeded {
			return typeError("cannot define property")
		}
		return nil
	}
	if !ordinaryDefineOwnProperty(object, key, descriptor) {
		return typeError("cannot define property")
	}
	return nil
}

func setProperty(runtime *Runtime, value Value, key PropertyKey, propertyValue Value) error {
	object := objectRecord(value)
	if object == nil {
		return typeError("value is not an object")
	}
	succeeded, err := ordinarySet(runtime, object, key, propertyValue, value)
	if err != nil {
		return err
	}
	if !succeeded {
		return typeError("cannot assign to property")
	}
	return nil
}

func ordinarySet(runtime *Runtime, object *Object, key PropertyKey, value, receiver Value) (bool, error) {
	ownDescriptor, found := ordinaryGetOwnProperty(object, key)
	if !found {
		if object.prototype != nil {
			return ordinarySet(runtime, object.prototype, key, value, receiver)
		}
		ownDescriptor = defaultProperty(Undefined())
	}
	if isDataDescriptor(ownDescriptor) {
		if !ownDescriptor.Writable {
			return false, nil
		}
		receiverObject := objectRecord(receiver)
		if receiverObject == nil {
			return false, nil
		}
		if existing, exists := ordinaryGetOwnProperty(receiverObject, key); exists {
			if isAccessorDescriptor(existing) || !existing.Writable {
				return false, nil
			}
			valueDescriptor := PropertyDescriptor{Value: value, HasValue: true}
			if receiverObject.array {
				return arrayDefineOwnProperty(runtime, receiverObject, key, valueDescriptor)
			}
			return ordinaryDefineOwnProperty(receiverObject, key, valueDescriptor), nil
		}
		if receiverObject.array {
			return arrayDefineOwnProperty(runtime, receiverObject, key, defaultProperty(value))
		}
		return ordinaryDefineOwnProperty(receiverObject, key, defaultProperty(value)), nil
	}
	if ownDescriptor.Set.IsUndefined() {
		return false, nil
	}
	_, err := callPropertyAccessor(runtime, ownDescriptor.Set, receiver, value)
	return err == nil, err
}

func arrayDefineOwnProperty(runtime *Runtime, object *Object, key PropertyKey, descriptor PropertyDescriptor) (bool, error) {
	if key == StringKey("length") {
		return defineArrayLength(runtime, object, descriptor)
	}
	if !key.isSymbol() {
		if index, ok := arrayIndex(key.goString()); ok {
			lengthDescriptor, _ := ordinaryGetOwnProperty(object, StringKey("length"))
			length := uint32(lengthDescriptor.Value.n)
			if uint32(index) >= length && !lengthDescriptor.Writable {
				return false, nil
			}
			if !ordinaryDefineOwnProperty(object, key, descriptor) {
				return false, nil
			}
			if uint32(index) >= length {
				lengthDescriptor.Value = Number(float64(index + 1))
				ordinaryDefineOwnProperty(object, StringKey("length"), lengthDescriptor)
			}
			return true, nil
		}
	}
	return ordinaryDefineOwnProperty(object, key, descriptor), nil
}

func defineArrayLength(runtime *Runtime, object *Object, descriptor PropertyDescriptor) (bool, error) {
	oldDescriptor, _ := ordinaryGetOwnProperty(object, StringKey("length"))
	if !descriptor.HasValue {
		return ordinaryDefineOwnProperty(object, StringKey("length"), descriptor), nil
	}
	var number float64
	var err error
	if runtime != nil {
		number, err = runtime.toNumber(descriptor.Value)
	} else {
		number, err = toNumber(descriptor.Value)
	}
	if err != nil {
		return false, err
	}
	uint32Length := uint32(number)
	if number < 0 || number > math.MaxUint32 || number != float64(uint32Length) {
		return false, &Exception{Name: "RangeError", Message: "invalid array length"}
	}
	oldLength := uint32(oldDescriptor.Value.n)
	descriptor.Value = Number(float64(uint32Length))
	if uint32Length >= oldLength {
		return ordinaryDefineOwnProperty(object, StringKey("length"), descriptor), nil
	}
	if !oldDescriptor.Writable {
		return false, nil
	}
	newWritable := true
	if descriptor.HasWritable && !descriptor.Writable {
		newWritable = false
		descriptor.Writable = true
	}
	if !ordinaryDefineOwnProperty(object, StringKey("length"), descriptor) {
		return false, nil
	}
	for index := oldLength; index > uint32Length; index-- {
		key := StringKey(strconv.FormatUint(uint64(index-1), 10))
		if !ordinaryDelete(object, key) {
			rollback := PropertyDescriptor{Value: Number(float64(index)), HasValue: true}
			if !newWritable {
				rollback.Writable, rollback.HasWritable = false, true
			}
			ordinaryDefineOwnProperty(object, StringKey("length"), rollback)
			return false, nil
		}
	}
	if !newWritable {
		ordinaryDefineOwnProperty(object, StringKey("length"), PropertyDescriptor{Writable: false, HasWritable: true})
	}
	return true, nil
}

func ordinaryDelete(object *Object, key PropertyKey) bool {
	descriptor, found := ordinaryGetOwnProperty(object, key)
	if !found {
		return true
	}
	if !descriptor.Configurable {
		return false
	}
	delete(object.properties, key)
	if object.parameterMap != nil {
		delete(object.parameterMap, key)
	}
	for index, existing := range object.propertyOrder {
		if existing == key {
			object.propertyOrder = append(object.propertyOrder[:index], object.propertyOrder[index+1:]...)
			break
		}
	}
	return true
}

func ordinaryOwnPropertyKeys(object *Object) []PropertyKey {
	indices := make([]PropertyKey, 0)
	strings := make([]PropertyKey, 0)
	symbols := make([]PropertyKey, 0)
	for _, key := range object.propertyOrder {
		if _, exists := object.properties[key]; !exists {
			continue
		}
		if key.isSymbol() {
			symbols = append(symbols, key)
		} else if _, isIndex := arrayIndex(key.goString()); isIndex {
			indices = append(indices, key)
		} else {
			strings = append(strings, key)
		}
	}
	sort.Slice(indices, func(left, right int) bool {
		leftIndex, _ := arrayIndex(indices[left].goString())
		rightIndex, _ := arrayIndex(indices[right].goString())
		return leftIndex < rightIndex
	})
	return append(append(indices, strings...), symbols...)
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
	return setProperty(runtime, value, propertyKey, propertyValue)
}

func (runtime *Runtime) DefineProperty(value, key Value, descriptor PropertyDescriptor) error {
	propertyKey, err := runtime.toPropertyKey(key)
	if err != nil {
		return err
	}
	return definePropertyWithRuntime(runtime, value, propertyKey, descriptor)
}

func ownProperty(value Value, key PropertyKey) (PropertyDescriptor, bool) {
	object := objectRecord(value)
	if object == nil {
		return PropertyDescriptor{}, false
	}
	return ordinaryGetOwnProperty(object, key)
}

func arrayIndex(name string) (int, bool) {
	index, err := strconv.ParseUint(name, 10, 32)
	return int(index), err == nil && index < math.MaxUint32 && strconv.FormatUint(index, 10) == name
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
