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
	}}
	functionValue.f.call = func(runtime *Runtime, this Value, arguments []Value) (Value, error) {
		return runtime.callECMAScriptFunction(functionValue, this, arguments)
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
	functionValue.f.object.properties[StringKey("prototype")] = PropertyDescriptor{
		Value: Value{k: KindObject, o: prototype}, Writable: true,
	}
	prototype.properties[StringKey("constructor")] = PropertyDescriptor{Value: functionValue, Writable: true, Configurable: true}
	return functionValue
}

func (interpreter *Runtime) evalCall(expression *callExpr, environment *environment) (Value, error) {
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
	return interpreter.invoke(callable, this, arguments, span, false)
}

func (interpreter *Runtime) construct(constructor Value, arguments []Value, span Span) (Value, error) {
	return interpreter.invoke(constructor, Undefined(), arguments, span, true)
}

func (interpreter *Runtime) invoke(functionValue, this Value, arguments []Value, span Span, construct bool) (Value, error) {
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
	value, err := call(interpreter, this, arguments)
	runtime.KeepAlive(functionValue.f.identity)
	return value, err
}

func (interpreter *Runtime) callECMAScriptFunction(functionValue, thisValue Value, arguments []Value) (Value, error) {
	function := functionValue.f
	environment := newEnvironment(function.closure)
	environment.bindings["this"] = binding{value: thisValue, initialized: true}
	for index, parameter := range function.params {
		argument := Undefined()
		if index < len(arguments) {
			argument = arguments[index]
		}
		environment.createMutableBinding(parameter, argument)
	}
	if function.name != "" && !environment.hasOwnBinding(function.name) {
		environment.bindings[function.name] = binding{value: functionValue, initialized: true}
	}
	if err := interpreter.instantiateDeclarations(function.body, environment); err != nil {
		return Undefined(), &Exception{Name: "SyntaxError", Message: err.Error()}
	}
	result := interpreter.evalStatements(function.body, environment)
	if result.err != nil {
		return Undefined(), result.err
	}
	if result.kind == completionThrow {
		return Undefined(), result.exception()
	}
	if result.kind == completionReturn {
		return result.value, nil
	}
	return Undefined(), nil
}
