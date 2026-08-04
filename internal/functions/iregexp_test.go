package functions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckIRegexpGrammar(t *testing.T) {
	t.Parallel()

	valid := []string{
		"",
		"|",
		"a|",
		"|a",
		"世界",
		"(a|b)",
		"a*b+c?",
		"a{1}b{2,}c{3,4}",
		".",
		`\(\)\*\+\-\.\?\[\\\]\^\n\r\t\{\|\}`,
	}
	for _, pattern := range valid {
		t.Run("valid/"+pattern, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, checkIRegexp(pattern))
		})
	}

	invalid := []string{
		string([]byte{0xff}),
		"(?=a)",
		`\1`,
		"*a",
		"a**",
		"(",
		")",
		"{1}a",
		"a{}",
		"a{1",
	}
	for _, pattern := range invalid {
		t.Run("invalid", func(t *testing.T) {
			t.Parallel()
			assert.Error(t, checkIRegexp(pattern), "pattern bytes %x", []byte(pattern))
		})
	}
}

func TestCheckIRegexpUnicodeProperties(t *testing.T) {
	t.Parallel()

	properties := []string{
		"L", "Ll", "Lm", "Lo", "Lt", "Lu",
		"M", "Mc", "Me", "Mn",
		"N", "Nd", "Nl", "No",
		"P", "Pc", "Pd", "Pe", "Pf", "Pi", "Po", "Ps",
		"Z", "Zl", "Zp", "Zs",
		"S", "Sc", "Sk", "Sm", "So",
		"C", "Cc", "Cf", "Cn", "Co",
	}
	for _, property := range properties {
		t.Run(property, func(t *testing.T) {
			t.Parallel()
			for _, pattern := range []string{
				`\p{` + property + `}`,
				`\P{` + property + `}`,
				`[\p{` + property + `}]`,
				`[\P{` + property + `}]`,
			} {
				assert.NoError(t, checkIRegexp(pattern))
				require.NotNil(t, compileIRegexp(pattern))
			}
		})
	}
}
