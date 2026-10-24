package jsonpath_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agentable/go-jsonpath"
)

func TestFunctionValuePublicResultType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value jsonpath.FunctionValue
		kind  jsonpath.FuncType
		text  string
	}{
		{"null", jsonpath.NewValue(nil), jsonpath.FuncValue, "Value"},
		{"nothing", jsonpath.NoValue(), jsonpath.FuncValue, "Value"},
		{"false", jsonpath.Logical(false), jsonpath.FuncLogical, "Logical"},
		{"empty nodes", jsonpath.Nodes(nil), jsonpath.FuncNodes, "Nodes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var kind jsonpath.FuncType = tt.value.ResultType() //nolint:staticcheck // Verify assignment to the public type without conversion.
			assert.Equal(t, tt.kind, kind)
			if tt.value.ResultType() != tt.kind {
				t.Errorf("ResultType() = %v, want %v", tt.value.ResultType(), tt.kind)
			}
			assert.Equal(t, tt.text, fmt.Sprint(kind))
		})
	}
	assert.False(t, jsonpath.NewValue(nil).IsNothing())
	assert.True(t, jsonpath.NoValue().IsNothing())
	assert.Equal(t, "FuncType(255)", fmt.Sprint(jsonpath.FuncType(255)))
}
