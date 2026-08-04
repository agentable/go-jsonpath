// Package functions provides the RFC 9535 §2.4 built-in function
// implementations for JSONPath filter expressions.
package functions

import (
	"regexp"
	"regexp/syntax"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/agentable/go-jsonpath/internal/ast"
)

// reCacheMaxSize is the maximum number of compiled regexes to cache.
const reCacheMaxSize = 1024

// reCache caches compiled regular expressions keyed by pattern string.
// Bounded to reCacheMaxSize entries; when full, the cache is cleared.
var reCache sync.Map

// reCacheSize tracks the approximate number of entries in the cache.
var reCacheSize atomic.Int64

type regexpCacheKey struct {
	pattern  string
	anchored bool
}

// Builtins returns the five RFC 9535 §2.4 built-in function implementations.
func Builtins() []ast.Function {
	return []ast.Function{
		&LengthFunc{},
		&CountFunc{},
		&MatchFunc{},
		&SearchFunc{},
		&ValueFunc{},
	}
}

// LengthFunc implements the RFC 9535 §2.4.4 length() function.
// It returns the number of Unicode scalar values in a string, array
// elements, or object members, and nil for other inputs.
type LengthFunc struct{}

// Name returns the RFC 9535 function name.
func (LengthFunc) Name() string { return "length" }

// ResultType returns the function result type.
func (LengthFunc) ResultType() ast.FuncType { return ast.Value }

// ParameterCount returns the fixed arity.
func (LengthFunc) ParameterCount() int { return 1 }

// ParameterType returns the parameter type.
func (LengthFunc) ParameterType(int) ast.FuncType { return ast.Value }

// Call returns the length of the argument:
//   - string: number of Unicode scalar values
//   - []any: number of elements
//   - map[string]any: number of members
//   - nil or other: nil
func (LengthFunc) Call(args []ast.FunctionValue) ast.FunctionValue {
	if len(args) == 0 {
		return ast.NoValue()
	}
	value, ok := args[0].(ast.TypedValue)
	if !ok || value.IsNothing() {
		return ast.NoValue()
	}
	switch v := value.Any().(type) {
	case string:
		return ast.NewValue(utf8.RuneCountInString(v))
	case []any:
		return ast.NewValue(len(v))
	case map[string]any:
		return ast.NewValue(len(v))
	default:
		return ast.NoValue()
	}
}

// CountFunc implements the RFC 9535 §2.4.6 count() function.
// It returns the number of nodes in a node list argument.
type CountFunc struct{}

// Name returns the RFC 9535 function name.
func (CountFunc) Name() string { return "count" }

// ResultType returns the function result type.
func (CountFunc) ResultType() ast.FuncType { return ast.Value }

// ParameterCount returns the fixed arity.
func (CountFunc) ParameterCount() int { return 1 }

// ParameterType returns the parameter type.
func (CountFunc) ParameterType(int) ast.FuncType { return ast.Nodes }

// Call returns the number of nodes in the node list argument.
func (CountFunc) Call(args []ast.FunctionValue) ast.FunctionValue {
	if len(args) == 0 {
		return ast.NewValue(0)
	}
	if nodes, ok := args[0].(ast.TypedNodes); ok {
		return ast.NewValue(len(nodes))
	}
	return ast.NewValue(0)
}

// MatchFunc implements the RFC 9535 §2.4.7 match() function.
// It reports whether the string argument fully matches the regex pattern.
type MatchFunc struct{}

// Name returns the RFC 9535 function name.
func (MatchFunc) Name() string { return "match" }

// ResultType returns the function result type.
func (MatchFunc) ResultType() ast.FuncType { return ast.Logical }

// ParameterCount returns the fixed arity.
func (MatchFunc) ParameterCount() int { return 2 }

// ParameterType returns the parameter type.
func (MatchFunc) ParameterType(int) ast.FuncType { return ast.Value }

// Call returns true if the string argument fully matches the regex pattern.
// Returns false if either argument is not a string or the regex is invalid.
func (MatchFunc) Call(args []ast.FunctionValue) ast.FunctionValue {
	return ast.TypedLogical(callRegexp(args, true))
}

// SearchFunc implements the RFC 9535 §2.4.7 search() function.
// It reports whether the string argument contains a substring matching the
// regex pattern.
type SearchFunc struct{}

// Name returns the RFC 9535 function name.
func (SearchFunc) Name() string { return "search" }

// ResultType returns the function result type.
func (SearchFunc) ResultType() ast.FuncType { return ast.Logical }

// ParameterCount returns the fixed arity.
func (SearchFunc) ParameterCount() int { return 2 }

// ParameterType returns the parameter type.
func (SearchFunc) ParameterType(int) ast.FuncType { return ast.Value }

// Call returns true if the string argument contains a match for the regex pattern.
// Returns false if either argument is not a string or the regex is invalid.
func (SearchFunc) Call(args []ast.FunctionValue) ast.FunctionValue {
	return ast.TypedLogical(callRegexp(args, false))
}

// ValueFunc implements the RFC 9535 §2.4.8 value() function.
// It returns the value of a single-node list and nil for empty or multi-node
// lists.
type ValueFunc struct{}

// Name returns the RFC 9535 function name.
func (ValueFunc) Name() string { return "value" }

// ResultType returns the function result type.
func (ValueFunc) ResultType() ast.FuncType { return ast.Value }

// ParameterCount returns the fixed arity.
func (ValueFunc) ParameterCount() int { return 1 }

// ParameterType returns the parameter type.
func (ValueFunc) ParameterType(int) ast.FuncType { return ast.Nodes }

// Call returns the value of the single node in the node list, or nil if
// the list is empty or contains more than one node.
func (ValueFunc) Call(args []ast.FunctionValue) ast.FunctionValue {
	if len(args) == 0 {
		return ast.NoValue()
	}
	nodes, ok := args[0].(ast.TypedNodes)
	if !ok || len(nodes) != 1 {
		return ast.NoValue()
	}
	return ast.NewValue(nodes[0])
}

// compileIRegexp compiles an I-Regexp pattern (RFC 9485) into a Go *regexp.Regexp.
// It replaces "." (OpAnyChar) with "[^\n\r]" per RFC 9485 §5.
// Results are cached with a bounded cache for concurrent safety.
// Returns nil if the pattern is invalid.
func compileIRegexp(pattern string) *regexp.Regexp {
	return compileIRegexpMode(pattern, false)
}

func compileIRegexpMode(pattern string, anchored bool) *regexp.Regexp {
	cacheKey := regexpCacheKey{pattern: pattern, anchored: anchored}
	if v, ok := reCache.Load(cacheKey); ok {
		return v.(*regexp.Regexp)
	}
	re, err := compileIRegexpUncached(pattern, anchored)
	if err != nil {
		return nil
	}
	// Evict all entries when cache is full to bound memory usage.
	if reCacheSize.Load() >= reCacheMaxSize {
		reCache.Clear()
		reCacheSize.Store(0)
	}
	actual, loaded := reCache.LoadOrStore(cacheKey, re)
	if loaded {
		return actual.(*regexp.Regexp)
	}
	reCacheSize.Add(1)
	return re
}

var nonLineBreak = mustParseSyntax(`[^\n\r]`, syntax.Perl)

// mustParseSyntax parses a constant regex pattern or panics.
func mustParseSyntax(pattern string, flags syntax.Flags) *syntax.Regexp {
	re, err := syntax.Parse(pattern, flags)
	if err != nil {
		panic("functions: bad constant pattern: " + err.Error())
	}
	return re
}

// compileIRegexpUncached compiles an I-Regexp pattern without caching.
func compileIRegexpUncached(pattern string, anchored bool) (*regexp.Regexp, error) {
	if err := checkIRegexp(pattern); err != nil {
		return nil, err
	}
	if anchored {
		pattern = anchorIRegexp(pattern)
	}
	parsed, err := syntax.Parse(pattern, syntax.Perl|syntax.DotNL)
	if err != nil {
		return nil, err
	}
	replaceDot(parsed)
	return regexp.Compile(parsed.String())
}

// callRegexp evaluates match() and search() style regex calls.
func callRegexp(args []ast.FunctionValue, anchored bool) bool {
	str, pattern, ok := regexpArgs(args)
	if !ok {
		return false
	}
	re := compileIRegexpMode(pattern, anchored)
	return re != nil && re.MatchString(str)
}

func anchorIRegexp(pattern string) string {
	return `\A(?:` + pattern + `)\z`
}

func regexpArgs(args []ast.FunctionValue) (string, string, bool) {
	if len(args) < 2 {
		return "", "", false
	}
	strValue, ok := args[0].(ast.TypedValue)
	if !ok || strValue.IsNothing() {
		return "", "", false
	}
	str, ok := strValue.Any().(string)
	if !ok {
		return "", "", false
	}
	patternValue, ok := args[1].(ast.TypedValue)
	if !ok || patternValue.IsNothing() {
		return "", "", false
	}
	pattern, ok := patternValue.Any().(string)
	if !ok {
		return "", "", false
	}
	return str, pattern, true
}

// replaceDot recursively replaces all OpAnyChar nodes with [^\n\r] nodes
// to comply with RFC 9485 I-Regexp semantics.
func replaceDot(re *syntax.Regexp) {
	if re.Op == syntax.OpAnyChar {
		*re = *nonLineBreak
		return
	}
	for _, sub := range re.Sub {
		replaceDot(sub)
	}
}
