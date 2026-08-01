package gots

import (
	"fmt"
	"math"
	"strconv"
	"weak"
)

type Kind uint8

const (
	KindUndefined Kind = iota
	KindNull
	KindBoolean
	KindNumber
	KindString
	KindObject
	KindFunction
	KindSymbol
)

type Value struct {
	k  Kind
	b  bool
	n  float64
	s  string
	o  *Object
	f  *function
	sy *symbolValue
}

func Undefined() Value            { return Value{k: KindUndefined} }
func Null() Value                 { return Value{k: KindNull} }
func Boolean(v bool) Value        { return Value{k: KindBoolean, b: v} }
func Number(v float64) Value      { return Value{k: KindNumber, n: v} }
func String(v string) Value       { return Value{k: KindString, s: v} }
func (v Value) Kind() Kind        { return v.k }
func (v Value) IsUndefined() bool { return v.k == KindUndefined }

// IsConstructor reports whether a value implements ECMAScript [[Construct]].
func (v Value) IsConstructor() bool { return v.k == KindFunction && !v.f.noConstruct }
func (v Value) Bool() bool          { return truthy(v) }
func (v Value) Float64() float64    { return number(v) }
func (v Value) String() string {
	switch v.k {
	case KindUndefined:
		return "undefined"
	case KindNull:
		return "null"
	case KindBoolean:
		return strconv.FormatBool(v.b)
	case KindNumber:
		if math.IsNaN(v.n) {
			return "NaN"
		}
		if math.IsInf(v.n, 1) {
			return "Infinity"
		}
		if math.IsInf(v.n, -1) {
			return "-Infinity"
		}
		return strconv.FormatFloat(v.n, 'g', -1, 64)
	case KindString:
		return v.s
	case KindFunction:
		return "function"
	case KindSymbol:
		return "Symbol(" + v.sy.description + ")"
	case KindObject:
		if v.o.array {
			return "[object Array]"
		}
		return "[object Object]"
	}
	return "undefined"
}

type weakIdentity struct {
	padding [32]byte
	p       *byte
}
type symbolValue struct {
	identity    *weakIdentity
	description string
	registered  bool
}

func newIdentity() *weakIdentity { return &weakIdentity{p: new(byte)} }

type Object struct {
	identity *weakIdentity
	props    map[string]Value
	array    bool
	weakmap  *weakMapData
}

func NewObject() Value {
	return Value{k: KindObject, o: &Object{identity: newIdentity(), props: map[string]Value{}}}
}

// SetProperty defines an own property on an object for host-provided APIs.
func (v Value) SetProperty(name string, value Value) error { return setProperty(v, name, value) }

// SameValue implements the ECMAScript SameValue abstract operation. Unlike
// strict equality it considers NaN equal to itself and distinguishes signed
// zero. It never coerces either operand.
func SameValue(a, b Value) bool {
	if a.k != b.k {
		return false
	}
	switch a.k {
	case KindUndefined, KindNull:
		return true
	case KindBoolean:
		return a.b == b.b
	case KindNumber:
		if math.IsNaN(a.n) || math.IsNaN(b.n) {
			return math.IsNaN(a.n) && math.IsNaN(b.n)
		}
		if a.n == 0 && b.n == 0 {
			return math.Signbit(a.n) == math.Signbit(b.n)
		}
		return a.n == b.n
	case KindString:
		return a.s == b.s
	case KindObject:
		return a.o == b.o
	case KindFunction:
		return a.f == b.f
	case KindSymbol:
		return a.sy == b.sy
	}
	return false
}

// sameValueZero is used by collection-like operations such as
// Array.prototype.includes. It differs from SameValue only for signed zero.
func sameValueZero(a, b Value) bool {
	if a.k == KindNumber && b.k == KindNumber && math.IsNaN(a.n) && math.IsNaN(b.n) {
		return true
	}
	return strictlyEqual(a, b)
}
func NewArray(values ...Value) Value {
	o := &Object{identity: newIdentity(), props: map[string]Value{}, array: true}
	for i, v := range values {
		o.props[strconv.Itoa(i)] = v
	}
	o.props["length"] = Number(float64(len(values)))
	return Value{k: KindObject, o: o}
}

type NativeFunction func(runtime *Runtime, this Value, args []Value) (Value, error)
type function struct {
	identity      *weakIdentity
	native        NativeFunction
	props         map[string]Value
	constructOnly bool
	noConstruct   bool
	params        []string
	body          []stmt
	closure       *environment
	name          string
}

func nativeValue(fn NativeFunction) Value {
	return Value{k: KindFunction, f: &function{identity: newIdentity(), native: fn, props: map[string]Value{}}}
}
func truthy(v Value) bool {
	switch v.k {
	case KindUndefined, KindNull:
		return false
	case KindBoolean:
		return v.b
	case KindNumber:
		return v.n != 0
	case KindString:
		return v.s != ""
	default:
		return true
	}
}
func number(v Value) float64 {
	switch v.k {
	case KindNumber:
		return v.n
	case KindBoolean:
		if v.b {
			return 1
		}
		return 0
	case KindNull:
		return 0
	case KindString:
		n, _ := strconv.ParseFloat(v.s, 64)
		return n
	default:
		return 0
	}
}
func identity(v Value) (weak.Pointer[weakIdentity], *weakIdentity, bool) {
	var id *weakIdentity
	if v.k == KindObject && v.o != nil {
		id = v.o.identity
	} else if v.k == KindFunction && v.f != nil {
		id = v.f.identity
	} else if v.k == KindSymbol && v.sy != nil && !v.sy.registered {
		id = v.sy.identity
	} else {
		return weak.Pointer[weakIdentity]{}, nil, false
	}
	return weak.Make(id), id, true
}
func setProperty(v Value, key string, x Value) error {
	if v.k != KindObject {
		if v.k == KindFunction {
			v.f.props[key] = x
			return nil
		}
		return fmt.Errorf("cannot set property on %s", v.String())
	}
	v.o.props[key] = x
	return nil
}
