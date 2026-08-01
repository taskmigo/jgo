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
func (e *environment) define(n string, v Value, c bool) { e.values[n] = binding{v, c} }
func (e *environment) get(n string) (Value, bool) {
	for x := e; x != nil; x = x.parent {
		if v, ok := x.values[n]; ok {
			return v.value, true
		}
	}
	return Undefined(), false
}
func (e *environment) set(n string, v Value) error {
	for x := e; x != nil; x = x.parent {
		if b, ok := x.values[n]; ok {
			if b.constant {
				return fmt.Errorf("assignment to constant %s", n)
			}
			b.value = v
			x.values[n] = b
			return nil
		}
	}
	return fmt.Errorf("%s is not defined", n)
}
