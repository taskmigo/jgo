package gots

func (r *Runtime) installSymbolBuiltin() {
	r.global.define("Symbol", newSymbolConstructor(), false)
}

func newSymbolConstructor() Value {
	constructor := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		description := ""
		if value := argument(args, 0); !value.IsUndefined() {
			description = value.String()
		}
		return newSymbol(description, false), nil
	})
	constructor.f.noConstruct = true
	constructor.f.props["for"] = nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		return newSymbol(argument(args, 0).String(), true), nil
	})
	return constructor
}

func newSymbol(description string, registered bool) Value {
	return Value{k: KindSymbol, sy: &symbolValue{
		identity:    newIdentity(),
		description: description,
		registered:  registered,
	}}
}

func (r *Runtime) installObjectBuiltin() {
	r.global.define("Object", newObjectConstructor(), false)
}

func newObjectConstructor() Value {
	constructor := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		value := argument(args, 0)
		if value.k == KindObject || value.k == KindFunction {
			return value, nil
		}
		return NewObject(), nil
	})
	constructor.f.props["hasOwn"] = nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		if len(args) < 2 {
			return Boolean(false), nil
		}
		_, found := property(args[0], args[1].String())
		return Boolean(found), nil
	})
	constructor.f.props["is"] = newObjectIsFunction()
	return constructor
}

func newObjectIsFunction() Value {
	objectIs := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		return Boolean(SameValue(argument(args, 0), argument(args, 1))), nil
	})
	objectIs.f.noConstruct = true
	objectIs.f.props["name"] = String("is")
	objectIs.f.props["length"] = Number(2)
	objectIs.f.props["call"] = nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		return Boolean(SameValue(argument(args, 1), argument(args, 2))), nil
	})
	return objectIs
}
