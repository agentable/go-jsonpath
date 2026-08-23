package jsonpath

import (
	jsonv1 "encoding/json"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentable/go-jsonpath/internal/ast"
)

func TestPath_ZeroValueSelectsNothing(t *testing.T) {
	t.Parallel()

	var path Path

	assert.Nil(t, path.Select(map[string]any{"a": 1}))
	assert.Nil(t, path.SelectLocated(map[string]any{"a": 1}))
}

func TestPath_Select_NameSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		selector ast.Selector
		want     []any
	}{
		{
			name:     "select existing key",
			input:    map[string]any{"a": 1, "b": 2},
			selector: ast.NameSelector("a"),
			want:     []any{1},
		},
		{
			name:     "select missing key",
			input:    map[string]any{"a": 1},
			selector: ast.NameSelector("b"),
			want:     []any{},
		},
		{
			name:     "select from non-object",
			input:    []any{1, 2, 3},
			selector: ast.NameSelector("a"),
			want:     []any{},
		},
		{
			name:     "select nested object",
			input:    map[string]any{"a": map[string]any{"b": 42}},
			selector: ast.NameSelector("a"),
			want:     []any{map[string]any{"b": 42}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seg := ast.Child(tt.selector)
			query := ast.NewPathQuery(true, seg)
			path := &Path{query: query}
			got := path.Select(tt.input)
			if diff := cmp.Diff(tt.want, []any(got)); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_Select_IndexSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		selector ast.Selector
		want     []any
	}{
		{
			name:     "select positive index",
			input:    []any{10, 20, 30},
			selector: ast.IndexSelector(1),
			want:     []any{20},
		},
		{
			name:     "select negative index",
			input:    []any{10, 20, 30},
			selector: ast.IndexSelector(-1),
			want:     []any{30},
		},
		{
			name:     "select negative index -2",
			input:    []any{10, 20, 30},
			selector: ast.IndexSelector(-2),
			want:     []any{20},
		},
		{
			name:     "select out of bounds positive",
			input:    []any{10, 20},
			selector: ast.IndexSelector(5),
			want:     []any{},
		},
		{
			name:     "select out of bounds negative",
			input:    []any{10, 20},
			selector: ast.IndexSelector(-5),
			want:     []any{},
		},
		{
			name:     "select from non-array",
			input:    map[string]any{"a": 1},
			selector: ast.IndexSelector(0),
			want:     []any{},
		},
		{
			name:     "select from empty array",
			input:    []any{},
			selector: ast.IndexSelector(0),
			want:     []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seg := ast.Child(tt.selector)
			query := ast.NewPathQuery(true, seg)
			path := &Path{query: query}
			got := path.Select(tt.input)
			if diff := cmp.Diff(tt.want, []any(got)); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_Select_SliceSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		slice ast.SliceArgs
		want  []any
	}{
		{
			name:  "slice with start and end",
			input: []any{0, 1, 2, 3, 4},
			slice: ast.SliceArgs{Start: 1, End: 3, HasStart: true, HasEnd: true},
			want:  []any{1, 2},
		},
		{
			name:  "slice with only start",
			input: []any{0, 1, 2, 3, 4},
			slice: ast.SliceArgs{Start: 2, HasStart: true},
			want:  []any{2, 3, 4},
		},
		{
			name:  "slice with only end",
			input: []any{0, 1, 2, 3, 4},
			slice: ast.SliceArgs{End: 3, HasEnd: true},
			want:  []any{0, 1, 2},
		},
		{
			name:  "slice with step",
			input: []any{0, 1, 2, 3, 4, 5},
			slice: ast.SliceArgs{Start: 0, End: 6, Step: 2, HasStart: true, HasEnd: true, HasStep: true},
			want:  []any{0, 2, 4},
		},
		{
			name:  "slice with negative start",
			input: []any{0, 1, 2, 3, 4},
			slice: ast.SliceArgs{Start: -2, HasStart: true},
			want:  []any{3, 4},
		},
		{
			name:  "slice with negative end",
			input: []any{0, 1, 2, 3, 4},
			slice: ast.SliceArgs{End: -1, HasEnd: true},
			want:  []any{0, 1, 2, 3},
		},
		{
			name:  "slice with negative step",
			input: []any{0, 1, 2, 3, 4},
			slice: ast.SliceArgs{Start: 4, End: 0, Step: -1, HasStart: true, HasEnd: true, HasStep: true},
			want:  []any{4, 3, 2, 1},
		},
		{
			name:  "slice with out-of-range negative step bounds",
			input: []any{0, 1, 2},
			slice: ast.SliceArgs{Start: 100, End: -100, Step: -1, HasStart: true, HasEnd: true, HasStep: true},
			want:  []any{2, 1, 0},
		},
		{
			name:  "slice with step 0 returns empty",
			input: []any{0, 1, 2, 3, 4},
			slice: ast.SliceArgs{Step: 0, HasStep: true},
			want:  []any{},
		},
		{
			name:  "slice from empty array",
			input: []any{},
			slice: ast.SliceArgs{Start: 0, End: 5, HasStart: true, HasEnd: true},
			want:  []any{},
		},
		{
			name:  "slice from non-array",
			input: map[string]any{"a": 1},
			slice: ast.SliceArgs{Start: 0, End: 5, HasStart: true, HasEnd: true},
			want:  []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seg := ast.Child(ast.SliceSelector(tt.slice))
			query := ast.NewPathQuery(true, seg)
			path := &Path{query: query}
			got := path.Select(tt.input)
			if diff := cmp.Diff(tt.want, []any(got)); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_Select_WildcardSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  []any
	}{
		{
			name:  "wildcard on object",
			input: map[string]any{"z": "z", "a": "a", "m": "m", "b": "b", "y": "y", "c": "c", "x": "x"},
			want:  []any{"a", "b", "c", "m", "x", "y", "z"},
		},
		{
			name:  "wildcard on array",
			input: []any{10, 20, 30},
			want:  []any{10, 20, 30},
		},
		{
			name:  "wildcard on empty object",
			input: map[string]any{},
			want:  []any{},
		},
		{
			name:  "wildcard on empty array",
			input: []any{},
			want:  []any{},
		},
		{
			name:  "wildcard on primitive",
			input: 42,
			want:  []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seg := ast.Child(ast.WildcardSelector())
			query := ast.NewPathQuery(true, seg)
			path := &Path{query: query}
			got := path.Select(tt.input)
			if diff := cmp.Diff(tt.want, []any(got)); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_Select_MultipleSelectors(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	seg := ast.Child(ast.NameSelector("a"), ast.NameSelector("c"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}
	got := path.Select(input)

	if diff := cmp.Diff([]any{1, 3}, []any(got)); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_Select_MultipleSegments(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"title": "Book 1", "price": 10},
				map[string]any{"title": "Book 2", "price": 20},
			},
		},
	}

	seg1 := ast.Child(ast.NameSelector("store"))
	seg2 := ast.Child(ast.NameSelector("book"))
	seg3 := ast.Child(ast.IndexSelector(0))
	seg4 := ast.Child(ast.NameSelector("title"))

	query := ast.NewPathQuery(true, seg1, seg2, seg3, seg4)
	path := &Path{query: query}
	got := path.Select(input)

	if diff := cmp.Diff([]any{"Book 1"}, []any(got)); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_Select_DescendantSelector(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"z": map[string]any{"value": "z"},
		"b": map[string]any{
			"value": "b",
			"c": map[string]any{
				"value": "b.c",
			},
		},
		"a": []any{
			map[string]any{"value": "a.0"},
			map[string]any{"b": 5},
		},
	}

	seg := ast.Descendant(ast.NameSelector("value"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}
	got := path.Select(input)

	if diff := cmp.Diff([]any{"a.0", "b", "b.c", "z"}, []any(got)); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_Select_DescendantArrayOrder(t *testing.T) {
	t.Parallel()

	input := []any{
		map[string]any{"a": 1},
		[]any{
			map[string]any{"a": 2},
			map[string]any{"a": 3},
		},
		map[string]any{"a": 4},
	}

	path := MustParse("$..a")
	got := path.Select(input)

	if diff := cmp.Diff([]any{1, 2, 3, 4}, []any(got)); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_Select_DescendantWildcard(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"z": []any{"z.0"},
		"a": map[string]any{
			"c": "a.c",
			"b": "a.b",
		},
		"m": "m",
	}

	seg := ast.Descendant(ast.WildcardSelector())
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}
	got := path.Select(input)

	want := []any{
		map[string]any{"c": "a.c", "b": "a.b"},
		"m",
		[]any{"z.0"},
		"a.b",
		"a.c",
		"z.0",
	}
	if diff := cmp.Diff(want, []any(got)); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_Select_NilQuery(t *testing.T) {
	t.Parallel()

	path := &Path{query: nil}
	got := path.Select(map[string]any{"a": 1})
	assert.Nil(t, got)
}

func TestPath_Select_ComplexPath(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95},
				map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": 12.99},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99},
			},
		},
	}

	// $['store']['book'][*]['price']
	seg1 := ast.Child(ast.NameSelector("store"))
	seg2 := ast.Child(ast.NameSelector("book"))
	seg3 := ast.Child(ast.WildcardSelector())
	seg4 := ast.Child(ast.NameSelector("price"))

	query := ast.NewPathQuery(true, seg1, seg2, seg3, seg4)
	path := &Path{query: query}
	got := path.Select(input)

	if diff := cmp.Diff([]any{8.95, 12.99, 8.99}, []any(got)); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func BenchmarkSelect_NameSelector(b *testing.B) {
	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	seg := ast.Child(ast.NameSelector("b"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_IndexSelector(b *testing.B) {
	input := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	seg := ast.Child(ast.IndexSelector(5))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_SliceSelector(b *testing.B) {
	input := make([]any, 100)
	for i := range input {
		input[i] = i
	}
	seg := ast.Child(ast.SliceSelector(ast.SliceArgs{
		Start: 10, End: 50, Step: 2,
		HasStart: true, HasEnd: true, HasStep: true,
	}))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_WildcardSelector(b *testing.B) {
	input := map[string]any{
		"a": 1, "b": 2, "c": 3, "d": 4, "e": 5,
		"f": 6, "g": 7, "h": 8, "i": 9, "j": 10,
	}
	seg := ast.Child(ast.WildcardSelector())
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_DescendantSelector(b *testing.B) {
	input := map[string]any{
		"a": 1,
		"b": map[string]any{
			"a": 2,
			"c": map[string]any{
				"a": 3,
				"d": map[string]any{
					"a": 4,
				},
			},
		},
	}
	seg := ast.Descendant(ast.NameSelector("a"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_ComplexPath(b *testing.B) {
	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"title": "Book 1", "price": 10},
				map[string]any{"title": "Book 2", "price": 20},
				map[string]any{"title": "Book 3", "price": 30},
				map[string]any{"title": "Book 4", "price": 40},
				map[string]any{"title": "Book 5", "price": 50},
			},
		},
	}

	seg1 := ast.Child(ast.NameSelector("store"))
	seg2 := ast.Child(ast.NameSelector("book"))
	seg3 := ast.Child(ast.WildcardSelector())
	seg4 := ast.Child(ast.NameSelector("price"))

	query := ast.NewPathQuery(true, seg1, seg2, seg3, seg4)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func TestPath_Select_FilterSelector(t *testing.T) {
	t.Parallel()

	// An empty filter has no true branch and selects nothing.
	input := []any{
		map[string]any{"price": 10},
		map[string]any{"price": 20},
	}

	seg := ast.Child(ast.FilterSelector(&ast.FilterExpr{}))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}
	got := path.Select(input)

	require.Empty(t, got, "filter selectors should select nothing until implemented")
}

func TestPath_Select_FilterQueries(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"expensive": 15,
		"numbers":   []any{jsonv1.Number("1e1000001")},
		"items": []any{
			map[string]any{"name": "paper", "price": 5, "tags": []any{"office"}, "code": "a", "meta": map[string]any{"target": "paper"}},
			map[string]any{"name": "pencil", "price": 2, "tags": []any{"office", "writing"}, "code": "b"},
			map[string]any{"name": "stapler", "price": 20, "tags": []any{"office", "metal"}, "code": "ab", "details": []any{map[string]any{"target": "stapler"}}},
			map[string]any{"name": "desk", "price": 50, "tags": []any{"furniture"}, "code": "x"},
		},
	}

	for _, tc := range []struct {
		name string
		path string
		want []any
	}{
		{
			name: "relative comparison",
			path: "$.items[?@.price < 10].name",
			want: []any{"paper", "pencil"},
		},
		{
			name: "root comparison",
			path: "$.items[?@.price < $.expensive].name",
			want: []any{"paper", "pencil"},
		},
		{
			name: "function comparison",
			path: "$.items[?count(@.tags[*]) > 1].name",
			want: []any{"pencil", "stapler"},
		},
		{
			name: "match function anchors alternation",
			path: `$.items[?match(@.code, "a|b")].name`,
			want: []any{"paper", "pencil"},
		},
		{
			name: "relative descendant existence",
			path: "$.items[?@..target].name",
			want: []any{"paper", "stapler"},
		},
		{
			name: "absolute descendant existence",
			path: "$.items[?$..target].name",
			want: []any{"paper", "pencil", "stapler", "desk"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MustParse(tc.path).Select(input)
			if diff := cmp.Diff(tc.want, []any(got)); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_Select_InvalidDynamicIRegexpReturnsFalse(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"items": []any{
			map[string]any{"name": "valid", "value": "hello", "pattern": "hello"},
			map[string]any{"name": "invalid", "value": "HELLO", "pattern": "(?i)hello"},
		},
	}

	got := MustParse(`$.items[?match(@.value, @.pattern)].name`).Select(input)
	assert.Equal(t, NodeList{"valid"}, got)
}

func TestPath_Select_ExactLargeIntegerComparison(t *testing.T) {
	t.Parallel()

	input := []any{
		int64(9007199254740992),
		int64(9007199254740993),
	}

	for _, tc := range []struct {
		name string
		expr string
		want []any
	}{
		{name: "equal", expr: `$[?@ == 9007199254740992]`, want: []any{input[0]}},
		{name: "not_equal", expr: `$[?@ != 9007199254740992]`, want: []any{input[1]}},
		{name: "less", expr: `$[?@ < 9007199254740993]`, want: []any{input[0]}},
		{name: "less_equal", expr: `$[?@ <= 9007199254740992]`, want: []any{input[0]}},
		{name: "greater", expr: `$[?@ > 9007199254740992]`, want: []any{input[1]}},
		{name: "greater_equal", expr: `$[?@ >= 9007199254740993]`, want: []any{input[1]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse(tc.expr)
			if diff := cmp.Diff(tc.want, []any(path.Select(input))); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.want, slices.Collect(path.SelectLocated(input).Values())); diff != "" {
				t.Errorf("SelectLocated().Values() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_Select_JSONNumberComparison(t *testing.T) {
	t.Parallel()

	input := []any{
		jsonv1.Number("9007199254740992"),
		jsonv1.Number("9007199254740993"),
	}
	path := MustParse(`$[?@ == 9007199254740992]`)
	want := []any{input[0]}

	if diff := cmp.Diff(want, []any(path.Select(input))); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_Select_ExactLargeUnsignedIntegerComparison(t *testing.T) {
	t.Parallel()

	input := []any{
		uint64(9007199254740992),
		uint64(9007199254740993),
	}
	path := MustParse(`$[?@ == 9007199254740992]`)
	want := []any{input[0]}

	if diff := cmp.Diff(want, []any(path.Select(input))); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want, slices.Collect(path.SelectLocated(input).Values())); diff != "" {
		t.Errorf("SelectLocated().Values() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_Select_ExactDecimalComparison(t *testing.T) {
	t.Parallel()

	decimal := jsonv1.Number("0.1")
	float := float64(0.1)
	path := MustParse(`$[?@ == 0.1]`)
	assert.Equal(t, `$[?@ == 0.1]`, path.String())
	want := []any{decimal}

	if diff := cmp.Diff(want, []any(path.Select([]any{decimal, float}))); diff != "" {
		t.Errorf("Select() exact decimal comparison mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want, slices.Collect(path.SelectLocated([]any{decimal, float}).Values())); diff != "" {
		t.Errorf("SelectLocated().Values() exact decimal comparison mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]any{float}, []any(MustParse(`$[?@ > 0.1]`).Select([]any{decimal, float}))); diff != "" {
		t.Errorf("Select() positive binary float ordering mismatch (-want +got):\n%s", diff)
	}

	negativeDecimal := jsonv1.Number("-0.1")
	negativeFloat := float64(-0.1)
	if diff := cmp.Diff(
		[]any{negativeFloat},
		[]any(MustParse(`$[?@ < -0.1]`).Select([]any{negativeDecimal, negativeFloat})),
	); diff != "" {
		t.Errorf("Select() negative binary float ordering mismatch (-want +got):\n%s", diff)
	}

	zeroInput := []any{-0.5, math.Copysign(0, -1), 0.5}
	for _, tc := range []struct {
		name string
		expr string
		want []any
	}{
		{name: "negative float before decimal zero", expr: "$[?@ < 0]", want: []any{-0.5}},
		{name: "negative zero equals decimal zero", expr: "$[?@ == 0]", want: []any{zeroInput[1]}},
		{name: "positive float after decimal zero", expr: "$[?@ > 0]", want: []any{0.5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MustParse(tc.expr).Select(zeroInput)
			if diff := cmp.Diff(tc.want, []any(got)); diff != "" {
				t.Errorf("Select() decimal/float ordering mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_Select_InvalidJSONNumbersAreNotComparable(t *testing.T) {
	t.Parallel()

	input := []any{
		math.Inf(1),
		jsonv1.Number("invalid"),
	}
	got := MustParse(`$[?@ == @]`).Select(input)

	assert.Empty(t, got)
}

func TestPath_Select_LargeExponentJSONNumberIsComparable(t *testing.T) {
	t.Parallel()

	path := MustParse(`$[?@ == @]`)
	value := jsonv1.Number("1e1000001")
	want := []any{value}

	if diff := cmp.Diff(want, []any(path.Select([]any{value}))); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want, slices.Collect(path.SelectLocated([]any{value}).Values())); diff != "" {
		t.Errorf("SelectLocated().Values() mismatch (-want +got):\n%s", diff)
	}

	got, err := QueryJSON([]byte(`[1e1000001]`), path)
	require.NoError(t, err)
	if diff := cmp.Diff(NodeList{value}, got); diff != "" {
		t.Errorf("QueryJSON() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_SelectLocated_NameSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		selector ast.Selector
		want     LocatedNodeList
	}{
		{
			name:     "select existing key",
			input:    map[string]any{"a": 1, "b": 2},
			selector: ast.NameSelector("a"),
			want: LocatedNodeList{
				{Value: 1, Path: mustNormalizedPath(NameElement("a"))},
			},
		},
		{
			name:     "select missing key",
			input:    map[string]any{"a": 1},
			selector: ast.NameSelector("b"),
			want:     LocatedNodeList{},
		},
		{
			name:     "select from non-object",
			input:    []any{1, 2, 3},
			selector: ast.NameSelector("a"),
			want:     LocatedNodeList{},
		},
		{
			name:     "select nested object",
			input:    map[string]any{"a": map[string]any{"b": 42}},
			selector: ast.NameSelector("a"),
			want: LocatedNodeList{
				{Value: map[string]any{"b": 42}, Path: mustNormalizedPath(NameElement("a"))},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seg := ast.Child(tt.selector)
			query := ast.NewPathQuery(true, seg)
			path := &Path{query: query}
			got := path.SelectLocated(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_SelectLocated_IndexSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		selector ast.Selector
		want     LocatedNodeList
	}{
		{
			name:     "select positive index",
			input:    []any{10, 20, 30},
			selector: ast.IndexSelector(1),
			want: LocatedNodeList{
				{Value: 20, Path: mustNormalizedPath(IndexElement(1))},
			},
		},
		{
			name:     "select negative index",
			input:    []any{10, 20, 30},
			selector: ast.IndexSelector(-1),
			want: LocatedNodeList{
				{Value: 30, Path: mustNormalizedPath(IndexElement(2))},
			},
		},
		{
			name:     "select out of bounds",
			input:    []any{10, 20},
			selector: ast.IndexSelector(5),
			want:     LocatedNodeList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seg := ast.Child(tt.selector)
			query := ast.NewPathQuery(true, seg)
			path := &Path{query: query}
			got := path.SelectLocated(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_SelectLocated_MultipleSelectors(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	seg := ast.Child(ast.NameSelector("a"), ast.NameSelector("c"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}
	got := path.SelectLocated(input)

	want := LocatedNodeList{
		{Value: 1, Path: mustNormalizedPath(NameElement("a"))},
		{Value: 3, Path: mustNormalizedPath(NameElement("c"))},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_SelectLocated_SliceSelector(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		path  string
		input any
		want  LocatedNodeList
	}{
		{
			name:  "positive step",
			path:  "$[1:4:2]",
			input: []any{0, 1, 2, 3, 4},
			want: LocatedNodeList{
				{Value: 1, Path: mustNormalizedPath(IndexElement(1))},
				{Value: 3, Path: mustNormalizedPath(IndexElement(3))},
			},
		},
		{
			name:  "negative step",
			path:  "$[3:0:-1]",
			input: []any{0, 1, 2, 3, 4},
			want: LocatedNodeList{
				{Value: 3, Path: mustNormalizedPath(IndexElement(3))},
				{Value: 2, Path: mustNormalizedPath(IndexElement(2))},
				{Value: 1, Path: mustNormalizedPath(IndexElement(1))},
			},
		},
		{
			name:  "negative step clamps out-of-range bounds",
			path:  "$[100:-100:-1]",
			input: []any{0, 1, 2},
			want: LocatedNodeList{
				{Value: 2, Path: mustNormalizedPath(IndexElement(2))},
				{Value: 1, Path: mustNormalizedPath(IndexElement(1))},
				{Value: 0, Path: mustNormalizedPath(IndexElement(0))},
			},
		},
		{
			name:  "zero step",
			path:  "$[::0]",
			input: []any{0, 1, 2, 3, 4},
			want:  LocatedNodeList{},
		},
		{
			name:  "non array",
			path:  "$[1:3]",
			input: map[string]any{"a": 1},
			want:  LocatedNodeList{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MustParse(tc.path).SelectLocated(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_SelectLocated_WildcardSelector(t *testing.T) {
	t.Parallel()

	t.Run("wildcard on object", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{"z": "z", "a": "a", "m": "m"}
		seg := ast.Child(ast.WildcardSelector())
		query := ast.NewPathQuery(true, seg)
		path := &Path{query: query}
		got := path.SelectLocated(input)

		want := LocatedNodeList{
			{Value: "a", Path: mustNormalizedPath(NameElement("a"))},
			{Value: "m", Path: mustNormalizedPath(NameElement("m"))},
			{Value: "z", Path: mustNormalizedPath(NameElement("z"))},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("wildcard on array", func(t *testing.T) {
		t.Parallel()

		input := []any{10, 20, 30}
		seg := ast.Child(ast.WildcardSelector())
		query := ast.NewPathQuery(true, seg)
		path := &Path{query: query}
		got := path.SelectLocated(input)

		want := LocatedNodeList{
			{Value: 10, Path: mustNormalizedPath(IndexElement(0))},
			{Value: 20, Path: mustNormalizedPath(IndexElement(1))},
			{Value: 30, Path: mustNormalizedPath(IndexElement(2))},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPath_SelectLocated_FilterSelector(t *testing.T) {
	t.Parallel()

	t.Run("array filter preserves indexes", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"items": []any{
				map[string]any{"name": "paper", "price": 5},
				map[string]any{"name": "pencil", "price": 2},
				map[string]any{"name": "stapler", "price": 20},
			},
		}

		got := MustParse("$.items[?@.price < 10].name").SelectLocated(input)
		want := LocatedNodeList{
			{
				Value: "paper",
				Path:  mustNormalizedPath(NameElement("items"), IndexElement(0), NameElement("name")),
			},
			{
				Value: "pencil",
				Path:  mustNormalizedPath(NameElement("items"), IndexElement(1), NameElement("name")),
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("object filter preserves keys", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"stock": map[string]any{
				"pencil": map[string]any{"name": "Pencil", "price": 2},
				"desk":   map[string]any{"name": "Desk", "price": 50},
				"paper":  map[string]any{"name": "Paper", "price": 5},
				"binder": map[string]any{"name": "Binder", "price": 4},
			},
		}

		got := MustParse("$.stock[?@.price < 10].name").SelectLocated(input)
		want := LocatedNodeList{
			{
				Value: "Binder",
				Path:  mustNormalizedPath(NameElement("stock"), NameElement("binder"), NameElement("name")),
			},
			{
				Value: "Paper",
				Path:  mustNormalizedPath(NameElement("stock"), NameElement("paper"), NameElement("name")),
			},
			{
				Value: "Pencil",
				Path:  mustNormalizedPath(NameElement("stock"), NameElement("pencil"), NameElement("name")),
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPath_SelectLocated_MultipleSegments(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"title": "Book 1", "price": 10},
				map[string]any{"title": "Book 2", "price": 20},
			},
		},
	}

	seg1 := ast.Child(ast.NameSelector("store"))
	seg2 := ast.Child(ast.NameSelector("book"))
	seg3 := ast.Child(ast.IndexSelector(0))
	seg4 := ast.Child(ast.NameSelector("title"))

	query := ast.NewPathQuery(true, seg1, seg2, seg3, seg4)
	path := &Path{query: query}
	got := path.SelectLocated(input)

	want := LocatedNodeList{
		{
			Value: "Book 1",
			Path: mustNormalizedPath(
				NameElement("store"),
				NameElement("book"),
				IndexElement(0),
				NameElement("title")),
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_SelectLocated_MissingIntermediate(t *testing.T) {
	t.Parallel()

	input := map[string]any{"existing": map[string]any{"child": true}}
	emptyPath := MustParse("$.missing.child")
	assert.Empty(t, emptyPath.SelectLocated(input))
	assert.Empty(t, emptyPath.Select(input))
}

func TestPath_SelectLocated_DescendantSelector(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"z": map[string]any{"value": "z"},
		"b": map[string]any{
			"value": "b",
			"c": map[string]any{
				"value": "b.c",
			},
		},
		"a": []any{
			map[string]any{"value": "a.0"},
		},
	}

	seg := ast.Descendant(ast.NameSelector("value"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}
	got := path.SelectLocated(input)

	want := LocatedNodeList{
		{Value: "a.0", Path: mustNormalizedPath(NameElement("a"), IndexElement(0), NameElement("value"))},
		{Value: "b", Path: mustNormalizedPath(NameElement("b"), NameElement("value"))},
		{Value: "b.c", Path: mustNormalizedPath(NameElement("b"), NameElement("c"), NameElement("value"))},
		{Value: "z", Path: mustNormalizedPath(NameElement("z"), NameElement("value"))},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_SelectLocated_DescendantFilter(t *testing.T) {
	t.Parallel()

	filterInput := map[string]any{
		"z": []any{
			map[string]any{"name": "z", "enabled": true},
		},
		"a": map[string]any{
			"nested": []any{
				map[string]any{"name": "a", "enabled": true},
				map[string]any{"name": "ignored", "enabled": false},
			},
		},
		"m": map[string]any{"name": "m", "enabled": true},
	}
	filterPath := MustParse("$..[?@.enabled == true].name")
	filterGot := filterPath.SelectLocated(filterInput)
	filterWant := LocatedNodeList{
		{Value: "m", Path: mustNormalizedPath(NameElement("m"), NameElement("name"))},
		{
			Value: "a",
			Path: mustNormalizedPath(
				NameElement("a"),
				NameElement("nested"),
				IndexElement(0),
				NameElement("name"),
			),
		},
		{Value: "z", Path: mustNormalizedPath(NameElement("z"), IndexElement(0), NameElement("name"))},
	}
	if diff := cmp.Diff(filterWant, filterGot); diff != "" {
		t.Errorf("SelectLocated() descendant filter mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]any(filterPath.Select(filterInput)), slices.Collect(filterGot.Values())); diff != "" {
		t.Errorf("SelectLocated().Values() descendant filter mismatch Select() (-want +got):\n%s", diff)
	}
}

func TestPath_SelectLocated_DescendantMultiSelector(t *testing.T) {
	t.Parallel()

	multiInput := map[string]any{
		"name": "root",
		"a":    []any{map[string]any{"name": "nested"}, "tail"},
		"z":    []any{"zero"},
	}
	multiPath := MustParse(`$..["name",0]`)
	multiGot := multiPath.SelectLocated(multiInput)
	multiWant := LocatedNodeList{
		{Value: "root", Path: mustNormalizedPath(NameElement("name"))},
		{Value: map[string]any{"name": "nested"}, Path: mustNormalizedPath(NameElement("a"), IndexElement(0))},
		{Value: "nested", Path: mustNormalizedPath(NameElement("a"), IndexElement(0), NameElement("name"))},
		{Value: "zero", Path: mustNormalizedPath(NameElement("z"), IndexElement(0))},
	}
	if diff := cmp.Diff(multiWant, multiGot); diff != "" {
		t.Errorf("SelectLocated() descendant multi-selector mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]any(multiPath.Select(multiInput)), slices.Collect(multiGot.Values())); diff != "" {
		t.Errorf("SelectLocated().Values() descendant multi-selector mismatch Select() (-want +got):\n%s", diff)
	}
}

func TestPath_SelectLocated_ComplexPath(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"title": "Book 1", "price": 8.95},
				map[string]any{"title": "Book 2", "price": 12.99},
				map[string]any{"title": "Book 3", "price": 8.99},
			},
		},
	}

	// $['store']['book'][*]['price']
	seg1 := ast.Child(ast.NameSelector("store"))
	seg2 := ast.Child(ast.NameSelector("book"))
	seg3 := ast.Child(ast.WildcardSelector())
	seg4 := ast.Child(ast.NameSelector("price"))

	query := ast.NewPathQuery(true, seg1, seg2, seg3, seg4)
	path := &Path{query: query}
	got := path.SelectLocated(input)

	want := LocatedNodeList{
		{
			Value: 8.95,
			Path: mustNormalizedPath(
				NameElement("store"),
				NameElement("book"),
				IndexElement(0),
				NameElement("price")),
		},
		{
			Value: 12.99,
			Path: mustNormalizedPath(
				NameElement("store"),
				NameElement("book"),
				IndexElement(1),
				NameElement("price")),
		},
		{
			Value: 8.99,
			Path: mustNormalizedPath(
				NameElement("store"),
				NameElement("book"),
				IndexElement(2),
				NameElement("price")),
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_SelectLocatedValuesMatchSelect(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"catalog": map[string]any{
			"books": []any{
				map[string]any{"title": "A", "price": 5, "tags": []any{"fiction", "paper"}, "meta": map[string]any{"target": "paper"}},
				map[string]any{"title": "B", "price": 15, "tags": []any{"fiction"}},
				map[string]any{"title": "C", "price": 8, "tags": []any{"reference", "paper"}, "details": []any{map[string]any{"target": "ref"}}},
			},
			"stock": map[string]any{
				"z": map[string]any{"title": "Z", "price": 1},
				"a": map[string]any{"title": "A-stock", "price": 2},
				"m": map[string]any{"title": "M", "price": 3},
			},
		},
		"meta": map[string]any{
			"title": "root title",
		},
	}

	for _, tc := range []struct {
		name string
		expr string
	}{
		{name: "name", expr: "$.catalog"},
		{name: "index", expr: "$.catalog.books[1]"},
		{name: "slice", expr: "$.catalog.books[2:0:-1].title"},
		{name: "multiple_selectors", expr: "$.catalog.books[0,2].title"},
		{name: "wildcard_object", expr: "$.catalog.stock[*].title"},
		{name: "filter_array", expr: "$.catalog.books[?@.price < 10].title"},
		{name: "filter_object", expr: "$.catalog.stock[?@.price < 3].title"},
		{name: "filter_relative_descendant", expr: "$.catalog.books[?@..target].title"},
		{name: "filter_absolute_descendant", expr: "$.catalog.books[?$..target].title"},
		{name: "descendant_name", expr: "$..title"},
		{name: "descendant_wildcard", expr: "$.catalog..*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse(tc.expr)
			got := slices.Collect(path.SelectLocated(input).Values())
			want := []any(path.Select(input))
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("SelectLocated().Values() mismatch Select() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_SelectionDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	newInput := func() map[string]any {
		return map[string]any{
			"items": []any{
				map[string]any{"enabled": true, "nested": map[string]any{"value": 1}},
				map[string]any{"enabled": false, "nested": map[string]any{"value": 2}},
			},
		}
	}
	path := MustParse("$.items[?@.enabled == true]..value")

	for _, tc := range []struct {
		name string
		run  func(Path, any)
	}{
		{name: "Select", run: func(path Path, input any) { path.Select(input) }},
		{name: "SelectLocated", run: func(path Path, input any) { path.SelectLocated(input) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := newInput()
			tc.run(path, input)
			if diff := cmp.Diff(newInput(), input); diff != "" {
				t.Errorf("selection mutated input (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPath_SelectLocated_NilQuery(t *testing.T) {
	t.Parallel()

	path := &Path{query: nil}
	got := path.SelectLocated(map[string]any{"a": 1})
	assert.Nil(t, got)
}

func TestPath_SelectLocated_NegativeSlicePaths(t *testing.T) {
	t.Parallel()

	path := MustParse("$[::-2]")
	got := path.SelectLocated([]any{0, 1, 2, 3, 4})

	values := slices.Collect(got.Values())
	if diff := cmp.Diff([]any{4, 2, 0}, values); diff != "" {
		t.Errorf("Values() mismatch (-want +got):\n%s", diff)
	}

	paths := make([]string, 0, len(got))
	for p := range got.Paths() {
		paths = append(paths, p.String())
	}
	if diff := cmp.Diff([]string{"$[4]", "$[2]", "$[0]"}, paths); diff != "" {
		t.Errorf("Paths() mismatch (-want +got):\n%s", diff)
	}
}

func BenchmarkSelectLocated_NameSelector(b *testing.B) {
	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	seg := ast.Child(ast.NameSelector("b"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.SelectLocated(input)
	}
}

func BenchmarkSelectLocated_ComplexPath(b *testing.B) {
	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"title": "Book 1", "price": 10},
				map[string]any{"title": "Book 2", "price": 20},
				map[string]any{"title": "Book 3", "price": 30},
				map[string]any{"title": "Book 4", "price": 40},
				map[string]any{"title": "Book 5", "price": 50},
			},
		},
	}

	seg1 := ast.Child(ast.NameSelector("store"))
	seg2 := ast.Child(ast.NameSelector("book"))
	seg3 := ast.Child(ast.WildcardSelector())
	seg4 := ast.Child(ast.NameSelector("price"))

	query := ast.NewPathQuery(true, seg1, seg2, seg3, seg4)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.SelectLocated(input)
	}
}

func BenchmarkSelectLocated_WildcardSelector(b *testing.B) {
	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
		"d": 4,
		"e": 5,
		"f": 6,
	}
	seg := ast.Child(ast.WildcardSelector())
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.SelectLocated(input)
	}
}

func BenchmarkSelectLocated_SliceSelector(b *testing.B) {
	input := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	seg := ast.Child(ast.SliceSelector(ast.SliceArgs{
		HasStart: true,
		Start:    1,
		HasEnd:   true,
		End:      9,
		HasStep:  true,
		Step:     2,
	}))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.SelectLocated(input)
	}
}

func BenchmarkSelectLocated_DescendantSelector(b *testing.B) {
	input := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"target": 1,
			},
			"c": []any{
				map[string]any{"target": 2},
				map[string]any{"skip": 3},
			},
		},
		"target": 4,
	}
	seg := ast.Descendant(ast.NameSelector("target"))
	query := ast.NewPathQuery(true, seg)
	path := &Path{query: query}

	b.ResetTimer()
	for b.Loop() {
		_ = path.SelectLocated(input)
	}
}

func BenchmarkExtendPath(b *testing.B) {
	b.Run("short", func(b *testing.B) {
		path := mustNormalizedPath(NameElement("store"))
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = extendPath(path, namePathStep("book"))
		}
	})

	b.Run("deep", func(b *testing.B) {
		path := mustNormalizedPath(
			NameElement("store"),
			NameElement("book"),
			IndexElement(12),
			NameElement("author"),
			NameElement("name"))

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = extendPath(path, namePathStep("first"))
		}
	})
}

// BenchmarkSelect suite covering name, index, slice, wildcard, filter, and descendant selectors

func BenchmarkSelect_Name_SmallObject(b *testing.B) {
	input := map[string]any{
		"a": 1, "b": 2, "c": 3, "d": 4, "e": 5,
	}
	path := MustParse("$.c")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Name_LargeObject(b *testing.B) {
	input := make(map[string]any, 100)
	for i := range 100 {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}
	path := MustParse("$.z9")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Name_NestedPath(b *testing.B) {
	input := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"level4": map[string]any{
						"value": 42,
					},
				},
			},
		},
	}
	path := MustParse("$.level1.level2.level3.level4.value")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Index_SmallArray(b *testing.B) {
	input := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	path := MustParse("$[5]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Index_LargeArray(b *testing.B) {
	input := make([]any, 1000)
	for i := range input {
		input[i] = i
	}
	path := MustParse("$[500]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Index_NegativeIndex(b *testing.B) {
	input := make([]any, 100)
	for i := range input {
		input[i] = i
	}
	path := MustParse("$[-1]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Slice_SmallRange(b *testing.B) {
	input := make([]any, 100)
	for i := range input {
		input[i] = i
	}
	path := MustParse("$[10:20]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Slice_LargeRange(b *testing.B) {
	input := make([]any, 1000)
	for i := range input {
		input[i] = i
	}
	path := MustParse("$[100:900]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Slice_WithStep(b *testing.B) {
	input := make([]any, 1000)
	for i := range input {
		input[i] = i
	}
	path := MustParse("$[0:1000:10]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Slice_NegativeStep(b *testing.B) {
	input := make([]any, 100)
	for i := range input {
		input[i] = i
	}
	path := MustParse("$[99:0:-1]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Wildcard_SmallObject(b *testing.B) {
	input := map[string]any{
		"a": 1, "b": 2, "c": 3, "d": 4, "e": 5,
	}
	path := MustParse("$[*]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Wildcard_LargeObject(b *testing.B) {
	input := make(map[string]any, 100)
	for i := range 100 {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}
	path := MustParse("$[*]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Wildcard_Array(b *testing.B) {
	input := make([]any, 100)
	for i := range input {
		input[i] = i
	}
	path := MustParse("$[*]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Wildcard_NestedArrays(b *testing.B) {
	input := make([]any, 10)
	for i := range input {
		inner := make([]any, 10)
		for j := range inner {
			inner[j] = i*10 + j
		}
		input[i] = inner
	}
	path := MustParse("$[*][*]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Descendant_ShallowStructure(b *testing.B) {
	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
		"d": 4,
		"e": 5,
	}
	path := MustParse("$..a")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Descendant_DeepStructure(b *testing.B) {
	input := map[string]any{
		"a": 1,
		"b": map[string]any{
			"a": 2,
			"c": map[string]any{
				"a": 3,
				"d": map[string]any{
					"a": 4,
					"e": map[string]any{
						"a": 5,
					},
				},
			},
		},
	}
	path := MustParse("$..a")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Descendant_WideStructure(b *testing.B) {
	input := make(map[string]any, 20)
	for i := range 20 {
		inner := make(map[string]any, 5)
		for j := range 5 {
			inner[string(rune('a'+j))] = i*5 + j
		}
		input[string(rune('A'+i))] = inner
	}
	path := MustParse("$..a")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_Descendant_Wildcard(b *testing.B) {
	input := map[string]any{
		"a": 1,
		"b": map[string]any{
			"c": 2,
			"d": 3,
		},
		"e": []any{4, 5, 6},
		"f": map[string]any{
			"g": map[string]any{
				"h": 7,
			},
		},
	}
	path := MustParse("$..[*]")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_FilterQueries(b *testing.B) {
	input := map[string]any{
		"expensive": 15,
		"numbers": []any{
			jsonv1.Number("1e1000"),
			jsonv1.Number("-1e-1000"),
		},
		"items": []any{
			map[string]any{"name": "paper", "price": 5, "tags": []any{"office"}},
			map[string]any{"name": "pencil", "price": 2, "tags": []any{"office", "writing"}},
			map[string]any{"name": "stapler", "price": 20, "tags": []any{"office", "metal"}},
			map[string]any{"name": "desk", "price": 50, "tags": []any{"furniture"}},
		},
	}

	benchmarks := []struct {
		name string
		path Path
	}{
		{
			name: "singular_query",
			path: MustParse("$.items[?@.price < 10].name"),
		},
		{
			name: "root_query",
			path: MustParse("$.items[?@.price < $.expensive].name"),
		},
		{
			name: "count_function_query",
			path: MustParse("$.items[?count(@.tags[*]) > 1].name"),
		},
		{
			name: "large_decimal_exponent",
			path: MustParse("$.numbers[?@ == @]"),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			if bm.name == "large_decimal_exponent" {
				require.Equal(b, NodeList{
					jsonv1.Number("1e1000"),
					jsonv1.Number("-1e-1000"),
				}, bm.path.Select(input))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = bm.path.Select(input)
			}
		})
	}
}

func BenchmarkSelect_RealWorld_BookStore(b *testing.B) {
	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95},
				map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": 12.99},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99},
				map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": 22.99},
			},
			"bicycle": map[string]any{
				"color": "red",
				"price": 19.95,
			},
		},
	}
	path := MustParse("$.store.book[*].price")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_RealWorld_AllPrices(b *testing.B) {
	input := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95},
				map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": 12.99},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99},
				map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": 22.99},
			},
			"bicycle": map[string]any{
				"color": "red",
				"price": 19.95,
			},
		},
	}
	path := MustParse("$..price")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func BenchmarkSelect_RealWorld_DeepJSON(b *testing.B) {
	input := map[string]any{
		"users": []any{
			map[string]any{
				"id":   1,
				"name": "Alice",
				"profile": map[string]any{
					"age":   30,
					"email": "alice@example.com",
					"address": map[string]any{
						"city":    "New York",
						"country": "USA",
					},
				},
			},
			map[string]any{
				"id":   2,
				"name": "Bob",
				"profile": map[string]any{
					"age":   25,
					"email": "bob@example.com",
					"address": map[string]any{
						"city":    "London",
						"country": "UK",
					},
				},
			},
		},
	}
	path := MustParse("$.users[*].profile.address.city")

	b.ResetTimer()
	for b.Loop() {
		_ = path.Select(input)
	}
}

func TestDescendantSelectionHandlesDeepJSON(t *testing.T) {
	t.Parallel()

	root := map[string]any{}
	current := root
	for range 300 {
		next := map[string]any{}
		current["nested"] = next
		current = next
	}
	current["target"] = "found"

	path := MustParse("$..target")

	got := path.Select(root)
	if diff := cmp.Diff(NodeList{"found"}, got); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}

	located := path.SelectLocated(root)
	require.Len(t, located, 1)
	assert.Equal(t, "found", located[0].Value)
	assert.True(t, strings.HasSuffix(located[0].Path.String(), "['target']"))
}
