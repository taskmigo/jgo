package gots

type referenceKind uint8

const (
	referenceBinding referenceKind = iota
	referenceProperty
	referencePrivate
)

type reference struct {
	runtime     *Runtime
	kind        referenceKind
	environment *environment
	name        string
	base        Value
	receiver    Value
	hasReceiver bool
	key         PropertyKey
	strict      bool
	privateName *privateIdentifier
}

func bindingReference(runtime *Runtime, environment *environment, name string) reference {
	return reference{runtime: runtime, kind: referenceBinding, environment: environment, name: name, strict: runtime.exec != nil && runtime.exec.strict}
}

func propertyReference(runtime *Runtime, base Value, key PropertyKey) reference {
	return reference{runtime: runtime, kind: referenceProperty, base: base, key: key, strict: runtime.exec != nil && runtime.exec.strict}
}

func propertyReferenceWithReceiver(runtime *Runtime, base Value, key PropertyKey, receiver Value) reference {
	return reference{runtime: runtime, kind: referenceProperty, base: base, key: key, receiver: receiver, hasReceiver: true, strict: runtime.exec != nil && runtime.exec.strict}
}

func privateReference(runtime *Runtime, base Value, name *privateIdentifier) reference {
	return reference{runtime: runtime, kind: referencePrivate, base: base, privateName: name, strict: true}
}

func (reference reference) getValue() (Value, error) {
	switch reference.kind {
	case referenceBinding:
		return reference.environment.getBindingValue(reference.name)
	case referenceProperty:
		receiver := reference.base
		if reference.hasReceiver {
			receiver = reference.receiver
		}
		value, _, err := getPropertyWithReceiver(reference.runtime, reference.base, reference.key, receiver)
		return value, err
	case referencePrivate:
		return reference.getPrivateValue()
	default:
		return Undefined(), referenceError("invalid reference")
	}
}

func (reference reference) putValue(value Value) error {
	switch reference.kind {
	case referenceBinding:
		if reference.environment.resolveBinding(reference.name) == nil {
			if reference.strict {
				return referenceError(reference.name + " is not defined")
			}
			reference.runtime.global.createMutableBinding(reference.name, value)
			globalThis, _ := reference.runtime.global.getBindingValue("globalThis")
			_ = setProperty(reference.runtime, globalThis, StringKey(reference.name), value)
			return nil
		}
		return reference.environment.putMutableBinding(reference.name, value, reference.strict)
	case referenceProperty:
		object := objectRecord(reference.base)
		if object == nil {
			if reference.strict {
				return typeError("cannot assign to property")
			}
			return nil
		}
		receiver := reference.base
		if reference.hasReceiver {
			receiver = reference.receiver
		}
		succeeded, err := ordinarySet(reference.runtime, object, reference.key, value, receiver)
		if err != nil {
			return err
		}
		if !succeeded && reference.strict {
			return typeError("cannot assign to property")
		}
		return nil
	case referencePrivate:
		return reference.putPrivateValue(value)
	default:
		return referenceError("invalid assignment target")
	}
}

func (reference reference) delete() (bool, error) {
	switch reference.kind {
	case referenceBinding:
		if reference.environment.resolveBinding(reference.name) == nil {
			return true, nil
		}
		return false, nil
	case referenceProperty:
		if reference.base.k == KindNull || reference.base.k == KindUndefined {
			return false, typeError("cannot convert null or undefined to object")
		}
		object := objectRecord(reference.base)
		if object == nil {
			if reference.base.k == KindString && !reference.key.isSymbol() {
				if _, found := stringOwnProperty(reference.base, reference.key.goString()); found {
					return false, nil
				}
			}
			return true, nil
		}
		return ordinaryDelete(object, reference.key), nil
	case referencePrivate:
		return false, typeError("private class elements cannot be deleted")
	default:
		return true, nil
	}
}

func (reference reference) thisValue() Value {
	if reference.kind == referenceProperty || reference.kind == referencePrivate {
		if reference.hasReceiver {
			return reference.receiver
		}
		return reference.base
	}
	return Undefined()
}

func (reference reference) getPrivateValue() (Value, error) {
	object := objectRecord(reference.base)
	if object == nil {
		return Undefined(), typeError("cannot read a private class element from a non-object")
	}
	element, found := object.privateElements[reference.privateName]
	if !found {
		return Undefined(), typeError("object does not have the requested private class element")
	}
	if !element.accessor {
		return element.value, nil
	}
	if element.getter.k != KindFunction || element.getter.f.call == nil {
		return Undefined(), typeError("private accessor has no getter")
	}
	return reference.runtime.call(element.getter, reference.base, nil, Span{})
}

func (reference reference) putPrivateValue(value Value) error {
	object := objectRecord(reference.base)
	if object == nil {
		return typeError("cannot write a private class element on a non-object")
	}
	element, found := object.privateElements[reference.privateName]
	if !found {
		return typeError("object does not have the requested private class element")
	}
	if !element.accessor {
		if !element.writable {
			return typeError("private method is not writable")
		}
		element.value = value
		object.privateElements[reference.privateName] = element
		return nil
	}
	if element.setter.k != KindFunction || element.setter.f.call == nil {
		return typeError("private accessor has no setter")
	}
	_, err := reference.runtime.call(element.setter, reference.base, []Value{value}, Span{})
	return err
}
