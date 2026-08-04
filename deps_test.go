package jsonpath_test

import (
	"testing"

	"github.com/go-json-experiment/json"
	"github.com/stretchr/testify/require"
)

func TestDependencies(t *testing.T) {
	t.Parallel()

	// Verify go-json-experiment/json works
	var v any
	err := json.Unmarshal([]byte(`{"key":"value"}`), &v)
	require.NoError(t, err)

	m, ok := v.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "value", m["key"])
}
