package functions

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentable/go-jsonpath/internal/ast"
)

func clearRegexCache() {
	reCache.Range(func(key, value any) bool {
		reCache.Delete(key)
		return true
	})
	reCacheSize.Store(0)
}

func valueArgs(values ...any) []ast.FunctionValue {
	args := make([]ast.FunctionValue, len(values))
	for i, value := range values {
		args[i] = ast.NewValue(value)
	}
	return args
}

func nodesArgs(nodes ...any) []ast.FunctionValue {
	return []ast.FunctionValue{ast.TypedNodes(nodes)}
}

func assertValueResult(t *testing.T, got ast.FunctionValue, want any, wantNothing bool) {
	t.Helper()

	value, ok := got.(ast.TypedValue)
	require.True(t, ok)
	assert.Equal(t, wantNothing, value.IsNothing())
	if !wantNothing {
		assert.Equal(t, want, value.Any())
	}
}

func assertLogicalResult(t *testing.T, got ast.FunctionValue, want bool) {
	t.Helper()

	logical, ok := got.(ast.TypedLogical)
	require.True(t, ok)
	assert.Equal(t, want, logical.Bool())
}

func TestClearRegexCache(t *testing.T) {
	// Mutates the package-level regexp cache.
	clearRegexCache()
	t.Cleanup(clearRegexCache)

	pattern := "clear.*cache"
	otherPattern := "cache.*size"

	require.NotNil(t, compileIRegexp(pattern))
	require.NotNil(t, compileIRegexp(otherPattern))

	_, ok := reCache.Load(regexpCacheKey{pattern: pattern})
	require.True(t, ok)
	_, ok = reCache.Load(regexpCacheKey{pattern: otherPattern})
	require.True(t, ok)
	require.GreaterOrEqual(t, reCacheSize.Load(), int64(2))

	clearRegexCache()

	_, ok = reCache.Load(regexpCacheKey{pattern: pattern})
	assert.False(t, ok)
	_, ok = reCache.Load(regexpCacheKey{pattern: otherPattern})
	assert.False(t, ok)
	assert.Zero(t, reCacheSize.Load())
}

func TestMustParseSyntaxPanicPreservesCause(t *testing.T) {
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		mustParseSyntax("[", syntax.Perl)
	}()

	panicErr, ok := recovered.(error)
	require.True(t, ok, "panic value = %#v, want error", recovered)
	assert.ErrorContains(t, panicErr, "go-jsonpath/internal/functions: parse fixed regexp syntax")
	var syntaxErr *syntax.Error
	require.ErrorAs(t, panicErr, &syntaxErr)
}

func TestBuiltins(t *testing.T) {
	t.Parallel()
	fns := Builtins()
	require.Len(t, fns, 5)

	names := make([]string, len(fns))
	for i, fn := range fns {
		names[i] = fn.Name()
	}
	if diff := cmp.Diff([]string{"length", "count", "match", "search", "value"}, names); diff != "" {
		t.Errorf("Builtins() names mismatch (-want +got):\n%s", diff)
	}
}

func TestLengthFunc(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		args        []ast.FunctionValue
		want        any
		wantNothing bool
	}{
		{name: "empty_string", args: valueArgs(""), want: 0},
		{name: "ascii_string", args: valueArgs("abc def"), want: 7},
		{name: "unicode_string", args: valueArgs("foö"), want: 3},
		{name: "emoji_string", args: valueArgs("Hi 👋🏻"), want: 5},
		{name: "empty_array", args: valueArgs([]any{}), want: 0},
		{name: "array", args: valueArgs([]any{1, 2, 3, 4, 5}), want: 5},
		{name: "nested_array", args: valueArgs([]any{1, 2, []any{3, 4}}), want: 3},
		{name: "empty_object", args: valueArgs(map[string]any{}), want: 0},
		{name: "object", args: valueArgs(map[string]any{"x": 1, "y": 2, "z": 3}), want: 3},
		{name: "integer", args: valueArgs(42), wantNothing: true},
		{name: "float", args: valueArgs(3.14), wantNothing: true},
		{name: "bool", args: valueArgs(true), wantNothing: true},
		{name: "nil_arg", args: valueArgs(nil), wantNothing: true},
		{name: "no_args", args: nil, wantNothing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertValueResult(t, LengthFunc{}.Call(tc.args), tc.want, tc.wantNothing)
		})
	}
}

func TestCountFunc(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []ast.FunctionValue
		want any
	}{
		{name: "empty_nodes", args: nodesArgs(), want: 0},
		{name: "one_node", args: nodesArgs(1), want: 1},
		{name: "three_nodes", args: nodesArgs(1, true, nil), want: 3},
		{name: "nil_arg", args: valueArgs(nil), want: 0},
		{name: "not_nodes", args: valueArgs("hello"), want: 0},
		{name: "no_args", args: nil, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertValueResult(t, CountFunc{}.Call(tc.args), tc.want, false)
		})
	}
}

func TestMatchFunc(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []ast.FunctionValue
		want bool
	}{
		{name: "full_match", args: valueArgs("foo", "foo"), want: true},
		{name: "dot_star", args: valueArgs("foo", ".*"), want: true},
		{name: "dot_single", args: valueArgs("x", "."), want: true},
		{name: "dot_two_chars", args: valueArgs("xx", "."), want: false},
		{name: "alternation_requires_full_match", args: valueArgs("ab", "a|b"), want: false},
		{name: "no_match", args: valueArgs("foo", "bar"), want: false},
		{name: "partial_not_full", args: valueArgs("foobar", "foo"), want: false},
		{name: "multiline_newline", args: valueArgs("xx\nyz", ".*"), want: false},
		{name: "multiline_crlf", args: valueArgs("xx\r\nyz", ".*"), want: false},
		{name: "not_string_input", args: valueArgs(42, "."), want: false},
		{name: "not_string_pattern", args: valueArgs("x", 42), want: false},
		{name: "invalid_regex", args: valueArgs("x", ".["), want: false},
		{name: "no_args", args: nil, want: false},
		{name: "one_arg", args: valueArgs("foo"), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertLogicalResult(t, MatchFunc{}.Call(tc.args), tc.want)
		})
	}
}

func TestSearchFunc(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []ast.FunctionValue
		want bool
	}{
		{name: "found", args: valueArgs("foobar", "bar"), want: true},
		{name: "dot", args: valueArgs("x", "."), want: true},
		{name: "dot_in_longer", args: valueArgs("xx", "."), want: true},
		{name: "no_match", args: valueArgs("foo", "baz"), want: false},
		{name: "multiline_partial", args: valueArgs("xx\nyz", "xx"), want: true},
		{name: "multiline_dot_star", args: valueArgs("xx\nyz", ".*"), want: true},
		{name: "not_string_input", args: valueArgs(42, "."), want: false},
		{name: "not_string_pattern", args: valueArgs("x", 42), want: false},
		{name: "invalid_regex", args: valueArgs("x", ".["), want: false},
		{name: "no_args", args: nil, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertLogicalResult(t, SearchFunc{}.Call(tc.args), tc.want)
		})
	}
}

func TestValueFunc(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		args        []ast.FunctionValue
		want        any
		wantNothing bool
	}{
		{name: "single_node", args: nodesArgs(42), want: 42},
		{name: "single_string", args: nodesArgs("hello"), want: "hello"},
		{name: "single_nil", args: nodesArgs(nil), want: nil},
		{name: "empty_nodes", args: nodesArgs(), wantNothing: true},
		{name: "multiple_nodes", args: nodesArgs(1, 2, 3), wantNothing: true},
		{name: "nil_arg", args: valueArgs(nil), wantNothing: true},
		{name: "not_nodes", args: valueArgs("hello"), wantNothing: true},
		{name: "no_args", args: nil, wantNothing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertValueResult(t, ValueFunc{}.Call(tc.args), tc.want, tc.wantNothing)
		})
	}
}

func TestCompileIRegexpCache(t *testing.T) {
	t.Parallel()

	// Clear cache for this test.
	clearRegexCache()

	re1 := compileIRegexp("abc")
	require.NotNil(t, re1)

	// Second call should return cached value.
	re2 := compileIRegexp("abc")
	require.NotNil(t, re2)
	assert.Equal(t, re1, re2)

	// Invalid pattern returns nil and is not cached.
	reInvalid := compileIRegexp(".[")
	assert.Nil(t, reInvalid)
	_, loaded := reCache.Load(regexpCacheKey{pattern: ".["})
	assert.False(t, loaded)
}

func TestReplaceDotIRegexp(t *testing.T) {
	t.Parallel()

	// "." should NOT match \n or \r per RFC 9485.
	re := compileIRegexp(".")
	require.NotNil(t, re)
	assert.True(t, re.MatchString("x"))
	assert.False(t, re.MatchString("\n"))
	assert.False(t, re.MatchString("\r"))
}

func TestCompileIRegexpConcurrent(t *testing.T) {
	t.Parallel()

	clearRegexCache()

	var wg sync.WaitGroup
	patterns := []string{"a+", "b+", "c+", "d+", "e+"}
	for _, p := range patterns {
		wg.Add(1)
		go func(pattern string) {
			defer wg.Done()
			re := compileIRegexp(pattern)
			assert.NotNil(t, re)
			assert.IsType(t, &regexp.Regexp{}, re)
		}(p)
	}
	wg.Wait()
}

func TestRegexCacheBehavior(t *testing.T) {
	t.Parallel()

	t.Run("cache_stores_compiled_regex", func(t *testing.T) {
		clearRegexCache()
		pattern := "test.*pattern"

		// First compilation
		re1 := compileIRegexp(pattern)
		require.NotNil(t, re1)

		// Verify it's in cache
		cached, ok := reCache.Load(regexpCacheKey{pattern: pattern})
		require.True(t, ok)
		assert.Same(t, re1, cached)
	})

	t.Run("cache_returns_same_instance", func(t *testing.T) {
		clearRegexCache()
		pattern := "same.*instance"

		re1 := compileIRegexp(pattern)
		re2 := compileIRegexp(pattern)
		re3 := compileIRegexp(pattern)

		require.NotNil(t, re1)
		require.NotNil(t, re2)
		require.NotNil(t, re3)

		// All should be the exact same pointer
		assert.Same(t, re1, re2)
		assert.Same(t, re1, re3)
	})

	t.Run("cache_handles_different_patterns", func(t *testing.T) {
		clearRegexCache()

		re1 := compileIRegexp("pattern1")
		re2 := compileIRegexp("pattern2")
		re3 := compileIRegexp("pattern3")

		require.NotNil(t, re1)
		require.NotNil(t, re2)
		require.NotNil(t, re3)

		// All should be different instances
		assert.NotSame(t, re1, re2)
		assert.NotSame(t, re1, re3)
		assert.NotSame(t, re2, re3)
	})

	t.Run("invalid_pattern_not_cached", func(t *testing.T) {
		clearRegexCache()
		invalidPattern := "["

		re := compileIRegexp(invalidPattern)
		assert.Nil(t, re)

		// Should not be in cache
		_, ok := reCache.Load(regexpCacheKey{pattern: invalidPattern})
		assert.False(t, ok)

		// Second call should also return nil
		re2 := compileIRegexp(invalidPattern)
		assert.Nil(t, re2)
	})

	t.Run("match_function_uses_cache", func(t *testing.T) {
		clearRegexCache()
		fn := MatchFunc{}

		pattern := "hello"
		// First call compiles and caches
		result1 := fn.Call(valueArgs("hello", pattern))
		assertLogicalResult(t, result1, true)

		// Verify anchored pattern is cached
		cached, ok := reCache.Load(regexpCacheKey{pattern: pattern, anchored: true})
		require.True(t, ok)
		require.NotNil(t, cached)

		// Second call uses cache
		result2 := fn.Call(valueArgs("hello", pattern))
		assertLogicalResult(t, result2, true)

		// Third call with different input but same pattern
		result3 := fn.Call(valueArgs("world", pattern))
		assertLogicalResult(t, result3, false)
	})

	t.Run("search_function_uses_cache", func(t *testing.T) {
		clearRegexCache()
		fn := SearchFunc{}

		pattern := "world"

		// First call compiles and caches
		result1 := fn.Call(valueArgs("hello world", pattern))
		assertLogicalResult(t, result1, true)

		// Verify pattern is cached
		cached, ok := reCache.Load(regexpCacheKey{pattern: pattern})
		require.True(t, ok)
		require.NotNil(t, cached)

		// Second call uses cache
		result2 := fn.Call(valueArgs("world hello", pattern))
		assertLogicalResult(t, result2, true)
	})

	t.Run("concurrent_cache_access_same_pattern", func(t *testing.T) {
		clearRegexCache()
		pattern := "concurrent.*test"

		var wg sync.WaitGroup
		results := make([]*regexp.Regexp, 20)

		// Launch 20 goroutines compiling the same pattern
		for i := range 20 {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx] = compileIRegexp(pattern)
			}(i)
		}
		wg.Wait()

		// All should be non-nil
		for i, re := range results {
			require.NotNil(t, re, "result %d should not be nil", i)
		}

		cached, ok := reCache.Load(regexpCacheKey{pattern: pattern})
		require.True(t, ok)
		require.NotNil(t, cached)

		for _, re := range results {
			assert.Same(t, cached, re)
			assert.True(t, re.MatchString("concurrent test"))
			assert.True(t, re.MatchString("concurrent xyz test"))
		}
	})

	t.Run("concurrent_cache_access_different_patterns", func(t *testing.T) {
		clearRegexCache()
		patterns := []string{
			"pattern1", "pattern2", "pattern3", "pattern4", "pattern5",
			"pattern6", "pattern7", "pattern8", "pattern9", "pattern10",
		}

		var wg sync.WaitGroup
		results := make(map[string]*regexp.Regexp)
		var mu sync.Mutex

		for _, p := range patterns {
			wg.Add(1)
			go func(pattern string) {
				defer wg.Done()
				re := compileIRegexp(pattern)
				mu.Lock()
				results[pattern] = re
				mu.Unlock()
			}(p)
		}
		wg.Wait()

		// All patterns should be compiled
		assert.Equal(t, len(patterns), len(results))

		// All should be non-nil
		for pattern, re := range results {
			require.NotNil(t, re, "pattern %s should compile", pattern)
		}

		// Verify all are in cache
		for _, pattern := range patterns {
			cached, ok := reCache.Load(regexpCacheKey{pattern: pattern})
			require.True(t, ok, "pattern %s should be cached", pattern)
			require.NotNil(t, cached)
		}
	})
}

func TestRegexpCacheSeparatesMatchAndSearchPatterns(t *testing.T) {
	clearRegexCache()
	t.Cleanup(clearRegexCache)

	assertLogicalResult(t, MatchFunc{}.Call(valueArgs("foo", "foo")), true)
	assertLogicalResult(t, SearchFunc{}.Call(valueArgs("foo", anchorIRegexp("foo"))), false)
}

func TestRegexCacheBounds(t *testing.T) {
	// Mutates the package-level regexp cache.
	clearRegexCache()

	// Fill cache beyond the max size
	for i := range reCacheMaxSize + 10 {
		re := compileIRegexp(fmt.Sprintf("pattern_%d", i))
		require.NotNil(t, re)
	}

	// Cache should have been evicted and rebuilt; size should be small
	assert.True(t, reCacheSize.Load() <= reCacheMaxSize,
		"cache size %d should be <= %d", reCacheSize.Load(), reCacheMaxSize)
}

func TestCompileIRegexpCacheEvictionClearsEntries(t *testing.T) {
	// Mutates the package-level regexp cache.
	clearRegexCache()
	t.Cleanup(clearRegexCache)

	for i := range reCacheMaxSize {
		pattern := fmt.Sprintf("pattern_%d", i)
		require.NotNil(t, compileIRegexp(pattern))
	}

	_, ok := reCache.Load(regexpCacheKey{pattern: "pattern_0"})
	require.True(t, ok)

	require.NotNil(t, compileIRegexp("pattern_overflow"))

	_, ok = reCache.Load(regexpCacheKey{pattern: "pattern_0"})
	assert.False(t, ok)
	_, ok = reCache.Load(regexpCacheKey{pattern: "pattern_overflow"})
	assert.True(t, ok)
	assert.Equal(t, int64(1), reCacheSize.Load())
}

func TestIRegexpCompliance(t *testing.T) {
	t.Parallel()

	t.Run("dot_does_not_match_newline", func(t *testing.T) {
		re := compileIRegexp("a.b")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("axb"))
		assert.True(t, re.MatchString("a b"))
		assert.True(t, re.MatchString("a\tb"))
		assert.False(t, re.MatchString("a\nb"))
		assert.False(t, re.MatchString("a\rb"))
	})

	t.Run("dot_star_does_not_match_across_newlines", func(t *testing.T) {
		re := compileIRegexp("a.*b")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("ab"))
		assert.True(t, re.MatchString("axyzb"))
		assert.False(t, re.MatchString("a\nb"))
		assert.False(t, re.MatchString("a\r\nb"))
		assert.False(t, re.MatchString("axy\nzb"))
	})

	t.Run("multiple_dots", func(t *testing.T) {
		re := compileIRegexp("^...$")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("abc"))
		assert.True(t, re.MatchString("xyz"))
		assert.False(t, re.MatchString("ab\n"))
		assert.False(t, re.MatchString("a\nbc"))
		assert.False(t, re.MatchString("\nabc"))
	})

	t.Run("character_classes_unaffected", func(t *testing.T) {
		re := compileIRegexp("[abc]")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("a"))
		assert.True(t, re.MatchString("b"))
		assert.True(t, re.MatchString("c"))
		assert.False(t, re.MatchString("d"))
	})

	t.Run("negated_character_classes", func(t *testing.T) {
		re := compileIRegexp("[^abc]")
		require.NotNil(t, re)

		assert.False(t, re.MatchString("a"))
		assert.False(t, re.MatchString("b"))
		assert.True(t, re.MatchString("d"))
		assert.True(t, re.MatchString("x"))
	})

	t.Run("anchors", func(t *testing.T) {
		re := compileIRegexp("^hello$")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("hello"))
		assert.False(t, re.MatchString("hello world"))
		assert.False(t, re.MatchString("say hello"))
	})

	t.Run("quantifiers", func(t *testing.T) {
		tests := []struct {
			pattern string
			input   string
			want    bool
		}{
			{"^a+$", "a", true},
			{"^a+$", "aaa", true},
			{"^a+$", "", false},
			{"^a*$", "", true},
			{"^a*$", "aaa", true},
			{"^a?$", "", true},
			{"^a?$", "a", true},
			{"^a?$", "aa", false},
			{"^a{2}$", "aa", true},
			{"^a{2}$", "a", false},
			{"^a{2,4}$", "aa", true},
			{"^a{2,4}$", "aaa", true},
			{"^a{2,4}$", "aaaa", true},
			{"^a{2,4}$", "a", false},
			{"^a{2,4}$", "aaaaa", false},
		}

		for _, tt := range tests {
			re := compileIRegexp(tt.pattern)
			require.NotNil(t, re, "pattern %s should compile", tt.pattern)
			got := re.MatchString(tt.input)
			assert.Equal(t, tt.want, got, "pattern %s with input %q", tt.pattern, tt.input)
		}
	})

	t.Run("alternation", func(t *testing.T) {
		re := compileIRegexp("cat|dog")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("cat"))
		assert.True(t, re.MatchString("dog"))
		assert.False(t, re.MatchString("bird"))
	})

	t.Run("unicode_support", func(t *testing.T) {
		re := compileIRegexp("世界")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("世界"))
		assert.False(t, re.MatchString("世"))
		assert.False(t, re.MatchString("界"))
	})

	t.Run("unicode_with_dot", func(t *testing.T) {
		re := compileIRegexp("世.界")
		require.NotNil(t, re)

		assert.True(t, re.MatchString("世x界"))
		assert.True(t, re.MatchString("世 界"))
		assert.False(t, re.MatchString("世\n界"))
	})
}

func TestRegexpFunctionsRejectNonIRegexpSyntax(t *testing.T) {
	t.Parallel()

	patterns := []struct {
		name    string
		input   string
		pattern string
	}{
		{name: "inline flag", input: "HELLO", pattern: "(?i)hello"},
		{name: "scoped flag", input: "HELLO", pattern: "(?i:hello)"},
		{name: "non-capturing group", input: "hello", pattern: "(?:hello)"},
		{name: "multi-character escape", input: "1", pattern: `\d`},
		{name: "word boundary", input: "hello", pattern: `\bhello\b`},
		{name: "Unicode block", input: "α", pattern: `\p{Greek}`},
	}
	functions := []struct {
		name string
		call func([]ast.FunctionValue) ast.FunctionValue
	}{
		{name: "match", call: MatchFunc{}.Call},
		{name: "search", call: SearchFunc{}.Call},
	}

	for _, fn := range functions {
		for _, tt := range patterns {
			t.Run(fn.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				assertLogicalResult(t, fn.call(valueArgs(tt.input, tt.pattern)), false)
			})
		}
	}
}

func TestIRegexpCharacterClassGrammar(t *testing.T) {
	t.Parallel()

	valid := []string{
		`[-]`,
		`[-a]`,
		`[a-]`,
		`[a-z]`,
		`[^a]`,
		`[\[-\]]`,
		`[\p{L}\P{Nd}]`,
		`[\n-\r]`,
	}
	for _, pattern := range valid {
		t.Run("valid/"+pattern, func(t *testing.T) {
			t.Parallel()
			require.NotNil(t, compileIRegexp(pattern))
		})
	}

	invalid := []string{
		`[]`,
		`[^]`,
		`[---]`,
		`[a--b]`,
		`[a-b-c]`,
		`[a-\p{L}]`,
		`[\d]`,
		`[\w]`,
		`[\s]`,
		`[\p{Greek}]`,
		`[\P{L]`,
		`[a`,
		`[\]`,
	}
	for _, pattern := range invalid {
		t.Run("invalid/"+pattern, func(t *testing.T) {
			t.Parallel()
			assert.Nil(t, compileIRegexp(pattern))
		})
	}
}

func TestMatchFuncAnchoring(t *testing.T) {
	t.Parallel()

	fn := MatchFunc{}

	t.Run("full_match_required", func(t *testing.T) {
		// match() implicitly anchors with \A and \z
		assertLogicalResult(t, fn.Call(valueArgs("hello", "hello")), true)
		assertLogicalResult(t, fn.Call(valueArgs("hello world", "hello")), false)
		assertLogicalResult(t, fn.Call(valueArgs("say hello", "hello")), false)
		assertLogicalResult(t, fn.Call(valueArgs("say hello world", "hello")), false)
	})

	t.Run("pattern_must_match_entire_string", func(t *testing.T) {
		assertLogicalResult(t, fn.Call(valueArgs("abc", "abc")), true)
		assertLogicalResult(t, fn.Call(valueArgs("abc", "a.c")), true)
		assertLogicalResult(t, fn.Call(valueArgs("abc", ".*")), true)
		assertLogicalResult(t, fn.Call(valueArgs("abcd", "abc")), false)
		assertLogicalResult(t, fn.Call(valueArgs("xabc", "abc")), false)
	})
}

func TestSearchFuncNoAnchoring(t *testing.T) {
	t.Parallel()

	fn := SearchFunc{}

	t.Run("substring_match_allowed", func(t *testing.T) {
		// search() does not anchor
		assertLogicalResult(t, fn.Call(valueArgs("hello", "hello")), true)
		assertLogicalResult(t, fn.Call(valueArgs("hello world", "hello")), true)
		assertLogicalResult(t, fn.Call(valueArgs("say hello", "hello")), true)
		assertLogicalResult(t, fn.Call(valueArgs("say hello world", "hello")), true)
	})

	t.Run("pattern_can_match_anywhere", func(t *testing.T) {
		assertLogicalResult(t, fn.Call(valueArgs("abc", "abc")), true)
		assertLogicalResult(t, fn.Call(valueArgs("abc", "a")), true)
		assertLogicalResult(t, fn.Call(valueArgs("abc", "b")), true)
		assertLogicalResult(t, fn.Call(valueArgs("abc", "c")), true)
		assertLogicalResult(t, fn.Call(valueArgs("xabcy", "abc")), true)
	})
}

func BenchmarkLengthFunc(b *testing.B) {
	fn := LengthFunc{}

	b.Run("string", func(b *testing.B) {
		args := valueArgs("hello world")
		for b.Loop() {
			fn.Call(args)
		}
	})

	b.Run("array", func(b *testing.B) {
		args := valueArgs([]any{1, 2, 3, 4, 5})
		for b.Loop() {
			fn.Call(args)
		}
	})

	b.Run("object", func(b *testing.B) {
		args := valueArgs(map[string]any{"a": 1, "b": 2, "c": 3})
		for b.Loop() {
			fn.Call(args)
		}
	})
}

func BenchmarkCountFunc(b *testing.B) {
	fn := CountFunc{}
	args := nodesArgs(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

	for b.Loop() {
		fn.Call(args)
	}
}

func BenchmarkMatchFunc(b *testing.B) {
	fn := MatchFunc{}

	b.Run("simple_match", func(b *testing.B) {
		args := valueArgs("hello", "hello")
		for b.Loop() {
			fn.Call(args)
		}
	})

	b.Run("pattern_match", func(b *testing.B) {
		args := valueArgs("hello world", "hello.*")
		for b.Loop() {
			fn.Call(args)
		}
	})

	b.Run("no_match", func(b *testing.B) {
		args := valueArgs("hello", "world")
		for b.Loop() {
			fn.Call(args)
		}
	})
}

func BenchmarkSearchFunc(b *testing.B) {
	fn := SearchFunc{}

	b.Run("found", func(b *testing.B) {
		args := valueArgs("hello world", "world")
		for b.Loop() {
			fn.Call(args)
		}
	})

	b.Run("not_found", func(b *testing.B) {
		args := valueArgs("hello world", "xyz")
		for b.Loop() {
			fn.Call(args)
		}
	})
}

func BenchmarkValueFunc(b *testing.B) {
	fn := ValueFunc{}

	b.Run("single_node", func(b *testing.B) {
		args := nodesArgs(42)
		for b.Loop() {
			fn.Call(args)
		}
	})

	b.Run("empty_nodes", func(b *testing.B) {
		args := nodesArgs()
		for b.Loop() {
			fn.Call(args)
		}
	})

	b.Run("multiple_nodes", func(b *testing.B) {
		args := nodesArgs(1, 2, 3)
		for b.Loop() {
			fn.Call(args)
		}
	})
}

func BenchmarkRegexCache(b *testing.B) {
	b.Run("cache_hit", func(b *testing.B) {
		clearRegexCache()
		pattern := "test.*pattern"

		// Prime the cache
		compileIRegexp(pattern)

		b.ResetTimer()
		for b.Loop() {
			compileIRegexp(pattern)
		}
	})

	b.Run("cache_miss", func(b *testing.B) {
		for b.Loop() {
			clearRegexCache()
			compileIRegexp("test.*pattern")
		}
	})

	b.Run("concurrent_cache_hit", func(b *testing.B) {
		clearRegexCache()
		pattern := "concurrent.*test"

		// Prime the cache
		compileIRegexp(pattern)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				compileIRegexp(pattern)
			}
		})
	})
}
