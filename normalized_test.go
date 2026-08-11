package jsonpath

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustNormalizedPath(elements ...PathElement) NormalizedPath {
	path, err := NewNormalizedPath(elements...)
	if err != nil {
		panic(err)
	}
	return path
}

func TestNameElement_Normalized(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		elem NameElement
		norm string
		ptr  string
	}{
		{
			name: "simple",
			elem: NameElement("a"),
			norm: `['a']`,
			ptr:  `a`,
		},
		{
			name: "escape_apostrophes",
			elem: NameElement("'hi'"),
			norm: `['\'hi\'']`,
			ptr:  "'hi'",
		},
		{
			name: "escape_special",
			elem: NameElement("'\b\f\n\r\t\\'"),
			norm: `['\'\b\f\n\r\t\\\'']`,
			ptr:  "'\b\f\n\r\t\\'",
		},
		{
			name: "escape_vertical_tab",
			elem: NameElement("\u000B"),
			norm: `['\u000b']`,
			ptr:  "\u000B",
		},
		{
			name: "escape_null",
			elem: NameElement("\u0000"),
			norm: `['\u0000']`,
			ptr:  "\u0000",
		},
		{
			name: "escape_control_chars",
			elem: NameElement("\u0001\u0002\u0003\u0004\u0005\u0006\u0007\u000e\u000F"),
			norm: `['\u0001\u0002\u0003\u0004\u0005\u0006\u0007\u000e\u000f']`,
			ptr:  "\u0001\u0002\u0003\u0004\u0005\u0006\u0007\u000e\u000F",
		},
		{
			name: "escape_upper_control_chars",
			elem: NameElement("\u0010\u001f"),
			norm: `['\u0010\u001f']`,
			ptr:  "\u0010\u001f",
		},
		{
			name: "escape_pointer_chars",
			elem: NameElement("this / ~that"),
			norm: `['this / ~that']`,
			ptr:  "this ~1 ~0that",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := assert.New(t)

			p := mustNormalizedPath(tc.elem)
			// String includes the $ prefix, so strip it for element check.
			got := p.String()
			a.Equal("$"+tc.norm, got)

			// Check pointer output for single element.
			a.Equal("/"+tc.ptr, p.Pointer())
		})
	}
}

func TestNormalizedPathRejectsInvalidUTF8Name(t *testing.T) {
	t.Parallel()

	invalidByte := NameElement("\xff")
	truncated := NameElement("\xe2\x82")
	mixed := NameElement("valid\xf0\x28\x8c\x28")
	validReplacement := NameElement("\uFFFD")

	tests := []struct {
		name    string
		build   func() error
		wantErr bool
	}{
		{
			name: "constructor value",
			build: func() error {
				_, err := NewNormalizedPath(invalidByte)
				return err
			},
			wantErr: true,
		},
		{
			name: "constructor pointer",
			build: func() error {
				_, err := NewNormalizedPath(&truncated)
				return err
			},
			wantErr: true,
		},
		{
			name: "append value",
			build: func() error {
				_, err := (NormalizedPath{}).Append(mixed)
				return err
			},
			wantErr: true,
		},
		{
			name: "append pointer",
			build: func() error {
				_, err := (NormalizedPath{}).Append(&invalidByte)
				return err
			},
			wantErr: true,
		},
		{
			name: "valid replacement rune",
			build: func() error {
				_, err := NewNormalizedPath(validReplacement)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.build()
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidPath)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNormalizedPathPreservesReplacementRune(t *testing.T) {
	t.Parallel()

	name := NameElement("\uFFFD")
	path, err := NewNormalizedPath(name)
	require.NoError(t, err)
	assert.Equal(t, "$['\uFFFD']", path.String())
	assert.Equal(t, "/\uFFFD", path.Pointer())
	assert.Equal(t, []PathElement{name}, path.Elements())
}

func TestIndexElement_Normalized(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		elem IndexElement
		norm string
		ptr  string
	}{
		{
			name: "zero",
			elem: IndexElement(0),
			norm: "[0]",
			ptr:  "0",
		},
		{
			name: "positive",
			elem: IndexElement(42),
			norm: "[42]",
			ptr:  "42",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := assert.New(t)

			p := mustNormalizedPath(tc.elem)
			a.Equal("$"+tc.norm, p.String())
			a.Equal("/"+tc.ptr, p.Pointer())
		})
	}
}

func TestNormalizedPath_String(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		path NormalizedPath
		str  string
		ptr  string
	}{
		{
			name: "empty",
			path: mustNormalizedPath(),
			str:  "$",
			ptr:  "",
		},
		{
			name: "single_name",
			path: mustNormalizedPath(NameElement("a")),
			str:  "$['a']",
			ptr:  "/a",
		},
		{
			name: "single_index",
			path: mustNormalizedPath(IndexElement(1)),
			str:  "$[1]",
			ptr:  "/1",
		},
		{
			name: "nested",
			path: mustNormalizedPath(NameElement("a"), NameElement("b"), IndexElement(1)),
			str:  "$['a']['b'][1]",
			ptr:  "/a/b/1",
		},
		{
			name: "unicode_escape",
			path: mustNormalizedPath(NameElement("\u000B")),
			str:  `$['\u000b']`,
			ptr:  "/\u000b",
		},
		{
			name: "unicode_printable",
			path: mustNormalizedPath(NameElement("\u0061")),
			str:  "$['a']",
			ptr:  "/a",
		},
		{
			name: "pointer_escapes",
			path: mustNormalizedPath(NameElement("a~x"), NameElement("b/2"), IndexElement(1)),
			str:  "$['a~x']['b/2'][1]",
			ptr:  "/a~0x/b~12/1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := assert.New(t)
			a.Equal(tc.str, tc.path.String())
			a.Equal(tc.ptr, tc.path.Pointer())
		})
	}
}

func TestNormalizedPath_Compare(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		p1   NormalizedPath
		p2   NormalizedPath
		exp  int
	}{
		{name: "empty", exp: 0},
		{
			name: "same_name",
			p1:   mustNormalizedPath(NameElement("a")),
			p2:   mustNormalizedPath(NameElement("a")),
			exp:  0,
		},
		{
			name: "diff_names",
			p1:   mustNormalizedPath(NameElement("a")),
			p2:   mustNormalizedPath(NameElement("b")),
			exp:  -1,
		},
		{
			name: "diff_names_rev",
			p1:   mustNormalizedPath(NameElement("b")),
			p2:   mustNormalizedPath(NameElement("a")),
			exp:  1,
		},
		{
			name: "longer_first",
			p1:   mustNormalizedPath(NameElement("a"), NameElement("b")),
			p2:   mustNormalizedPath(NameElement("a")),
			exp:  1,
		},
		{
			name: "shorter_first",
			p1:   mustNormalizedPath(NameElement("a")),
			p2:   mustNormalizedPath(NameElement("a"), NameElement("b")),
			exp:  -1,
		},
		{
			name: "name_vs_index",
			p1:   mustNormalizedPath(NameElement("a")),
			p2:   mustNormalizedPath(IndexElement(0)),
			exp:  1,
		},
		{
			name: "index_vs_name",
			p1:   mustNormalizedPath(IndexElement(0)),
			p2:   mustNormalizedPath(NameElement("a")),
			exp:  -1,
		},
		{
			name: "same_index",
			p1:   mustNormalizedPath(IndexElement(42)),
			p2:   mustNormalizedPath(IndexElement(42)),
			exp:  0,
		},
		{
			name: "diff_indexes",
			p1:   mustNormalizedPath(IndexElement(42)),
			p2:   mustNormalizedPath(IndexElement(99)),
			exp:  -1,
		},
		{
			name: "diff_indexes_rev",
			p1:   mustNormalizedPath(IndexElement(99)),
			p2:   mustNormalizedPath(IndexElement(42)),
			exp:  1,
		},
		{
			name: "nested_type_diff",
			p1:   mustNormalizedPath(NameElement("a"), IndexElement(1024)),
			p2:   mustNormalizedPath(NameElement("a"), NameElement("b")),
			exp:  -1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.exp, tc.p1.Compare(tc.p2))
		})
	}
}

func TestNormalizedPath_Equal(t *testing.T) {
	t.Parallel()

	path := mustNormalizedPath(NameElement("store"), IndexElement(0), NameElement("title"))

	assert.True(t, path.Equal(mustNormalizedPath(NameElement("store"), IndexElement(0), NameElement("title"))))
	assert.False(t, path.Equal(mustNormalizedPath(NameElement("store"), IndexElement(0), NameElement("name"))))
	assert.False(t, path.Equal(mustNormalizedPath(NameElement("store"), IndexElement(1), NameElement("title"))))
	assert.False(t, path.Equal(mustNormalizedPath(NameElement("store"), IndexElement(0))))
}

func TestNormalizedPath_MarshalText(t *testing.T) {
	t.Parallel()

	p := mustNormalizedPath(NameElement("a"), IndexElement(0))
	text, err := p.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, "$['a'][0]", string(text))
}

func TestNormalizedPathConstructionRejectsInvalidElements(t *testing.T) {
	t.Parallel()

	root, err := NewNormalizedPath()
	require.NoError(t, err)

	_, err = NewNormalizedPath(nil)
	require.ErrorIs(t, err, ErrInvalidPath)
	_, err = root.Append(nil)
	require.ErrorIs(t, err, ErrInvalidPath)

	var nilName *NameElement
	var nilIndex *IndexElement
	for _, elem := range []PathElement{nilName, nilIndex} {
		_, err = NewNormalizedPath(elem)
		require.ErrorIs(t, err, ErrInvalidPath)
		_, err = root.Append(elem)
		require.ErrorIs(t, err, ErrInvalidPath)
	}

	_, err = NewNormalizedPath(IndexElement(-1))
	require.ErrorIs(t, err, ErrInvalidPath)
	_, err = root.Append(IndexElement(-1))
	require.ErrorIs(t, err, ErrInvalidPath)
}

func TestNormalizedPath_ImmutableBoundaries(t *testing.T) {
	t.Parallel()

	elements := []PathElement{NameElement("a")}
	path := mustNormalizedPath(elements...)
	elements[0] = NameElement("mutated")
	assert.Equal(t, "$['a']", path.String())

	got := path.Elements()
	require.Len(t, got, 1)
	got[0] = NameElement("changed")
	assert.Equal(t, "$['a']", path.String())

	next, err := path.Append(IndexElement(0))
	require.NoError(t, err)
	assert.Equal(t, "$['a']", path.String())
	assert.Equal(t, "$['a'][0]", next.String())
	assert.Equal(t, 1, path.Len())
	assert.Equal(t, 2, next.Len())
	assert.Equal(t, NameElement("a"), next.Element(0))
	assert.Equal(t, IndexElement(0), next.Element(1))
}

func TestNormalizedPath_ElementChecked(t *testing.T) {
	t.Parallel()

	path := mustNormalizedPath(NameElement("store"), IndexElement(0))
	elem, err := path.ElementChecked(1)
	require.NoError(t, err)
	assert.Equal(t, IndexElement(0), elem)

	for _, index := range []int{-1, path.Len()} {
		elem, err = path.ElementChecked(index)
		assert.Nil(t, elem)
		require.ErrorIs(t, err, ErrIndexOutOfBounds)
		assert.ErrorContains(t, err, fmt.Sprintf("index %d", index))
	}
}

func TestNormalizedPath_ElementPanicsWithCheckedCause(t *testing.T) {
	path := mustNormalizedPath(NameElement("store"), IndexElement(0))

	for _, index := range []int{-1, path.Len()} {
		t.Run(fmt.Sprintf("index_%d", index), func(t *testing.T) {
			var recovered any
			func() {
				defer func() {
					recovered = recover()
				}()
				path.Element(index)
			}()

			panicErr, ok := recovered.(error)
			require.True(t, ok, "panic value = %#v, want error", recovered)
			require.ErrorIs(t, panicErr, ErrIndexOutOfBounds)
			assert.ErrorContains(t, panicErr, "jsonpath: NormalizedPath.Element")
		})
	}
}

func BenchmarkNewNormalizedPath(b *testing.B) {
	elements := []PathElement{
		NameElement("store"),
		NameElement("book"),
		IndexElement(12),
		NameElement("title"),
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewNormalizedPath(elements...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormalizedPathString(b *testing.B) {
	paths := map[string]NormalizedPath{
		"simple": mustNormalizedPath(
			NameElement("store"),
			NameElement("book"),
			IndexElement(12),
			NameElement("title"),
		),
		"escaped": mustNormalizedPath(
			NameElement("users"),
			IndexElement(2048),
			NameElement("line\nbreak"),
			NameElement("quote'and\\slash"),
		),
	}

	for name, path := range paths {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = path.String()
			}
		})
	}
}

func BenchmarkNormalizedPathPointer(b *testing.B) {
	path := mustNormalizedPath(
		NameElement("users"),
		IndexElement(2048),
		NameElement("nested/value"),
		NameElement("~meta"),
	)

	b.ReportAllocs()
	for b.Loop() {
		_ = path.Pointer()
	}
}

func BenchmarkNormalizedPathHash(b *testing.B) {
	paths := map[string]NormalizedPath{
		"simple": mustNormalizedPath(
			NameElement("users"),
			IndexElement(12),
			NameElement("name"),
		),
		"deep": mustNormalizedPath(
			NameElement("store"),
			NameElement("book"),
			IndexElement(2048),
			NameElement("metadata"),
			NameElement("author"),
		),
	}

	for name, path := range paths {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = path.hash()
			}
		})
	}
}
