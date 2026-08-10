package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

// literalTrue / literalFalse are BasicExpr that always return a fixed bool.
type literalBoolExpr bool

func (b literalBoolExpr) Eval(_, _ any) bool { return bool(b) }

func trueExpr() BasicExpr  { return literalBoolExpr(true) }
func falseExpr() BasicExpr { return literalBoolExpr(false) }

// logicalMockFunc implements Function for testing filter logic.
type logicalMockFunc struct {
	name   string
	result any
}

func (f *logicalMockFunc) Name() string               { return f.name }
func (f *logicalMockFunc) ResultType() FuncType       { return Logical }
func (f *logicalMockFunc) ParameterCount() int        { return 0 }
func (f *logicalMockFunc) ParameterType(int) FuncType { return Logical }
func (f *logicalMockFunc) Call(_ []FunctionValue) FunctionValue {
	result, _ := f.result.(bool)
	return TypedLogical(result)
}

// valueMockFunc implements Function returning a Value type.
type valueMockFunc struct {
	name   string
	result any
}

func (f *valueMockFunc) Name() string               { return f.name }
func (f *valueMockFunc) ResultType() FuncType       { return Value }
func (f *valueMockFunc) ParameterCount() int        { return 0 }
func (f *valueMockFunc) ParameterType(int) FuncType { return Value }
func (f *valueMockFunc) Call(_ []FunctionValue) FunctionValue {
	return NewValue(f.result)
}

// --- Tests ---

func TestFilterExprEval(t *testing.T) {
	t.Parallel()

	t.Run("delegates_to_or", func(t *testing.T) {
		t.Parallel()
		f := &FilterExpr{Or: LogicalOr{LogicalAnd{trueExpr()}}}
		assert.True(t, f.Eval(nil, nil))

		f2 := &FilterExpr{Or: LogicalOr{LogicalAnd{falseExpr()}}}
		assert.False(t, f2.Eval(nil, nil))
	})
}

func TestLogicalOrEval(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		or   LogicalOr
		want bool
	}{
		{name: "empty", or: LogicalOr{}, want: false},
		{name: "single_true", or: LogicalOr{LogicalAnd{trueExpr()}}, want: true},
		{name: "single_false", or: LogicalOr{LogicalAnd{falseExpr()}}, want: false},
		{name: "first_true_short_circuits", or: LogicalOr{
			LogicalAnd{trueExpr()},
			LogicalAnd{falseExpr()},
		}, want: true},
		{name: "second_true", or: LogicalOr{
			LogicalAnd{falseExpr()},
			LogicalAnd{trueExpr()},
		}, want: true},
		{name: "all_false", or: LogicalOr{
			LogicalAnd{falseExpr()},
			LogicalAnd{falseExpr()},
		}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.or.Eval(nil, nil))
		})
	}
}

func TestLogicalAndEval(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		and  LogicalAnd
		want bool
	}{
		{name: "empty", and: LogicalAnd{}, want: true},
		{name: "single_true", and: LogicalAnd{trueExpr()}, want: true},
		{name: "single_false", and: LogicalAnd{falseExpr()}, want: false},
		{name: "all_true", and: LogicalAnd{trueExpr(), trueExpr()}, want: true},
		{name: "first_false_short_circuits", and: LogicalAnd{falseExpr(), trueExpr()}, want: false},
		{name: "last_false", and: LogicalAnd{trueExpr(), falseExpr()}, want: false},
		{name: "all_false", and: LogicalAnd{falseExpr(), falseExpr()}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.and.Eval(nil, nil))
		})
	}
}

func TestExistExprEval(t *testing.T) {
	t.Parallel()

	t.Run("bare_query_no_segments_always_true", func(t *testing.T) {
		t.Parallel()
		// @ with no segments
		e := &ExistExpr{Query: NewPathQuery(false)}
		assert.True(t, e.Eval(nil, nil))
		// $ with no segments
		e2 := &ExistExpr{Query: NewPathQuery(true)}
		assert.True(t, e2.Eval(nil, nil))
	})

	t.Run("query_selects_node", func(t *testing.T) {
		t.Parallel()
		// @.key where current has "key"
		q := NewPathQuery(false, Child(NameSelector("key")))
		e := &ExistExpr{Query: q}
		current := map[string]any{"key": "value"}
		assert.True(t, e.Eval(current, nil))
	})

	t.Run("query_selects_no_nodes", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("missing")))
		e := &ExistExpr{Query: q}
		current := map[string]any{"key": "value"}
		assert.False(t, e.Eval(current, nil))
	})

	t.Run("root_query", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(true, Child(NameSelector("a")))
		e := &ExistExpr{Query: q}
		root := map[string]any{"a": 1}
		assert.True(t, e.Eval(nil, root))
	})

	t.Run("null_value_still_exists", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("key")))
		e := &ExistExpr{Query: q}
		current := map[string]any{"key": nil}
		assert.True(t, e.Eval(current, nil))
	})
}

func TestNonExistExprEval(t *testing.T) {
	t.Parallel()

	t.Run("bare_query_no_segments_always_false", func(t *testing.T) {
		t.Parallel()
		e := &NonExistExpr{Query: NewPathQuery(false)}
		assert.False(t, e.Eval(nil, nil))
		e2 := &NonExistExpr{Query: NewPathQuery(true)}
		assert.False(t, e2.Eval(nil, nil))
	})

	t.Run("query_selects_node", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("key")))
		e := &NonExistExpr{Query: q}
		current := map[string]any{"key": "value"}
		assert.False(t, e.Eval(current, nil))
	})

	t.Run("query_selects_no_nodes", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("missing")))
		e := &NonExistExpr{Query: q}
		current := map[string]any{"key": "value"}
		assert.True(t, e.Eval(current, nil))
	})
}

func TestParenExprEval(t *testing.T) {
	t.Parallel()

	t.Run("delegates_true", func(t *testing.T) {
		t.Parallel()
		or := LogicalOr{LogicalAnd{trueExpr()}}
		p := &ParenExpr{Expr: &or}
		assert.True(t, p.Eval(nil, nil))
	})

	t.Run("delegates_false", func(t *testing.T) {
		t.Parallel()
		or := LogicalOr{LogicalAnd{falseExpr()}}
		p := &ParenExpr{Expr: &or}
		assert.False(t, p.Eval(nil, nil))
	})
}

func TestNotParenExprEval(t *testing.T) {
	t.Parallel()

	t.Run("negates_true", func(t *testing.T) {
		t.Parallel()
		or := LogicalOr{LogicalAnd{trueExpr()}}
		np := &NotParenExpr{Expr: &or}
		assert.False(t, np.Eval(nil, nil))
	})

	t.Run("negates_false", func(t *testing.T) {
		t.Parallel()
		or := LogicalOr{LogicalAnd{falseExpr()}}
		np := &NotParenExpr{Expr: &or}
		assert.True(t, np.Eval(nil, nil))
	})
}

func TestNegFuncExprEval(t *testing.T) {
	t.Parallel()

	t.Run("negates_true_func", func(t *testing.T) {
		t.Parallel()
		fn := &logicalMockFunc{name: "test", result: true}
		fe := NewFuncExpr(fn)
		neg := &NegFuncExpr{Func: fe}
		assert.False(t, neg.Eval(nil, nil))
	})

	t.Run("negates_false_func", func(t *testing.T) {
		t.Parallel()
		fn := &logicalMockFunc{name: "test", result: false}
		fe := NewFuncExpr(fn)
		neg := &NegFuncExpr{Func: fe}
		assert.True(t, neg.Eval(nil, nil))
	})
}

func TestCompExprEval(t *testing.T) {
	t.Parallel()

	lit := func(v any) *LiteralValue { return &LiteralValue{Val: v} }
	nothing := nothingRuntimeValue()

	for _, tc := range []struct {
		name  string
		left  any
		op    CompOp
		right any
		want  bool
	}{
		// Equal
		{name: "eq_int", left: 1, op: Equal, right: 1, want: true},
		{name: "eq_int_diff", left: 1, op: Equal, right: 2, want: false},
		{name: "eq_string", left: "a", op: Equal, right: "a", want: true},
		{name: "eq_string_diff", left: "a", op: Equal, right: "b", want: false},
		{name: "eq_bool", left: true, op: Equal, right: true, want: true},
		{name: "eq_bool_diff", left: true, op: Equal, right: false, want: false},
		{name: "eq_nil", left: nil, op: Equal, right: nil, want: true},
		{name: "eq_int_float", left: 1, op: Equal, right: 1.0, want: true},
		{name: "eq_int_float_diff", left: 1, op: Equal, right: 1.5, want: false},

		// NotEqual
		{name: "neq_same", left: 1, op: NotEqual, right: 1, want: false},
		{name: "neq_diff", left: 1, op: NotEqual, right: 2, want: true},
		{name: "neq_string", left: "a", op: NotEqual, right: "b", want: true},

		// Less
		{name: "lt_int", left: 1, op: Less, right: 2, want: true},
		{name: "lt_int_eq", left: 2, op: Less, right: 2, want: false},
		{name: "lt_int_gt", left: 3, op: Less, right: 2, want: false},
		{name: "lt_string", left: "a", op: Less, right: "b", want: true},
		{name: "lt_string_eq", left: "b", op: Less, right: "b", want: false},
		{name: "lt_float", left: 1.5, op: Less, right: 2.5, want: true},

		// LessEqual
		{name: "le_lt", left: 1, op: LessEqual, right: 2, want: true},
		{name: "le_eq", left: 2, op: LessEqual, right: 2, want: true},
		{name: "le_gt", left: 3, op: LessEqual, right: 2, want: false},

		// Greater
		{name: "gt_gt", left: 3, op: Greater, right: 2, want: true},
		{name: "gt_eq", left: 2, op: Greater, right: 2, want: false},
		{name: "gt_lt", left: 1, op: Greater, right: 2, want: false},

		// GreaterEqual
		{name: "ge_gt", left: 3, op: GreaterEqual, right: 2, want: true},
		{name: "ge_eq", left: 2, op: GreaterEqual, right: 2, want: true},
		{name: "ge_lt", left: 1, op: GreaterEqual, right: 2, want: false},
		{name: "le_bool_equal", left: true, op: LessEqual, right: true, want: true},
		{name: "ge_bool_equal", left: false, op: GreaterEqual, right: false, want: true},
		{name: "gt_bool_diff", left: true, op: Greater, right: false, want: false},
		{name: "ge_bool_diff", left: true, op: GreaterEqual, right: false, want: false},

		// Cross-type ordering: not same type => false for ordering ops
		{name: "lt_string_vs_int", left: "a", op: Less, right: 1, want: false},
		{name: "gt_bool_vs_int", left: true, op: Greater, right: 1, want: false},

		// nothing sentinel
		{name: "eq_nothing_nothing", left: nothing, op: Equal, right: nothing, want: true},
		{name: "eq_nothing_int", left: nothing, op: Equal, right: 1, want: false},
		{name: "lt_nothing_int", left: nothing, op: Less, right: 1, want: false},

		// jsonNull sentinel
		{name: "eq_jsonnull_nil", left: JSONNull(), op: Equal, right: nil, want: true},
		{name: "eq_jsonnull_jsonnull", left: JSONNull(), op: Equal, right: JSONNull(), want: true},
		{name: "eq_jsonnull_int", left: JSONNull(), op: Equal, right: 1, want: false},
		{name: "neq_jsonnull_int", left: JSONNull(), op: NotEqual, right: 1, want: true},

		// null ordering: sameType returns true for null/null
		{name: "lt_nil_nil", left: nil, op: Less, right: nil, want: false},
		{name: "le_nil_nil", left: nil, op: LessEqual, right: nil, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &CompExpr{Left: lit(tc.left), Op: tc.op, Right: lit(tc.right)}
			assert.Equal(t, tc.want, c.Eval(nil, nil))
		})
	}

	t.Run("invalid_op_returns_false", func(t *testing.T) {
		t.Parallel()
		c := &CompExpr{Left: lit(1), Op: CompOp(99), Right: lit(1)}
		assert.False(t, c.Eval(nil, nil))
	})
}

func TestLiteralValue(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		val  any
	}{
		{name: "string", val: "hello"},
		{name: "int", val: 42},
		{name: "float", val: 3.14},
		{name: "bool", val: true},
		{name: "nil", val: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lv := &LiteralValue{Val: tc.val}
			assert.Equal(t, runtimeValueFromAny(tc.val), lv.Value(nil, nil))
		})
	}
}

func TestQueryValue(t *testing.T) {
	t.Parallel()

	t.Run("single_result", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("a")))
		qv := &QueryValue{Query: q}
		current := map[string]any{"a": 42}
		assert.Equal(t, runtimeValueFromAny(42), qv.Value(current, nil))
	})

	t.Run("no_result_returns_nothing", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("missing")))
		qv := &QueryValue{Query: q}
		current := map[string]any{"a": 42}
		assert.Equal(t, nothingRuntimeValue(), qv.Value(current, nil))
	})

	t.Run("multiple_results_returns_nothing", func(t *testing.T) {
		t.Parallel()
		// Use wildcard to get multiple results
		q := NewPathQuery(false, Child(WildcardSelector()))
		qv := &QueryValue{Query: q}
		current := map[string]any{"a": 1, "b": 2}
		assert.Equal(t, nothingRuntimeValue(), qv.Value(current, nil))
	})

	t.Run("root_query", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(true, Child(NameSelector("x")))
		qv := &QueryValue{Query: q}
		root := map[string]any{"x": "root_val"}
		assert.Equal(t, runtimeValueFromAny("root_val"), qv.Value(nil, root))
	})
}

func TestFuncValue(t *testing.T) {
	t.Parallel()

	fn := &valueMockFunc{name: "testval", result: 99}
	fe := NewFuncExpr(fn)
	fv := &FuncValue{Func: fe}
	assert.Equal(t, runtimeValueFromAny(99), fv.Value(nil, nil))
}

func TestSameType(t *testing.T) {
	t.Parallel()

	nothing := nothingRuntimeValue()
	value := runtimeValueFromAny

	for _, tc := range []struct {
		name string
		a, b runtimeValue
		want bool
	}{
		{name: "nothing_left", a: nothing, b: value(1), want: false},
		{name: "nothing_right", a: value(1), b: nothing, want: false},
		{name: "nothing_both", a: nothing, b: nothing, want: false},
		{name: "nil_nil", a: value(nil), b: value(nil), want: true},
		{name: "jsonnull_nil", a: value(JSONNull()), b: value(nil), want: true},
		{name: "nil_jsonnull", a: value(nil), b: value(JSONNull()), want: true},
		{name: "jsonnull_jsonnull", a: value(JSONNull()), b: value(JSONNull()), want: true},
		{name: "nil_int", a: value(nil), b: value(1), want: false},
		{name: "int_nil", a: value(1), b: value(nil), want: false},
		{name: "jsonnull_string", a: value(JSONNull()), b: value("x"), want: false},
		{name: "int_int", a: value(1), b: value(2), want: true},
		{name: "int_float64", a: value(1), b: value(2.0), want: true},
		{name: "float32_float64", a: value(float32(1)), b: value(float64(2)), want: true},
		{name: "int8_uint64", a: value(int8(1)), b: value(uint64(2)), want: true},
		{name: "string_string", a: value("a"), b: value("b"), want: true},
		{name: "bool_bool", a: value(true), b: value(false), want: true},
		{name: "string_int", a: value("a"), b: value(1), want: false},
		{name: "bool_int", a: value(true), b: value(1), want: false},
		{name: "string_bool", a: value("a"), b: value(true), want: false},
		{name: "array_array", a: value([]any{1}), b: value([]any{2}), want: false},
		{name: "map_map", a: value(map[string]any{}), b: value(map[string]any{}), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, sameType(tc.a, tc.b))
		})
	}
}

func TestEqualTo(t *testing.T) {
	t.Parallel()

	nothing := nothingRuntimeValue()
	value := runtimeValueFromAny

	for _, tc := range []struct {
		name string
		a, b runtimeValue
		want bool
	}{
		// nothing sentinel
		{name: "nothing_nothing", a: nothing, b: nothing, want: true},
		{name: "nothing_int", a: nothing, b: value(1), want: false},
		{name: "int_nothing", a: value(1), b: nothing, want: false},
		{name: "nothing_nil", a: nothing, b: value(nil), want: false},
		{name: "nil_nothing", a: value(nil), b: nothing, want: false},

		// nil / jsonNull
		{name: "nil_nil", a: value(nil), b: value(nil), want: true},
		{name: "jsonnull_nil", a: value(JSONNull()), b: value(nil), want: true},
		{name: "nil_jsonnull", a: value(nil), b: value(JSONNull()), want: true},
		{name: "jsonnull_jsonnull", a: value(JSONNull()), b: value(JSONNull()), want: true},
		{name: "jsonnull_int", a: value(JSONNull()), b: value(1), want: false},
		{name: "int_jsonnull", a: value(1), b: value(JSONNull()), want: false},

		// numeric coercion
		{name: "int_int_equal", a: value(1), b: value(1), want: true},
		{name: "int_float64_equal", a: value(1), b: value(1.0), want: true},
		{name: "int_float64_diff", a: value(1), b: value(1.5), want: false},
		{name: "int8_uint32", a: value(int8(10)), b: value(uint32(10)), want: true},
		{name: "float32_float64", a: value(float32(2.5)), b: value(float64(2.5)), want: true},
		{name: "int64_uint64", a: value(int64(100)), b: value(uint64(100)), want: true},

		// strings
		{name: "string_equal", a: value("abc"), b: value("abc"), want: true},
		{name: "string_diff", a: value("abc"), b: value("def"), want: false},

		// booleans
		{name: "bool_true", a: value(true), b: value(true), want: true},
		{name: "bool_false", a: value(false), b: value(false), want: true},
		{name: "bool_diff", a: value(true), b: value(false), want: false},

		// cross-type
		{name: "string_vs_int", a: value("1"), b: value(1), want: false},
		{name: "bool_vs_int", a: value(true), b: value(1), want: false},

		// arrays
		{name: "array_equal", a: value([]any{1, "two", true}), b: value([]any{1, "two", true}), want: true},
		{name: "array_diff_len", a: value([]any{1, 2}), b: value([]any{1}), want: false},
		{name: "array_diff_elem", a: value([]any{1, 2}), b: value([]any{1, 3}), want: false},
		{name: "array_nested", a: value([]any{[]any{1, 2}}), b: value([]any{[]any{1, 2}}), want: true},
		{name: "array_nested_diff", a: value([]any{[]any{1, 2}}), b: value([]any{[]any{1, 3}}), want: false},
		{name: "array_empty", a: value([]any{}), b: value([]any{}), want: true},
		{name: "array_vs_string", a: value([]any{1}), b: value("1"), want: false},

		// objects
		{name: "obj_equal", a: value(map[string]any{"a": 1}), b: value(map[string]any{"a": 1}), want: true},
		{name: "obj_diff_val", a: value(map[string]any{"a": 1}), b: value(map[string]any{"a": 2}), want: false},
		{name: "obj_diff_key", a: value(map[string]any{"a": 1}), b: value(map[string]any{"b": 1}), want: false},
		{name: "obj_diff_len", a: value(map[string]any{"a": 1, "b": 2}), b: value(map[string]any{"a": 1}), want: false},
		{name: "obj_empty", a: value(map[string]any{}), b: value(map[string]any{}), want: true},
		{name: "obj_nested", a: value(map[string]any{"x": map[string]any{"y": 1}}), b: value(map[string]any{"x": map[string]any{"y": 1}}), want: true},
		{name: "obj_vs_array", a: value(map[string]any{"a": 1}), b: value([]any{1}), want: false},
		{name: "obj_vs_string", a: value(map[string]any{"a": 1}), b: value("a"), want: false},
		{name: "array_vs_obj", a: value([]any{1}), b: value(map[string]any{"a": 1}), want: false},

		// numeric coercion in arrays
		{name: "array_numeric_coercion", a: value([]any{int(1)}), b: value([]any{float64(1.0)}), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, equalTo(tc.a, tc.b))
		})
	}
}

func TestEqualToDeepCompositeStructures(t *testing.T) {
	t.Parallel()

	a := map[string]any{
		"items": []any{
			map[string]any{"id": int(1), "tags": []any{"a", "b"}},
			[]any{float64(2), JSONNull(), map[string]any{"ok": true}},
		},
	}
	b := map[string]any{
		"items": []any{
			map[string]any{"id": float64(1), "tags": []any{"a", "b"}},
			[]any{int(2), nil, map[string]any{"ok": true}},
		},
	}
	c := map[string]any{
		"items": []any{
			map[string]any{"id": int(1), "tags": []any{"a", "c"}},
			[]any{float64(2), JSONNull(), map[string]any{"ok": true}},
		},
	}

	assert.True(t, equalTo(runtimeValueFromAny(a), runtimeValueFromAny(b)))
	assert.False(t, equalTo(runtimeValueFromAny(a), runtimeValueFromAny(c)))
	assert.False(t, equalTo(
		runtimeValueFromAny(map[string]any{"id": int64(9007199254740992)}),
		runtimeValueFromAny(map[string]any{"id": int64(9007199254740993)}),
	))
}

func TestLessThan(t *testing.T) {
	t.Parallel()

	value := runtimeValueFromAny

	for _, tc := range []struct {
		name string
		a, b runtimeValue
		want bool
	}{
		{name: "int_lt", a: value(1), b: value(2), want: true},
		{name: "int_eq", a: value(2), b: value(2), want: false},
		{name: "int_gt", a: value(3), b: value(2), want: false},
		{name: "float64_lt", a: value(1.5), b: value(2.5), want: true},
		{name: "int_vs_float64", a: value(1), b: value(2.0), want: true},
		{name: "string_lt", a: value("a"), b: value("b"), want: true},
		{name: "string_eq", a: value("b"), b: value("b"), want: false},
		{name: "string_gt", a: value("c"), b: value("b"), want: false},
		{name: "nil_nil", a: value(nil), b: value(nil), want: false},
		{name: "bool_not_ordered", a: value(true), b: value(false), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, lessThan(tc.a, tc.b))
		})
	}
}

func TestJSONNull(t *testing.T) {
	t.Parallel()

	n := JSONNull()
	_, ok := any(n).(jsonNull)
	assert.True(t, ok)
}

func TestFuncExprEvalAsBasicExpr(t *testing.T) {
	t.Parallel()

	t.Run("logical_func_returning_true", func(t *testing.T) {
		t.Parallel()
		fn := &logicalMockFunc{name: "yes", result: true}
		fe := NewFuncExpr(fn)
		assert.True(t, fe.Eval(nil, nil))
	})

	t.Run("logical_func_returning_false", func(t *testing.T) {
		t.Parallel()
		fn := &logicalMockFunc{name: "no", result: false}
		fe := NewFuncExpr(fn)
		assert.False(t, fe.Eval(nil, nil))
	})

	t.Run("non_logical_func_returns_false", func(t *testing.T) {
		t.Parallel()
		fn := &valueMockFunc{name: "val", result: 42}
		fe := NewFuncExpr(fn)
		assert.False(t, fe.Eval(nil, nil))
	})

	t.Run("logical_func_returning_non_bool", func(t *testing.T) {
		t.Parallel()
		fn := &logicalMockFunc{name: "bad", result: "not a bool"}
		fe := NewFuncExpr(fn)
		assert.False(t, fe.Eval(nil, nil))
	})
}

func TestCompExprWithQueryValue(t *testing.T) {
	t.Parallel()

	t.Run("query_value_equals_literal", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("age")))
		qv := &QueryValue{Query: q}
		lv := &LiteralValue{Val: 30}
		c := &CompExpr{Left: qv, Op: Equal, Right: lv}

		current := map[string]any{"age": 30}
		assert.True(t, c.Eval(current, nil))
	})

	t.Run("query_value_missing_key", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(NameSelector("missing")))
		qv := &QueryValue{Query: q}
		lv := &LiteralValue{Val: 30}
		c := &CompExpr{Left: qv, Op: Equal, Right: lv}

		current := map[string]any{"age": 30}
		assert.False(t, c.Eval(current, nil))
	})
}

func TestCompExprMissingQueryDoesNotEqualNullQuery(t *testing.T) {
	t.Parallel()

	missing := &QueryValue{Query: NewPathQuery(false, Child(NameSelector("missing")))}
	nullValue := &QueryValue{Query: NewPathQuery(false, Child(NameSelector("null")))}
	current := map[string]any{"null": nil}

	assert.False(t, (&CompExpr{Left: missing, Op: Equal, Right: nullValue}).Eval(current, nil))
	assert.True(t, (&CompExpr{Left: missing, Op: NotEqual, Right: nullValue}).Eval(current, nil))
}

func TestCompExprWithFuncValue(t *testing.T) {
	t.Parallel()

	fn := &valueMockFunc{name: "len", result: 5}
	fe := NewFuncExpr(fn)
	fv := &FuncValue{Func: fe}
	lv := &LiteralValue{Val: 5}
	c := &CompExpr{Left: fv, Op: Equal, Right: lv}
	assert.True(t, c.Eval(nil, nil))
}

func TestFilterExprIntegration(t *testing.T) {
	t.Parallel()

	t.Run("or_of_ands", func(t *testing.T) {
		t.Parallel()
		// (false && true) || (true && true) => true
		f := &FilterExpr{
			Or: LogicalOr{
				LogicalAnd{falseExpr(), trueExpr()},
				LogicalAnd{trueExpr(), trueExpr()},
			},
		}
		assert.True(t, f.Eval(nil, nil))
	})

	t.Run("exist_and_comparison", func(t *testing.T) {
		t.Parallel()
		// @.name exists AND @.age == 30
		existQ := NewPathQuery(false, Child(NameSelector("name")))
		ageQ := NewPathQuery(false, Child(NameSelector("age")))

		f := &FilterExpr{
			Or: LogicalOr{
				LogicalAnd{
					&ExistExpr{Query: existQ},
					&CompExpr{
						Left:  &QueryValue{Query: ageQ},
						Op:    Equal,
						Right: &LiteralValue{Val: 30},
					},
				},
			},
		}

		current := map[string]any{"name": "Alice", "age": 30}
		assert.True(t, f.Eval(current, nil))

		missing := map[string]any{"age": 30}
		assert.False(t, f.Eval(missing, nil))
	})

	t.Run("not_paren_with_exist", func(t *testing.T) {
		t.Parallel()
		// !(@.key) where key exists => false
		existQ := NewPathQuery(false, Child(NameSelector("key")))
		or := LogicalOr{LogicalAnd{&ExistExpr{Query: existQ}}}
		np := &NotParenExpr{Expr: &or}

		current := map[string]any{"key": "val"}
		assert.False(t, np.Eval(current, nil))
	})
}

func TestEqualToNothingVsNilVsJsonNull(t *testing.T) {
	t.Parallel()

	n := nothingRuntimeValue()
	nullValue := runtimeValueFromAny(nil)
	literalNull := runtimeValueFromAny(JSONNull())

	assert.True(t, equalTo(n, n))
	assert.False(t, equalTo(n, nullValue))
	assert.False(t, equalTo(nullValue, n))

	assert.True(t, equalTo(literalNull, nullValue))
	assert.True(t, equalTo(nullValue, literalNull))
	assert.True(t, equalTo(literalNull, literalNull))

	assert.False(t, equalTo(n, runtimeValueFromAny(42)))
	assert.False(t, equalTo(runtimeValueFromAny(42), n))
	assert.False(t, equalTo(literalNull, runtimeValueFromAny(42)))
	assert.False(t, equalTo(runtimeValueFromAny(42), literalNull))
}

func TestExistExprWithArrayIndex(t *testing.T) {
	t.Parallel()

	t.Run("index_exists", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(IndexSelector(0)))
		e := &ExistExpr{Query: q}
		current := []any{"a", "b"}
		assert.True(t, e.Eval(current, nil))
	})

	t.Run("index_out_of_range", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(IndexSelector(5)))
		e := &ExistExpr{Query: q}
		current := []any{"a", "b"}
		assert.False(t, e.Eval(current, nil))
	})

	t.Run("negative_index", func(t *testing.T) {
		t.Parallel()
		q := NewPathQuery(false, Child(IndexSelector(-1)))
		e := &ExistExpr{Query: q}
		current := []any{"a", "b"}
		assert.True(t, e.Eval(current, nil))
	})
}

func TestCompExprNullOrdering(t *testing.T) {
	t.Parallel()

	lit := func(v any) *LiteralValue { return &LiteralValue{Val: v} }

	// null <= null is true (sameType true, lessThan false, equalTo true)
	c := &CompExpr{Left: lit(nil), Op: LessEqual, Right: lit(nil)}
	assert.True(t, c.Eval(nil, nil))

	// null >= null is true
	c2 := &CompExpr{Left: lit(nil), Op: GreaterEqual, Right: lit(nil)}
	assert.True(t, c2.Eval(nil, nil))

	// null > null is false
	c3 := &CompExpr{Left: lit(nil), Op: Greater, Right: lit(nil)}
	assert.False(t, c3.Eval(nil, nil))

	// null == jsonNull is true
	c4 := &CompExpr{Left: lit(nil), Op: Equal, Right: lit(JSONNull())}
	assert.True(t, c4.Eval(nil, nil))
}

func TestQueryValueNilResult(t *testing.T) {
	t.Parallel()

	q := NewPathQuery(false, Child(NameSelector("key")))
	qv := &QueryValue{Query: q}
	current := map[string]any{"key": nil}
	val := qv.Value(current, nil)
	require.Equal(t, runtimeJSON, val.kind)
	assert.Nil(t, val.value)
}
