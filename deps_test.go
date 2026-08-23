package jsonpath_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDependencies(t *testing.T) {
	t.Parallel()

	// Verify the Go 1.27 standard-library JSON v2 package works.
	var v any
	err := json.Unmarshal([]byte(`{"key":"value"}`), &v)
	require.NoError(t, err)

	m, ok := v.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "value", m["key"])
}
