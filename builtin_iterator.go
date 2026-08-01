package gots

import (
	"unicode"
	"unicode/utf16"
)

func arrayValuesIterator(runtime *Runtime, receiver Value, _ []Value) (Value, error) {
	object := objectRecord(receiver)
	if object == nil || !object.array {
		return Undefined(), typeError("Array iterator called on incompatible receiver")
	}
	values := make([]Value, arrayLength(object))
	for index := range values {
		if descriptor, found := object.properties[StringKey(fmtInt(index))]; found {
			values[index] = descriptor.Value
		} else {
			values[index] = Undefined()
		}
	}
	return newListIterator(runtime, values), nil
}

func stringValuesIterator(runtime *Runtime, receiver Value, _ []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	values := make([]Value, 0, len(text))
	for index := 0; index < len(text); index++ {
		width := 1
		if utf16.IsSurrogate(rune(text[index])) && index+1 < len(text) {
			decoded := utf16.DecodeRune(rune(text[index]), rune(text[index+1]))
			if decoded != unicode.ReplacementChar {
				width = 2
			}
		}
		values = append(values, StringUTF16(text[index:index+width]))
		index += width - 1
	}
	return newListIterator(runtime, values), nil
}

func newListIterator(runtime *Runtime, values []Value) Value {
	iterator := runtime.newOrdinaryObject()
	index := 0
	next := nativeValue(func(runtime *Runtime, _ Value, _ []Value) (Value, error) {
		result := runtime.newOrdinaryObject()
		done := index >= len(values)
		value := Undefined()
		if !done {
			value = values[index]
			index++
		}
		_ = defineProperty(result, StringKey("value"), defaultProperty(value))
		_ = defineProperty(result, StringKey("done"), defaultProperty(Boolean(done)))
		return result, nil
	})
	_ = defineProperty(iterator, StringKey("next"), defaultProperty(next))
	return iterator
}

func iteratorToList(runtime *Runtime, iterable Value) ([]Value, bool, error) {
	method, found, err := getProperty(runtime, iterable, PropertyKey{symbol: runtime.iteratorSymbol})
	if err != nil || !found || method.IsUndefined() {
		return nil, false, err
	}
	if method.k != KindFunction || method.f.call == nil {
		return nil, false, typeError("iterator method is not callable")
	}
	iterator, err := runtime.call(method, iterable, nil, Span{})
	if err != nil {
		return nil, false, err
	}
	if objectRecord(iterator) == nil {
		return nil, false, typeError("iterator is not an object")
	}
	values := make([]Value, 0)
	for {
		next, _, err := getProperty(runtime, iterator, StringKey("next"))
		if err != nil {
			return nil, false, err
		}
		if next.k != KindFunction || next.f.call == nil {
			return nil, false, typeError("iterator next is not callable")
		}
		result, err := runtime.call(next, iterator, nil, Span{})
		if err != nil {
			return nil, false, err
		}
		if objectRecord(result) == nil {
			return nil, false, typeError("iterator result is not an object")
		}
		done, _, err := getProperty(runtime, result, StringKey("done"))
		if err != nil {
			return nil, false, err
		}
		if toBoolean(done) {
			return values, true, nil
		}
		value, _, err := getProperty(runtime, result, StringKey("value"))
		if err != nil {
			return nil, false, err
		}
		values = append(values, value)
	}
}
