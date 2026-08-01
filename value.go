package gots

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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

// SameValue reports primitive equality or object/function identity.
func SameValue(a, b Value) bool { return equal(a, b) }
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
func property(v Value, key string) (Value, bool) {
	if v.k == KindObject {
		x, ok := v.o.props[key]
		if ok {
			return x, true
		}
	}
	if v.k == KindFunction {
		x, ok := v.f.props[key]
		return x, ok
	}
	if v.k == KindString {
		runes := []rune(v.s)
		if key == "length" {
			return Number(float64(len(runes))), true
		}
		switch key {
		case "at":
			return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
				i := 0
				if len(args) > 0 {
					i = int(number(args[0]))
				}
				if i < 0 {
					i += len(runes)
				}
				if i < 0 || i >= len(runes) {
					return Undefined(), nil
				}
				return String(string(runes[i])), nil
			}), true
		case "includes":
			return stringSearch(v.s, func(s, q string, n int) bool { return strings.Contains(s[n:], q) }), true
		case "startsWith":
			return stringSearch(v.s, func(s, q string, n int) bool { return strings.HasPrefix(s[n:], q) }), true
		case "endsWith":
			return stringSearch(v.s, func(s, q string, n int) bool {
				if n == 0 || n > len(s) {
					n = len(s)
				}
				return strings.HasSuffix(s[:n], q)
			}), true
		case "repeat":
			return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
				n := 0
				if len(args) > 0 {
					n = int(number(args[0]))
				}
				if n < 0 {
					return Undefined(), &RuntimeError{Message: "invalid repeat count"}
				}
				return String(strings.Repeat(v.s, n)), nil
			}), true
		case "padStart", "padEnd":
			start := key == "padStart"
			return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
				target := 0
				if len(args) > 0 {
					target = int(number(args[0]))
				}
				fill := " "
				if len(args) > 1 {
					fill = args[1].String()
				}
				if len(v.s) >= target || fill == "" {
					return v, nil
				}
				need := target - len(v.s)
				padding := strings.Repeat(fill, (need+len(fill)-1)/len(fill))[:need]
				if start {
					return String(padding + v.s), nil
				}
				return String(v.s + padding), nil
			}), true
		case "trimStart":
			return nativeValue(func(_ *Runtime, _ Value, _ []Value) (Value, error) {
				return String(strings.TrimLeftFunc(v.s, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '\r' })), nil
			}), true
		case "trimEnd":
			return nativeValue(func(_ *Runtime, _ Value, _ []Value) (Value, error) {
				return String(strings.TrimRightFunc(v.s, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '\r' })), nil
			}), true
		case "replaceAll":
			return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
				if len(args) < 2 {
					return v, nil
				}
				return String(strings.ReplaceAll(v.s, args[0].String(), args[1].String())), nil
			}), true
		}
	}
	if v.k == KindObject && v.o.array {
		switch key {
		case "at":
			return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
				n := int(number(v.o.props["length"]))
				i := 0
				if len(args) > 0 {
					i = int(number(args[0]))
				}
				if i < 0 {
					i += n
				}
				if i < 0 || i >= n {
					return Undefined(), nil
				}
				return v.o.props[strconv.Itoa(i)], nil
			}), true
		case "includes":
			return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
				if len(args) == 0 {
					return Boolean(false), nil
				}
				n := int(number(v.o.props["length"]))
				for i := 0; i < n; i++ {
					if SameValue(v.o.props[strconv.Itoa(i)], args[0]) {
						return Boolean(true), nil
					}
				}
				return Boolean(false), nil
			}), true
		}
		return Undefined(), false
	}
	return Undefined(), false
}

func stringSearch(receiver string, search func(string, string, int) bool) Value {
	return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		query := "undefined"
		if len(args) > 0 {
			query = args[0].String()
		}
		start := 0
		if len(args) > 1 {
			start = int(number(args[1]))
			if start < 0 {
				start = 0
			}
			if start > len(receiver) {
				start = len(receiver)
			}
		}
		return Boolean(search(receiver, query, start)), nil
	})
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
