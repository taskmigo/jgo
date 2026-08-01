package gots

import (
	"strconv"
	"strings"
)

func (r *Runtime) installArrayBuiltin() {
	r.global.define("Array", newArrayConstructor(), false)
}

func newArrayConstructor() Value {
	constructor := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		return NewArray(args...), nil
	})
	constructor.f.props["of"] = nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		return NewArray(args...), nil
	})
	constructor.f.props["from"] = nativeValue(arrayFrom)
	return constructor
}

func arrayFrom(_ *Runtime, _ Value, args []Value) (Value, error) {
	if len(args) == 0 {
		return NewArray(), nil
	}
	source := args[0]
	if source.k == KindObject && source.o.array {
		length := int(number(source.o.props["length"]))
		values := make([]Value, length)
		for index := range length {
			values[index] = source.o.props[strconv.Itoa(index)]
		}
		return NewArray(values...), nil
	}
	if source.k == KindString {
		values := make([]Value, 0, len([]rune(source.s)))
		for _, character := range source.s {
			values = append(values, String(string(character)))
		}
		return NewArray(values...), nil
	}
	return Undefined(), &RuntimeError{Message: "Array.from source is not iterable"}
}

func stringProperty(value Value, key string) (Value, bool) {
	characters := []rune(value.s)
	if key == "length" {
		return Number(float64(len(characters))), true
	}

	switch key {
	case "at":
		return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
			index := int(number(argument(args, 0)))
			if index < 0 {
				index += len(characters)
			}
			if index < 0 || index >= len(characters) {
				return Undefined(), nil
			}
			return String(string(characters[index])), nil
		}), true
	case "includes":
		return stringSearch(value.s, func(receiver, query string, start int) bool {
			return strings.Contains(receiver[start:], query)
		}), true
	case "startsWith":
		return stringSearch(value.s, func(receiver, query string, start int) bool {
			return strings.HasPrefix(receiver[start:], query)
		}), true
	case "endsWith":
		return stringSearch(value.s, func(receiver, query string, end int) bool {
			if end == 0 || end > len(receiver) {
				end = len(receiver)
			}
			return strings.HasSuffix(receiver[:end], query)
		}), true
	case "repeat":
		return repeatString(value), true
	case "padStart", "padEnd":
		return padString(value, key == "padStart"), true
	case "trimStart":
		return nativeValue(func(_ *Runtime, _ Value, _ []Value) (Value, error) {
			return String(strings.TrimLeftFunc(value.s, isTrimSpace)), nil
		}), true
	case "trimEnd":
		return nativeValue(func(_ *Runtime, _ Value, _ []Value) (Value, error) {
			return String(strings.TrimRightFunc(value.s, isTrimSpace)), nil
		}), true
	case "replaceAll":
		return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
			if len(args) < 2 {
				return value, nil
			}
			return String(strings.ReplaceAll(value.s, args[0].String(), args[1].String())), nil
		}), true
	default:
		return Undefined(), false
	}
}

func arrayProperty(value Value, key string) (Value, bool) {
	switch key {
	case "at":
		return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
			length := int(number(value.o.props["length"]))
			index := int(number(argument(args, 0)))
			if index < 0 {
				index += length
			}
			if index < 0 || index >= length {
				return Undefined(), nil
			}
			return value.o.props[strconv.Itoa(index)], nil
		}), true
	case "includes":
		return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
			if len(args) == 0 {
				return Boolean(false), nil
			}
			length := int(number(value.o.props["length"]))
			for index := 0; index < length; index++ {
				if sameValueZero(value.o.props[strconv.Itoa(index)], args[0]) {
					return Boolean(true), nil
				}
			}
			return Boolean(false), nil
		}), true
	default:
		return Undefined(), false
	}
}

func stringSearch(receiver string, search func(string, string, int) bool) Value {
	return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		query := argument(args, 0).String()
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

func repeatString(value Value) Value {
	return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		count := int(number(argument(args, 0)))
		if count < 0 {
			return Undefined(), &RuntimeError{Message: "invalid repeat count"}
		}
		return String(strings.Repeat(value.s, count)), nil
	})
}

func padString(value Value, atStart bool) Value {
	return nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		targetLength := int(number(argument(args, 0)))
		fill := " "
		if len(args) > 1 {
			fill = args[1].String()
		}
		if len(value.s) >= targetLength || fill == "" {
			return value, nil
		}
		paddingLength := targetLength - len(value.s)
		padding := strings.Repeat(fill, (paddingLength+len(fill)-1)/len(fill))[:paddingLength]
		if atStart {
			return String(padding + value.s), nil
		}
		return String(value.s + padding), nil
	})
}

func isTrimSpace(character rune) bool {
	return character == ' ' || character == '\n' || character == '\t' || character == '\r'
}
