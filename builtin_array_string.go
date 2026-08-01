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
	linkConstructor(constructor, runtime.intrinsics.arrayPrototype)
	defineBuiltin(constructor.f.object, "of", defineFunctionMetadata(nativeValue(arrayOf), "of", 0))
	defineBuiltin(constructor.f.object, "from", defineFunctionMetadata(nativeValue(arrayFrom), "from", 1))
	defineBuiltin(runtime.intrinsics.arrayPrototype, "at", defineFunctionMetadata(nativeValue(arrayAt), "at", 1))
	defineBuiltin(runtime.intrinsics.arrayPrototype, "includes", defineFunctionMetadata(nativeValue(arrayIncludes), "includes", 1))
	defineBuiltin(runtime.intrinsics.arrayPrototype, "join", defineFunctionMetadata(nativeValue(arrayJoin), "join", 1))
	defineBuiltin(runtime.intrinsics.arrayPrototype, "push", defineFunctionMetadata(nativeValue(arrayPush), "push", 1))
	runtime.intrinsics.arrayPrototype.properties[PropertyKey{symbol: runtime.iteratorSymbol}] = PropertyDescriptor{
		Value: defineFunctionMetadata(nativeValue(arrayValuesIterator), "values", 0), Writable: true, Configurable: true,
	}
	runtime.global.createMutableBinding("Array", defineFunctionMetadata(constructor, "Array", 1))
}

func arrayOf(runtime *Runtime, constructor Value, arguments []Value) (Value, error) {
	result := runtime.newArray()
	if constructor.IsConstructor() {
		constructed, err := runtime.construct(constructor, []Value{Number(float64(len(arguments)))}, Span{})
		if err != nil {
			return Undefined(), err
		}
		result = constructed
	}
	if objectRecord(result) == nil {
		return Undefined(), typeError("Array.of constructor did not return an object")
	}
	for index, value := range arguments {
		if err := defineProperty(result, StringKey(fmtInt(index)), defaultProperty(value)); err != nil {
			return Undefined(), err
		}
	}
	if err := setProperty(result, StringKey("length"), Number(float64(len(arguments)))); err != nil {
		return Undefined(), err
	}
	return result, nil
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
	length, err := lengthOfArrayLike(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
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
	value, _, err := getProperty(runtime, receiver, StringKey(fmtInt(int(index))))
	return value, err
}

func arrayIncludes(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	length, err := lengthOfArrayLike(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
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
		value, _, err := getProperty(runtime, receiver, StringKey(fmtInt(index)))
		if err != nil {
			return Undefined(), err
		}
		if sameValueZero(value, search) {
			return Boolean(true), nil
		}
	}
	return Boolean(false), nil
}

func arrayJoin(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	length, err := lengthOfArrayLike(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	separator := jsString{','}
	if value := argument(arguments, 0); !value.IsUndefined() {
		separator, err = runtime.toString(value)
		if err != nil {
			return Undefined(), err
		}
	}

	var result jsString
	for index := 0; index < length; index++ {
		if index > 0 {
			result = append(result, separator...)
		}
		element, _, err := getProperty(runtime, receiver, StringKey(fmtInt(index)))
		if err != nil {
			return Undefined(), err
		}
		if element.k == KindUndefined || element.k == KindNull {
			continue
		}
		text, err := runtime.toString(element)
		if err != nil {
			return Undefined(), err
		}
		result = append(result, text...)
	}
	return StringUTF16(result), nil
}

func arrayPush(runtime *Runtime, receiver Value, arguments []Value) (Value, error) {
	length, err := lengthOfArrayLike(runtime, receiver)
	if err != nil {
		return Undefined(), err
	}
	const maxSafeInteger = 1<<53 - 1
	if length > maxSafeInteger-len(arguments) {
		return Undefined(), typeError("Array.prototype.push exceeds the maximum safe integer")
	}
	for offset, value := range arguments {
		if err := setProperty(receiver, StringKey(fmtInt(length+offset)), value); err != nil {
			return Undefined(), err
		}
	}
	newLength := length + len(arguments)
	if err := setProperty(receiver, StringKey("length"), Number(float64(newLength))); err != nil {
		return Undefined(), err
	}
	return Number(float64(newLength)), nil
}

func lengthOfArrayLike(runtime *Runtime, value Value) (int, error) {
	if value.k == KindNull || value.k == KindUndefined {
		return 0, typeError("cannot convert null or undefined to object")
	}
	length, _, err := getProperty(runtime, value, StringKey("length"))
	if err != nil {
		return 0, err
	}
	return runtime.toLength(length)
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
	constructor.f.construct = func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		if argument(arguments, 0).k == KindSymbol {
			return Undefined(), typeError("cannot convert a Symbol value to a string")
		}
		primitive, err := constructor.f.call(runtime, Undefined(), arguments)
		if err != nil {
			return Undefined(), err
		}
		object := newObject(runtime.intrinsics.stringPrototype)
		object.boxed = primitive
		return Value{k: KindObject, o: object}, nil
	}
	linkConstructor(constructor, runtime.intrinsics.stringPrototype)
	defineBuiltin(runtime.intrinsics.stringPrototype, "valueOf", defineFunctionMetadata(nativeValue(stringValueOf), "valueOf", 0))
	defineBuiltin(runtime.intrinsics.stringPrototype, "toString", defineFunctionMetadata(nativeValue(stringValueOf), "toString", 0))
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

func stringValueOf(_ *Runtime, receiver Value, _ []Value) (Value, error) {
	if receiver.k == KindString {
		return receiver, nil
	}
	if receiver.k == KindObject && receiver.o.boxed.k == KindString {
		return receiver.o.boxed, nil
	}
	return Undefined(), typeError("String method called on incompatible receiver")
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
