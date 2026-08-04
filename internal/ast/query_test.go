package ast

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPathQuery(t *testing.T) {
	t.Parallel()

	t.Run("root_no_segments", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(true)
		assert.True(t, q.IsRoot())
		assert.Empty(t, q.Segments())
	})

	t.Run("relative_no_segments", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false)
		assert.False(t, q.IsRoot())
		assert.Empty(t, q.Segments())
	})

	t.Run("root_with_segments", func(t *testing.T) {
		t.Parallel()
		segs := []Segment{Child(NameSelector("x")), Child(IndexSelector(0))}
		q := NewPathQuery(true, segs...)
		assert.True(t, q.IsRoot())
		require.Len(t, q.Segments(), 2)
		assert.Equal(t, Name, q.Segments()[0].Selectors()[0].Kind)
		assert.Equal(t, Index, q.Segments()[1].Selectors()[0].Kind)
	})
}

func TestPathQueryString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		q    *PathQuery
		want string
	}{
		{
			name: "root_empty",
			q:    NewPathQuery(true),
			want: "$",
		},
		{
			name: "relative_empty",
			q:    NewPathQuery(false),
			want: "@",
		},
		{
			name: "root_single_name",
			q:    NewPathQuery(true, Child(NameSelector("foo"))),
			want: `$["foo"]`,
		},
		{
			name: "relative_single_name",
			q:    NewPathQuery(false, Child(NameSelector("bar"))),
			want: `@["bar"]`,
		},
		{
			name: "root_name_then_index",
			q:    NewPathQuery(true, Child(NameSelector("a")), Child(IndexSelector(0))),
			want: `$["a"][0]`,
		},
		{
			name: "descendant_name",
			q:    NewPathQuery(true, Descendant(NameSelector("x"))),
			want: `$..["x"]`,
		},
		{
			name: "wildcard",
			q:    NewPathQuery(true, Child(WildcardSelector())),
			want: `$[*]`,
		},
		{
			name: "multiple_selectors",
			q:    NewPathQuery(true, Child(NameSelector("a"), NameSelector("b"))),
			want: `$["a","b"]`,
		},
		{
			name: "slice_full",
			q: NewPathQuery(true, Child(SliceSelector(SliceArgs{
				Start: 1, End: 5, Step: 2,
				HasStart: true, HasEnd: true, HasStep: true,
			}))),
			want: `$[1:5:2]`,
		},
		{
			name: "slice_no_start",
			q: NewPathQuery(true, Child(SliceSelector(SliceArgs{
				End: 3, HasEnd: true,
			}))),
			want: `$[:3]`,
		},
		{
			name: "mixed_segments",
			q: NewPathQuery(true,
				Child(NameSelector("store")),
				Descendant(WildcardSelector()),
				Child(IndexSelector(0)),
			),
			want: `$["store"]..[*][0]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.q.String())
		})
	}
}

func TestPathQueryIsSingular(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		q        *PathQuery
		singular bool
	}{
		{
			name:     "empty_query",
			q:        NewPathQuery(true),
			singular: true,
		},
		{
			name:     "single_name",
			q:        NewPathQuery(true, Child(NameSelector("x"))),
			singular: true,
		},
		{
			name:     "single_index",
			q:        NewPathQuery(true, Child(IndexSelector(0))),
			singular: true,
		},
		{
			name:     "name_then_index",
			q:        NewPathQuery(true, Child(NameSelector("a")), Child(IndexSelector(0))),
			singular: true,
		},
		{
			name:     "descendant_not_singular",
			q:        NewPathQuery(true, Descendant(NameSelector("x"))),
			singular: false,
		},
		{
			name:     "wildcard_not_singular",
			q:        NewPathQuery(true, Child(WildcardSelector())),
			singular: false,
		},
		{
			name:     "slice_not_singular",
			q:        NewPathQuery(true, Child(SliceSelector(SliceArgs{HasStart: true, Start: 0}))),
			singular: false,
		},
		{
			name:     "filter_not_singular",
			q:        NewPathQuery(true, Child(FilterSelector(&FilterExpr{}))),
			singular: false,
		},
		{
			name:     "multiple_selectors_not_singular",
			q:        NewPathQuery(true, Child(NameSelector("a"), NameSelector("b"))),
			singular: false,
		},
		{
			name: "singular_then_non_singular",
			q: NewPathQuery(true,
				Child(NameSelector("a")),
				Child(WildcardSelector()),
			),
			singular: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.singular, tc.q.IsSingular())
		})
	}
}

func TestPathQuerySelect(t *testing.T) {
	t.Parallel()

	root := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"title": "A"},
				map[string]any{"title": "B"},
			},
		},
	}

	for _, tc := range []struct {
		name    string
		query   *PathQuery
		current any
		root    any
		want    []any
	}{
		{
			name:  "root_no_segments_returns_root",
			query: NewPathQuery(true),
			root:  root,
			want:  []any{root},
		},
		{
			name:  "root_single_child",
			query: NewPathQuery(true, Child(NameSelector("store"))),
			root:  root,
			want:  []any{root["store"]},
		},
		{
			name:  "root_multi_segment",
			query: NewPathQuery(true, Child(NameSelector("store")), Child(NameSelector("book")), Child(IndexSelector(0))),
			root:  root,
			want:  []any{map[string]any{"title": "A"}},
		},
		{
			name:    "relative_uses_current",
			query:   NewPathQuery(false, Child(NameSelector("title"))),
			current: map[string]any{"title": "hello"},
			root:    root,
			want:    []any{"hello"},
		},
		{
			name:    "relative_no_segments_returns_current",
			query:   NewPathQuery(false),
			current: float64(42),
			root:    root,
			want:    []any{float64(42)},
		},
		{
			name:  "root_with_wildcard",
			query: NewPathQuery(true, Child(NameSelector("store")), Child(NameSelector("book")), Child(WildcardSelector()), Child(NameSelector("title"))),
			root:  root,
			want:  []any{"A", "B"},
		},
		{
			name:  "root_miss_returns_empty",
			query: NewPathQuery(true, Child(NameSelector("nonexistent"))),
			root:  root,
			want:  []any{},
		},
		{
			name:  "descendant_segment_select",
			query: NewPathQuery(true, Descendant(NameSelector("title"))),
			root:  root,
			want:  []any{"A", "B"},
		},
		{
			name:  "descendant_segment_select_orders_object_keys",
			query: NewPathQuery(true, Descendant(NameSelector("value"))),
			root: map[string]any{
				"z": map[string]any{"value": "z"},
				"a": []any{
					map[string]any{"value": "a.0"},
				},
				"m": map[string]any{"value": "m"},
			},
			want: []any{"a.0", "m", "z"},
		},
		{
			name:  "relative_descendant_select_orders_object_keys",
			query: NewPathQuery(false, Descendant(NameSelector("value"))),
			current: map[string]any{
				"z": map[string]any{"value": "z"},
				"a": []any{
					map[string]any{"value": "a.0"},
				},
				"m": map[string]any{"value": "m"},
			},
			root: root,
			want: []any{"a.0", "m", "z"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.query.Select(tc.current, tc.root)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("PathQuery.Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
