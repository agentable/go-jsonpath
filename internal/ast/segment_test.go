package ast

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestChild(t *testing.T) {
	t.Parallel()

	t.Run("no_selectors", func(t *testing.T) {
		t.Parallel()
		s := Child()
		assert.Empty(t, s.Selectors())
		assert.False(t, s.IsDescendant())
	})

	t.Run("single_selector", func(t *testing.T) {
		t.Parallel()
		s := Child(NameSelector("foo"))
		assert.Len(t, s.Selectors(), 1)
		assert.Equal(t, Name, s.Selectors()[0].Kind)
		assert.False(t, s.IsDescendant())
	})

	t.Run("multiple_selectors", func(t *testing.T) {
		t.Parallel()
		s := Child(NameSelector("a"), IndexSelector(1), WildcardSelector())
		assert.Len(t, s.Selectors(), 3)
		assert.False(t, s.IsDescendant())
	})
}

func TestDescendant(t *testing.T) {
	t.Parallel()

	t.Run("no_selectors", func(t *testing.T) {
		t.Parallel()
		s := Descendant()
		assert.Empty(t, s.Selectors())
		assert.True(t, s.IsDescendant())
	})

	t.Run("single_selector", func(t *testing.T) {
		t.Parallel()
		s := Descendant(NameSelector("x"))
		assert.Len(t, s.Selectors(), 1)
		assert.True(t, s.IsDescendant())
	})
}

func TestSegmentIsSingular(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		seg      Segment
		singular bool
	}{
		{
			name:     "child_single_name",
			seg:      Child(NameSelector("a")),
			singular: true,
		},
		{
			name:     "child_single_index",
			seg:      Child(IndexSelector(0)),
			singular: true,
		},
		{
			name:     "child_wildcard",
			seg:      Child(WildcardSelector()),
			singular: false,
		},
		{
			name:     "child_slice",
			seg:      Child(SliceSelector(SliceArgs{HasStart: true, Start: 0})),
			singular: false,
		},
		{
			name:     "child_filter",
			seg:      Child(FilterSelector(&FilterExpr{})),
			singular: false,
		},
		{
			name:     "child_multiple_selectors",
			seg:      Child(NameSelector("a"), NameSelector("b")),
			singular: false,
		},
		{
			name:     "child_no_selectors",
			seg:      Child(),
			singular: false,
		},
		{
			name:     "descendant_single_name",
			seg:      Descendant(NameSelector("a")),
			singular: false,
		},
		{
			name:     "descendant_single_index",
			seg:      Descendant(IndexSelector(0)),
			singular: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.singular, tc.seg.IsSingular())
		})
	}
}

func TestSegmentString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		seg  Segment
		want string
	}{
		{
			name: "child_single_name",
			seg:  Child(NameSelector("foo")),
			want: `["foo"]`,
		},
		{
			name: "child_single_index",
			seg:  Child(IndexSelector(3)),
			want: `[3]`,
		},
		{
			name: "child_wildcard",
			seg:  Child(WildcardSelector()),
			want: `[*]`,
		},
		{
			name: "child_multiple_selectors",
			seg:  Child(NameSelector("a"), NameSelector("b"), IndexSelector(0)),
			want: `["a","b",0]`,
		},
		{
			name: "child_slice",
			seg:  Child(SliceSelector(SliceArgs{Start: 1, End: 5, Step: 2, HasStart: true, HasEnd: true, HasStep: true})),
			want: `[1:5:2]`,
		},
		{
			name: "descendant_single_name",
			seg:  Descendant(NameSelector("x")),
			want: `..["x"]`,
		},
		{
			name: "descendant_wildcard",
			seg:  Descendant(WildcardSelector()),
			want: `..[*]`,
		},
		{
			name: "descendant_multiple",
			seg:  Descendant(NameSelector("a"), IndexSelector(1)),
			want: `..["a",1]`,
		},
		{
			name: "child_empty",
			seg:  Child(),
			want: `[]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.seg.String())
		})
	}
}

func TestSegmentWriteTo(t *testing.T) {
	t.Parallel()

	// Verify writeTo produces the same result as String.
	seg := Child(NameSelector("test"), IndexSelector(2))
	var buf strings.Builder
	seg.writeTo(&buf)
	assert.Equal(t, seg.String(), buf.String())
}

func TestSegmentApply(t *testing.T) {
	t.Parallel()

	root := map[string]any{
		"a": float64(1),
		"b": float64(2),
		"c": []any{float64(10), float64(20), float64(30)},
	}

	for _, tc := range []struct {
		name  string
		seg   Segment
		nodes []any
		root  any
		want  []any
	}{
		{
			name:  "empty_nodes_returns_empty",
			seg:   Child(NameSelector("a")),
			nodes: []any{},
			root:  root,
			want:  []any{},
		},
		{
			name:  "nil_nodes_returns_nil",
			seg:   Child(NameSelector("a")),
			nodes: nil,
			root:  root,
			want:  nil,
		},
		{
			name:  "child_name_hit",
			seg:   Child(NameSelector("a")),
			nodes: []any{root},
			root:  root,
			want:  []any{float64(1)},
		},
		{
			name:  "child_name_miss",
			seg:   Child(NameSelector("z")),
			nodes: []any{root},
			root:  root,
			want:  []any{},
		},
		{
			name:  "child_index_on_array",
			seg:   Child(IndexSelector(1)),
			nodes: []any{root["c"]},
			root:  root,
			want:  []any{float64(20)},
		},
		{
			name:  "child_multiple_selectors",
			seg:   Child(NameSelector("a"), NameSelector("b")),
			nodes: []any{root},
			root:  root,
			want:  []any{float64(1), float64(2)},
		},
		{
			name:  "child_multiple_input_nodes",
			seg:   Child(IndexSelector(0)),
			nodes: []any{[]any{float64(1)}, []any{float64(2)}},
			root:  root,
			want:  []any{float64(1), float64(2)},
		},
		{
			name:  "descendant_name",
			seg:   Descendant(NameSelector("a")),
			nodes: []any{map[string]any{"a": float64(1), "nested": map[string]any{"a": float64(2)}}},
			root:  root,
			want:  []any{float64(1), float64(2)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.seg.Apply(tc.nodes, tc.root)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Segment.Apply() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAppendSelectors(t *testing.T) {
	t.Parallel()

	node := map[string]any{"x": float64(1), "y": float64(2)}
	root := node

	t.Run("multiple_selectors_applied", func(t *testing.T) {
		t.Parallel()
		selectors := []Selector{NameSelector("x"), NameSelector("y")}
		got := appendSelectors(nil, selectors, node, root)
		if diff := cmp.Diff([]any{float64(1), float64(2)}, got); diff != "" {
			t.Errorf("appendSelectors() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("no_selectors", func(t *testing.T) {
		t.Parallel()
		got := appendSelectors(nil, nil, node, root)
		assert.Nil(t, got)
	})

	t.Run("selector_miss", func(t *testing.T) {
		t.Parallel()
		selectors := []Selector{NameSelector("z")}
		got := appendSelectors(nil, selectors, node, root)
		assert.Empty(t, got)
	})
}

func TestAppendDescendant(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		selectors []Selector
		node      any
		root      any
		want      []any
	}{
		{
			name:      "flat_map",
			selectors: []Selector{NameSelector("a")},
			node:      map[string]any{"a": float64(1), "b": float64(2)},
			root:      nil,
			want:      []any{float64(1)},
		},
		{
			name:      "nested_map",
			selectors: []Selector{NameSelector("a")},
			node: map[string]any{
				"a": float64(1),
				"b": map[string]any{"a": float64(2)},
			},
			root: nil,
			want: []any{float64(1), float64(2)},
		},
		{
			name:      "nested_array",
			selectors: []Selector{NameSelector("x")},
			node: []any{
				map[string]any{"x": float64(10)},
				map[string]any{"x": float64(20)},
			},
			root: nil,
			want: []any{float64(10), float64(20)},
		},
		{
			name:      "primitive_node",
			selectors: []Selector{NameSelector("a")},
			node:      float64(42),
			root:      nil,
			want:      nil,
		},
		{
			name:      "deeply_nested",
			selectors: []Selector{NameSelector("val")},
			node: map[string]any{
				"val": float64(1),
				"child": map[string]any{
					"val": float64(2),
					"child": map[string]any{
						"val": float64(3),
					},
				},
			},
			root: nil,
			want: []any{float64(1), float64(2), float64(3)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := appendDescendant(nil, tc.selectors, tc.node, tc.root)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("appendDescendant() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
