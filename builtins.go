package gots

import "math"

func (r *Runtime) installGlobalBuiltins() {
	r.installWeakMapBuiltin()
	r.installSymbolBuiltin()
	r.global.define("NaN", Number(math.NaN()), true)
	r.global.define("Infinity", Number(math.Inf(1)), true)
	r.installArrayBuiltin()
	r.installObjectBuiltin()
	r.installGlobalThis()
}

func (r *Runtime) installGlobalThis() {
	globalThis := NewObject()
	globalThis.o.props["globalThis"] = globalThis
	r.global.define("globalThis", globalThis, true)
}

func argument(args []Value, index int) Value {
	if index >= len(args) {
		return Undefined()
	}
	return args[index]
}
