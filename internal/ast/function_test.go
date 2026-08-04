package ast

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuncTypeString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		ft   FuncType
		want string
	}{
		{name: "logical", ft: Logical, want: "Logical"},
		{name: "value", ft: Value, want: "Value"},
		{name: "nodes", ft: Nodes, want: "Nodes"},
		{name: "unknown", ft: FuncType(99), want: "FuncType(99)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.ft.String())
		})
	}
}

// mockFunc implements Function for testing.
type mockFunc struct {
	name       string
	resultType FuncType
	params     []FuncType
	callFn     func([]FunctionValue) FunctionValue
}

func (f *mockFunc) Name() string                 { return f.name }
func (f *mockFunc) ResultType() FuncType         { return f.resultType }
func (f *mockFunc) ParameterCount() int          { return len(f.params) }
func (f *mockFunc) ParameterType(i int) FuncType { return f.params[i] }
func (f *mockFunc) Call(args []FunctionValue) FunctionValue {
	return f.callFn(args)
}

func TestFuncExpr(t *testing.T) {
	t.Parallel()

	fn := &mockFunc{
		name:       "testfn",
		resultType: Value,
		callFn:     func(args []FunctionValue) FunctionValue { return NewValue(42) },
	}

	t.Run("constructor", func(t *testing.T) {
		t.Parallel()
		fe := NewFuncExpr(fn, "arg1", 2)
		assert.Equal(t, "testfn", fe.Name())
		assert.Same(t, fn, fe.Func())
		require.Len(t, fe.Args(), 2)
		assert.Equal(t, "arg1", fe.Args()[0])
		assert.Equal(t, 2, fe.Args()[1])
	})

	t.Run("result_type", func(t *testing.T) {
		t.Parallel()
		fe := NewFuncExpr(fn)
		assert.Equal(t, Value, fe.ResultType())
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		fe := NewFuncExpr(fn, "a")
		assert.Equal(t, `testfn("a")`, fe.String())
	})

	t.Run("no_args", func(t *testing.T) {
		t.Parallel()
		fe := NewFuncExpr(fn)
		assert.Empty(t, fe.Args())
		assert.Equal(t, "testfn()", fe.String())
	})

	t.Run("call_with_path_query_args", func(t *testing.T) {
		t.Parallel()

		root := map[string]any{"items": []any{"a", "b"}}
		current := map[string]any{"name": "foo"}
		queryArg := NewPathQuery(false, Child(NameSelector("name")))
		filterArg := NewPathQuery(true, Child(NameSelector("items")), Child(WildcardSelector()))

		var got []FunctionValue
		fe := NewFuncExpr(&mockFunc{
			name:       "capture",
			resultType: Value,
			params:     []FuncType{Value, Nodes},
			callFn: func(args []FunctionValue) FunctionValue {
				got = append([]FunctionValue(nil), args...)
				return NoValue()
			},
		}, queryArg, filterArg)

		fe.Call(current, root)
		require.Len(t, got, 2)
		value, ok := got[0].(TypedValue)
		require.True(t, ok)
		require.False(t, value.IsNothing())
		assert.Equal(t, "foo", value.Any())
		nodes, ok := got[1].(TypedNodes)
		require.True(t, ok)
		if diff := cmp.Diff([]any{"a", "b"}, []any(nodes)); diff != "" {
			t.Errorf("Call() filter arg mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("call_with_missing_singular_query_returns_nil", func(t *testing.T) {
		t.Parallel()

		current := map[string]any{"name": "foo"}
		queryArg := NewPathQuery(false, Child(NameSelector("missing")))

		var got []FunctionValue
		fe := NewFuncExpr(&mockFunc{
			name:       "capture",
			resultType: Value,
			params:     []FuncType{Value},
			callFn: func(args []FunctionValue) FunctionValue {
				got = append([]FunctionValue(nil), args...)
				return NoValue()
			},
		}, queryArg)

		fe.Call(current, nil)
		require.Len(t, got, 1)
		value, ok := got[0].(TypedValue)
		require.True(t, ok)
		assert.True(t, value.IsNothing())
	})

	t.Run("call_with_non_singular_query_returns_nodes", func(t *testing.T) {
		t.Parallel()

		current := []any{"a", "b"}
		queryArg := NewPathQuery(false, Child(WildcardSelector()))

		var got []FunctionValue
		fe := NewFuncExpr(&mockFunc{
			name:       "capture",
			resultType: Value,
			params:     []FuncType{Nodes},
			callFn: func(args []FunctionValue) FunctionValue {
				got = append([]FunctionValue(nil), args...)
				return NoValue()
			},
		}, queryArg)

		fe.Call(current, nil)
		require.Len(t, got, 1)
		nodes, ok := got[0].(TypedNodes)
		require.True(t, ok)
		if diff := cmp.Diff([]any{"a", "b"}, []any(nodes)); diff != "" {
			t.Errorf("Call() query arg mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("call_evaluates_nested_function_compvalue_and_literal_args", func(t *testing.T) {
		t.Parallel()

		child := NewFuncExpr(&mockFunc{
			name:       "child",
			resultType: Value,
			callFn: func(args []FunctionValue) FunctionValue {
				return NewValue("child")
			},
		})

		var got []FunctionValue
		fe := NewFuncExpr(&mockFunc{
			name:       "capture",
			resultType: Value,
			params:     []FuncType{Value, Value, Value},
			callFn: func(args []FunctionValue) FunctionValue {
				got = append([]FunctionValue(nil), args...)
				return NoValue()
			},
		}, child, &LiteralValue{Val: 99}, "plain")

		fe.Call(nil, nil)
		require.Len(t, got, 3)
		first, ok := got[0].(TypedValue)
		require.True(t, ok)
		assert.Equal(t, "child", first.Any())
		second, ok := got[1].(TypedValue)
		require.True(t, ok)
		assert.Equal(t, 99, second.Any())
		third, ok := got[2].(TypedValue)
		require.True(t, ok)
		assert.Equal(t, "plain", third.Any())
	})
}
