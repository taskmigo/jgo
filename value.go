package gots

import (
	"fmt"
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
)

type Value struct {
	k Kind
	b bool
	n float64
	s string
	o *Object
	f *function
}

func Undefined() Value            { return Value{k: KindUndefined} }
func Null() Value                 { return Value{k: KindNull} }
func Boolean(v bool) Value        { return Value{k: KindBoolean, b: v} }
func Number(v float64) Value      { return Value{k: KindNumber, n: v} }
func String(v string) Value       { return Value{k: KindString, s: v} }
func (v Value) Kind() Kind        { return v.k }
func (v Value) IsUndefined() bool { return v.k == KindUndefined }
func (v Value) Bool() bool        { return truthy(v) }
func (v Value) Float64() float64  { return number(v) }
func (v Value) String() string {
	switch v.k {
	case KindUndefined:
		return "undefined"
	case KindNull:
		return "null"
	case KindBoolean:
		return strconv.FormatBool(v.b)
	case KindNumber:
		return strconv.FormatFloat(v.n, 'g', -1, 64)
	case KindString:
		return v.s
	case KindFunction:
		return "function"
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
	identity *weakIdentity
	native   NativeFunction
	params   []string
	body     []stmt
	closure  *environment
	name     string
}

func nativeValue(fn NativeFunction) Value {
	return Value{k: KindFunction, f: &function{identity: newIdentity(), native: fn}}
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
	} else {
		return weak.Pointer[weakIdentity]{}, nil, false
	}
	return weak.Make(id), id, true
}
func property(v Value, key string) (Value, bool) {
	if v.k == KindObject {
		x, ok := v.o.props[key]
		return x, ok
	}
	return Undefined(), false
}
func setProperty(v Value, key string, x Value) error {
	if v.k != KindObject {
		return fmt.Errorf("cannot set property on %s", v.String())
	}
	v.o.props[key] = x
	return nil
}
