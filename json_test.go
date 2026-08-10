package jsonpath

import (
	stdjson "encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestQueryJSON_InvalidPath(t *testing.T) {
	t.Parallel()

	got, err := QueryJSON([]byte(`{"a":1}`), Path{})
	require.ErrorIs(t, err, ErrInvalidPath)
	assert.Nil(t, got)
}

func TestQueryJSONLocated_InvalidPath(t *testing.T) {
	t.Parallel()

	got, err := QueryJSONLocated([]byte(`{"a":1}`), Path{})
	require.ErrorIs(t, err, ErrInvalidPath)
	assert.Nil(t, got)
}

func TestQueryJSONRead_InvalidPath(t *testing.T) {
	t.Parallel()

	got, err := QueryJSONRead(strings.NewReader(`{"a":1}`), Path{})
	require.ErrorIs(t, err, ErrInvalidPath)
	assert.Nil(t, got)
}

func TestQueryJSONReadLocated_InvalidPath(t *testing.T) {
	t.Parallel()

	got, err := QueryJSONReadLocated(strings.NewReader(`{"a":1}`), Path{})
	require.ErrorIs(t, err, ErrInvalidPath)
	assert.Nil(t, got)
}

func TestQueryJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		path    string
		want    []any
		wantErr bool
	}{
		{
			name: "simple name selector",
			json: `{"a": 1, "b": 2}`,
			path: "$.a",
			want: []any{stdjson.Number("1")},
		},
		{
			name: "array index selector",
			json: `[10, 20, 30]`,
			path: "$[1]",
			want: []any{stdjson.Number("20")},
		},
		{
			name: "nested path",
			json: `{"store": {"book": [{"title": "Book 1", "price": 8.95}]}}`,
			path: "$.store.book[0].title",
			want: []any{"Book 1"},
		},
		{
			name: "wildcard selector",
			json: `{"a": 1, "b": 2, "c": 3}`,
			path: "$[*]",
			want: []any{stdjson.Number("1"), stdjson.Number("2"), stdjson.Number("3")},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			path:    "$.a",
			wantErr: true,
		},
		{
			name: "empty result",
			json: `{"a": 1}`,
			path: "$.b",
			want: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse(tt.path)
			got, err := QueryJSON([]byte(tt.json), path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if diff := cmp.Diff(tt.want, []any(got)); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestQueryJSONLocated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		path    string
		want    LocatedNodeList
		wantErr bool
	}{
		{
			name: "simple name selector",
			json: `{"a": 1, "b": 2}`,
			path: "$.a",
			want: LocatedNodeList{
				{Value: stdjson.Number("1"), Path: mustNormalizedPath(NameElement("a"))},
			},
		},
		{
			name: "array index selector",
			json: `[10, 20, 30]`,
			path: "$[1]",
			want: LocatedNodeList{
				{Value: stdjson.Number("20"), Path: mustNormalizedPath(IndexElement(1))},
			},
		},
		{
			name: "nested path",
			json: `{"store": {"book": [{"title": "Book 1"}]}}`,
			path: "$.store.book[0].title",
			want: LocatedNodeList{
				{
					Value: "Book 1",
					Path: mustNormalizedPath(
						NameElement("store"),
						NameElement("book"),
						IndexElement(0),
						NameElement("title")),
				},
			},
		},
		{
			name: "multiple results",
			json: `{"store": {"book": [{"price": 8.95}, {"price": 12.99}]}}`,
			path: "$.store.book[*].price",
			want: LocatedNodeList{
				{
					Value: stdjson.Number("8.95"),
					Path: mustNormalizedPath(
						NameElement("store"),
						NameElement("book"),
						IndexElement(0),
						NameElement("price")),
				},
				{
					Value: stdjson.Number("12.99"),
					Path: mustNormalizedPath(
						NameElement("store"),
						NameElement("book"),
						IndexElement(1),
						NameElement("price")),
				},
			},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			path:    "$.a",
			wantErr: true,
		},
		{
			name: "empty result",
			json: `{"a": 1}`,
			path: "$.b",
			want: LocatedNodeList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse(tt.path)
			got, err := QueryJSONLocated([]byte(tt.json), path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestQueryJSON_PreservesNumberLexemes(t *testing.T) {
	t.Parallel()

	path := MustParse(`$[?@ == 0.1]`)
	got, err := QueryJSON([]byte(`[0.1, 0.10, 0.2]`), path)
	require.NoError(t, err)

	want := NodeList{stdjson.Number("0.1"), stdjson.Number("0.10")}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("QueryJSON() mismatch (-want +got):\n%s", diff)
	}
}

func TestQueryJSON_NumberBoundaries(t *testing.T) {
	t.Parallel()

	src := []byte(`[1, 1.10, 1e-2, 9007199254740992, 9007199254740993]`)
	all, err := QueryJSON(src, MustParse(`$[*]`))
	require.NoError(t, err)
	wantAll := NodeList{
		stdjson.Number("1"),
		stdjson.Number("1.10"),
		stdjson.Number("1e-2"),
		stdjson.Number("9007199254740992"),
		stdjson.Number("9007199254740993"),
	}
	if diff := cmp.Diff(wantAll, all); diff != "" {
		t.Errorf("QueryJSON() lexemes mismatch (-want +got):\n%s", diff)
	}

	path := MustParse(`$[?@ > 9007199254740992]`)
	got, err := QueryJSONLocated(src, path)
	require.NoError(t, err)
	want := LocatedNodeList{{
		Value: stdjson.Number("9007199254740993"),
		Path:  mustNormalizedPath(IndexElement(4)),
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("QueryJSONLocated() mismatch (-want +got):\n%s", diff)
	}
}

func TestQueryJSONRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		path    string
		want    []any
		wantErr bool
	}{
		{
			name: "simple name selector",
			json: `{"a": 1, "b": 2}`,
			path: "$.a",
			want: []any{stdjson.Number("1")},
		},
		{
			name: "array index selector",
			json: `[10, 20, 30]`,
			path: "$[1]",
			want: []any{stdjson.Number("20")},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			path:    "$.a",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse(tt.path)
			got, err := QueryJSONRead(strings.NewReader(tt.json), path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if diff := cmp.Diff(tt.want, []any(got)); diff != "" {
				t.Errorf("Select() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestQueryJSONReadLocated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		path    string
		want    LocatedNodeList
		wantErr bool
	}{
		{
			name: "simple name selector",
			json: `{"a": 1, "b": 2}`,
			path: "$.a",
			want: LocatedNodeList{
				{Value: stdjson.Number("1"), Path: mustNormalizedPath(NameElement("a"))},
			},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			path:    "$.a",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse(tt.path)
			got, err := QueryJSONReadLocated(strings.NewReader(tt.json), path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SelectLocated() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestQueryJSON_ComplexDocument(t *testing.T) {
	t.Parallel()

	jsonDoc := `{
		"store": {
			"book": [
				{
					"category": "reference",
					"author": "Nigel Rees",
					"title": "Sayings of the Century",
					"price": 8.95
				},
				{
					"category": "fiction",
					"author": "Evelyn Waugh",
					"title": "Sword of Honour",
					"price": 12.99
				},
				{
					"category": "fiction",
					"author": "Herman Melville",
					"title": "Moby Dick",
					"isbn": "0-553-21311-3",
					"price": 8.99
				}
			],
			"bicycle": {
				"color": "red",
				"price": 19.95
			}
		}
	}`

	t.Run("all book prices", func(t *testing.T) {
		t.Parallel()

		path := MustParse("$.store.book[*].price")
		got, err := QueryJSON([]byte(jsonDoc), path)
		require.NoError(t, err)
		if diff := cmp.Diff([]any{stdjson.Number("8.95"), stdjson.Number("12.99"), stdjson.Number("8.99")}, []any(got)); diff != "" {
			t.Errorf("QueryJSON() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("all authors", func(t *testing.T) {
		t.Parallel()

		path := MustParse("$.store.book[*].author")
		got, err := QueryJSON([]byte(jsonDoc), path)
		require.NoError(t, err)
		if diff := cmp.Diff([]any{"Nigel Rees", "Evelyn Waugh", "Herman Melville"}, []any(got)); diff != "" {
			t.Errorf("QueryJSON() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("first book", func(t *testing.T) {
		t.Parallel()

		path := MustParse("$.store.book[0]")
		got, err := QueryJSON([]byte(jsonDoc), path)
		require.NoError(t, err)
		require.Len(t, got, 1)
		book := got[0].(map[string]any)
		assert.Equal(t, "Sayings of the Century", book["title"])
		assert.Equal(t, stdjson.Number("8.95"), book["price"])
	})
}

func BenchmarkQueryJSON(b *testing.B) {
	jsonDoc := []byte(`{"store": {"book": [{"title": "Book 1", "price": 10}, {"title": "Book 2", "price": 20}]}}`)
	path := MustParse("$.store.book[*].price")

	b.ResetTimer()
	for b.Loop() {
		_, _ = QueryJSON(jsonDoc, path)
	}
}

func BenchmarkQueryJSONLocated(b *testing.B) {
	jsonDoc := []byte(`{"store": {"book": [{"title": "Book 1", "price": 10}, {"title": "Book 2", "price": 20}]}}`)
	path := MustParse("$.store.book[*].price")

	b.ResetTimer()
	for b.Loop() {
		_, _ = QueryJSONLocated(jsonDoc, path)
	}
}

func BenchmarkQueryJSONRead(b *testing.B) {
	jsonDoc := `{"store": {"book": [{"title": "Book 1", "price": 10}, {"title": "Book 2", "price": 20}]}}`
	path := MustParse("$.store.book[*].price")

	b.ResetTimer()
	for b.Loop() {
		_, _ = QueryJSONRead(strings.NewReader(jsonDoc), path)
	}
}

func BenchmarkQueryJSONReadLocated(b *testing.B) {
	jsonDoc := `{"store": {"book": [{"title": "Book 1", "price": 10}, {"title": "Book 2", "price": 20}]}}`
	path := MustParse("$.store.book[*].price")

	b.ResetTimer()
	for b.Loop() {
		_, _ = QueryJSONReadLocated(strings.NewReader(jsonDoc), path)
	}
}

func TestQueryJSON_ErrUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
	}{
		{
			name: "invalid json syntax",
			json: `{invalid}`,
		},
		{
			name: "unclosed object",
			json: `{"a": 1`,
		},
		{
			name: "unclosed array",
			json: `[1, 2, 3`,
		},
		{
			name: "trailing comma",
			json: `{"a": 1,}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse("$.a")
			_, err := QueryJSON([]byte(tt.json), path)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnmarshal)
		})
	}
}

func TestQueryJSONLocated_ErrUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
	}{
		{
			name: "invalid json syntax",
			json: `{invalid}`,
		},
		{
			name: "unclosed object",
			json: `{"a": 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := MustParse("$.a")
			_, err := QueryJSONLocated([]byte(tt.json), path)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnmarshal)
		})
	}
}

func TestQueryJSONRead_ErrUnmarshal(t *testing.T) {
	t.Parallel()

	path := MustParse("$.a")
	_, err := QueryJSONRead(strings.NewReader(`{"a": 1`), path)

	require.ErrorIs(t, err, ErrUnmarshal)
}

func TestQueryJSONReadLocated_ErrUnmarshal(t *testing.T) {
	t.Parallel()

	path := MustParse("$.a")
	_, err := QueryJSONReadLocated(strings.NewReader(`{"a": 1`), path)

	require.ErrorIs(t, err, ErrUnmarshal)
}

func TestQueryJSONRead_Errors(t *testing.T) {
	t.Parallel()

	path := MustParse("$")

	t.Run("read error keeps unmarshal sentinel", func(t *testing.T) {
		t.Parallel()

		got, err := QueryJSONRead(errReader{}, path)
		require.ErrorIs(t, err, ErrUnmarshal)
		assert.Nil(t, got)
	})

	t.Run("nil reader keeps unmarshal sentinel", func(t *testing.T) {
		t.Parallel()

		got, err := QueryJSONRead(nil, path)
		require.ErrorIs(t, err, ErrUnmarshal)
		assert.Nil(t, got)
	})

	t.Run("located nil reader keeps unmarshal sentinel", func(t *testing.T) {
		t.Parallel()

		got, err := QueryJSONReadLocated(nil, path)
		require.ErrorIs(t, err, ErrUnmarshal)
		assert.Nil(t, got)
	})
}

func TestQueryJSONRead_DoesNotConsumeReaderOnInvalidPath(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(`{"kept":true}`)
	got, err := QueryJSONRead(reader, Path{})
	require.ErrorIs(t, err, ErrInvalidPath)
	assert.Nil(t, got)

	remaining, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, `{"kept":true}`, string(remaining))
}
