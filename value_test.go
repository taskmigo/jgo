package gots

import (
	"math"
	"testing"
)

func TestSameValue(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	if !math.Signbit(negativeZero) {
		t.Fatal("negative-zero fixture is invalid")
	}
	object, otherObject := NewObject(), NewObject()
	function := nativeValue(func(*Runtime, Value, []Value) (Value, error) { return Undefined(), nil })
	otherFunction := nativeValue(func(*Runtime, Value, []Value) (Value, error) { return Undefined(), nil })
	symbol := Value{k: KindSymbol, sy: &symbolValue{identity: newIdentity()}}
	otherSymbol := Value{k: KindSymbol, sy: &symbolValue{identity: newIdentity()}}

	tests := []struct {
		name        string
		left, right Value
		want        bool
	}{
		{"undefined", Undefined(), Undefined(), true},
		{"undefined and null", Undefined(), Null(), false},
		{"null", Null(), Null(), true},
		{"true", Boolean(true), Boolean(true), true},
		{"booleans differ", Boolean(true), Boolean(false), false},
		{"same string", String("a"), String("a"), true},
		{"strings differ", String("a"), String("b"), false},
		{"same number", Number(1), Number(1), true},
		{"numbers differ", Number(1), Number(2), false},
		{"NaN", Number(math.NaN()), Number(math.NaN()), true},
		{"signed zero", Number(0), Number(negativeZero), false},
		{"positive zero", Number(0), Number(0), true},
		{"negative zero", Number(negativeZero), Number(negativeZero), true},
		{"infinity", Number(math.Inf(1)), Number(math.Inf(1)), true},
		{"opposite infinities", Number(math.Inf(1)), Number(math.Inf(-1)), false},
		{"same object", object, object, true},
		{"distinct objects", object, otherObject, false},
		{"same function", function, function, true},
		{"distinct functions", function, otherFunction, false},
		{"same symbol", symbol, symbol, true},
		{"distinct symbols", symbol, otherSymbol, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SameValue(test.left, test.right); got != test.want {
				t.Fatalf("SameValue(%v, %v) = %t, want %t", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestEqualityOperationsRemainDistinct(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	if strictlyEqual(Number(math.NaN()), Number(math.NaN())) {
		t.Fatal("strict equality considered NaN equal to itself")
	}
	if !strictlyEqual(Number(0), Number(negativeZero)) {
		t.Fatal("strict equality distinguished signed zero")
	}
	if !sameValueZero(Number(math.NaN()), Number(math.NaN())) || !sameValueZero(Number(0), Number(negativeZero)) {
		t.Fatal("SameValueZero numeric semantics are incorrect")
	}
}
