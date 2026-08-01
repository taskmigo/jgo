package gots

import "fmt"

type binding struct {
	value       Value
	mutable     bool
	initialized bool
}

type environment struct {
	outer    *environment
	bindings map[string]binding
}

func newEnvironment(outer *environment) *environment {
	return &environment{outer: outer, bindings: make(map[string]binding)}
}

func (environment *environment) cloneLocal() *environment {
	clone := newEnvironment(environment.outer)
	for name, binding := range environment.bindings {
		clone.bindings[name] = binding
	}
	return clone
}

func (environment *environment) hasOwnBinding(name string) bool {
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
	environment.bindings[name] = binding{mutable: mutable}
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
	binding := target.bindings[name]
	if !binding.initialized {
		return Undefined(), referenceError("cannot access " + name + " before initialization")
	}
	return binding.value, nil
}

func (environment *environment) setMutableBinding(name string, value Value) error {
	target := environment.resolveBinding(name)
	if target == nil {
		return referenceError(name + " is not defined")
	}
	binding := target.bindings[name]
	if !binding.initialized {
		return referenceError("cannot access " + name + " before initialization")
	}
	if !binding.mutable {
		return typeError("assignment to constant " + name)
	}
	binding.value = value
	target.bindings[name] = binding
	return nil
}
