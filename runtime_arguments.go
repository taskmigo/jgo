package gots

func (runtime *Runtime) createArgumentsObject(functionValue Value, arguments []Value, environment *environment, mapped bool) Value {
	object := newObject(runtime.intrinsics.objectPrototype)
	for index, argument := range arguments {
		storeProperty(object, StringKey(fmtInt(index)), defaultProperty(argument))
	}
	storeProperty(object, StringKey("length"), dataProperty(Number(float64(len(arguments))), true, false, true))
	iterator, _, _ := getProperty(runtime, Value{k: KindObject, o: runtime.intrinsics.arrayPrototype}, PropertyKey{symbol: runtime.iteratorSymbol})
	storeProperty(object, PropertyKey{symbol: runtime.iteratorSymbol}, dataProperty(iterator, true, false, true))

	if mapped {
		storeProperty(object, StringKey("callee"), dataProperty(functionValue, true, false, true))
		object.argumentsEnv = environment
		object.parameterMap = make(map[PropertyKey]string)
		mappedNames := make(map[string]struct{})
		parameterCount := min(len(arguments), len(functionValue.f.params))
		for index := parameterCount - 1; index >= 0; index-- {
			parameterName := functionValue.f.params[index]
			if _, exists := mappedNames[parameterName]; exists {
				continue
			}
			mappedNames[parameterName] = struct{}{}
			object.parameterMap[StringKey(fmtInt(index))] = parameterName
		}
	} else {
		storeProperty(object, StringKey("callee"), completePropertyDescriptor(PropertyDescriptor{
			Get: runtime.intrinsics.throwTypeError, Set: runtime.intrinsics.throwTypeError,
			HasGet: true, HasSet: true, Enumerable: false, Configurable: false,
			HasEnumerable: true, HasConfigurable: true,
		}))
	}
	return Value{k: KindObject, o: object}
}
