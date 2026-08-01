package gots

import (
	"fmt"
	"reflect"
)

func (runtime *Runtime) fromGo(value any) (Value, error) {
	switch converted := value.(type) {
	case nil:
		return Null(), nil
	case Value:
		return converted, nil
	case NativeFunction:
		return nativeValue(converted), nil
	case string:
		return String(converted), nil
	case bool:
		return Boolean(converted), nil
	case int:
		return Number(float64(converted)), nil
	case int8:
		return Number(float64(converted)), nil
	case int16:
		return Number(float64(converted)), nil
	case int32:
		return Number(float64(converted)), nil
	case int64:
		return Number(float64(converted)), nil
	case uint:
		return Number(float64(converted)), nil
	case uint8:
		return Number(float64(converted)), nil
	case uint16:
		return Number(float64(converted)), nil
	case uint32:
		return Number(float64(converted)), nil
	case uint64:
		return Number(float64(converted)), nil
	case float32:
		return Number(float64(converted)), nil
	case float64:
		return Number(converted), nil
	case []Value:
		return runtime.newArray(converted...), nil
	case map[string]Value:
		object := runtime.newOrdinaryObject()
		for name, propertyValue := range converted {
			_ = defineProperty(object, StringKey(name), defaultProperty(propertyValue))
		}
		return object, nil
	}

	reflected := reflect.ValueOf(value)
	if reflected.IsValid() && reflected.Kind() == reflect.Func {
		return nativeValue(reflectFunction(reflected)), nil
	}
	return Undefined(), fmt.Errorf("unsupported Go value %T", value)
}

func reflectFunction(function reflect.Value) NativeFunction {
	return func(_ *Runtime, _ Value, arguments []Value) (Value, error) {
		functionType := function.Type()
		if !validArgumentCount(functionType, len(arguments)) {
			return Undefined(), typeError("invalid host function argument count")
		}
		inputs := make([]reflect.Value, len(arguments))
		for index, argument := range arguments {
			converted, err := toReflectedValue(argument, reflectedParameterType(functionType, index))
			if err != nil {
				return Undefined(), err
			}
			inputs[index] = converted
		}
		outputs := function.Call(inputs)
		if len(outputs) == 0 {
			return Undefined(), nil
		}
		return fromReflectedValue(outputs[0])
	}
}

func validArgumentCount(functionType reflect.Type, count int) bool {
	if functionType.IsVariadic() {
		return count >= functionType.NumIn()-1
	}
	return count == functionType.NumIn()
}

func reflectedParameterType(functionType reflect.Type, index int) reflect.Type {
	if functionType.IsVariadic() && index >= functionType.NumIn()-1 {
		return functionType.In(functionType.NumIn() - 1).Elem()
	}
	return functionType.In(index)
}

func toReflectedValue(value Value, target reflect.Type) (reflect.Value, error) {
	switch target.Kind() {
	case reflect.String:
		converted, err := value.ToString()
		return reflect.ValueOf(converted).Convert(target), err
	case reflect.Float64:
		converted, err := value.ToNumber()
		return reflect.ValueOf(converted).Convert(target), err
	case reflect.Int:
		converted, err := value.ToNumber()
		return reflect.ValueOf(int(converted)).Convert(target), err
	case reflect.Bool:
		return reflect.ValueOf(value.ToBoolean()).Convert(target), nil
	default:
		return reflect.Value{}, typeError("unsupported host argument type")
	}
}

func fromReflectedValue(value reflect.Value) (Value, error) {
	switch converted := value.Interface().(type) {
	case string:
		return String(converted), nil
	case int:
		return Number(float64(converted)), nil
	case float64:
		return Number(converted), nil
	case bool:
		return Boolean(converted), nil
	case Value:
		return converted, nil
	default:
		return Undefined(), typeError("unsupported host return type")
	}
}
