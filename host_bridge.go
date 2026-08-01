package gots

import (
	"fmt"
	"reflect"
)

func (r *Runtime) fromGo(value any) (Value, error) {
	if value == nil {
		return Null(), nil
	}
	if runtimeValue, ok := value.(Value); ok {
		return runtimeValue, nil
	}
	if nativeFunction, ok := value.(NativeFunction); ok {
		return nativeValue(nativeFunction), nil
	}

	reflectedValue := reflect.ValueOf(value)
	if reflectedValue.Kind() == reflect.Func {
		return nativeValue(reflectFunction(reflectedValue)), nil
	}
	switch converted := value.(type) {
	case string:
		return String(converted), nil
	case bool:
		return Boolean(converted), nil
	case int:
		return Number(float64(converted)), nil
	case int64:
		return Number(float64(converted)), nil
	case float64:
		return Number(converted), nil
	default:
		return Undefined(), fmt.Errorf("unsupported Go value %T", value)
	}
}

func reflectFunction(function reflect.Value) NativeFunction {
	return func(_ *Runtime, _ Value, args []Value) (Value, error) {
		functionType := function.Type()
		if !validArgumentCount(functionType, len(args)) {
			return Undefined(), &RuntimeError{Message: "invalid host function argument count"}
		}

		inputs := make([]reflect.Value, len(args))
		for index, argument := range args {
			parameterType := reflectedParameterType(functionType, index)
			converted, err := toReflectedValue(argument, parameterType)
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
	// Preserve the bridge's existing failure mode when callers pass more
	// variadic arguments than the reflected function has declared inputs.
	parameterType := functionType.In(index)
	if functionType.IsVariadic() && index >= functionType.NumIn()-1 {
		return functionType.In(functionType.NumIn() - 1).Elem()
	}
	return parameterType
}

func toReflectedValue(value Value, target reflect.Type) (reflect.Value, error) {
	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(value.String()).Convert(target), nil
	case reflect.Float64:
		return reflect.ValueOf(number(value)).Convert(target), nil
	case reflect.Int:
		return reflect.ValueOf(int(number(value))).Convert(target), nil
	case reflect.Bool:
		return reflect.ValueOf(truthy(value)).Convert(target), nil
	default:
		return reflect.Value{}, &RuntimeError{Message: "unsupported host argument type"}
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
		return Undefined(), &RuntimeError{Message: "unsupported host return type"}
	}
}
