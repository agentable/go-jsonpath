package parser

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentable/go-jsonpath/internal/ast"
	"github.com/agentable/go-jsonpath/internal/functions"
	"github.com/agentable/go-jsonpath/internal/lexer"
)

// testFuncs returns a function registry with the RFC 9535 built-in functions.
func testFuncs() map[string]ast.Function {
	return map[string]ast.Function{
		"length": &functions.LengthFunc{},
		"count":  &functions.CountFunc{},
		"match":  &functions.MatchFunc{},
		"search": &functions.SearchFunc{},
		"value":  &functions.ValueFunc{},
	}
}

func testFuncsWithNodes() map[string]ast.Function {
	funcs := testFuncs()
	funcs["nodes"] = nodesTestFunc{}
	return funcs
}

type nodesTestFunc struct{}

func (nodesTestFunc) Name() string             { return "nodes" }
func (nodesTestFunc) ResultType() ast.FuncType { return ast.Nodes }
func (nodesTestFunc) ParameterCount() int      { return 1 }
func (nodesTestFunc) ParameterType(int) ast.FuncType {
	return ast.Nodes
}
func (nodesTestFunc) Call(_ []ast.FunctionValue) ast.FunctionValue {
	return ast.TypedNodes(nil)
}

func TestParserMatchConsumesExpectedKind(t *testing.T) {
	t.Parallel()

	p, err := New("$[0]", nil)
	require.NoError(t, err)

	assert.False(t, p.match(lexer.LeftBracket))
	assert.Equal(t, 0, p.pos)

	assert.True(t, p.match(lexer.Dollar, lexer.At))
	assert.Equal(t, lexer.Dollar, p.previous().Kind)
	assert.Equal(t, 1, p.pos)

	assert.True(t, p.match(lexer.LeftBracket))
	assert.Equal(t, lexer.LeftBracket, p.previous().Kind)
	assert.Equal(t, 2, p.pos)
}

// TestParseRootIdentifier tests parsing of root ($) and current (@) identifiers.
func TestParseRootIdentifier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		wantRoot bool
		wantErr  bool
	}{
		{
			name:     "root identifier",
			input:    "$",
			wantRoot: true,
		},
		{
			name:    "current identifier",
			input:   "@",
			wantErr: true,
		},
		{
			name:    "current identifier with segments",
			input:   "@.a",
			wantErr: true,
		},
		{
			name:    "missing identifier",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid identifier",
			input:   "foo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRoot, query.IsRoot())
			assert.Empty(t, query.Segments())
		})
	}
}

func TestParseRejectsCurrentIdentifierAtRoot(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"@", "@.a"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			p, err := New(input, nil)
			require.NoError(t, err)

			_, err = p.Parse()
			var parseErr *Error
			require.ErrorAs(t, err, &parseErr)
			assert.ErrorIs(t, err, ErrParsePosition)
			assert.Equal(t, 0, parseErr.Offset)
			assert.Equal(t, "expected $", parseErr.Reason)
			assert.Equal(t, "'"+input+"'", parseErr.Snippet)
		})
	}
}

// TestParseNameSelector tests parsing of name selectors.
func TestParseNameSelector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantNames []string
		wantErr   bool
	}{
		{
			name:      "bracketed single-quoted name",
			input:     "$['foo']",
			wantNames: []string{"foo"},
		},
		{
			name:      "bracketed double-quoted name",
			input:     `$["bar"]`,
			wantNames: []string{"bar"},
		},
		{
			name:      "dot-child shorthand",
			input:     "$.name",
			wantNames: []string{"name"},
		},
		{
			name:      "name with spaces",
			input:     `$["name with spaces"]`,
			wantNames: []string{"name with spaces"},
		},
		{
			name:      "name with unicode",
			input:     `$["名前"]`,
			wantNames: []string{"名前"},
		},
		{
			name:      "name with escape sequences",
			input:     `$["line\nbreak"]`,
			wantNames: []string{"line\nbreak"},
		},
		{
			name:      "chained name selectors",
			input:     `$["a"]["b"]["c"]`,
			wantNames: []string{"a", "b", "c"},
		},
		{
			name:      "mixed dot and bracket notation",
			input:     `$.foo["bar"].baz`,
			wantNames: []string{"foo", "bar", "baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Collect all name selectors from all segments
			var gotNames []string
			for _, seg := range query.Segments() {
				for _, sel := range seg.Selectors() {
					if sel.Kind == ast.Name {
						gotNames = append(gotNames, sel.Name)
					}
				}
			}

			if diff := cmp.Diff(tt.wantNames, gotNames); diff != "" {
				t.Errorf("name selectors mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestParseIndexSelector tests parsing of array index selectors.
func TestParseIndexSelector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		input       string
		wantIndices []int64
		wantErr     bool
	}{
		{
			name:        "single positive index",
			input:       "$[0]",
			wantIndices: []int64{0},
		},
		{
			name:        "single negative index",
			input:       "$[-1]",
			wantIndices: []int64{-1},
		},
		{
			name:        "multiple indices",
			input:       "$[0,1,2]",
			wantIndices: []int64{0, 1, 2},
		},
		{
			name:        "mixed positive and negative",
			input:       "$[0,-1,5,-3]",
			wantIndices: []int64{0, -1, 5, -3},
		},
		{
			name:        "chained index selectors",
			input:       "$[0][1][2]",
			wantIndices: []int64{0, 1, 2},
		},
		{
			name:        "large index",
			input:       "$[999999]",
			wantIndices: []int64{999999},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			segments := query.Segments()

			var gotIndices []int64
			for _, seg := range segments {
				for _, sel := range seg.Selectors() {
					if sel.Kind == ast.Index {
						gotIndices = append(gotIndices, sel.Index)
					}
				}
			}

			if diff := cmp.Diff(tt.wantIndices, gotIndices); diff != "" {
				t.Errorf("index selectors mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestParseSliceSelector tests parsing of array slice selectors.
func TestParseSliceSelector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantSlice ast.SliceArgs
		wantErr   bool
	}{
		{
			name:  "start and end",
			input: "$[1:5]",
			wantSlice: ast.SliceArgs{
				Start:    1,
				End:      5,
				HasStart: true,
				HasEnd:   true,
			},
		},
		{
			name:  "start only",
			input: "$[2:]",
			wantSlice: ast.SliceArgs{
				Start:    2,
				HasStart: true,
			},
		},
		{
			name:  "end only",
			input: "$[:3]",
			wantSlice: ast.SliceArgs{
				End:    3,
				HasEnd: true,
			},
		},
		{
			name:      "no start or end",
			input:     "$[:]",
			wantSlice: ast.SliceArgs{},
		},
		{
			name:  "with step",
			input: "$[1:10:2]",
			wantSlice: ast.SliceArgs{
				Start:    1,
				End:      10,
				Step:     2,
				HasStart: true,
				HasEnd:   true,
				HasStep:  true,
			},
		},
		{
			name:  "step only",
			input: "$[::2]",
			wantSlice: ast.SliceArgs{
				Step:    2,
				HasStep: true,
			},
		},
		{
			name:  "negative indices",
			input: "$[-5:-1]",
			wantSlice: ast.SliceArgs{
				Start:    -5,
				End:      -1,
				HasStart: true,
				HasEnd:   true,
			},
		},
		{
			name:  "negative step",
			input: "$[10:0:-1]",
			wantSlice: ast.SliceArgs{
				Start:    10,
				End:      0,
				Step:     -1,
				HasStart: true,
				HasEnd:   true,
				HasStep:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Slice, selectors[0].Kind)
			if diff := cmp.Diff(tt.wantSlice, selectors[0].Slice); diff != "" {
				t.Errorf("slice selector mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestParseWildcardSelector tests parsing of wildcard selectors.
func TestParseWildcardSelector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "bracketed wildcard",
			input: "$[*]",
		},
		{
			name:  "dot wildcard",
			input: "$.*",
		},
		{
			name:  "multiple wildcards",
			input: "$[*][*]",
		},
		{
			name:  "wildcard with other selectors",
			input: `$[*,"name",0]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify at least one wildcard selector exists
			hasWildcard := false
			for _, seg := range query.Segments() {
				for _, sel := range seg.Selectors() {
					if sel.Kind == ast.Wildcard {
						hasWildcard = true
					}
				}
			}
			assert.True(t, hasWildcard, "expected at least one wildcard selector")
		})
	}
}

// TestParseFilterSelector tests parsing of filter selectors (placeholder).
func TestParseFilterSelector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "simple filter",
			input: "$[?@]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Filter, selectors[0].Kind)
		})
	}
}

// TestParseDescendantSegment tests parsing of descendant (..) segments.
func TestParseDescendantSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "descendant with name",
			input: "$..name",
		},
		{
			name:  "descendant with wildcard",
			input: "$..*",
		},
		{
			name:  "descendant with bracket",
			input: `$..["foo"]`,
		},
		{
			name:  "descendant with index",
			input: "$..[0]",
		},
		{
			name:  "multiple descendants",
			input: "$..foo..bar",
		},
		{
			name:  "descendant with multiple selectors",
			input: `$..["a","b",0]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify at least one descendant segment exists
			hasDescendant := false
			for _, seg := range query.Segments() {
				if seg.IsDescendant() {
					hasDescendant = true
				}
			}
			assert.True(t, hasDescendant, "expected at least one descendant segment")
		})
	}
}

// TestParseMixedSelectors tests parsing of mixed selector types.
func TestParseMixedSelectors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "name and index",
			input: `$["foo",0]`,
		},
		{
			name:  "name, index, and wildcard",
			input: `$["foo",0,*]`,
		},
		{
			name:  "complex path",
			input: `$.store.book[0].title`,
		},
		{
			name:  "mixed notation",
			input: `$["store"]["book"][0]["title"]`,
		},
		{
			name:  "descendant with mixed",
			input: `$..book[0,1]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, query.Segments())
		})
	}
}

// TestParseSingularQuery tests singular query validation.
func TestParseSingularQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		input        string
		wantSingular bool
	}{
		{
			name:         "single name selector",
			input:        `$["foo"]`,
			wantSingular: true,
		},
		{
			name:         "single index selector",
			input:        "$[0]",
			wantSingular: true,
		},
		{
			name:         "chained name selectors",
			input:        `$["a"]["b"]["c"]`,
			wantSingular: true,
		},
		{
			name:         "chained index selectors",
			input:        "$[0][1][2]",
			wantSingular: true,
		},
		{
			name:         "mixed name and index",
			input:        `$["foo"][0]["bar"]`,
			wantSingular: true,
		},
		{
			name:         "dot notation",
			input:        "$.foo.bar",
			wantSingular: true,
		},
		{
			name:         "wildcard is not singular",
			input:        "$[*]",
			wantSingular: false,
		},
		{
			name:         "slice is not singular",
			input:        "$[0:5]",
			wantSingular: false,
		},
		{
			name:         "multiple selectors not singular",
			input:        `$["a","b"]`,
			wantSingular: false,
		},
		{
			name:         "descendant not singular",
			input:        `$..foo`,
			wantSingular: false,
		},
		{
			name:         "filter not singular",
			input:        "$[?@]",
			wantSingular: false,
		},
		{
			name:         "singular then wildcard",
			input:        `$["foo"][*]`,
			wantSingular: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			assert.Equal(t, tt.wantSingular, query.IsSingular())
		})
	}
}

// TestParseErrorContextSnippet verifies error messages include source context.
func TestParseErrorContextSnippet(t *testing.T) {
	t.Parallel()

	p, err := New("$[0,*bar]", nil)
	require.NoError(t, err)

	_, err = p.Parse()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "near '")
	assert.Contains(t, err.Error(), "position")
}

// TestParseErrors tests various parse error conditions.
func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "missing root identifier",
			input: "foo",
		},
		{
			name:  "unclosed bracket",
			input: "$[0",
		},
		{
			name:  "unexpected token after path",
			input: "$ foo",
		},
		{
			name:  "invalid selector",
			input: "$[#]",
		},
		{
			name:  "dot without selector",
			input: "$.",
		},
		{
			name:  "double dot without selector",
			input: "$..",
		},
		{
			name:  "empty brackets",
			input: "$[]",
		},
		{
			name:  "trailing comma",
			input: `$["foo",]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			if err != nil {
				// Lexer error
				return
			}

			_, err = p.Parse()
			assert.Error(t, err, "expected parse error for input: %s", tt.input)
		})
	}
}

// TestParseStringRepresentation tests that parsed queries can be converted back to strings.
func TestParseStringRepresentation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		wantString string
	}{
		{
			name:       "simple name",
			input:      `$["foo"]`,
			wantString: `$["foo"]`,
		},
		{
			name:       "control name",
			input:      `$["\u000b"]`,
			wantString: `$["\u000b"]`,
		},
		{
			name:       "simple index",
			input:      "$[0]",
			wantString: "$[0]",
		},
		{
			name:       "wildcard",
			input:      "$[*]",
			wantString: "$[*]",
		},
		{
			name:       "slice",
			input:      "$[1:5]",
			wantString: "$[1:5]",
		},
		{
			name:       "slice with step",
			input:      "$[::2]",
			wantString: "$[::2]",
		},
		{
			name:       "multiple selectors",
			input:      `$["a","b",0]`,
			wantString: `$["a","b",0]`,
		},
		{
			name:       "descendant",
			input:      `$..["foo"]`,
			wantString: `$..["foo"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			assert.Equal(t, tt.wantString, query.String())
		})
	}
}

// TestParseSegmentTypes tests that segments are correctly identified as child or descendant.
func TestParseSegmentTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		input           string
		wantDescendants []bool
	}{
		{
			name:            "all child segments",
			input:           `$["a"]["b"]["c"]`,
			wantDescendants: []bool{false, false, false},
		},
		{
			name:            "all descendant segments",
			input:           `$..["a"]..["b"]..["c"]`,
			wantDescendants: []bool{true, true, true},
		},
		{
			name:            "mixed segments",
			input:           `$["a"]..["b"]["c"]`,
			wantDescendants: []bool{false, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.Len(t, segments, len(tt.wantDescendants))

			for i, wantDesc := range tt.wantDescendants {
				assert.Equal(t, wantDesc, segments[i].IsDescendant(),
					"segment %d: expected descendant=%v", i, wantDesc)
			}
		})
	}
}

// TestParseSelectorIsSingular tests the IsSingular method on individual selectors.
func TestParseSelectorIsSingular(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		input        string
		selectorIdx  int
		wantSingular bool
	}{
		{
			name:         "name selector is singular",
			input:        `$["foo"]`,
			selectorIdx:  0,
			wantSingular: true,
		},
		{
			name:         "index selector is singular",
			input:        "$[0]",
			selectorIdx:  0,
			wantSingular: true,
		},
		{
			name:         "wildcard is not singular",
			input:        "$[*]",
			selectorIdx:  0,
			wantSingular: false,
		},
		{
			name:         "slice is not singular",
			input:        "$[0:5]",
			selectorIdx:  0,
			wantSingular: false,
		},
		{
			name:         "filter is not singular",
			input:        "$[?@]",
			selectorIdx:  0,
			wantSingular: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.NotEmpty(t, segments)

			selectors := segments[0].Selectors()
			require.Greater(t, len(selectors), tt.selectorIdx)

			assert.Equal(t, tt.wantSingular, selectors[tt.selectorIdx].IsSingular())
		})
	}
}

// TestParseComplexPaths tests parsing of complex real-world JSONPath expressions.
func TestParseComplexPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "store example",
			input: "$.store.book[0].title",
		},
		{
			name:  "all books",
			input: "$..book[*]",
		},
		{
			name:  "all authors",
			input: "$..author",
		},
		{
			name:  "all prices",
			input: "$.store..price",
		},
		{
			name:  "third book",
			input: "$..book[2]",
		},
		{
			name:  "last book",
			input: "$..book[-1]",
		},
		{
			name:  "first two books",
			input: "$..book[0:2]",
		},
		{
			name:  "all with wildcard",
			input: "$..*",
		},
		{
			name:  "deeply nested",
			input: `$["a"]["b"]["c"]["d"]["e"]["f"]`,
		},
		{
			name:  "mixed everything",
			input: `$.store..book[0,1]["title","author"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, nil)
			require.NoError(t, err)

			query, err := p.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, query)

			// Verify the query can be converted to string
			str := query.String()
			assert.NotEmpty(t, str)
		})
	}
}

// TestParseFilterComparison tests filter expressions with comparison operators.
func TestParseFilterComparison(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{name: "greater than int", input: "$[?@.price > 10]"},
		{name: "equal string", input: `$[?@.name == "foo"]`},
		{name: "equal true", input: "$[?@.active == true]"},
		{name: "equal null", input: "$[?@.val == null]"},
		{name: "greater equal float", input: "$[?@.price >= 10.5]"},
		{name: "less than", input: "$[?@.age < 30]"},
		{name: "less equal", input: "$[?@.age <= 30]"},
		{name: "not equal", input: `$[?@.name != "bar"]`},
		{name: "equal int", input: "$[?@.count == 0]"},
		{name: "equal false", input: "$[?@.active == false]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, testFuncs())
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Filter, selectors[0].Kind)
		})
	}
}

// TestParseLiteralComparison tests filter expressions with a literal on the left side.
func TestParseLiteralComparison(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{name: "string equals query", input: `$[?"foo" == @.name]`},
		{name: "int less than query", input: "$[?10 < @.price]"},
		{name: "true equals query", input: "$[?true == @.active]"},
		{name: "null equals query", input: "$[?null == @.val]"},
		{name: "float greater than query", input: "$[?9.99 > @.price]"},
		{name: "false not equal query", input: "$[?false != @.active]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, testFuncs())
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Filter, selectors[0].Kind)
		})
	}
}

// TestParseFunctionCall tests parsing of function call expressions in filters.
func TestParseFunctionCall(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{name: "length comparison", input: "$[?length(@.name) > 3]"},
		{name: "match test", input: `$[?match(@.name, "foo.*")]`},
		{name: "search test", input: `$[?search(@.name, "bar")]`},
		{name: "count comparison", input: "$[?count(@.items[*]) > 0]"},
		{name: "value comparison", input: `$[?value(@.items[0]) == "x"]`},
		{name: "length equals zero", input: "$[?length(@.name) == 0]"},
		{name: "count less than", input: "$[?count(@.items[*]) < 5]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, testFuncs())
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Filter, selectors[0].Kind)
		})
	}
}

func TestParseNodeFunctionAsTest(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"$[?nodes(@[*])]",
		"$[?!nodes(@[*])]",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			p, err := New(input, testFuncsWithNodes())
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Filter, selectors[0].Kind)
		})
	}
}

// TestParseLogicalExpressions tests complex logical filter expressions.
func TestParseCountUsesNodeArgumentForSingularQuery(t *testing.T) {
	t.Parallel()

	p, err := New("$[?count(@.items[0]) == 1]", testFuncs())
	require.NoError(t, err)

	query, err := p.Parse()
	require.NoError(t, err)

	segments := query.Segments()
	require.Len(t, segments, 1)

	selectors := segments[0].Selectors()
	require.Len(t, selectors, 1)
	require.Equal(t, ast.Filter, selectors[0].Kind)
	require.NotNil(t, selectors[0].Filter)
	require.Len(t, selectors[0].Filter.Or, 1)
	require.Len(t, selectors[0].Filter.Or[0], 1)

	comp, ok := selectors[0].Filter.Or[0][0].(*ast.CompExpr)
	require.True(t, ok)

	left, ok := comp.Left.(*ast.FuncValue)
	require.True(t, ok)
	require.Equal(t, "count", left.Func.Name())

	current := map[string]any{"items": []any{[]any{"a", "b"}}}
	assert.True(t, comp.Eval(current, current))
}

func TestParseLogicalExpressions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{name: "logical and", input: "$[?@.a && @.b]"},
		{name: "logical or", input: "$[?@.a || @.b]"},
		{name: "and then or", input: "$[?@.a && @.b || @.c]"},
		{name: "parenthesized or then and", input: "$[?(@.a || @.b) && @.c]"},
		{name: "negated existence", input: "$[?!@.a]"},
		{name: "negated parenthesized", input: "$[?!(@.a && @.b)]"},
		{name: "negated function call", input: `$[?!match(@.name, "foo")]`},
		{name: "double negation paren", input: "$[?!(!@.a)]"},
		{name: "complex nested", input: "$[?(@.a > 1 && @.b < 2) || @.c == true]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, testFuncsWithNodes())
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Filter, selectors[0].Kind)
		})
	}
}

// TestParseFilterErrors tests error cases in filter expression parsing.
func TestParseFilterErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{name: "missing right side of comparison", input: "$[?@.a > ]"},
		{name: "unknown function", input: "$[?unknown(@.a)]"},
		{name: "wrong arg count for length", input: "$[?length(@.a, @.b)]"},
		{name: "wrong arg count for match", input: `$[?match(@.a)]`},
		{name: "wrong arg count for count", input: "$[?count(@.a, @.b)]"},
		{name: "logical function in comparison", input: `$[?match(@.a, "b") > 1]`},
		{name: "nodes function in comparison", input: "$[?nodes(@[*]) == 1]"},
		{name: "nodes function on right side of comparison", input: "$[?1 == nodes(@[*])]"},
		{name: "value function without comparison", input: "$[?value(@.a)]"},
		{name: "non-singular query in comparison", input: "$[?@[*] > 1]"},
		{name: "non-singular query on right side of comparison", input: "$[?1 < @[*]]"},
		{name: "literal without comparison op", input: `$[?"foo"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := New(tt.input, testFuncs())
			if err != nil {
				return
			}

			_, err = p.Parse()
			assert.Error(t, err, "expected parse error for input: %s", tt.input)
		})
	}
}

func TestParseComparisonValueVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "right side value function", input: `$[?@.name == value(@.aliases[0])]`},
		{name: "nested function argument", input: `$[?length(value(@.name)) > 0]`},
		{name: "root query comparison value", input: `$[?@.name == $.selected]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := New(tt.input, testFuncs())
			require.NoError(t, err)

			query, err := p.Parse()
			require.NoError(t, err)

			segments := query.Segments()
			require.Len(t, segments, 1)

			selectors := segments[0].Selectors()
			require.Len(t, selectors, 1)
			assert.Equal(t, ast.Filter, selectors[0].Kind)
		})
	}
}

func TestParseIndexAndSliceBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "negative zero index", input: "$[-0]"},
		{name: "index above JSON integer range", input: "$[9007199254740992]"},
		{name: "negative zero slice end", input: "$[0:-0]"},
		{name: "negative zero slice step", input: "$[0:1:-0]"},
		{name: "slice end above JSON integer range", input: "$[0:9007199254740992]"},
		{name: "slice step below JSON integer range", input: "$[0:1:-9007199254740992]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := New(tt.input, testFuncs())
			require.NoError(t, err)

			_, err = p.Parse()
			require.Error(t, err)
		})
	}
}

func TestParseErrorSentinels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		funcs map[string]ast.Function
		want  error
	}{
		{name: "end of input", input: "$.", want: ErrParseEnd},
		{name: "specific position", input: " $", want: ErrParsePosition},
		{name: "unknown function", input: "$[?unknown(@)]", funcs: testFuncs(), want: ErrUnknownFunction},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := New(tt.input, tt.funcs)
			require.NoError(t, err)

			_, err = p.Parse()
			require.ErrorIs(t, err, tt.want)
		})
	}
}
