package gots

import "runtime"

func (r *Runtime) makeFunction(expression *functionExpr, closure *environment) Value {
	return Value{k: KindFunction, f: &function{
		identity: newIdentity(),
		props:    map[string]Value{},
		params:   expression.params,
		body:     expression.body,
		closure:  closure,
		name:     expression.name,
	}}
}

func (r *Runtime) evalCall(expression *callExpr, env *environment) (Value, error) {
	this := Undefined()
	callee, err := r.eval(expression.callee, env)
	if err != nil {
		return Undefined(), err
	}
	if member, ok := expression.callee.(*memberExpr); ok {
		this, err = r.eval(member.object, env)
		if err != nil {
			return Undefined(), err
		}
	}
	if message := callValidationMessage(callee, expression.construct); message != "" {
		return Undefined(), r.err(expression.span(), message, nil)
	}

	args := make([]Value, len(expression.args))
	for index, argument := range expression.args {
		args[index], err = r.eval(argument, env)
		if err != nil {
			return Undefined(), err
		}
	}
	return r.call(callee, this, args, expression.span())
}

func callValidationMessage(callee Value, construct bool) string {
	if callee.k != KindFunction {
		return "value is not callable"
	}
	if callee.f.constructOnly && !construct {
		return "constructor requires new"
	}
	if callee.f.noConstruct && construct {
		return "function is not a constructor"
	}
	return ""
}

func (r *Runtime) call(value, this Value, args []Value, span Span) (Value, error) {
	function := value.f
	r.exec.depth++
	defer func() { r.exec.depth-- }()
	if r.exec.depth > r.exec.maxSeen {
		r.exec.maxSeen = r.exec.depth
	}
	if r.maxDepth > 0 && r.exec.depth > r.maxDepth {
		return Undefined(), r.err(span, ErrCallDepth.Error(), ErrCallDepth)
	}
	if err := r.checkpoint(span); err != nil {
		return Undefined(), err
	}
	if function.native != nil {
		return function.native(r, this, args)
	}

	env := newEnvironment(function.closure)
	for index, parameter := range function.params {
		argument := Undefined()
		if index < len(args) {
			argument = args[index]
		}
		env.define(parameter, argument, false)
	}
	if function.name != "" {
		env.define(function.name, Value{k: KindFunction, f: function}, true)
	}
	result, _, err := r.evalStatements(function.body, env)
	runtime.KeepAlive(function.identity)
	return result, err
}
