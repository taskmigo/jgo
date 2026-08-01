package gots

func property(value Value, key string) (Value, bool) {
	if ownProperty, found := ownProperty(value, key); found {
		return ownProperty, true
	}
	if value.k == KindString {
		return stringProperty(value, key)
	}
	if value.k == KindObject && value.o.array {
		return arrayProperty(value, key)
	}
	return Undefined(), false
}

func ownProperty(value Value, key string) (Value, bool) {
	switch value.k {
	case KindObject:
		property, found := value.o.props[key]
		return property, found
	case KindFunction:
		property, found := value.f.props[key]
		return property, found
	default:
		return Undefined(), false
	}
}
