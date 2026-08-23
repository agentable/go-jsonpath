package ast

import (
	"cmp"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// isNumeric returns true if v is a numeric type.
func isNumeric(v any) bool {
	_, ok := parseJSONNumber(v)
	return ok
}

func isNumberType(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, jsonv1.Number:
		return true
	default:
		return false
	}
}

func compareJSONNumbers(a, b any) (int, bool) {
	left, ok := parseJSONNumber(a)
	if !ok {
		return 0, false
	}
	right, ok := parseJSONNumber(b)
	if !ok {
		return 0, false
	}

	switch {
	case left.kind == numberSigned && right.kind == numberSigned:
		return cmp.Compare(left.signed, right.signed), true
	case left.kind == numberUnsigned && right.kind == numberUnsigned:
		return cmp.Compare(left.unsigned, right.unsigned), true
	case left.kind == numberFloat && right.kind == numberFloat:
		return cmp.Compare(left.float, right.float), true
	case left.kind == numberSigned && right.kind == numberUnsigned:
		if left.signed < 0 {
			return -1, true
		}
		return cmp.Compare(uint64(left.signed), right.unsigned), true
	case left.kind == numberUnsigned && right.kind == numberSigned:
		if right.signed < 0 {
			return 1, true
		}
		return cmp.Compare(left.unsigned, uint64(right.signed)), true
	case left.kind != numberFloat && right.kind != numberFloat:
		return compareDecimalNumbers(left.decimalValue(), right.decimalValue()), true
	case left.kind == numberFloat:
		return -compareDecimalToFloat(right.decimalValue(), left.float), true
	default:
		return compareDecimalToFloat(left.decimalValue(), right.float), true
	}
}

type numberKind uint8

const (
	numberSigned numberKind = iota
	numberUnsigned
	numberFloat
	numberDecimal
)

type numberValue struct {
	kind     numberKind
	signed   int64
	unsigned uint64
	float    float64
	decimal  decimalNumber
}

type decimalNumber struct {
	negative bool
	digits   string
	exponent *big.Int
}

func parseJSONNumber(v any) (numberValue, bool) {
	switch n := v.(type) {
	case int:
		return numberValue{kind: numberSigned, signed: int64(n)}, true
	case int8:
		return numberValue{kind: numberSigned, signed: int64(n)}, true
	case int16:
		return numberValue{kind: numberSigned, signed: int64(n)}, true
	case int32:
		return numberValue{kind: numberSigned, signed: int64(n)}, true
	case int64:
		return numberValue{kind: numberSigned, signed: n}, true
	case uint:
		return numberValue{kind: numberUnsigned, unsigned: uint64(n)}, true
	case uint8:
		return numberValue{kind: numberUnsigned, unsigned: uint64(n)}, true
	case uint16:
		return numberValue{kind: numberUnsigned, unsigned: uint64(n)}, true
	case uint32:
		return numberValue{kind: numberUnsigned, unsigned: uint64(n)}, true
	case uint64:
		return numberValue{kind: numberUnsigned, unsigned: n}, true
	case float32:
		f := float64(n)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return numberValue{}, false
		}
		return numberValue{kind: numberFloat, float: f}, true
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return numberValue{}, false
		}
		return numberValue{kind: numberFloat, float: n}, true
	case jsonv1.Number:
		decimal, ok := parseDecimalNumber(string(n))
		if !ok {
			return numberValue{}, false
		}
		return numberValue{kind: numberDecimal, decimal: decimal}, true
	default:
		return numberValue{}, false
	}
}

func parseDecimalNumber(s string) (decimalNumber, bool) {
	if s == "" || strings.TrimSpace(s) != s || !jsontext.Value(s).IsValid() {
		return decimalNumber{}, false
	}
	if s[0] != '-' && (s[0] < '0' || s[0] > '9') {
		return decimalNumber{}, false
	}

	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}

	exponent := new(big.Int)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		if _, ok := exponent.SetString(s[i+1:], 10); !ok {
			return decimalNumber{}, false
		}
		s = s[:i]
	}

	fractionDigits := 0
	if i := strings.IndexByte(s, '.'); i >= 0 {
		fractionDigits = len(s) - i - 1
		s = s[:i] + s[i+1:]
	}
	exponent.Sub(exponent, big.NewInt(int64(fractionDigits)))
	return normalizeDecimal(negative, s, exponent), true
}

func normalizeDecimal(negative bool, digits string, exponent *big.Int) decimalNumber {
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return decimalNumber{exponent: new(big.Int)}
	}

	end := len(digits)
	for end > 0 && digits[end-1] == '0' {
		end--
	}
	if trailing := len(digits) - end; trailing > 0 {
		exponent = new(big.Int).Add(exponent, big.NewInt(int64(trailing)))
		digits = digits[:end]
	}
	return decimalNumber{negative: negative, digits: digits, exponent: exponent}
}

func (n numberValue) decimalValue() decimalNumber {
	switch n.kind {
	case numberSigned:
		digits := strconv.FormatInt(n.signed, 10)
		negative := n.signed < 0
		if negative {
			digits = digits[1:]
		}
		return normalizeDecimal(negative, digits, new(big.Int))
	case numberUnsigned:
		return normalizeDecimal(false, strconv.FormatUint(n.unsigned, 10), new(big.Int))
	case numberDecimal:
		return n.decimal
	default:
		return decimalNumber{exponent: new(big.Int)}
	}
}

func compareDecimalNumbers(a, b decimalNumber) int {
	if a.digits == "" || b.digits == "" {
		switch {
		case a.digits == "" && b.digits == "":
			return 0
		case a.digits == "":
			if b.negative {
				return 1
			}
			return -1
		default:
			if a.negative {
				return -1
			}
			return 1
		}
	}
	if a.negative != b.negative {
		if a.negative {
			return -1
		}
		return 1
	}

	order := compareDecimalMagnitudes(a, b)
	if a.negative {
		return -order
	}
	return order
}

func compareDecimalMagnitudes(a, b decimalNumber) int {
	aMagnitude := new(big.Int).Add(a.exponent, big.NewInt(int64(len(a.digits))))
	bMagnitude := new(big.Int).Add(b.exponent, big.NewInt(int64(len(b.digits))))
	if order := aMagnitude.Cmp(bMagnitude); order != 0 {
		return order
	}

	for i := range max(len(a.digits), len(b.digits)) {
		aDigit, bDigit := byte('0'), byte('0')
		if i < len(a.digits) {
			aDigit = a.digits[i]
		}
		if i < len(b.digits) {
			bDigit = b.digits[i]
		}
		if order := cmp.Compare(aDigit, bDigit); order != 0 {
			return order
		}
	}
	return 0
}

func compareDecimalToFloat(decimal decimalNumber, f float64) int {
	if decimal.digits == "" {
		switch {
		case f < 0:
			return 1
		case f > 0:
			return -1
		default:
			return 0
		}
	}
	if f == 0 {
		if decimal.negative {
			return -1
		}
		return 1
	}

	floatNegative := math.Signbit(f)
	if decimal.negative != floatNegative {
		if decimal.negative {
			return -1
		}
		return 1
	}

	decimalMagnitude := new(big.Int).Add(decimal.exponent, big.NewInt(int64(len(decimal.digits))))
	floatMagnitude := big.NewInt(int64(floatDecimalMagnitude(math.Abs(f))))
	order := decimalMagnitude.Cmp(floatMagnitude)
	if order == 0 {
		order = decimal.rat().Cmp(new(big.Rat).SetFloat64(math.Abs(f)))
	}
	if decimal.negative {
		return -order
	}
	return order
}

func floatDecimalMagnitude(f float64) int {
	formatted := strconv.FormatFloat(f, 'e', -1, 64)
	i := strings.IndexByte(formatted, 'e')
	exponent, _ := strconv.Atoi(formatted[i+1:])
	return exponent + 1
}

func (d decimalNumber) rat() *big.Rat {
	coefficient, _ := new(big.Int).SetString(d.digits, 10)
	power := new(big.Int).Exp(big.NewInt(10), new(big.Int).Abs(d.exponent), nil)
	if d.exponent.Sign() >= 0 {
		return new(big.Rat).SetInt(coefficient.Mul(coefficient, power))
	}
	return new(big.Rat).SetFrac(coefficient, power)
}
