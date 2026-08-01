package gots

import (
	"math"
	"unicode"
)

const maxStringCodeUnits = 1 << 24

func (runtime *Runtime) installArrayBuiltin() {
	constructor := nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		if len(arguments) == 1 && arguments[0].k == KindNumber {
			length := arguments[0].n
			if length < 0 || length > math.MaxUint32 || length != math.Trunc(length) {
				return Undefined(), &Exception{Name: "RangeError", Message: "invalid array length"}
			}
			array := runtime.newArray()
			array.o.properties[StringKey("length")] = PropertyDescriptor{Value: Number(length), Writable: true}
			return array, nil
		}
		return runtime.newArray(arguments...), nil
	})
	constructor.f.construct = constructor.f.call
	constructor.f.object.prototype = runtime.intrinsics.functionPrototype
	defineBuiltin(constructor.f.object, "of", defineFunctionMetadata(nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		return runtime.newArray(arguments...), nil
	}), "of", 0))
	defineBuiltin(constructor.f.object, "from", defineFunctionMetadata(nativeValue(arrayFrom), "from", 1))
	defineBuiltin(runtime.intrinsics.arrayPrototype, "at", defineFunctionMetadata(nativeValue(arrayAt), "at", 1))
	defineBuiltin(runtime.intrinsics.arrayPrototype, "includes", defineFunctionMetadata(nativeValue(arrayIncludes), "includes", 1))
	runtime.intrinsics.arrayPrototype.properties[PropertyKey{symbol: runtime.iteratorSymbol}] = PropertyDescriptor{
		Value: defineFunctionMetadata(nativeValue(arrayValuesIterator), "values", 0), Writable: true, Configurable: true,
	}
	runtime.global.createMutableBinding("Array", defineFunctionMetadata(constructor, "Array", 1))
}

func arrayFrom(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
	source := argument(arguments, 0)
	if source.k == KindNull || source.k == KindUndefined {
		return Undefined(), typeError("Array.from source is not iterable")
	}
	if values, found, err := iteratorToList(runtime, source); err != nil {
		return Undefined(), err
	} else if found {
		return runtime.newArray(values...), nil
	}
	object := objectRecord(source)
	if object == nil || !object.array {
		return Undefined(), typeError("Array.from source is not iterable")
	}
	length := arrayLength(object)
	values := make([]Value, length)
	for index := range values {
		value, _, err := getProperty(runtime, source, StringKey(fmtInt(index)))
		if err != nil {
			return Undefined(), err
		}
		values[index] = value
	}
	return runtime.newArray(values...), nil
}

func arrayAt(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	object := objectRecord(receiver)
	if object == nil || !object.array {
		return Undefined(), typeError("Array.prototype.at called on incompatible receiver")
	}
	length := arrayLength(object)
	index, err := runtime.toIntegerOrInfinity(argument(arguments, 0))
	if err != nil {
		return Undefined(), err
	}
	if index < 0 {
		index += float64(length)
	}
	if index < 0 || index >= float64(length) || math.IsInf(index, 0) {
		return Undefined(), nil
	}
	descriptor, found := object.properties[StringKey(fmtInt(int(index)))]
	if !found {
		return Undefined(), nil
	}
	return descriptor.Value, nil
}

func arrayIncludes(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	object := objectRecord(receiver)
	if object == nil || !object.array {
		return Undefined(), typeError("Array.prototype.includes called on incompatible receiver")
	}
	length := arrayLength(object)
	if length == 0 {
		return Boolean(false), nil
	}
	start, err := runtime.toIntegerOrInfinity(argument(arguments, 1))
	if err != nil {
		return Undefined(), err
	}
	if math.IsInf(start, 1) {
		return Boolean(false), nil
	}
	index := int(start)
	if index < 0 {
		index = length + index
		if index < 0 {
			index = 0
		}
	}
	search := argument(arguments, 0)
	for ; index < length; index++ {
		value := Undefined()
		if descriptor, found := object.properties[StringKey(fmtInt(index))]; found {
			value = descriptor.Value
		}
		if sameValueZero(value, search) {
			return Boolean(true), nil
		}
	}
	return Boolean(false), nil
}

func (runtime *Runtime) installStringBuiltin() {
	constructor := nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		value := argument(arguments, 0)
		if value.IsUndefined() {
			return String(""), nil
		}
		if value.k == KindSymbol {
			return String("Symbol(" + value.sy.description.goString() + ")"), nil
		}
		converted, err := runtime.toString(value)
		if err != nil {
			return Undefined(), err
		}
		return StringUTF16(converted), nil
	})
	constructor.f.object.prototype = runtime.intrinsics.functionPrototype
	for name, method := range map[string]NativeFunction{
		"at": stringAt, "includes": stringIncludes, "startsWith": stringStartsWith,
		"endsWith": stringEndsWith, "repeat": stringRepeat, "padStart": stringPadStart,
		"padEnd": stringPadEnd, "trimStart": stringTrimStart, "trimEnd": stringTrimEnd,
		"replaceAll": stringReplaceAll,
	} {
		length := 1
		if name == "includes" || name == "startsWith" || name == "endsWith" || name == "padStart" || name == "padEnd" || name == "replaceAll" {
			length = 2
		}
		defineBuiltin(runtime.intrinsics.stringPrototype, name, defineFunctionMetadata(nativeValue(method), name, length))
	}
	runtime.intrinsics.stringPrototype.properties[PropertyKey{symbol: runtime.iteratorSymbol}] = PropertyDescriptor{
		Value: defineFunctionMetadata(nativeValue(stringValuesIterator), "[Symbol.iterator]", 0), Writable: true, Configurable: true,
	}
	runtime.global.createMutableBinding("String", defineFunctionMetadata(constructor, "String", 1))
}

func stringReceiver(runtime *Runtime, receiver Value) (jsString, error) {
	if receiver.k == KindNull || receiver.k == KindUndefined {
		return nil, typeError("String method called on null or undefined")
	}
	return runtime.toString(receiver)
}

func stringAt(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	index, err := runtime.toIntegerOrInfinity(argument(arguments, 0))
	if index < 0 {
		index += float64(len(text))
	}
	if err != nil || index < 0 || index >= float64(len(text)) || math.IsInf(index, 0) {
		return Undefined(), err
	}
	return StringUTF16([]uint16{text[int(index)]}), nil
}

func stringIncludes(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	query, err := runtime.toString(argument(arguments, 0))
	if err != nil {
		return Undefined(), err
	}
	start, err := stringPosition(runtime, argument(arguments, 1), len(text), 0)
	return Boolean(indexUTF16(text, query, start) >= 0), err
}

func stringStartsWith(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	query, err := runtime.toString(argument(arguments, 0))
	if err != nil {
		return Undefined(), err
	}
	start, err := stringPosition(runtime, argument(arguments, 1), len(text), 0)
	return Boolean(start+len(query) <= len(text) && jsString(text[start:start+len(query)]).equal(query)), err
}

func stringEndsWith(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	query, err := runtime.toString(argument(arguments, 0))
	if err != nil {
		return Undefined(), err
	}
	end := len(text)
	if len(arguments) > 1 && !arguments[1].IsUndefined() {
		end, err = stringPosition(runtime, arguments[1], len(text), len(text))
	}
	start := end - len(query)
	return Boolean(start >= 0 && jsString(text[start:end]).equal(query)), err
}

func stringRepeat(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	count, err := runtime.toIntegerOrInfinity(argument(arguments, 0))
	if err != nil {
		return Undefined(), err
	}
	if count < 0 || math.IsInf(count, 1) {
		return Undefined(), &Exception{Name: "RangeError", Message: "invalid repeat count"}
	}
	if len(text) == 0 || count == 0 {
		return String(""), nil
	}
	if count > float64(maxStringCodeUnits/len(text)) {
		return Undefined(), &Exception{Name: "RangeError", Message: "invalid string length"}
	}
	result := make([]uint16, 0, len(text)*int(count))
	for range int(count) {
		result = append(result, text...)
	}
	return StringUTF16(result), nil
}

func stringPadStart(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	return padString(runtime, receiver, arguments, true)
}
func stringPadEnd(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	return padString(runtime, receiver, arguments, false)
}
func padString(runtime *Runtime, receiver Value, arguments []Value, atStart bool) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	target, err := runtime.toLength(argument(arguments, 0))
	if err != nil || target <= len(text) {
		return StringUTF16(text), err
	}
	fill := jsString(String(" ").s)
	if len(arguments) > 1 && !arguments[1].IsUndefined() {
		fill, err = runtime.toString(arguments[1])
	}
	if err != nil || len(fill) == 0 {
		return StringUTF16(text), err
	}
	paddingLength := target - len(text)
	if target > maxStringCodeUnits {
		return Undefined(), &Exception{Name: "RangeError", Message: "invalid string length"}
	}
	padding := make([]uint16, paddingLength)
	for index := range padding {
		padding[index] = fill[index%len(fill)]
	}
	if atStart {
		return StringUTF16(append(padding, text...)), nil
	}
	return StringUTF16(append(text.clone(), padding...)), nil
}

func stringTrimStart(runtime *Runtime, receiver Value, _ []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	start := 0
	for start < len(text) && isTrimCodeUnit(text[start]) {
		start++
	}
	return StringUTF16(text[start:]), nil
}
func stringTrimEnd(runtime *Runtime, receiver Value, _ []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	end := len(text)
	for end > 0 && isTrimCodeUnit(text[end-1]) {
		end--
	}
	return StringUTF16(text[:end]), nil
}

func stringReplaceAll(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	text, err := stringReceiver(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	search, err := runtime.toString(argument(arguments, 0))
	if err != nil {
		return Undefined(), err
	}
	replacement, err := runtime.toString(argument(arguments, 1))
	if err != nil {
		return Undefined(), err
	}
	if len(search) == 0 {
		result := make([]uint16, 0, len(text)+(len(text)+1)*len(replacement))
		result = append(result, replacement...)
		for _, codeUnit := range text {
			result = append(result, codeUnit)
			result = append(result, replacement...)
		}
		return StringUTF16(result), nil
	}
	result := make([]uint16, 0, len(text))
	for offset := 0; offset < len(text); {
		index := indexUTF16(text, search, offset)
		if index < 0 {
			result = append(result, text[offset:]...)
			break
		}
		result = append(result, text[offset:index]...)
		result = append(result, replacement...)
		offset = index + len(search)
	}
	return StringUTF16(result), nil
}

func stringPosition(runtime *Runtime, value Value, length, defaultValue int) (int, error) {
	if value.IsUndefined() {
		return defaultValue, nil
	}
	position, err := runtime.toIntegerOrInfinity(value)
	if err != nil {
		return 0, err
	}
	if position < 0 {
		return 0, nil
	}
	if position > float64(length) {
		return length, nil
	}
	return int(position), nil
}

func indexUTF16(text, query jsString, start int) int {
	if len(query) == 0 {
		return start
	}
	for index := start; index+len(query) <= len(text); index++ {
		if jsString(text[index : index+len(query)]).equal(query) {
			return index
		}
	}
	return -1
}

func isTrimCodeUnit(codeUnit uint16) bool {
	return codeUnit == 0xfeff || unicode.IsSpace(rune(codeUnit))
}
