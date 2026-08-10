package ast

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestSelectorConstructors(t *testing.T) {
	t.Parallel()

	t.Run("name_selector", func(t *testing.T) {
		t.Parallel()
		s := NameSelector("foo")
		assert.Equal(t, Name, s.Kind)
		assert.Equal(t, "foo", s.Name)
	})

	t.Run("index_selector", func(t *testing.T) {
		t.Parallel()
		s := IndexSelector(42)
		assert.Equal(t, Index, s.Kind)
		assert.Equal(t, int64(42), s.Index)
	})

	t.Run("index_selector_negative", func(t *testing.T) {
		t.Parallel()
		s := IndexSelector(-1)
		assert.Equal(t, Index, s.Kind)
		assert.Equal(t, int64(-1), s.Index)
	})

	t.Run("slice_selector", func(t *testing.T) {
		t.Parallel()
		args := SliceArgs{Start: 1, End: 5, Step: 2, HasStart: true, HasEnd: true, HasStep: true}
		s := SliceSelector(args)
		assert.Equal(t, Slice, s.Kind)
		if diff := cmp.Diff(args, s.Slice); diff != "" {
			t.Errorf("SliceSelector() args mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("wildcard_selector", func(t *testing.T) {
		t.Parallel()
		s := WildcardSelector()
		assert.Equal(t, Wildcard, s.Kind)
	})

	t.Run("filter_selector", func(t *testing.T) {
		t.Parallel()
		expr := &FilterExpr{}
		s := FilterSelector(expr)
		assert.Equal(t, Filter, s.Kind)
		assert.Same(t, expr, s.Filter)
	})
}

func TestSelectorIsSingular(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		sel      Selector
		singular bool
	}{
		{
			name:     "name_is_singular",
			sel:      NameSelector("x"),
			singular: true,
		},
		{
			name:     "index_is_singular",
			sel:      IndexSelector(0),
			singular: true,
		},
		{
			name:     "slice_not_singular",
			sel:      SliceSelector(SliceArgs{HasStart: true, Start: 0}),
			singular: false,
		},
		{
			name:     "wildcard_not_singular",
			sel:      WildcardSelector(),
			singular: false,
		},
		{
			name:     "filter_not_singular",
			sel:      FilterSelector(&FilterExpr{}),
			singular: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.singular, tc.sel.IsSingular())
		})
	}
}

func TestSelectorString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		sel  Selector
		want string
	}{
		{
			name: "name_simple",
			sel:  NameSelector("foo"),
			want: `"foo"`,
		},
		{
			name: "name_with_space",
			sel:  NameSelector("hello world"),
			want: `"hello world"`,
		},
		{
			name: "name_with_quote",
			sel:  NameSelector(`say "hi"`),
			want: `"say \"hi\""`,
		},
		{
			name: "name_unicode",
			sel:  NameSelector("日本語"),
			want: `"日本語"`,
		},
		{
			name: "name_empty",
			sel:  NameSelector(""),
			want: `""`,
		},
		{
			name: "name_control",
			sel:  NameSelector("\v"),
			want: `"\u000b"`,
		},
		{
			name: "index_zero",
			sel:  IndexSelector(0),
			want: "0",
		},
		{
			name: "index_positive",
			sel:  IndexSelector(42),
			want: "42",
		},
		{
			name: "index_negative",
			sel:  IndexSelector(-1),
			want: "-1",
		},
		{
			name: "index_large",
			sel:  IndexSelector(9007199254740992),
			want: "9007199254740992",
		},
		{
			name: "wildcard",
			sel:  WildcardSelector(),
			want: "*",
		},
		{
			name: "filter",
			sel:  FilterSelector(&FilterExpr{}),
			want: "?",
		},
		{
			name: "slice_full",
			sel:  SliceSelector(SliceArgs{Start: 1, End: 5, Step: 2, HasStart: true, HasEnd: true, HasStep: true}),
			want: "1:5:2",
		},
		{
			name: "slice_start_only",
			sel:  SliceSelector(SliceArgs{Start: 3, HasStart: true}),
			want: "3:",
		},
		{
			name: "slice_end_only",
			sel:  SliceSelector(SliceArgs{End: 5, HasEnd: true}),
			want: ":5",
		},
		{
			name: "slice_step_only",
			sel:  SliceSelector(SliceArgs{Step: 2, HasStep: true}),
			want: "::2",
		},
		{
			name: "slice_start_end",
			sel:  SliceSelector(SliceArgs{Start: 1, End: 3, HasStart: true, HasEnd: true}),
			want: "1:3",
		},
		{
			name: "slice_start_step",
			sel:  SliceSelector(SliceArgs{Start: 1, Step: 2, HasStart: true, HasStep: true}),
			want: "1::2",
		},
		{
			name: "slice_end_step",
			sel:  SliceSelector(SliceArgs{End: 5, Step: 2, HasEnd: true, HasStep: true}),
			want: ":5:2",
		},
		{
			name: "slice_no_args",
			sel:  SliceSelector(SliceArgs{}),
			want: ":",
		},
		{
			name: "slice_negative_step",
			sel:  SliceSelector(SliceArgs{Start: 5, End: 1, Step: -1, HasStart: true, HasEnd: true, HasStep: true}),
			want: "5:1:-1",
		},
		{
			name: "slice_negative_start",
			sel:  SliceSelector(SliceArgs{Start: -3, HasStart: true}),
			want: "-3:",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.sel.String())
		})
	}
}

func TestSelectorWriteTo(t *testing.T) {
	t.Parallel()

	// Verify writeTo produces the same result as String for each kind.
	selectors := []Selector{
		NameSelector("test"),
		IndexSelector(7),
		SliceSelector(SliceArgs{Start: 1, End: 5, HasStart: true, HasEnd: true}),
		WildcardSelector(),
		FilterSelector(&FilterExpr{}),
	}
	for _, sel := range selectors {
		var buf strings.Builder
		sel.writeTo(&buf)
		assert.Equal(t, sel.String(), buf.String())
	}
}

func TestSliceArgsWriteTo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args SliceArgs
		want string
	}{
		{
			name: "all_set",
			args: SliceArgs{Start: 0, End: 10, Step: 2, HasStart: true, HasEnd: true, HasStep: true},
			want: "0:10:2",
		},
		{
			name: "none_set",
			args: SliceArgs{},
			want: ":",
		},
		{
			name: "only_start",
			args: SliceArgs{Start: 5, HasStart: true},
			want: "5:",
		},
		{
			name: "only_end",
			args: SliceArgs{End: 5, HasEnd: true},
			want: ":5",
		},
		{
			name: "only_step",
			args: SliceArgs{Step: 3, HasStep: true},
			want: "::3",
		},
		{
			name: "start_and_end",
			args: SliceArgs{Start: 1, End: 4, HasStart: true, HasEnd: true},
			want: "1:4",
		},
		{
			name: "negative_values",
			args: SliceArgs{Start: -5, End: -1, Step: -1, HasStart: true, HasEnd: true, HasStep: true},
			want: "-5:-1:-1",
		},
		{
			name: "zero_start_set",
			args: SliceArgs{Start: 0, End: 3, HasStart: true, HasEnd: true},
			want: "0:3",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf strings.Builder
			tc.args.writeTo(&buf)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func TestSelectorApplyName(t *testing.T) {
	t.Parallel()

	node := map[string]any{"foo": float64(1), "bar": float64(2)}

	for _, tc := range []struct {
		name string
		sel  Selector
		node any
		want []any
	}{
		{
			name: "hit",
			sel:  NameSelector("foo"),
			node: node,
			want: []any{float64(1)},
		},
		{
			name: "miss",
			sel:  NameSelector("missing"),
			node: node,
			want: nil,
		},
		{
			name: "not_a_map",
			sel:  NameSelector("foo"),
			node: []any{float64(1)},
			want: nil,
		},
		{
			name: "primitive_node",
			sel:  NameSelector("foo"),
			node: "hello",
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.sel.Apply(nil, tc.node, nil)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Selector.Apply() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectorApplyIndex(t *testing.T) {
	t.Parallel()

	arr := []any{"a", "b", "c", "d"}

	for _, tc := range []struct {
		name string
		sel  Selector
		node any
		want []any
	}{
		{
			name: "positive_index",
			sel:  IndexSelector(0),
			node: arr,
			want: []any{"a"},
		},
		{
			name: "positive_last",
			sel:  IndexSelector(3),
			node: arr,
			want: []any{"d"},
		},
		{
			name: "negative_index",
			sel:  IndexSelector(-1),
			node: arr,
			want: []any{"d"},
		},
		{
			name: "negative_first",
			sel:  IndexSelector(-4),
			node: arr,
			want: []any{"a"},
		},
		{
			name: "out_of_bounds_positive",
			sel:  IndexSelector(10),
			node: arr,
			want: nil,
		},
		{
			name: "out_of_bounds_negative",
			sel:  IndexSelector(-10),
			node: arr,
			want: nil,
		},
		{
			name: "not_an_array",
			sel:  IndexSelector(0),
			node: map[string]any{"a": float64(1)},
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.sel.Apply(nil, tc.node, nil)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Selector.Apply() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectorApplySlice(t *testing.T) {
	t.Parallel()

	arr := []any{float64(0), float64(1), float64(2), float64(3), float64(4)}

	for _, tc := range []struct {
		name string
		sel  Selector
		node any
		want []any
	}{
		{
			name: "basic_range",
			sel:  SliceSelector(SliceArgs{Start: 1, End: 3, HasStart: true, HasEnd: true}),
			node: arr,
			want: []any{float64(1), float64(2)},
		},
		{
			name: "full_slice_defaults",
			sel:  SliceSelector(SliceArgs{}),
			node: arr,
			want: []any{float64(0), float64(1), float64(2), float64(3), float64(4)},
		},
		{
			name: "negative_step_reverse",
			sel:  SliceSelector(SliceArgs{Step: -1, HasStep: true}),
			node: arr,
			want: []any{float64(4), float64(3), float64(2), float64(1), float64(0)},
		},
		{
			name: "step_zero_returns_nothing",
			sel:  SliceSelector(SliceArgs{Step: 0, HasStep: true}),
			node: arr,
			want: nil,
		},
		{
			name: "empty_array",
			sel:  SliceSelector(SliceArgs{HasStart: true, Start: 0}),
			node: []any{},
			want: nil,
		},
		{
			name: "negative_start",
			sel:  SliceSelector(SliceArgs{Start: -2, HasStart: true}),
			node: arr,
			want: []any{float64(3), float64(4)},
		},
		{
			name: "step_2",
			sel:  SliceSelector(SliceArgs{Start: 0, End: 5, Step: 2, HasStart: true, HasEnd: true, HasStep: true}),
			node: arr,
			want: []any{float64(0), float64(2), float64(4)},
		},
		{
			name: "negative_step_with_range",
			sel:  SliceSelector(SliceArgs{Start: 3, End: 0, Step: -1, HasStart: true, HasEnd: true, HasStep: true}),
			node: arr,
			want: []any{float64(3), float64(2), float64(1)},
		},
		{
			name: "not_an_array",
			sel:  SliceSelector(SliceArgs{HasStart: true, Start: 0}),
			node: map[string]any{"a": float64(1)},
			want: nil,
		},
		{
			name: "start_beyond_length",
			sel:  SliceSelector(SliceArgs{Start: 100, HasStart: true}),
			node: arr,
			want: nil,
		},
		{
			name: "negative_start_and_end",
			sel:  SliceSelector(SliceArgs{Start: -3, End: -1, HasStart: true, HasEnd: true}),
			node: arr,
			want: []any{float64(2), float64(3)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.sel.Apply(nil, tc.node, nil)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Selector.Apply() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectorApplyWildcard(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		node any
		want []any
	}{
		{
			name: "array",
			node: []any{float64(1), float64(2), float64(3)},
			want: []any{float64(1), float64(2), float64(3)},
		},
		{
			name: "map_returns_values",
			node: map[string]any{"a": float64(1)},
			want: []any{float64(1)},
		},
		{
			name: "primitive_returns_nothing",
			node: "hello",
			want: nil,
		},
		{
			name: "empty_array",
			node: []any{},
			want: nil,
		},
		{
			name: "empty_map",
			node: map[string]any{},
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sel := WildcardSelector()
			got := sel.Apply(nil, tc.node, nil)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Selector.Apply() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectorApplyFilter(t *testing.T) {
	t.Parallel()

	// alwaysTrue: a filter that always matches (ExistExpr on bare @)
	alwaysTrue := &FilterExpr{
		Or: LogicalOr{
			LogicalAnd{
				&ExistExpr{Query: NewPathQuery(false)}, // @ with no segments always exists
			},
		},
	}

	// neverTrue: a filter that never matches (NonExistExpr on bare @)
	neverTrue := &FilterExpr{
		Or: LogicalOr{
			LogicalAnd{
				&NonExistExpr{Query: NewPathQuery(false)}, // !@ never true
			},
		},
	}

	// hasName: a filter that checks if current node has key "name" via @["name"]
	hasName := &FilterExpr{
		Or: LogicalOr{
			LogicalAnd{
				&ExistExpr{
					Query: NewPathQuery(false, Child(NameSelector("name"))),
				},
			},
		},
	}

	for _, tc := range []struct {
		name string
		sel  Selector
		node any
		root any
		want []any
	}{
		{
			name: "always_true_on_array",
			sel:  FilterSelector(alwaysTrue),
			node: []any{float64(1), float64(2), float64(3)},
			root: nil,
			want: []any{float64(1), float64(2), float64(3)},
		},
		{
			name: "never_true_on_array",
			sel:  FilterSelector(neverTrue),
			node: []any{float64(1), float64(2)},
			root: nil,
			want: nil,
		},
		{
			name: "always_true_on_map",
			sel:  FilterSelector(alwaysTrue),
			node: map[string]any{"a": float64(1)},
			root: nil,
			want: []any{float64(1)},
		},
		{
			name: "never_true_on_map",
			sel:  FilterSelector(neverTrue),
			node: map[string]any{"a": float64(1)},
			root: nil,
			want: nil,
		},
		{
			name: "has_name_filter_on_array",
			sel:  FilterSelector(hasName),
			node: []any{
				map[string]any{"name": "Alice"},
				map[string]any{"age": float64(30)},
				map[string]any{"name": "Bob"},
			},
			root: nil,
			want: []any{
				map[string]any{"name": "Alice"},
				map[string]any{"name": "Bob"},
			},
		},
		{
			name: "filter_on_primitive",
			sel:  FilterSelector(alwaysTrue),
			node: "hello",
			root: nil,
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.sel.Apply(nil, tc.node, tc.root)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Selector.Apply() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApplySliceEdgeCases(t *testing.T) {
	t.Parallel()

	arr := []any{float64(0), float64(1), float64(2), float64(3), float64(4)}

	for _, tc := range []struct {
		name string
		sel  Selector
		node []any
		want []any
	}{
		{
			name: "negative_end_with_positive_step",
			sel:  SliceSelector(SliceArgs{Start: 0, End: -1, HasStart: true, HasEnd: true}),
			node: arr,
			want: []any{float64(0), float64(1), float64(2), float64(3)},
		},
		{
			name: "very_negative_start_clamped",
			sel:  SliceSelector(SliceArgs{Start: -100, End: 2, HasStart: true, HasEnd: true}),
			node: arr,
			want: []any{float64(0), float64(1)},
		},
		{
			name: "very_large_end_clamped",
			sel:  SliceSelector(SliceArgs{Start: 3, End: 100, HasStart: true, HasEnd: true}),
			node: arr,
			want: []any{float64(3), float64(4)},
		},
		{
			name: "negative_step_no_start_no_end",
			sel:  SliceSelector(SliceArgs{Step: -2, HasStep: true}),
			node: arr,
			want: []any{float64(4), float64(2), float64(0)},
		},
		{
			name: "single_element_array",
			sel:  SliceSelector(SliceArgs{}),
			node: []any{float64(42)},
			want: []any{float64(42)},
		},
		{
			name: "start_equals_end",
			sel:  SliceSelector(SliceArgs{Start: 2, End: 2, HasStart: true, HasEnd: true}),
			node: arr,
			want: nil,
		},
		{
			name: "start_beyond_end_with_positive_step",
			sel:  SliceSelector(SliceArgs{Start: 3, End: 1, Step: 1, HasStart: true, HasEnd: true, HasStep: true}),
			node: arr,
			want: nil,
		},
		{
			name: "negative_step_from_start_to_end",
			sel:  SliceSelector(SliceArgs{Start: 3, End: 1, Step: -1, HasStart: true, HasEnd: true, HasStep: true}),
			node: arr,
			want: []any{float64(3), float64(2)},
		},
		{
			name: "very_negative_start_without_end",
			sel:  SliceSelector(SliceArgs{Start: -100, HasStart: true}),
			node: arr,
			want: arr,
		},
		{
			name: "very_large_end_without_start",
			sel:  SliceSelector(SliceArgs{End: 100, HasEnd: true}),
			node: arr,
			want: arr,
		},
		{
			name: "step_larger_than_array",
			sel:  SliceSelector(SliceArgs{Step: 10, HasStep: true}),
			node: arr,
			want: []any{float64(0)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.sel.Apply(nil, tc.node, nil)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Selector.Apply() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
