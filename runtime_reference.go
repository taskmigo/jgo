package gots

type referenceKind uint8

const (
	referenceBinding referenceKind = iota
	referenceProperty
)

type reference struct {
	runtime     *Runtime
	kind        referenceKind
	environment *environment
	name        string
	base        Value
	key         PropertyKey
}

func bindingReference(runtime *Runtime, environment *environment, name string) reference {
	return reference{runtime: runtime, kind: referenceBinding, environment: environment, name: name}
}

func propertyReference(runtime *Runtime, base Value, key PropertyKey) reference {
	return reference{runtime: runtime, kind: referenceProperty, base: base, key: key}
}

func (reference reference) getValue() (Value, error) {
	switch reference.kind {
	case referenceBinding:
		return reference.environment.getBindingValue(reference.name)
	case referenceProperty:
		value, _, err := getProperty(reference.runtime, reference.base, reference.key)
		return value, err
	default:
		return Undefined(), referenceError("invalid reference")
	}
}

func (reference reference) putValue(value Value) error {
	switch reference.kind {
	case referenceBinding:
		return reference.environment.setMutableBinding(reference.name, value)
	case referenceProperty:
		return setProperty(reference.base, reference.key, value)
	default:
		return referenceError("invalid assignment target")
	}
}

func (reference reference) thisValue() Value {
	if reference.kind == referenceProperty {
		return reference.base
	}
	return Undefined()
}
