package jsonpath

import (
	"encoding"
	"errors"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalparser "github.com/agentable/go-jsonpath/internal/parser"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{
			name:    "root only",
			expr:    "$",
			wantErr: false,
		},
		{
			name:    "root with name selector",
			expr:    "$['a']",
			wantErr: false,
		},
		{
			name:    "root with index selector",
			expr:    "$[0]",
			wantErr: false,
		},
		{
			name:    "root with wildcard",
			expr:    "$[*]",
			wantErr: false,
		},
		{
			name:    "root with slice",
			expr:    "$[1:3]",
			wantErr: false,
		},
		{
			name:    "dot notation",
			expr:    "$.store.book",
			wantErr: false,
		},
		{
			name:    "descendant",
			expr:    "$..book",
			wantErr: false,
		},
		{
			name:    "complex path",
			expr:    "$.store.book[*].price",
			wantErr: false,
		},
		{
			name:    "invalid - no root",
			expr:    "store",
			wantErr: true,
		},
		{
			name:    "invalid - current node root",
			expr:    "@.store",
			wantErr: true,
		},
		{
			name:    "invalid - empty",
			expr:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path, err := Parse(tt.expr)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrPathParse)
				assert.Equal(t, "", path.String())
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, path.String())
			}
		})
	}
}

func TestMustParse(t *testing.T) {
	t.Parallel()

	t.Run("valid expression", func(t *testing.T) {
		t.Parallel()

		path := MustParse("$.store.book")
		assert.NotEmpty(t, path.String())
	})

	t.Run("invalid expression panics with ErrPathParse", func(t *testing.T) {
		t.Parallel()

		defer func() {
			r := recover()
			require.NotNil(t, r)

			err, ok := r.(error)
			require.True(t, ok)
			require.ErrorIs(t, err, ErrPathParse)
		}()

		MustParse("invalid")
	})
}

func TestPath_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		want string
	}{
		{
			name: "root only",
			expr: "$",
			want: "$",
		},
		{
			name: "name selector",
			expr: "$['store']",
			want: "$[\"store\"]",
		},
		{
			name: "index selector",
			expr: "$[0]",
			want: "$[0]",
		},
		{
			name: "wildcard",
			expr: "$[*]",
			want: "$[*]",
		},
		{
			name: "slice",
			expr: "$[1:3]",
			want: "$[1:3]",
		},
		{
			name: "dot notation",
			expr: "$.store",
			want: "$[\"store\"]",
		},
		{
			name: "descendant",
			expr: "$..book",
			want: "$..[\"book\"]",
		},
		{
			name: "filter comparison",
			expr: "$[?@.price < 10]",
			want: "$[?@[\"price\"] < 10]",
		},
		{
			name: "filter function",
			expr: `$[?match(@.name, "foo")]`,
			want: `$[?match(@["name"], "foo")]`,
		},
		{
			name: "filter logical expression",
			expr: "$[?@.a == 1 && (@.b == true || @.c == null)]",
			want: "$[?@[\"a\"] == 1 && (@[\"b\"] == true || @[\"c\"] == null)]",
		},
		{
			name: "filter non existence",
			expr: "$[?!@.missing]",
			want: "$[?!@[\"missing\"]]",
		},
		{
			name: "filter negated function",
			expr: `$[?!match(@.name, "foo")]`,
			want: `$[?!match(@["name"], "foo")]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse(tt.expr)
			got := path.String()
			assert.Equal(t, tt.want, got)

			reparsed, err := Parse(got)
			require.NoError(t, err)
			assert.NotNil(t, reparsed)
		})
	}
}

func TestPath_String_RoundTripFilterExpressions(t *testing.T) {
	t.Parallel()

	input := []any{
		map[string]any{"name": "foo", "price": 8, "a": 1, "b": true, "c": nil},
		map[string]any{"name": "bar", "price": 12, "a": 1, "b": false, "c": "x"},
	}

	for _, expr := range []string{
		"$[?@.price < 10]",
		`$[?match(@.name, "foo")]`,
		"$[?@.a == 1 && (@.b == true || @.c == null)]",
		"$[?!@.missing]",
		`$[?!match(@.name, "foo")]`,
		"$[?length(@.name) == 3]",
	} {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()

			original := MustParse(expr)
			text, err := original.MarshalText()
			require.NoError(t, err)

			reparsed, err := Parse(string(text))
			require.NoError(t, err)

			originalResult := []any(original.Select(input))
			reparsedResult := []any(reparsed.Select(input))
			if diff := cmp.Diff(originalResult, reparsedResult); diff != "" {
				t.Errorf("round-trip Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_String_ZeroValue(t *testing.T) {
	t.Parallel()

	var path Path
	assert.Equal(t, "", path.String())
}

func TestPath_MarshalText(t *testing.T) {
	t.Parallel()

	path := MustParse("$.store.book")

	// Verify it implements encoding.TextMarshaler
	var _ encoding.TextMarshaler = path

	text, err := path.MarshalText()
	require.NoError(t, err)
	assert.NotEmpty(t, text)

	// Should be able to parse the marshaled text
	reparsed, err := Parse(string(text))
	require.NoError(t, err)
	assert.NotEmpty(t, reparsed.String())
}

func TestPath_MarshalText_ZeroValue(t *testing.T) {
	t.Parallel()

	var path Path
	text, err := path.MarshalText()
	require.ErrorIs(t, err, ErrInvalidPath)
	assert.Nil(t, text)
}

func TestPath_UnmarshalText(t *testing.T) {
	t.Parallel()

	// Verify it implements encoding.TextUnmarshaler
	var path Path
	var _ encoding.TextUnmarshaler = &path

	t.Run("valid expression", func(t *testing.T) {
		t.Parallel()

		var p Path
		err := p.UnmarshalText([]byte("$.store.book"))
		require.NoError(t, err)
		assert.NotNil(t, p.query)
	})

	t.Run("invalid expression", func(t *testing.T) {
		t.Parallel()

		var p Path
		err := p.UnmarshalText([]byte("invalid"))
		require.ErrorIs(t, err, ErrPathParse)
	})
}

func TestPath_MarshalUnmarshal_RoundTrip(t *testing.T) {
	t.Parallel()

	original := MustParse("$.store.book[*].price")

	// Marshal
	text, err := original.MarshalText()
	require.NoError(t, err)

	// Unmarshal
	var restored Path
	err = restored.UnmarshalText(text)
	require.NoError(t, err)

	// Compare by evaluating on same input
	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"price": 10},
				map[string]any{"price": 20},
			},
		},
	}

	originalResult := []any(original.Select(input))
	restoredResult := []any(restored.Select(input))

	if diff := cmp.Diff(originalResult, restoredResult); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_UnmarshalText_InvalidKeepsExistingQuery(t *testing.T) {
	t.Parallel()

	var path Path
	require.NoError(t, path.UnmarshalText([]byte("$.ok")))

	err := path.UnmarshalText([]byte("invalid"))
	require.ErrorIs(t, err, ErrPathParse)

	got := path.Select(map[string]any{"ok": "kept", "invalid": "lost"})
	if diff := cmp.Diff(NodeList{"kept"}, got); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_UnmarshalText_InvalidReceiver(t *testing.T) {
	t.Parallel()

	var path *Path
	err := path.UnmarshalText([]byte("$.a"))
	require.ErrorIs(t, err, ErrInvalidPath)
}

func TestValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		expr  string
		valid bool
	}{
		{name: "valid simple path", expr: "$.store.book", valid: true},
		{name: "valid array index", expr: "$[0]", valid: true},
		{name: "valid wildcard", expr: "$[*]", valid: true},
		{name: "valid slice", expr: "$[0:5:2]", valid: true},
		{name: "valid descendant", expr: "$..book", valid: true},
		{name: "invalid missing root", expr: "store.book"},
		{name: "invalid current node root", expr: "@.store"},
		{name: "invalid syntax", expr: "$["},
		{name: "invalid empty", expr: ""},
		{name: "valid complex path", expr: "$.store.book[*].author", valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.valid, Valid(tt.expr))
		})
	}
}

func TestParse_ReturnsStructuredParseError(t *testing.T) {
	t.Parallel()

	expr := " $.store"
	_, err := Parse(expr)
	require.ErrorIs(t, err, ErrPathParse)

	var parseErr *ParseError
	require.True(t, errors.As(err, &parseErr))
	assert.Equal(t, 0, parseErr.Offset)
	assert.Equal(t, "leading whitespace not allowed", parseErr.Reason)
	assert.Contains(t, parseErr.Snippet, expr)
}

func TestParse_RejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	invalid := string([]byte{0xff})
	for _, tc := range []struct {
		name       string
		expr       string
		wantOffset int
	}{
		{name: "inside quoted selector", expr: "$['valid" + invalid + "name']", wantOffset: 8},
		{name: "after escape", expr: "$['\\" + invalid + "']", wantOffset: 4},
		{name: "outside string", expr: "$." + invalid, wantOffset: 2},
		{name: "inside shorthand name", expr: "$.valid" + invalid + "name", wantOffset: 7},
		{name: "byte offset after multibyte rune", expr: "$['\u00e9" + invalid + "']", wantOffset: 5},
		{name: "after number minus", expr: "$[-" + invalid + "]", wantOffset: 3},
		{name: "after decimal point", expr: "$[1." + invalid + "]", wantOffset: 4},
		{name: "after exponent", expr: "$[1e" + invalid + "]", wantOffset: 4},
		{name: "after exponent sign", expr: "$[1e+" + invalid + "]", wantOffset: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tc.expr)
			require.ErrorIs(t, err, ErrPathParse)

			var parseErr *ParseError
			require.ErrorAs(t, err, &parseErr)
			assert.Equal(t, tc.wantOffset, parseErr.Offset)
			assert.NotEmpty(t, parseErr.Reason)
		})
	}

	for _, tc := range []struct {
		name string
		expr string
	}{
		{name: "literal replacement rune", expr: "$['\uFFFD']"},
		{name: "escaped replacement rune", expr: `$['\uFFFD']`},
		{name: "shorthand replacement rune", expr: "$.\uFFFD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, err := Parse(tc.expr)
			require.NoError(t, err)

			got := []any(path.Select(map[string]any{"\uFFFD": "replacement"}))
			if diff := cmp.Diff([]any{"replacement"}, got); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParse_NumericConversionErrorsArePositioned(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expr       string
		wantOffset int
		wantReason string
	}{
		{
			name:       "index integer overflow",
			expr:       "$[9223372036854775808]",
			wantOffset: 2,
			wantReason: "invalid integer",
		},
		{
			name:       "slice end integer overflow",
			expr:       "$[0:9223372036854775808]",
			wantOffset: 4,
			wantReason: "invalid integer",
		},
		{
			name:       "slice step integer overflow",
			expr:       "$[0:1:9223372036854775808]",
			wantOffset: 6,
			wantReason: "invalid integer",
		},
		{
			name:       "filter literal number overflow",
			expr:       "$[?1e999 == 1]",
			wantOffset: 3,
			wantReason: "invalid number",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tc.expr)
			require.ErrorIs(t, err, ErrPathParse)

			var parseErr *ParseError
			require.True(t, errors.As(err, &parseErr))
			assert.Equal(t, tc.wantOffset, parseErr.Offset)
			assert.Contains(t, parseErr.Reason, tc.wantReason)
			assert.NotEmpty(t, parseErr.Snippet)
			var numErr *strconv.NumError
			require.True(t, errors.As(parseErr.Cause, &numErr))
		})
	}
}

func TestParse_PublicErrorOriginMatrix(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		expr         string
		wantFunction bool
	}{
		{
			name: "lexer invalid escape",
			expr: `$['\q']`,
		},
		{
			name: "parser trailing whitespace",
			expr: "$ ",
		},
		{
			name:         "unknown function",
			expr:         "$[?missing(@)]",
			wantFunction: true,
		},
		{
			name:         "invalid function arguments",
			expr:         "$[?length()]",
			wantFunction: true,
		},
		{
			name: "public current-node root",
			expr: "@.a",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tc.expr)
			require.ErrorIs(t, err, ErrPathParse)
			if tc.wantFunction {
				require.ErrorIs(t, err, ErrFunction)
			} else {
				require.NotErrorIs(t, err, ErrFunction)
			}

			var parseErr *ParseError
			require.True(t, errors.As(err, &parseErr))
			assert.GreaterOrEqual(t, parseErr.Offset, 0)
			assert.NotEmpty(t, parseErr.Reason)
			assert.NotEmpty(t, parseErr.Snippet)
			assert.NotNil(t, parseErr.Cause)
		})
	}
}

func TestParse_CurrentNodeRootErrorComesFromInternalParser(t *testing.T) {
	t.Parallel()

	_, err := Parse("@.a")
	require.ErrorIs(t, err, ErrPathParse)
	require.ErrorIs(t, err, internalparser.ErrParsePosition)

	var parseErr *ParseError
	require.ErrorAs(t, err, &parseErr)
	assert.Equal(t, 0, parseErr.Offset)
	assert.Equal(t, "expected $", parseErr.Reason)
	assert.Equal(t, "'@.a'", parseErr.Snippet)
}

func TestParse_CTSDerivedInvalidSyntaxExamples(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{
		"$[,0]",
		"$[0,]",
		"$[@.a]",
		"$[$.a]",
		"$. a",
		"$.. a",
		"$[?@[*] == 0]",
		"$[?true]",
	} {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(expr)
			require.ErrorIs(t, err, ErrPathParse)
			require.NotErrorIs(t, err, ErrFunction)

			var parseErr *ParseError
			require.True(t, errors.As(err, &parseErr))
			assert.NotEmpty(t, parseErr.Reason)
		})
	}
}

func TestParse_NonSingularComparisonReportsOffendingSelector(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expr       string
		wantOffset int
	}{
		{
			name:       "wildcard",
			expr:       "$[?@[*] == 0]",
			wantOffset: 5,
		},
		{
			name:       "slice",
			expr:       "$[?@[:] == 0]",
			wantOffset: 5,
		},
		{
			name:       "descendant",
			expr:       "$[?@..a == 0]",
			wantOffset: 4,
		},
		{
			name:       "multiple selectors",
			expr:       "$[?@[0,1] == 0]",
			wantOffset: 6,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tc.expr)
			require.ErrorIs(t, err, ErrPathParse)
			require.NotErrorIs(t, err, ErrFunction)

			var parseErr *ParseError
			require.True(t, errors.As(err, &parseErr))
			assert.Equal(t, tc.wantOffset, parseErr.Offset)
			assert.Equal(t, "non-singular query is not allowed in comparison", parseErr.Reason)
		})
	}
}

func TestParse_NonSingularQueriesRemainLegalOutsideComparison(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{
		"$[?@[*]]",
		"$[?@[:]]",
		"$[?@..a]",
		"$[?count(@[*]) > 0]",
	} {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(expr)
			require.NoError(t, err)
		})
	}
}

func TestParse_FunctionErrorsAreClassified(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expr       string
		wantReason string
	}{
		{
			name:       "unknown function",
			expr:       "$[?missing(@)]",
			wantReason: "unknown function missing",
		},
		{
			name:       "invalid function arguments",
			expr:       "$[?length()]",
			wantReason: "length: expected 1, got 0: wrong number of arguments",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tc.expr)
			require.ErrorIs(t, err, ErrPathParse)
			require.ErrorIs(t, err, ErrFunction)

			var parseErr *ParseError
			require.True(t, errors.As(err, &parseErr))
			assert.Equal(t, tc.wantReason, parseErr.Reason)
		})
	}
}

func TestParse_Integration(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"title": "Book 1", "price": 10},
				map[string]any{"title": "Book 2", "price": 20},
			},
		},
	}

	tests := []struct {
		name string
		expr string
		want []any
	}{
		{
			name: "select store",
			expr: "$.store",
			want: []any{map[string]any{
				"book": []any{
					map[string]any{"title": "Book 1", "price": 10},
					map[string]any{"title": "Book 2", "price": 20},
				},
			}},
		},
		{
			name: "select first book",
			expr: "$.store.book[0]",
			want: []any{map[string]any{"title": "Book 1", "price": 10}},
		},
		{
			name: "select all prices",
			expr: "$.store.book[*].price",
			want: []any{10, 20},
		},
		{
			name: "descendant price",
			expr: "$..price",
			want: []any{10, 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path, err := Parse(tt.expr)
			require.NoError(t, err)

			got := []any(path.Select(input))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
