package gots

import "fmt"

type binding struct {
	value          Value
	mutable        bool
	initialized    bool
	lexical        bool
	silentReadOnly bool
}

type environment struct {
	outer        *environment
	bindings     map[string]binding
	variable     *environment
	runtime      *Runtime
	objectValue  Value
	object       bool
	privateNames map[string]*privateIdentifier
}

func newObjectEnvironment(runtime *Runtime, value Value, outer *environment) *environment {
	environment := newEnvironment(outer)
	environment.runtime = runtime
	environment.objectValue = value
	environment.object = true
	return environment
}

func newEnvironment(outer *environment) *environment {
	environment := &environment{outer: outer, bindings: make(map[string]binding), privateNames: make(map[string]*privateIdentifier)}
	if outer == nil {
		environment.variable = environment
	} else {
		environment.variable = outer.variable
	}
	return environment
}

func (environment *environment) resolvePrivateName(name string) *privateIdentifier {
	for current := environment; current != nil; current = current.outer {
		if identifier := current.privateNames[name]; identifier != nil {
			return identifier
		}
	}
	return nil
}

func newFunctionEnvironment(outer *environment) *environment {
	environment := newEnvironment(outer)
	environment.variable = environment
	return environment
}

func (environment *environment) cloneLocal() *environment {
	clone := newEnvironment(environment.outer)
	clone.variable = environment.variable
	for name, binding := range environment.bindings {
		clone.bindings[name] = binding
	}
	return clone
}

func (environment *environment) hasOwnBinding(name string) bool {
	if environment.object {
		object := objectRecord(environment.objectValue)
		return object != nil && ordinaryHasProperty(object, StringKey(name))
	}
	_, found := environment.bindings[name]
	return found
}

func (environment *environment) createMutableBinding(name string, initial Value) {
	environment.bindings[name] = binding{value: initial, mutable: true, initialized: true}
}

func (environment *environment) createUninitializedBinding(name string, mutable bool) error {
	if environment.hasOwnBinding(name) {
		return fmt.Errorf("identifier %s has already been declared", name)
	}
	environment.bindings[name] = binding{mutable: mutable, lexical: true}
	return nil
}

func (environment *environment) initializeBinding(name string, value Value) error {
	binding, found := environment.bindings[name]
	if !found {
		return referenceError(name + " is not defined")
	}
	if binding.initialized {
		if !binding.mutable {
			return typeError("assignment to constant " + name)
		}
	}
	binding.value = value
	binding.initialized = true
	environment.bindings[name] = binding
	return nil
}

func (environment *environment) initializeThisBinding(value Value) error {
	target := environment.resolveBinding("this")
	if target == nil {
		return referenceError("super() is not valid in this context")
	}
	binding := target.bindings["this"]
	if binding.initialized {
		return referenceError("super() has already initialized this")
	}
	binding.value = value
	binding.initialized = true
	target.bindings["this"] = binding
	return nil
}

func (environment *environment) resolveBinding(name string) *environment {
	for current := environment; current != nil; current = current.outer {
		if current.hasOwnBinding(name) {
			return current
		}
	}
	return nil
}

func (environment *environment) getBindingValue(name string) (Value, error) {
	target := environment.resolveBinding(name)
	if target == nil {
		return Undefined(), referenceError(name + " is not defined")
	}
	if target.object {
		value, _, err := getProperty(target.runtime, target.objectValue, StringKey(name))
		return value, err
	}
	binding := target.bindings[name]
	if !binding.initialized {
		return Undefined(), referenceError("cannot access " + name + " before initialization")
	}
	return binding.value, nil
}

func (environment *environment) setMutableBinding(name string, value Value) error {
	return environment.putMutableBinding(name, value, true)
}

func (environment *environment) putMutableBinding(name string, value Value, strict bool) error {
	target := environment.resolveBinding(name)
	if target == nil {
		return referenceError(name + " is not defined")
	}
	if target.object {
		return setProperty(target.runtime, target.objectValue, StringKey(name), value)
	}
	binding := target.bindings[name]
	if !binding.initialized {
		return referenceError("cannot access " + name + " before initialization")
	}
	if !binding.mutable {
		return typeError("assignment to constant " + name)
	}
	if binding.silentReadOnly {
		if strict {
			return typeError("assignment to read-only global " + name)
		}
		return nil
	}
	binding.value = value
	target.bindings[name] = binding
	return nil
}
