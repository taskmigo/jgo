package gots

import (
	"context"
	"math"
	"strconv"
	"strings"
)

func toBoolean(value Value) bool {
	switch value.k {
	case KindUndefined, KindNull:
		return false
	case KindBoolean:
		return value.b
	case KindNumber:
		return value.n != 0 && !math.IsNaN(value.n)
	case KindString:
		return len(value.s) != 0
	default:
		return true
	}
}

func toNumber(value Value) (float64, error) {
	switch value.k {
	case KindUndefined:
		return math.NaN(), nil
	case KindNull:
		return 0, nil
	case KindBoolean:
		if value.b {
			return 1, nil
		}
		return 0, nil
	case KindNumber:
		return value.n, nil
	case KindString:
		return parseStringNumber(jsString(value.s).goString()), nil
	case KindSymbol:
		return 0, typeError("cannot convert a Symbol value to a number")
	default:
		primitive, err := toPrimitive(value)
		if err != nil {
			return 0, err
		}
		return toNumber(primitive)
	}
}

func parseStringNumber(text string) float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	sign := 1.0
	unsigned := text
	if strings.HasPrefix(unsigned, "+") {
		unsigned = unsigned[1:]
	} else if strings.HasPrefix(unsigned, "-") {
		sign = -1
		unsigned = unsigned[1:]
	}
	if unsigned == "Infinity" {
		return math.Inf(int(sign))
	}
	if sign > 0 && len(unsigned) > 2 {
		base := 0
		switch unsigned[:2] {
		case "0x", "0X":
			base = 16
		case "0o", "0O":
			base = 8
		case "0b", "0B":
			base = 2
		}
		if base != 0 {
			integer, err := strconv.ParseUint(unsigned[2:], base, 64)
			if err != nil {
				return math.NaN()
			}
			return float64(integer)
		}
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return math.NaN()
	}
	return number
}

func toPrimitive(value Value) (Value, error) {
	if value.k != KindObject && value.k != KindFunction {
		return value, nil
	}
	return String(value.Inspect()), nil
}

func (runtime *Runtime) toPrimitive(value Value, preferString bool) (Value, error) {
	if objectRecord(value) == nil {
		return value, nil
	}
	methodNames := [...]string{"valueOf", "toString"}
	if preferString {
		methodNames[0], methodNames[1] = methodNames[1], methodNames[0]
	}
	for _, methodName := range methodNames {
		method, found, err := getProperty(runtime, value, StringKey(methodName))
		if err != nil {
			return Undefined(), err
		}
		if !found || method.k != KindFunction || method.f.call == nil {
			continue
		}
		var result Value
		if runtime.exec == nil {
			callResult, callErr := runtime.Call(context.Background(), method, value)
			result, err = callResult.Value, callErr
		} else {
			result, err = runtime.call(method, value, nil, Span{})
		}
		if err != nil {
			return Undefined(), err
		}
		if objectRecord(result) == nil {
			return result, nil
		}
	}
	return Undefined(), typeError("cannot convert object to primitive value")
}

func (runtime *Runtime) toNumber(value Value) (float64, error) {
	if objectRecord(value) == nil {
		return toNumber(value)
	}
	primitive, err := runtime.toPrimitive(value, false)
	if err != nil {
		return 0, err
	}
	return toNumber(primitive)
}

func (runtime *Runtime) toString(value Value) (jsString, error) {
	if objectRecord(value) == nil {
		return toString(value)
	}
	primitive, err := runtime.toPrimitive(value, true)
	if err != nil {
		return nil, err
	}
	return toString(primitive)
}

func (runtime *Runtime) toPropertyKey(value Value) (PropertyKey, error) {
	primitive, err := runtime.toPrimitive(value, true)
	if err != nil {
		return PropertyKey{}, err
	}
	return toPropertyKey(primitive)
}

func (runtime *Runtime) looselyEqual(left, right Value) (bool, error) {
	if left.k == right.k {
		return strictlyEqual(left, right), nil
	}
	if (left.k == KindNull && right.k == KindUndefined) || (left.k == KindUndefined && right.k == KindNull) {
		return true, nil
	}
	if left.k == KindNumber && right.k == KindString {
		number, err := runtime.toNumber(right)
		return left.n == number, err
	}
	if left.k == KindString && right.k == KindNumber {
		return runtime.looselyEqual(right, left)
	}
	if left.k == KindBoolean {
		return runtime.looselyEqual(Number(boolNumber(left.b)), right)
	}
	if right.k == KindBoolean {
		return runtime.looselyEqual(left, Number(boolNumber(right.b)))
	}
	if (left.k == KindString || left.k == KindNumber || left.k == KindSymbol) && objectRecord(right) != nil {
		primitive, err := runtime.toPrimitive(right, false)
		if err != nil {
			return false, err
		}
		return runtime.looselyEqual(left, primitive)
	}
	if objectRecord(left) != nil && (right.k == KindString || right.k == KindNumber || right.k == KindSymbol) {
		primitive, err := runtime.toPrimitive(left, false)
		if err != nil {
			return false, err
		}
		return runtime.looselyEqual(primitive, right)
	}
	return false, nil
}

func toString(value Value) (jsString, error) {
	switch value.k {
	case KindString:
		return jsString(value.s).clone(), nil
	case KindUndefined:
		return jsString(String("undefined").s), nil
	case KindNull:
		return jsString(String("null").s), nil
	case KindBoolean:
		return jsString(String(strconv.FormatBool(value.b)).s), nil
	case KindNumber:
		return jsString(String(numberToString(value.n)).s), nil
	case KindSymbol:
		return nil, typeError("cannot convert a Symbol value to a string")
	default:
		primitive, err := toPrimitive(value)
		if err != nil {
			return nil, err
		}
		return toString(primitive)
	}
}

func toPropertyKey(value Value) (PropertyKey, error) {
	primitive, err := toPrimitive(value)
	if err != nil {
		return PropertyKey{}, err
	}
	if primitive.k == KindSymbol {
		return PropertyKey{symbol: primitive.sy}, nil
	}
	text, err := toString(primitive)
	if err != nil {
		return PropertyKey{}, err
	}
	return StringKey(text.goString()), nil
}

func strictlyEqual(left, right Value) bool {
	if left.k != right.k {
		return false
	}
	switch left.k {
	case KindUndefined, KindNull:
		return true
	case KindBoolean:
		return left.b == right.b
	case KindNumber:
		return left.n == right.n
	case KindString:
		return jsString(left.s).equal(right.s)
	case KindObject:
		return left.o == right.o
	case KindFunction:
		return left.f == right.f
	case KindSymbol:
		return left.sy == right.sy
	default:
		return false
	}
}

func boolNumber(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func (runtime *Runtime) toIntegerOrInfinity(value Value) (float64, error) {
	number, err := runtime.toNumber(value)
	if err != nil || math.IsNaN(number) || number == 0 {
		return 0, err
	}
	if math.IsInf(number, 0) {
		return number, nil
	}
	return math.Trunc(number), nil
}

func (runtime *Runtime) toLength(value Value) (int, error) {
	integer, err := runtime.toIntegerOrInfinity(value)
	if err != nil || integer <= 0 {
		return 0, err
	}
	const maxSafeInteger = 1<<53 - 1
	if math.IsInf(integer, 1) || integer > maxSafeInteger {
		integer = maxSafeInteger
	}
	if integer > float64(int(^uint(0)>>1)) {
		return int(^uint(0) >> 1), nil
	}
	return int(integer), nil
}

func SameValue(left, right Value) bool {
	if left.k == KindNumber && right.k == KindNumber {
		if math.IsNaN(left.n) && math.IsNaN(right.n) {
			return true
		}
		if left.n == 0 && right.n == 0 {
			return math.Signbit(left.n) == math.Signbit(right.n)
		}
	}
	return strictlyEqual(left, right)
}

func sameValueZero(left, right Value) bool {
	if left.k == KindNumber && right.k == KindNumber && math.IsNaN(left.n) && math.IsNaN(right.n) {
		return true
	}
	return strictlyEqual(left, right)
}
