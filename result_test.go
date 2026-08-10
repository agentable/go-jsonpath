package jsonpath

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeList_All(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, slices.Collect(NodeList(nil).All()))
	})
	t.Run("full_iteration", func(t *testing.T) {
		t.Parallel()
		got := slices.Collect(NodeList{1, "two", 3.0}.All())
		if diff := cmp.Diff([]any{1, "two", 3.0}, got); diff != "" {
			t.Errorf("NodeList.All() mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("early_break", func(t *testing.T) {
		t.Parallel()
		var got []any
		for v := range (NodeList{1, 2, 3, 4, 5}).All() {
			got = append(got, v)
			if len(got) == 2 {
				break
			}
		}
		assert.Equal(t, []any{1, 2}, got)
	})
}

func TestLocatedNodeList_All(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, slices.Collect(LocatedNodeList(nil).All()))
	})

	t.Run("full_iteration", func(t *testing.T) {
		t.Parallel()
		list := LocatedNodeList{
			{Value: 1, Path: mustNormalizedPath(NameElement("a"))},
			{Value: 2, Path: mustNormalizedPath(NameElement("b"))},
		}
		got := slices.Collect(list.All())
		if diff := cmp.Diff([]LocatedNode(list), got); diff != "" {
			t.Errorf("LocatedNodeList.All() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("early_break", func(t *testing.T) {
		t.Parallel()
		list := LocatedNodeList{{Value: 1}, {Value: 2}, {Value: 3}}
		var got LocatedNodeList
		for node := range list.All() {
			got = append(got, node)
			break
		}
		require.Len(t, got, 1)
		assert.Equal(t, 1, got[0].Value)
	})
}

func TestLocatedNodeList_Values(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, slices.Collect(LocatedNodeList(nil).Values()))
	})

	t.Run("full_iteration", func(t *testing.T) {
		t.Parallel()
		list := LocatedNodeList{{Value: "a"}, {Value: "b"}}
		assert.Equal(t, []any{"a", "b"}, slices.Collect(list.Values()))
	})

	t.Run("early_break", func(t *testing.T) {
		t.Parallel()
		var got []any
		for value := range (LocatedNodeList{{Value: 1}, {Value: 2}, {Value: 3}}).Values() {
			got = append(got, value)
			if len(got) == 2 {
				break
			}
		}
		assert.Equal(t, []any{1, 2}, got)
	})
}

func TestLocatedNodeList_Paths(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, slices.Collect(LocatedNodeList(nil).Paths()))
	})

	t.Run("full_iteration", func(t *testing.T) {
		t.Parallel()
		list := LocatedNodeList{
			{Path: mustNormalizedPath(NameElement("x"))},
			{Path: mustNormalizedPath(NameElement("y"))},
		}
		got := slices.Collect(list.Paths())
		require.Len(t, got, 2)
		assert.Equal(t, "$['x']", got[0].String())
		assert.Equal(t, "$['y']", got[1].String())
	})

	t.Run("early_break", func(t *testing.T) {
		t.Parallel()
		list := LocatedNodeList{
			{Path: mustNormalizedPath(NameElement("a"))},
			{Path: mustNormalizedPath(NameElement("b"))},
		}
		var got []NormalizedPath
		for path := range list.Paths() {
			got = append(got, path)
			break
		}
		require.Len(t, got, 1)
		assert.Equal(t, "$['a']", got[0].String())
	})
}

func TestLocatedNodeList_Deduplicate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		list LocatedNodeList
		exp  LocatedNodeList
	}{
		{name: "empty", list: LocatedNodeList{}, exp: LocatedNodeList{}},
		{
			name: "single",
			list: LocatedNodeList{{Value: "a", Path: mustNormalizedPath(NameElement("x"))}},
			exp:  LocatedNodeList{{Value: "a", Path: mustNormalizedPath(NameElement("x"))}},
		},
		{
			name: "no_duplicates",
			list: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
				{Value: "c", Path: mustNormalizedPath(IndexElement(0))},
			},
			exp: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
				{Value: "c", Path: mustNormalizedPath(IndexElement(0))},
			},
		},
		{
			name: "duplicates_same_value",
			list: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
			},
			exp: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
			},
		},
		{
			name: "duplicates_diff_value",
			list: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "different", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
			},
			exp: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
			},
		},
		{
			name: "multiple_duplicates",
			list: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
				{Value: "c", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "d", Path: mustNormalizedPath(NameElement("z"))},
				{Value: "e", Path: mustNormalizedPath(NameElement("y"))},
			},
			exp: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("y"))},
				{Value: "d", Path: mustNormalizedPath(NameElement("z"))},
			},
		},
		{
			name: "nested_paths",
			list: LocatedNodeList{
				{Value: 1, Path: mustNormalizedPath(NameElement("a"), IndexElement(0))},
				{Value: 2, Path: mustNormalizedPath(NameElement("a"), IndexElement(1))},
				{Value: 3, Path: mustNormalizedPath(NameElement("a"), IndexElement(0))},
			},
			exp: LocatedNodeList{
				{Value: 1, Path: mustNormalizedPath(NameElement("a"), IndexElement(0))},
				{Value: 2, Path: mustNormalizedPath(NameElement("a"), IndexElement(1))},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.list.Deduplicate()
			if diff := cmp.Diff(tc.exp, got); diff != "" {
				t.Errorf("LocatedNodeList.Deduplicate() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLocatedNodeList_DeduplicateWithHashCollisions(t *testing.T) {
	t.Parallel()

	list := LocatedNodeList{
		{Value: "first", Path: mustNormalizedPath(NameElement("x"))},
		{Value: "second", Path: mustNormalizedPath(NameElement("y"))},
		{Value: "duplicate first", Path: mustNormalizedPath(NameElement("x"))},
		{Value: "third", Path: mustNormalizedPath(NameElement("z"))},
		{Value: "duplicate second", Path: mustNormalizedPath(NameElement("y"))},
	}

	got := list.deduplicateWithHasher(func(NormalizedPath) uint64 { return 1 })
	want := LocatedNodeList{
		{Value: "first", Path: mustNormalizedPath(NameElement("x"))},
		{Value: "second", Path: mustNormalizedPath(NameElement("y"))},
		{Value: "third", Path: mustNormalizedPath(NameElement("z"))},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("LocatedNodeList.Deduplicate() mismatch (-want +got):\n%s", diff)
	}
}

func TestLocatedNodeList_DeduplicateClearsTrimmedSlots(t *testing.T) {
	t.Parallel()

	list := LocatedNodeList{
		{Value: "first", Path: mustNormalizedPath(NameElement("x"))},
		{Value: "ignored value", Path: mustNormalizedPath(NameElement("x"))},
		{Value: "second", Path: mustNormalizedPath(NameElement("y"))},
		{Value: "ignored again", Path: mustNormalizedPath(NameElement("x"))},
	}

	got := list.Deduplicate()
	want := LocatedNodeList{
		{Value: "first", Path: mustNormalizedPath(NameElement("x"))},
		{Value: "second", Path: mustNormalizedPath(NameElement("y"))},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("LocatedNodeList.Deduplicate() mismatch (-want +got):\n%s", diff)
	}
	for i, node := range list[len(got):] {
		assert.Zero(t, node, "trimmed slot %d should be cleared", i)
	}
}

func BenchmarkLocatedNodeList_Deduplicate(b *testing.B) {
	makeList := func(size int, dupEvery int) LocatedNodeList {
		list := make(LocatedNodeList, 0, size)
		for i := range size {
			idx := i
			if dupEvery > 0 {
				idx = i % dupEvery
			}
			list = append(list, LocatedNode{
				Value: i,
				Path: mustNormalizedPath(
					NameElement("users"),
					IndexElement(idx),
					NameElement("name")),
			})
		}
		return list
	}

	for _, tc := range []struct {
		name     string
		size     int
		dupEvery int
		hash     func(NormalizedPath) uint64
	}{
		{name: "small", size: 32, dupEvery: 8},
		{name: "large", size: 1024, dupEvery: 128},
		{name: "high_collision", size: 1024, hash: func(NormalizedPath) uint64 { return 1 }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			src := makeList(tc.size, tc.dupEvery)
			b.ReportAllocs()
			for b.Loop() {
				list := slices.Clone(src)
				if tc.hash == nil {
					_ = list.Deduplicate()
				} else {
					_ = list.deduplicateWithHasher(tc.hash)
				}
			}
		})
	}
}

func TestLocatedNodeList_Sort(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		list LocatedNodeList
		exp  LocatedNodeList
	}{
		{name: "empty", list: LocatedNodeList{}, exp: LocatedNodeList{}},
		{
			name: "single",
			list: LocatedNodeList{{Value: "a", Path: mustNormalizedPath(NameElement("x"))}},
			exp:  LocatedNodeList{{Value: "a", Path: mustNormalizedPath(NameElement("x"))}},
		},
		{
			name: "already_sorted",
			list: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("a"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("b"))},
				{Value: "c", Path: mustNormalizedPath(NameElement("c"))},
			},
			exp: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("a"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("b"))},
				{Value: "c", Path: mustNormalizedPath(NameElement("c"))},
			},
		},
		{
			name: "reverse_order",
			list: LocatedNodeList{
				{Value: "c", Path: mustNormalizedPath(NameElement("c"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("b"))},
				{Value: "a", Path: mustNormalizedPath(NameElement("a"))},
			},
			exp: LocatedNodeList{
				{Value: "a", Path: mustNormalizedPath(NameElement("a"))},
				{Value: "b", Path: mustNormalizedPath(NameElement("b"))},
				{Value: "c", Path: mustNormalizedPath(NameElement("c"))},
			},
		},
		{
			name: "indexes_before_names",
			list: LocatedNodeList{
				{Value: "name", Path: mustNormalizedPath(NameElement("x"))},
				{Value: "index", Path: mustNormalizedPath(IndexElement(0))},
			},
			exp: LocatedNodeList{
				{Value: "index", Path: mustNormalizedPath(IndexElement(0))},
				{Value: "name", Path: mustNormalizedPath(NameElement("x"))},
			},
		},
		{
			name: "mixed_indexes_and_names",
			list: LocatedNodeList{
				{Value: "n2", Path: mustNormalizedPath(NameElement("z"))},
				{Value: "i2", Path: mustNormalizedPath(IndexElement(5))},
				{Value: "n1", Path: mustNormalizedPath(NameElement("a"))},
				{Value: "i1", Path: mustNormalizedPath(IndexElement(0))},
			},
			exp: LocatedNodeList{
				{Value: "i1", Path: mustNormalizedPath(IndexElement(0))},
				{Value: "i2", Path: mustNormalizedPath(IndexElement(5))},
				{Value: "n1", Path: mustNormalizedPath(NameElement("a"))},
				{Value: "n2", Path: mustNormalizedPath(NameElement("z"))},
			},
		},
		{
			name: "nested_paths",
			list: LocatedNodeList{
				{Value: 3, Path: mustNormalizedPath(NameElement("b"), IndexElement(0))},
				{Value: 1, Path: mustNormalizedPath(NameElement("a"), IndexElement(0))},
				{Value: 4, Path: mustNormalizedPath(NameElement("b"), IndexElement(1))},
				{Value: 2, Path: mustNormalizedPath(NameElement("a"), IndexElement(1))},
			},
			exp: LocatedNodeList{
				{Value: 1, Path: mustNormalizedPath(NameElement("a"), IndexElement(0))},
				{Value: 2, Path: mustNormalizedPath(NameElement("a"), IndexElement(1))},
				{Value: 3, Path: mustNormalizedPath(NameElement("b"), IndexElement(0))},
				{Value: 4, Path: mustNormalizedPath(NameElement("b"), IndexElement(1))},
			},
		},
		{
			name: "different_lengths",
			list: LocatedNodeList{
				{Value: "long", Path: mustNormalizedPath(NameElement("a"), NameElement("b"), IndexElement(0))},
				{Value: "short", Path: mustNormalizedPath(NameElement("a"))},
				{Value: "medium", Path: mustNormalizedPath(NameElement("a"), NameElement("b"))},
			},
			exp: LocatedNodeList{
				{Value: "short", Path: mustNormalizedPath(NameElement("a"))},
				{Value: "medium", Path: mustNormalizedPath(NameElement("a"), NameElement("b"))},
				{Value: "long", Path: mustNormalizedPath(NameElement("a"), NameElement("b"), IndexElement(0))},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			list := slices.Clone(tc.list)
			list.Sort()
			if diff := cmp.Diff(tc.exp, list); diff != "" {
				t.Errorf("LocatedNodeList.Sort() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
