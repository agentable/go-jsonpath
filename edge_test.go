package jsonpath

import (
	"io"
	"slices"
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
