package gots

import "fmt"

type binding struct {
	value    Value
	constant bool
}
type environment struct {
	parent *environment
	values map[string]binding
}

func newEnvironment(parent *environment) *environment {
	return &environment{parent: parent, values: map[string]binding{}}
}
func (env *environment) cloneLocal() *environment {
	clone := newEnvironment(env.parent)
	for name, value := range env.values {
		clone.values[name] = value
	}
	return clone
}

func (env *environment) define(name string, value Value, constant bool) {
	env.values[name] = binding{value: value, constant: constant}
}

func (env *environment) get(name string) (Value, bool) {
	for current := env; current != nil; current = current.parent {
		if value, found := current.values[name]; found {
			return value.value, true
		}
	}
	return Undefined(), false
}

func (env *environment) set(name string, value Value) error {
	for current := env; current != nil; current = current.parent {
		if existing, found := current.values[name]; found {
			if existing.constant {
				return fmt.Errorf("assignment to constant %s", name)
			}
			existing.value = value
			current.values[name] = existing
			return nil
		}
	}
	return fmt.Errorf("%s is not defined", name)
}
