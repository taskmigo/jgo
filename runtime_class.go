package gots

func (runtime *Runtime) evalClass(definition *classExpr, outerEnvironment *environment) (Value, error) {
	classEnvironment := newEnvironment(outerEnvironment)
	for _, element := range definition.elements {
		if privateName, ok := element.key.(*privateNameExpr); ok {
			if classEnvironment.privateNames[privateName.name] == nil {
				classEnvironment.privateNames[privateName.name] = &privateIdentifier{description: privateName.name}
			}
		}
	}
	if definition.name != "" {
		if err := classEnvironment.createUninitializedBinding(definition.name, false); err != nil {
			return Undefined(), err
		}
	}
	parentConstructor := Undefined()
	parentPrototype := runtime.intrinsics.objectPrototype
	derived := definition.heritage != nil
	if derived {
		heritage, err := runtime.eval(definition.heritage, classEnvironment)
		if err != nil {
			return Undefined(), err
		}
		if heritage.k == KindNull {
			parentPrototype = nil
		} else {
			if heritage.k != KindFunction || heritage.f.construct == nil {
				return Undefined(), typeError("class heritage is not a constructor or null")
			}
			parentConstructor = heritage
			prototypeValue, _, err := getProperty(runtime, heritage, StringKey("prototype"))
			if err != nil {
				return Undefined(), err
			}
			if prototypeValue.k == KindNull {
				parentPrototype = nil
			} else {
				parentPrototype = objectRecord(prototypeValue)
				if parentPrototype == nil {
					return Undefined(), typeError("class heritage prototype is not an object or null")
				}
			}
		}
	}

	var constructorDefinition *functionExpr
	hasExplicitConstructor := false
	for index := range definition.elements {
		element := &definition.elements[index]
		if !element.field && !element.static && element.accessor == "" && literalClassElementName(element.key) == "constructor" {
			constructorDefinition = element.method
			hasExplicitConstructor = true
			break
		}
	}
	if constructorDefinition == nil {
		constructorDefinition = &functionExpr{name: definition.name, strict: true}
	}
	constructorDefinition.strict = true
	constructor := runtime.makeFunction(constructorDefinition, classEnvironment)
	constructor.f.name = definition.name
	constructor.f.derived = derived
	constructor.f.classConstructor = true
	constructor.f.superConstructor = parentConstructor
	constructor.f.object.prototype = runtime.intrinsics.functionPrototype
	if parentConstructor.k == KindFunction {
		constructor.f.object.prototype = parentConstructor.f.object
	}
	classPrototype := newObject(parentPrototype)
	constructor.f.homeObject = classPrototype
	storeProperty(constructor.f.object, StringKey("prototype"), dataProperty(Value{k: KindObject, o: classPrototype}, false, false, false))
	storeProperty(classPrototype, StringKey("constructor"), dataProperty(constructor, true, false, true))

	constructorBody := constructor.f.call
	instanceFields := make([]evaluatedClassField, 0)
	instancePrivateMethods := make([]evaluatedPrivateElement, 0)
	constructor.f.call = func(_ *Runtime, _ Value, _ []Value) (Value, error) {
		return Undefined(), typeError("class constructor cannot be invoked without new")
	}
	constructor.f.construct = func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		if derived {
			if !hasExplicitConstructor {
				if parentConstructor.k != KindFunction {
					return Undefined(), typeError("derived default constructor has no super constructor")
				}
				instance, err := runtime.constructWithNewTarget(parentConstructor, arguments, runtime.exec.newTarget, Span{})
				if err != nil {
					return Undefined(), err
				}
				if err := runtime.initializeClassElements(instance, instancePrivateMethods, instanceFields); err != nil {
					return Undefined(), err
				}
				return instance, nil
			}
			return constructorBody(runtime, Undefined(), arguments)
		}
		prototype := classPrototype
		if newTargetPrototype := runtime.prototypeFromConstructor(runtime.exec.newTarget); newTargetPrototype != nil {
			prototype = newTargetPrototype
		}
		instance := Value{k: KindObject, o: newObject(prototype)}
		if err := runtime.initializeClassElements(instance, instancePrivateMethods, instanceFields); err != nil {
			return Undefined(), err
		}
		result, err := constructorBody(runtime, instance, arguments)
		if err != nil {
			return Undefined(), err
		}
		if objectRecord(result) != nil {
			return result, nil
		}
		return instance, nil
	}
	constructor.f.instanceFields = instanceFields
	defineFunctionMetadata(constructor, definition.name, len(constructorDefinition.params))

	if definition.name != "" {
		if err := classEnvironment.initializeBinding(definition.name, constructor); err != nil {
			return Undefined(), err
		}
	}
	for _, element := range definition.elements {
		if element.staticBlock != nil {
			if err := runtime.evaluateClassStaticBlock(element.staticBlock, classEnvironment, constructor); err != nil {
				return Undefined(), err
			}
			continue
		}
		if !element.field && !element.static && element.accessor == "" && literalClassElementName(element.key) == "constructor" {
			continue
		}
		privateName, isPrivate := element.key.(*privateNameExpr)
		var privateIdentifier *privateIdentifier
		var key PropertyKey
		if isPrivate {
			privateIdentifier = classEnvironment.privateNames[privateName.name]
		} else {
			keyValue, err := runtime.eval(element.key, classEnvironment)
			if err != nil {
				return Undefined(), err
			}
			key, err = runtime.toPropertyKey(keyValue)
			if err != nil {
				return Undefined(), err
			}
		}
		if element.field {
			field := evaluatedClassField{key: key, privateName: privateIdentifier, initializer: element.initializer, environment: classEnvironment}
			if element.static {
				if err := runtime.initializeClassFields(constructor, []evaluatedClassField{field}); err != nil {
					return Undefined(), err
				}
			} else {
				instanceFields = append(instanceFields, field)
			}
			continue
		}
		var method Value
		if element.generator {
			method = nativeValue(func(_ *Runtime, _ Value, _ []Value) (Value, error) {
				return Undefined(), typeError("generator methods are not supported yet")
			})
			method.f.object.prototype = runtime.intrinsics.functionPrototype
		} else {
			method = runtime.makeFunction(element.method, classEnvironment)
		}
		method.f.construct = nil
		removeOwnProperty(method.f.object, StringKey("prototype"))
		methodName := privateNameDescription(privateIdentifier)
		if !isPrivate {
			methodName = key.goString()
			if key.isSymbol() {
				methodName = "[" + key.symbol.description.goString() + "]"
			}
		}
		defineFunctionMetadata(method, methodName, len(element.method.params))
		target := classPrototype
		if element.static {
			target = constructor.f.object
		}
		method.f.homeObject = target
		if isPrivate {
			privateMethod := evaluatedPrivateElement{name: privateIdentifier}
			if element.accessor == "get" {
				privateMethod.element = privateElementRecord(Undefined(), method)
			} else if element.accessor == "set" {
				privateMethod.element = privateElementRecord(method, Undefined())
			} else {
				privateMethod.element = privateElement{value: method}
			}
			if element.static {
				if err := addPrivateElement(constructor, privateMethod, true); err != nil {
					return Undefined(), err
				}
			} else {
				instancePrivateMethods = appendOrMergePrivateElement(instancePrivateMethods, privateMethod)
			}
			continue
		}
		if err := defineClassElement(target, key, method, element.accessor); err != nil {
			return Undefined(), err
		}
	}
	constructor.f.instanceFields = instanceFields
	constructor.f.privateMethods = instancePrivateMethods
	return constructor, nil
}

func (runtime *Runtime) prototypeFromConstructor(constructor Value) *Object {
	if constructor.k != KindFunction {
		return nil
	}
	prototype, _, err := getProperty(runtime, constructor, StringKey("prototype"))
	if err != nil {
		return nil
	}
	return objectRecord(prototype)
}

type evaluatedClassField struct {
	key         PropertyKey
	privateName *privateIdentifier
	initializer expr
	environment *environment
}

type evaluatedPrivateElement struct {
	name    *privateIdentifier
	element privateElement
}

func (runtime *Runtime) initializeClassElements(receiver Value, privateMethods []evaluatedPrivateElement, fields []evaluatedClassField) error {
	for _, method := range privateMethods {
		if err := addPrivateElement(receiver, method, false); err != nil {
			return err
		}
	}
	return runtime.initializeClassFields(receiver, fields)
}

func (runtime *Runtime) initializeClassFields(receiver Value, fields []evaluatedClassField) error {
	for _, field := range fields {
		value := Undefined()
		if field.initializer != nil {
			initializerEnvironment := newEnvironment(field.environment)
			initializerEnvironment.createMutableBinding("this", receiver)
			previousInitializer := runtime.exec.classFieldInitializer
			runtime.exec.classFieldInitializer = true
			var err error
			value, err = runtime.eval(field.initializer, initializerEnvironment)
			runtime.exec.classFieldInitializer = previousInitializer
			if err != nil {
				return err
			}
		}
		if field.privateName != nil {
			if err := addPrivateElement(receiver, evaluatedPrivateElement{name: field.privateName, element: privateElement{value: value, writable: true}}, false); err != nil {
				return err
			}
		} else if err := definePropertyWithRuntime(runtime, receiver, field.key, defaultProperty(value)); err != nil {
			return err
		}
	}
	return nil
}

func addPrivateElement(receiver Value, element evaluatedPrivateElement, mergeAccessors bool) error {
	object := objectRecord(receiver)
	if object == nil {
		return typeError("cannot add a private class element to a non-object")
	}
	if object.privateElements == nil {
		object.privateElements = make(map[*privateIdentifier]privateElement)
	}
	if existing, found := object.privateElements[element.name]; found {
		if mergeAccessors && existing.accessor && element.element.accessor {
			if element.element.getter.k == KindFunction {
				existing.getter = element.element.getter
			}
			if element.element.setter.k == KindFunction {
				existing.setter = element.element.setter
			}
			object.privateElements[element.name] = existing
			return nil
		}
		return typeError("private class element is already present")
	}
	object.privateElements[element.name] = element.element
	return nil
}

func appendOrMergePrivateElement(elements []evaluatedPrivateElement, addition evaluatedPrivateElement) []evaluatedPrivateElement {
	for index := range elements {
		if elements[index].name != addition.name || !elements[index].element.accessor || !addition.element.accessor {
			continue
		}
		if addition.element.getter.k == KindFunction {
			elements[index].element.getter = addition.element.getter
		}
		if addition.element.setter.k == KindFunction {
			elements[index].element.setter = addition.element.setter
		}
		return elements
	}
	return append(elements, addition)
}

func privateElementRecord(setter, getter Value) privateElement {
	return privateElement{getter: getter, setter: setter, accessor: true}
}

func privateNameDescription(name *privateIdentifier) string {
	if name == nil {
		return ""
	}
	return name.description
}

func (runtime *Runtime) evaluateClassStaticBlock(body []stmt, environment *environment, constructor Value) error {
	blockFunction := runtime.makeFunction(&functionExpr{body: body, strict: true}, environment)
	blockFunction.f.homeObject = constructor.f.object
	_, err := runtime.call(blockFunction, constructor, nil, Span{})
	return err
}

func literalClassElementName(expression expr) string {
	literal, ok := expression.(*literalExpr)
	if !ok || literal.value.k != KindString {
		return ""
	}
	return jsString(literal.value.s).goString()
}

func defineClassElement(target *Object, key PropertyKey, method Value, accessor string) error {
	if accessor == "" {
		if !ordinaryDefineOwnProperty(target, key, dataProperty(method, true, false, true)) {
			return typeError("cannot define class method")
		}
		return nil
	}
	descriptor, found := ordinaryGetOwnProperty(target, key)
	if !found {
		descriptor = completePropertyDescriptor(PropertyDescriptor{
			Get: Undefined(), Set: Undefined(), HasGet: true, HasSet: true,
			Enumerable: false, Configurable: true, HasEnumerable: true, HasConfigurable: true,
		})
	} else if !isAccessorDescriptor(descriptor) {
		newDescriptor := PropertyDescriptor{
			Get: Undefined(), Set: Undefined(), HasGet: true, HasSet: true,
			Enumerable: false, Configurable: true, HasEnumerable: true, HasConfigurable: true,
		}
		if accessor == "get" {
			newDescriptor.Get = method
		} else {
			newDescriptor.Set = method
		}
		if !ordinaryDefineOwnProperty(target, key, newDescriptor) {
			return typeError("cannot define class accessor")
		}
		return nil
	}
	if accessor == "get" {
		descriptor.Get = method
	} else {
		descriptor.Set = method
	}
	storeProperty(target, key, descriptor)
	return nil
}

func removeOwnProperty(object *Object, key PropertyKey) {
	delete(object.properties, key)
	for index, existing := range object.propertyOrder {
		if existing == key {
			object.propertyOrder = append(object.propertyOrder[:index], object.propertyOrder[index+1:]...)
			return
		}
	}
}
