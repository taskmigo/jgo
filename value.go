package gots

import (
	"fmt"
	"math"
	"strconv"
	"unicode/utf16"
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

// Value is an ECMAScript language value. String values are stored as UTF-16
// code units so lone surrogates and indexed string access remain lossless.
type Value struct {
	k  Kind
	b  bool
	n  float64
	s  []uint16
	o  *Object
	f  *function
	sy *symbolValue
}

func Undefined() Value           { return Value{k: KindUndefined} }
func Null() Value                { return Value{k: KindNull} }
func Boolean(value bool) Value   { return Value{k: KindBoolean, b: value} }
func Number(value float64) Value { return Value{k: KindNumber, n: value} }
func String(value string) Value  { return StringUTF16(utf16.Encode([]rune(value))) }

func StringUTF16(codeUnits []uint16) Value {
	return Value{k: KindString, s: append([]uint16(nil), codeUnits...)}
}

func (value Value) Kind() Kind        { return value.k }
func (value Value) IsUndefined() bool { return value.k == KindUndefined }
func (value Value) IsConstructor() bool {
	return value.k == KindFunction && value.f.construct != nil
}
func (value Value) ToBoolean() bool            { return toBoolean(value) }
func (value Value) ToNumber() (float64, error) { return toNumber(value) }
func (value Value) ToString() (string, error) {
	text, err := toString(value)
	if err != nil {
		return "", err
	}
	return text.goString(), nil
}
func (value Value) UTF16() ([]uint16, bool) {
	if value.k != KindString {
		return nil, false
	}
	return append([]uint16(nil), value.s...), true
}

// Inspect returns an error-free representation intended for diagnostics and
// command output. It is deliberately separate from ECMAScript ToString.
func (value Value) Inspect() string {
	switch value.k {
	case KindUndefined:
		return "undefined"
	case KindNull:
		return "null"
	case KindBoolean:
		return strconv.FormatBool(value.b)
	case KindNumber:
		return numberToString(value.n)
	case KindString:
		return jsString(value.s).goString()
	case KindFunction:
		return "function"
	case KindSymbol:
		return "Symbol(" + value.sy.description.goString() + ")"
	case KindObject:
		if value.o.array {
			return "[object Array]"
		}
		return "[object Object]"
	default:
		return "undefined"
	}
}

type jsString []uint16

func (value jsString) clone() jsString  { return append(jsString(nil), value...) }
func (value jsString) goString() string { return string(utf16.Decode(value)) }
func (value jsString) equal(other jsString) bool {
	if len(value) != len(other) {
		return false
	}
	for index := range value {
		if value[index] != other[index] {
			return false
		}
	}
	return true
}

type weakIdentity struct {
	padding [32]byte
	p       *byte
}

type symbolValue struct {
	identity    *weakIdentity
	description jsString
	registered  bool
}

func newIdentity() *weakIdentity { return &weakIdentity{p: new(byte)} }

// PropertyKey is either an ECMAScript String or Symbol property key.
type PropertyKey struct {
	name   string
	symbol *symbolValue
}

func StringKey(name string) PropertyKey { return PropertyKey{name: name} }
func SymbolKey(symbol Value) (PropertyKey, error) {
	if symbol.k != KindSymbol {
		return PropertyKey{}, typeError("property key is not a symbol")
	}
	return PropertyKey{symbol: symbol.sy}, nil
}

func (key PropertyKey) isSymbol() bool { return key.symbol != nil }

// PropertyDescriptor models a data property. Accessor descriptors can be
// added without changing the object storage or lookup algorithms.
type PropertyDescriptor struct {
	Value        Value
	Writable     bool
	Enumerable   bool
	Configurable bool
}

type Object struct {
	identity   *weakIdentity
	properties map[PropertyKey]PropertyDescriptor
	prototype  *Object
	array      bool
	weakmap    *weakMapData
	boxed      Value
	extensible bool
}

func newObject(prototype *Object) *Object {
	return &Object{identity: newIdentity(), properties: make(map[PropertyKey]PropertyDescriptor), prototype: prototype, extensible: true}
}

func NewObject() Value { return Value{k: KindObject, o: newObject(nil)} }

func NewArray(values ...Value) Value {
	object := newObject(nil)
	object.array = true
	for index, value := range values {
		object.properties[StringKey(strconv.Itoa(index))] = defaultProperty(value)
	}
	object.properties[StringKey("length")] = PropertyDescriptor{Value: Number(float64(len(values))), Writable: true}
	return Value{k: KindObject, o: object}
}

func defaultProperty(value Value) PropertyDescriptor {
	return PropertyDescriptor{Value: value, Writable: true, Enumerable: true, Configurable: true}
}

type NativeFunction func(runtime *Runtime, this Value, arguments []Value) (Value, error)

type function struct {
	identity  *weakIdentity
	object    *Object
	call      NativeFunction
	construct NativeFunction
	params    []string
	body      []stmt
	closure   *environment
	name      string
}

func nativeValue(call NativeFunction) Value {
	return Value{k: KindFunction, f: &function{identity: newIdentity(), object: newObject(nil), call: call}}
}

func identity(value Value) (weak.Pointer[weakIdentity], *weakIdentity, bool) {
	var identity *weakIdentity
	switch value.k {
	case KindObject:
		if value.o != nil {
			identity = value.o.identity
		}
	case KindFunction:
		if value.f != nil {
			identity = value.f.identity
		}
	case KindSymbol:
		if value.sy != nil && !value.sy.registered {
			identity = value.sy.identity
		}
	}
	if identity == nil {
		return weak.Pointer[weakIdentity]{}, nil, false
	}
	return weak.Make(identity), identity, true
}

func numberToString(number float64) string {
	switch {
	case math.IsNaN(number):
		return "NaN"
	case math.IsInf(number, 1):
		return "Infinity"
	case math.IsInf(number, -1):
		return "-Infinity"
	case number == 0:
		return "0"
	default:
		return strconv.FormatFloat(number, 'g', -1, 64)
	}
}

func (value Value) debugType() string {
	return fmt.Sprintf("Kind(%d)", value.k)
}
