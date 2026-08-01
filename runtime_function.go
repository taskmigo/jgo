package gots

import "runtime"

func (interpreter *Runtime) makeFunction(expression *functionExpr, closure *environment) Value {
	functionValue := Value{k: KindFunction, f: &function{
		identity: newIdentity(),
		object:   newObject(interpreter.intrinsics.functionPrototype),
		params:   expression.params,
		body:     expression.body,
		closure:  closure,
		name:     expression.name,
		strict:   expression.strict,
	}}
	functionValue.f.call = func(runtime *Runtime, this Value, arguments []Value) (Value, error) {
		return runtime.callECMAScriptFunction(functionValue, this, arguments)
	}
	if !expression.strict {
		storeProperty(functionValue.f.object, StringKey("caller"), dataProperty(Null(), false, false, false))
		storeProperty(functionValue.f.object, StringKey("arguments"), dataProperty(Null(), false, false, false))
	}
	functionValue.f.construct = func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		prototypeValue, _, _ := getProperty(runtime, functionValue, StringKey("prototype"))
		prototype := objectRecord(prototypeValue)
		if prototype == nil {
			prototype = runtime.intrinsics.objectPrototype
		}
		instance := Value{k: KindObject, o: newObject(prototype)}
		result, err := runtime.callECMAScriptFunction(functionValue, instance, arguments)
		if err != nil {
			return Undefined(), err
		}
		if objectRecord(result) != nil {
			return result, nil
		}
		return instance, nil
	}
	prototype := newObject(interpreter.intrinsics.objectPrototype)
	storeProperty(functionValue.f.object, StringKey("prototype"), dataProperty(Value{k: KindObject, o: prototype}, true, false, false))
	storeProperty(prototype, StringKey("constructor"), dataProperty(functionValue, true, false, true))
	return functionValue
}

func (interpreter *Runtime) evalCall(expression *callExpr, environment *environment) (Value, error) {
	if identifier, ok := expression.callee.(*identExpr); ok && identifier.name == "super" {
		return interpreter.evalSuperCall(expression, environment)
	}
	reference, isReference, err := interpreter.evalCallTarget(expression.callee, environment)
	if err != nil {
		return Undefined(), err
	}
	callee := reference.base
	thisValue := Undefined()
	if isReference {
		callee, err = reference.getValue()
		thisValue = reference.thisValue()
	}
	if err != nil {
		return Undefined(), err
	}

	arguments := make([]Value, len(expression.args))
	for index, argument := range expression.args {
		arguments[index], err = interpreter.eval(argument, environment)
		if err != nil {
			return Undefined(), err
		}
	}
	if identifier, ok := expression.callee.(*identExpr); ok && identifier.name == "eval" && SameValue(callee, interpreter.intrinsics.evalFunction) {
		return interpreter.performEval(argument(arguments, 0), environment, interpreter.exec.strict, true)
	}
	if expression.construct {
		if callee.k != KindFunction || callee.f.construct == nil {
			return Undefined(), typeError("value is not a constructor")
		}
		return interpreter.construct(callee, arguments, expression.span())
	}
	if callee.k != KindFunction || callee.f.call == nil {
		return Undefined(), typeError("value is not callable")
	}
	return interpreter.call(callee, thisValue, arguments, expression.span())
}

func (interpreter *Runtime) evalSuperCall(expression *callExpr, environment *environment) (Value, error) {
	activeFunction := interpreter.exec.activeFunction
	if activeFunction.k != KindFunction || !activeFunction.f.derived || activeFunction.f.superConstructor.k != KindFunction {
		return Undefined(), referenceError("super() is not valid in this context")
	}
	arguments := make([]Value, len(expression.args))
	for index, argument := range expression.args {
		value, err := interpreter.eval(argument, environment)
		if err != nil {
			return Undefined(), err
		}
		arguments[index] = value
	}
	result, err := interpreter.constructWithNewTarget(activeFunction.f.superConstructor, arguments, interpreter.exec.newTarget, expression.span())
	if err != nil {
		return Undefined(), err
	}
	if err := environment.initializeThisBinding(result); err != nil {
		return Undefined(), err
	}
	if err := interpreter.initializeClassElements(result, activeFunction.f.privateMethods, activeFunction.f.instanceFields); err != nil {
		return Undefined(), err
	}
	return result, nil
}

func (interpreter *Runtime) evalCallTarget(expression expr, environment *environment) (reference, bool, error) {
	switch expression.(type) {
	case *identExpr, *memberExpr:
		result, err := interpreter.evalReference(expression, environment)
		return result, true, err
	default:
		value, err := interpreter.eval(expression, environment)
		return reference{base: value}, false, err
	}
}

func (interpreter *Runtime) call(callable, this Value, arguments []Value, span Span) (Value, error) {
	return interpreter.invoke(callable, this, arguments, span, false, Undefined())
}

func (interpreter *Runtime) construct(constructor Value, arguments []Value, span Span) (Value, error) {
	return interpreter.constructWithNewTarget(constructor, arguments, constructor, span)
}

func (interpreter *Runtime) constructWithNewTarget(constructor Value, arguments []Value, newTarget Value, span Span) (Value, error) {
	return interpreter.invoke(constructor, Undefined(), arguments, span, true, newTarget)
}

func (interpreter *Runtime) invoke(functionValue, this Value, arguments []Value, span Span, construct bool, newTarget Value) (Value, error) {
	execution := interpreter.exec
	execution.depth++
	defer func() { execution.depth-- }()
	if execution.depth > execution.maxSeen {
		execution.maxSeen = execution.depth
	}
	if interpreter.config.MaxCallDepth > 0 && execution.depth > interpreter.config.MaxCallDepth {
		return Undefined(), fmtExecutionError(ErrCallDepth, span)
	}
	if err := interpreter.checkpoint(span); err != nil {
		return Undefined(), err
	}
	call := functionValue.f.call
	if construct {
		call = functionValue.f.construct
	}
	if call == nil {
		if construct {
			return Undefined(), typeError("value is not a constructor")
		}
		return Undefined(), typeError("value is not callable")
	}
	previousNewTarget := execution.newTarget
	execution.newTarget = newTarget
	defer func() { execution.newTarget = previousNewTarget }()
	value, err := call(interpreter, this, arguments)
	runtime.KeepAlive(functionValue.f.identity)
	return value, err
}

func (interpreter *Runtime) callECMAScriptFunction(functionValue, thisValue Value, arguments []Value) (Value, error) {
	function := functionValue.f
	if !function.strict {
		thisValue = interpreter.sloppyThisValue(thisValue)
	}
	environment := newFunctionEnvironment(function.closure)
	if function.derived {
		environment.bindings["this"] = binding{mutable: true}
	} else {
		environment.bindings["this"] = binding{value: thisValue, initialized: true}
	}
	for index, parameter := range function.params {
		argument := Undefined()
		if index < len(arguments) {
			argument = arguments[index]
		}
		environment.createMutableBinding(parameter, argument)
	}
	argumentsObject := interpreter.createArgumentsObject(functionValue, arguments, environment, !function.strict)
	if !containsString(function.params, "arguments") {
		environment.createMutableBinding("arguments", argumentsObject)
	}
	if function.name != "" && !environment.hasOwnBinding(function.name) {
		environment.bindings[function.name] = binding{value: functionValue, initialized: true}
	}
	bodyEnvironment := newEnvironment(environment)
	if err := interpreter.instantiateDeclarations(function.body, bodyEnvironment); err != nil {
		return Undefined(), &Exception{Name: "SyntaxError", Message: err.Error()}
	}
	previousStrict := interpreter.exec.strict
	previousFunction := interpreter.exec.activeFunction
	interpreter.exec.strict = function.strict
	interpreter.exec.activeFunction = functionValue
	if !function.strict {
		caller := Null()
		if previousFunction.k == KindFunction && !previousFunction.f.strict {
			caller = previousFunction
		}
		callerDescriptor := function.object.properties[StringKey("caller")]
		callerDescriptor.Value = caller
		function.object.properties[StringKey("caller")] = callerDescriptor
		argumentsDescriptor := function.object.properties[StringKey("arguments")]
		argumentsDescriptor.Value = argumentsObject
		function.object.properties[StringKey("arguments")] = argumentsDescriptor
	}
	defer func() {
		interpreter.exec.strict = previousStrict
		interpreter.exec.activeFunction = previousFunction
		if !function.strict {
			callerDescriptor := function.object.properties[StringKey("caller")]
			callerDescriptor.Value = Null()
			function.object.properties[StringKey("caller")] = callerDescriptor
			argumentsDescriptor := function.object.properties[StringKey("arguments")]
			argumentsDescriptor.Value = Null()
			function.object.properties[StringKey("arguments")] = argumentsDescriptor
		}
	}()
	result := interpreter.evalStatements(function.body, bodyEnvironment)
	if result.err != nil {
		return Undefined(), result.err
	}
	if result.kind == completionThrow {
		return Undefined(), result.exception()
	}
	if result.kind == completionReturn {
		if !function.derived || objectRecord(result.value) != nil {
			return result.value, nil
		}
		if !result.value.IsUndefined() {
			return Undefined(), typeError("derived constructor returned a non-object value")
		}
	}
	if function.derived {
		return environment.getBindingValue("this")
	}
	return Undefined(), nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (interpreter *Runtime) sloppyThisValue(value Value) Value {
	if value.k == KindUndefined || value.k == KindNull {
		globalThis, _ := interpreter.global.getBindingValue("globalThis")
		return globalThis
	}
	if objectRecord(value) != nil {
		return value
	}
	boxed, _ := interpreter.toObjectValue(value)
	return boxed
}

func (interpreter *Runtime) toObjectValue(value Value) (Value, error) {
	if value.k == KindUndefined || value.k == KindNull {
		return Undefined(), typeError("cannot convert null or undefined to object")
	}
	if objectRecord(value) != nil {
		return value, nil
	}
	prototype := interpreter.intrinsics.objectPrototype
	switch value.k {
	case KindBoolean:
		prototype = interpreter.intrinsics.booleanPrototype
	case KindNumber:
		prototype = interpreter.intrinsics.numberPrototype
	case KindString:
		prototype = interpreter.intrinsics.stringPrototype
	case KindSymbol:
		prototype = interpreter.intrinsics.symbolPrototype
	}
	object := newObject(prototype)
	object.boxed = value
	return Value{k: KindObject, o: object}, nil
}
