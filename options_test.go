package jsonpath

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentable/go-jsonpath/internal/ast"
	"github.com/agentable/go-jsonpath/internal/parser"
)

func testValueFunction(name string, params ...FuncType) Function {
	return NewValueFunction(name, params, func([]FunctionValue) Value { return NoValue() })
}

func testLogicalFunction(name string, params ...FuncType) Function {
	return NewLogicalFunction(name, params, func([]FunctionValue) Logical { return false })
}

func TestNewParser_NoOptions(t *testing.T) {
	t.Parallel()

	p, err := NewParser()
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestParser_ZeroValueUsesDefaults(t *testing.T) {
	t.Parallel()

	var zero Parser

	root, err := zero.Parse(`$`)
	require.NoError(t, err)
	assert.Equal(t, NodeList{map[string]any{"name": "book"}}, root.Select(map[string]any{"name": "book"}))

	name, err := zero.Parse(`$.name`)
	require.NoError(t, err)
	assert.Equal(t, NodeList{"book"}, name.Select(map[string]any{"name": "book"}))

	input := []any{
		map[string]any{"name": "foo"},
		map[string]any{"name": "food"},
		map[string]any{"name": "bar"},
	}
	for _, tc := range []struct {
		name string
		expr string
		want NodeList
	}{
		{name: "value built-in", expr: `$[?length(@.name) == 3]`, want: NodeList{input[0], input[2]}},
		{name: "logical built-in", expr: `$[?match(@.name, "foo")]`, want: NodeList{input[0]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, err := zero.Parse(tc.expr)
			require.NoError(t, err)
			assert.Equal(t, tc.want, path.Select(input))
		})
	}

	assert.Equal(t, `$[?length(@) == 3]`, zero.MustParse(`$[?length(@) == 3]`).String())
}

func TestParser_ZeroValuePreservesParseDiagnostics(t *testing.T) {
	t.Parallel()

	var zero Parser
	_, zeroErr := zero.Parse("invalid")
	_, packageErr := Parse("invalid")
	require.ErrorIs(t, zeroErr, ErrPathParse)
	require.ErrorIs(t, packageErr, ErrPathParse)

	var zeroParseErr, packageParseErr *ParseError
	require.ErrorAs(t, zeroErr, &zeroParseErr)
	require.ErrorAs(t, packageErr, &packageParseErr)
	assert.Equal(t, packageParseErr.Offset, zeroParseErr.Offset)
	assert.Equal(t, packageParseErr.Reason, zeroParseErr.Reason)
	assert.Equal(t, packageParseErr.Snippet, zeroParseErr.Snippet)
	assert.Equal(t,
		errors.Is(packageParseErr.Cause, parser.ErrParsePosition),
		errors.Is(zeroParseErr.Cause, parser.ErrParsePosition),
	)
}

func TestParser_ZeroValueMustParsePanicsWithErrPathParse(t *testing.T) {
	t.Parallel()

	var zero Parser
	defer func() {
		err, ok := recover().(error)
		require.True(t, ok)
		require.ErrorIs(t, err, ErrPathParse)
	}()
	zero.MustParse("invalid")
}

func TestParser_DefaultsSupportConcurrentReuse(t *testing.T) {
	t.Parallel()

	constructed, err := NewParser()
	require.NoError(t, err)
	parsers := map[string]*Parser{
		"zero":        {},
		"constructed": constructed,
	}

	for name, parser := range parsers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			const workers = 32
			errs := make(chan error, workers)
			var wg sync.WaitGroup
			for range workers {
				wg.Go(func() {
					_, err := parser.Parse(`$[?length(@) > 0]`)
					errs <- err
				})
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewParser_WithFunctions(t *testing.T) {
	t.Parallel()

	p, err := NewParser(WithFunctions(
		testValueFunction("myfunc", FuncValue),
		testLogicalFunction("other", FuncNodes),
	))
	require.NoError(t, err)
	require.Len(t, p.registry, 7)

	myfunc := p.registry["myfunc"]
	assert.Equal(t, ast.Value, myfunc.ResultType())
	assert.Equal(t, 1, myfunc.ParameterCount())
	assert.Equal(t, ast.Value, myfunc.ParameterType(0))

	other := p.registry["other"]
	assert.Equal(t, ast.Logical, other.ResultType())
	assert.Equal(t, ast.Nodes, other.ParameterType(0))
}

func TestNewParser_ReturnsOptionErrors(t *testing.T) {
	t.Parallel()

	_, err := NewParser(WithFunctions(testValueFunction("bad-name")))
	require.ErrorIs(t, err, ErrFunction)
	require.NotErrorIs(t, err, ErrPathParse)
}

func TestNewParser_RejectsNilOption(t *testing.T) {
	t.Parallel()

	var option Option
	parser, err := NewParser(option)
	require.ErrorIs(t, err, ErrFunction)
	assert.Nil(t, parser)
}

func TestNewParser_RejectsNilAmongValidOptions(t *testing.T) {
	t.Parallel()

	var option Option
	parser, err := NewParser(
		WithFunctions(testValueFunction("before")),
		option,
		WithFunctions(testValueFunction("after")),
	)
	require.ErrorIs(t, err, ErrFunction)
	assert.Nil(t, parser)
}

func TestValid_UsesDefaultParser(t *testing.T) {
	t.Parallel()

	fn := NewLogicalFunction("custom", nil, func([]FunctionValue) Logical { return true })
	p, err := NewParser(WithFunctions(fn))
	require.NoError(t, err)

	assert.False(t, Valid(`$[?custom()]`))
	_, err = p.Parse(`$[?custom()]`)
	require.NoError(t, err)
}

func TestWithFunctions_RejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	_, err := NewParser(WithFunctions(
		testValueFunction("dup"),
		testLogicalFunction("dup"),
	))
	require.ErrorIs(t, err, ErrFunction)
	require.NotErrorIs(t, err, ErrPathParse)
	assert.Contains(t, err.Error(), `duplicate function "dup"`)
}

func TestWithFunctions_RejectsBuiltinOverride(t *testing.T) {
	t.Parallel()

	_, err := NewParser(WithFunctions(testLogicalFunction("length")))
	require.ErrorIs(t, err, ErrFunction)
	require.NotErrorIs(t, err, ErrPathParse)
	assert.Contains(t, err.Error(), `function "length" overrides a built-in`)
}

func TestWithFunctions_MultipleOptions(t *testing.T) {
	t.Parallel()

	p, err := NewParser(
		WithFunctions(testValueFunction("a")),
		WithFunctions(NewNodesFunction("b", nil, func([]FunctionValue) Nodes { return nil })),
	)
	require.NoError(t, err)
	assert.Equal(t, ast.Value, p.registry["a"].ResultType())
	assert.Equal(t, ast.Nodes, p.registry["b"].ResultType())
}

func TestWithFunctions_RejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		fn   Function
	}{
		{name: "zero value", fn: Function{}},
		{name: "nil callback", fn: NewValueFunction("nil_callback", nil, nil)},
		{name: "invalid parameter", fn: testValueFunction("bad_param", FuncType(99))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewParser(WithFunctions(tc.fn))
			require.ErrorIs(t, err, ErrFunction)
			require.NotErrorIs(t, err, ErrPathParse)
		})
	}
}

func TestWithFunctions_ValidatesNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "1bad", "bad-name", "Bad", "badName", "_bad", "café", "true", "false", "null"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := NewParser(WithFunctions(testValueFunction(name)))
			require.ErrorIs(t, err, ErrFunction)
			assert.Contains(t, err.Error(), "invalid function name")
		})
	}

	for _, name := range []string{"x", "has_prefix", "count2"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := NewParser(WithFunctions(testValueFunction(name)))
			require.NoError(t, err)
		})
	}
}

func TestFunctionDefinition_ClonesParameters(t *testing.T) {
	t.Parallel()

	params := []FuncType{FuncValue}
	fn := NewValueFunction("identity", params, func(args []FunctionValue) Value {
		return args[0].(Value)
	})
	params[0] = FuncNodes

	p, err := NewParser(WithFunctions(fn))
	require.NoError(t, err)
	params[0] = FuncLogical

	path, err := p.Parse(`$[?identity(@.name) == "ok"]`)
	require.NoError(t, err)
	got := path.Select([]any{
		map[string]any{"name": "ok"},
		map[string]any{"name": "no"},
	})
	want := NodeList{map[string]any{"name": "ok"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Select() mismatch (-want +got):\n%s", diff)
	}
}

func TestWithFunctions_ClonesDefinitions(t *testing.T) {
	t.Parallel()

	definitions := []Function{
		NewLogicalFunction("original", []FuncType{FuncValue}, func([]FunctionValue) Logical { return true }),
	}
	option := WithFunctions(definitions...)
	definitions[0] = Function{}

	parser, err := NewParser(option)
	require.NoError(t, err)

	_, err = parser.Parse(`$[?original(@)]`)
	require.NoError(t, err)
}

func TestWithFunctions_ConvertsQueryArgumentsFromSignature(t *testing.T) {
	t.Parallel()

	isFoo := NewLogicalFunction("is_foo", []FuncType{FuncValue}, func(args []FunctionValue) Logical {
		value, ok := args[0].(Value)
		return Logical(ok && !value.IsNothing() && value.Any() == "foo")
	})
	hasFooNode := NewLogicalFunction("has_foo_node", []FuncType{FuncNodes}, func(args []FunctionValue) Logical {
		nodes, ok := args[0].(Nodes)
		return Logical(ok && len(nodes) == 1 && nodes[0] == "foo")
	})
	p, err := NewParser(WithFunctions(isFoo, hasFooNode))
	require.NoError(t, err)

	for _, expr := range []string{`$[?is_foo(@.name)]`, `$[?has_foo_node(@.name)]`} {
		path, err := p.Parse(expr)
		require.NoError(t, err)
		got := path.Select([]any{
			map[string]any{"name": "foo"},
			map[string]any{"name": "bar"},
		})
		want := NodeList{map[string]any{"name": "foo"}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s Select() mismatch (-want +got):\n%s", expr, diff)
		}
	}
}

func TestNewNodesFunction_ExecutesWithTypedResult(t *testing.T) {
	t.Parallel()

	fn := NewNodesFunction("nodes", []FuncType{FuncNodes}, func(args []FunctionValue) Nodes {
		nodes, _ := args[0].(Nodes)
		return nodes
	})
	p, err := NewParser(WithFunctions(fn))
	require.NoError(t, err)
	path, err := p.Parse(`$[?nodes(@[*])]`)
	require.NoError(t, err)

	input := []any{[]any{1}, []any{}}
	assert.Equal(t, NodeList{input[0]}, path.Select(input))
}

func TestWithFunctions_ValidatesNestedResultType(t *testing.T) {
	t.Parallel()

	child := NewLogicalFunction("child", nil, func([]FunctionValue) Logical { return true })
	acceptLogical := NewLogicalFunction("accept_logical", []FuncType{FuncLogical}, func(args []FunctionValue) Logical {
		logical, ok := args[0].(Logical)
		return Logical(ok && logical.Bool())
	})
	acceptValue := NewValueFunction("accept_value", []FuncType{FuncValue}, func(args []FunctionValue) Value {
		return args[0].(Value)
	})
	p, err := NewParser(WithFunctions(child, acceptLogical, acceptValue))
	require.NoError(t, err)

	path, err := p.Parse(`$[?accept_logical(child())]`)
	require.NoError(t, err)
	assert.Equal(t, NodeList{1}, path.Select([]any{1}))

	_, err = p.Parse(`$[?accept_value(child()) == true]`)
	require.ErrorIs(t, err, ErrPathParse)
	require.ErrorIs(t, err, ErrFunction)
}

func TestWithFunctions_LogicalExpressionArgument(t *testing.T) {
	t.Parallel()

	fn := NewLogicalFunction("accepts_logic", []FuncType{FuncLogical}, func(args []FunctionValue) Logical {
		logical, ok := args[0].(Logical)
		return Logical(ok && logical.Bool())
	})
	p, err := NewParser(WithFunctions(fn))
	require.NoError(t, err)
	path, err := p.Parse(`$[?accepts_logic((@.enabled == true && @.ready == true))]`)
	require.NoError(t, err)

	input := []any{
		map[string]any{"enabled": true, "ready": true},
		map[string]any{"enabled": true, "ready": false},
	}
	assert.Equal(t, NodeList{input[0]}, path.Select(input))
}

func TestWithFunctions_LogicalArgumentForms(t *testing.T) {
	t.Parallel()

	accept := NewLogicalFunction("accepts_logic", []FuncType{FuncLogical}, func(args []FunctionValue) Logical {
		logical, ok := args[0].(Logical)
		return Logical(ok && logical.Bool())
	})
	child := NewLogicalFunction("child", nil, func([]FunctionValue) Logical { return true })
	p, err := NewParser(WithFunctions(accept, child))
	require.NoError(t, err)

	input := []any{
		map[string]any{"enabled": true, "ready": false},
		map[string]any{"enabled": false, "ready": false, "disabled": true},
	}
	for _, tc := range []struct {
		name string
		expr string
		want NodeList
	}{
		{name: "comparison", expr: `$[?accepts_logic(@.enabled == true)]`, want: NodeList{input[0]}},
		{name: "negation", expr: `$[?accepts_logic(!@.disabled)]`, want: NodeList{input[0]}},
		{name: "parentheses and conjunction", expr: `$[?accepts_logic((@.enabled == true && !@.disabled))]`, want: NodeList{input[0]}},
		{name: "disjunction", expr: `$[?accepts_logic(@.enabled == true || @.ready == true)]`, want: NodeList{input[0]}},
		{name: "logical function", expr: `$[?accepts_logic(child())]`, want: NodeList{input[0], input[1]}},
		{name: "integer literal comparison", expr: `$[?accepts_logic(1 == 1)]`, want: NodeList{input[0], input[1]}},
		{name: "decimal literal comparison", expr: `$[?accepts_logic(1.5 == 1.5)]`, want: NodeList{input[0], input[1]}},
		{name: "string literal comparison", expr: `$[?accepts_logic("x" == "x")]`, want: NodeList{input[0], input[1]}},
		{name: "boolean literal comparison", expr: `$[?accepts_logic(true == true)]`, want: NodeList{input[0], input[1]}},
		{name: "null literal comparison", expr: `$[?accepts_logic(null == null)]`, want: NodeList{input[0], input[1]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, err := p.Parse(tc.expr)
			require.NoError(t, err)
			assert.Equal(t, tc.want, path.Select(input))

			roundTrip, err := p.Parse(path.String())
			require.NoError(t, err)
			assert.Equal(t, tc.want, roundTrip.Select(input))
		})
	}

	rootInput := map[string]any{"enabled": true, "items": []any{1, 2}}
	rootPath, err := p.Parse(`$["items"][?accepts_logic($.enabled == true)]`)
	require.NoError(t, err)
	assert.Equal(t, NodeList{1, 2}, rootPath.Select(rootInput))
}

func TestWithFunctions_LogicalArgumentErrors(t *testing.T) {
	t.Parallel()

	accept := NewLogicalFunction("accepts_logic", []FuncType{FuncLogical}, func([]FunctionValue) Logical { return false })
	value := NewValueFunction("value_fn", []FuncType{FuncNodes}, func([]FunctionValue) Value { return NoValue() })
	p, err := NewParser(WithFunctions(accept, value))
	require.NoError(t, err)

	_, err = p.Parse(`$[?accepts_logic(true)]`)
	require.ErrorIs(t, err, ErrPathParse)
	require.ErrorIs(t, err, ErrFunction)

	_, err = p.Parse(`$[?accepts_logic(value_fn(@[*]))]`)
	require.ErrorIs(t, err, ErrPathParse)
	require.ErrorIs(t, err, ErrFunction)

	_, err = p.Parse(`$[?accepts_logic(true, (@.enabled == true))]`)
	require.ErrorIs(t, err, ErrPathParse)
	require.ErrorIs(t, err, ErrFunction)

	expr := `$[?accepts_logic((@.enabled == true && ))]`
	_, err = p.Parse(expr)
	require.ErrorIs(t, err, ErrPathParse)
	var parseErr *ParseError
	require.ErrorAs(t, err, &parseErr)
	assert.Equal(t, 39, parseErr.Offset)
	assert.Equal(t, "expected filter expression", parseErr.Reason)
	assert.Equal(t, `'...led == true && ))]'`, parseErr.Snippet)
}

func TestWithFunctions_IncompatibleLogicalArgumentsReturnErrFunction(t *testing.T) {
	t.Parallel()

	acceptValue := NewLogicalFunction("accept_value", []FuncType{FuncValue}, func([]FunctionValue) Logical { return false })
	acceptNodes := NewLogicalFunction("accept_nodes", []FuncType{FuncNodes}, func([]FunctionValue) Logical { return false })
	child := NewLogicalFunction("child", nil, func([]FunctionValue) Logical { return true })
	p, err := NewParser(WithFunctions(acceptValue, acceptNodes, child))
	require.NoError(t, err)

	for _, expr := range []string{
		`$[?accept_value(!@.enabled)]`,
		`$[?accept_nodes((@.enabled))]`,
		`$[?accept_value(@.enabled == true)]`,
		`$[?accept_value(@.enabled && @.ready)]`,
		`$[?accept_nodes(1 == 1)]`,
		`$[?accept_nodes(value(@) == 1)]`,
		`$[?accept_nodes(child() || @.ready)]`,
	} {
		_, err := p.Parse(expr)
		require.ErrorIs(t, err, ErrPathParse, expr)
		require.ErrorIs(t, err, ErrFunction, expr)
	}
}

func TestWithFunctions_RuntimeValuesRemainDistinct(t *testing.T) {
	t.Parallel()

	identity := NewValueFunction("identity", []FuncType{FuncValue}, func(args []FunctionValue) Value {
		return args[0].(Value)
	})
	p, err := NewParser(WithFunctions(identity))
	require.NoError(t, err)

	nullPath, err := p.Parse(`$[?identity(@.name) == null]`)
	require.NoError(t, err)
	missingPath, err := p.Parse(`$[?identity(@.missing) == null]`)
	require.NoError(t, err)

	input := []any{map[string]any{"name": nil}}
	assert.Len(t, nullPath.Select(input), 1)
	assert.Empty(t, missingPath.Select(input))
	assert.False(t, Logical(false).Bool())
	assert.Empty(t, Nodes(nil))
}

func TestParserParse_ReturnsErrPathParse(t *testing.T) {
	t.Parallel()

	p, err := NewParser()
	require.NoError(t, err)
	_, err = p.Parse("invalid")
	require.ErrorIs(t, err, ErrPathParse)
}

func TestParserParse_PreservesInnerParseError(t *testing.T) {
	t.Parallel()

	p, err := NewParser()
	require.NoError(t, err)
	_, err = p.Parse(`$[?missing(@)]`)
	require.ErrorIs(t, err, ErrPathParse)
	require.ErrorIs(t, err, parser.ErrUnknownFunction)
	assert.Contains(t, err.Error(), "missing")
}

func TestParserMustParse_PanicsWithErrPathParse(t *testing.T) {
	t.Parallel()

	p, err := NewParser()
	require.NoError(t, err)

	defer func() {
		err, ok := recover().(error)
		require.True(t, ok)
		require.ErrorIs(t, err, ErrPathParse)
	}()
	p.MustParse("invalid")
}

func TestMapFuncType(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   FuncType
		want ast.FuncType
	}{
		{in: FuncLogical, want: ast.Logical},
		{in: FuncValue, want: ast.Value},
		{in: FuncNodes, want: ast.Nodes},
	} {
		got, ok := mapFuncType(tc.in)
		require.True(t, ok)
		assert.Equal(t, tc.want, got)
	}

	_, ok := mapFuncType(FuncType(99))
	assert.False(t, ok)
}

func BenchmarkParserParse(b *testing.B) {
	expr := "$.store.book[*].author"

	b.Run("new_parser_each_time", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			p, _ := NewParser()
			_, _ = p.Parse(expr)
		}
	})

	b.Run("reused_parser", func(b *testing.B) {
		p, _ := NewParser()
		b.ReportAllocs()
		for b.Loop() {
			_, _ = p.Parse(expr)
		}
	})
}
