package ast

import (
	"encoding/json/jsontext"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNumeric(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		v    any
		want bool
	}{
		{name: "int", v: int(1), want: true},
		{name: "int8", v: int8(1), want: true},
		{name: "int16", v: int16(1), want: true},
		{name: "int32", v: int32(1), want: true},
		{name: "int64", v: int64(1), want: true},
		{name: "uint", v: uint(1), want: true},
		{name: "uint8", v: uint8(1), want: true},
		{name: "uint16", v: uint16(1), want: true},
		{name: "uint32", v: uint32(1), want: true},
		{name: "uint64", v: uint64(1), want: true},
		{name: "float32", v: float32(1), want: true},
		{name: "float64", v: float64(1), want: true},
		{name: "jsontext_value", v: jsontext.Value("1.5"), want: true},
		{name: "invalid_jsontext_value", v: jsontext.Value("invalid"), want: false},
		{name: "nan", v: math.NaN(), want: false},
		{name: "infinity", v: math.Inf(1), want: false},
		{name: "string", v: "1", want: false},
		{name: "bool", v: true, want: false},
		{name: "nil", v: nil, want: false},
		{name: "slice", v: []any{1}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isNumeric(tc.v))
		})
	}
}

func TestCompareJSONNumbers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		a, b   any
		want   int
		wantOK bool
	}{
		{name: "signed_equal", a: int8(1), b: int64(1), wantOK: true},
		{name: "adjacent_large_signed", a: int64(9007199254740992), b: int64(9007199254740993), want: -1, wantOK: true},
		{name: "negative_signed_before_unsigned", a: int64(-1), b: uint64(0), want: -1, wantOK: true},
		{name: "max_signed_before_max_unsigned", a: int64(math.MaxInt64), b: uint64(math.MaxUint64), want: -1, wantOK: true},
		{name: "max_unsigned_before_two_to_64_float", a: uint64(math.MaxUint64), b: math.Exp2(64), want: -1, wantOK: true},
		{name: "large_signed_after_rounded_float", a: int64(9007199254740993), b: float64(9007199254740992), want: 1, wantOK: true},
		{name: "exact_integer_float", a: uint64(9007199254740992), b: float64(9007199254740992), wantOK: true},
		{name: "negative_zero", a: math.Copysign(0, -1), b: 0.0, wantOK: true},
		{name: "decimal_exponent", a: jsontext.Value("1e3"), b: int64(1000), wantOK: true},
		{name: "equivalent_decimals", a: jsontext.Value("0.1"), b: jsontext.Value("0.10"), wantOK: true},
		{name: "equivalent_large_exponents", a: jsontext.Value("1e1000001"), b: jsontext.Value("10e1000000"), wantOK: true},
		{name: "equivalent_unbounded_exponents", a: jsontext.Value("1e100000000000000000000"), b: jsontext.Value("10e99999999999999999999"), wantOK: true},
		{name: "ordered_large_exponents", a: jsontext.Value("9e1000000"), b: jsontext.Value("1e1000001"), want: -1, wantOK: true},
		{name: "ordered_unbounded_exponents", a: jsontext.Value("9e99999999999999999999"), b: jsontext.Value("1e100000000000000000000"), want: -1, wantOK: true},
		{name: "large_exponent_after_float", a: jsontext.Value("1e1000001"), b: math.MaxFloat64, want: 1, wantOK: true},
		{name: "small_exponent_before_float", a: jsontext.Value("1e-1000001"), b: math.SmallestNonzeroFloat64, want: -1, wantOK: true},
		{name: "negative_large_exponent", a: jsontext.Value("-1e1000001"), b: jsontext.Value("-9e1000000"), want: -1, wantOK: true},
		{name: "unbounded_exponent_zero", a: jsontext.Value("-0e100000000000000000000"), b: jsontext.Value("0"), wantOK: true},
		{name: "decimal_before_binary_float", a: jsontext.Value("0.1"), b: float64(0.1), want: -1, wantOK: true},
		{name: "invalid_decimal", a: jsontext.Value("invalid"), b: 0, wantOK: false},
		{name: "nan", a: math.NaN(), b: 0, wantOK: false},
		{name: "positive_infinity", a: math.Inf(1), b: 0, wantOK: false},
		{name: "non_numeric", a: "1", b: 1, wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := compareJSONNumbers(tc.a, tc.b)
			assert.Equal(t, tc.wantOK, ok)
			if !ok {
				return
			}
			assert.Equal(t, tc.want, got)

			reverse, reverseOK := compareJSONNumbers(tc.b, tc.a)
			assert.True(t, reverseOK)
			assert.Equal(t, -tc.want, reverse)
		})
	}
}

func TestCompareJSONNumbersEquivalentRepresentations(t *testing.T) {
	t.Parallel()

	values := []any{
		int(1), int8(1), int16(1), int32(1), int64(1),
		uint(1), uint8(1), uint16(1), uint32(1), uint64(1),
		float32(1), float64(1), jsontext.Value("1.0"),
	}
	for _, left := range values {
		for _, right := range values {
			order, ok := compareJSONNumbers(left, right)
			assert.True(t, ok, "compareJSONNumbers(%T, %T)", left, right)
			assert.Zero(t, order, "compareJSONNumbers(%T, %T)", left, right)
		}
	}
}
