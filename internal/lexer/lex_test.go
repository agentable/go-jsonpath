package lexer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKindString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind Kind
		want string
	}{
		{Invalid, "invalid"},
		{EOF, "EOF"},
		{Dollar, "$"},
		{At, "@"},
		{Dot, "."},
		{DotDot, ".."},
		{LeftBracket, "["},
		{RightBracket, "]"},
		{LeftParen, "("},
		{RightParen, ")"},
		{Star, "*"},
		{Question, "?"},
		{Comma, ","},
		{Colon, ":"},
		{Equal, "=="},
		{NotEqual, "!="},
		{Less, "<"},
		{LessEqual, "<="},
		{Greater, ">"},
		{GreaterEqual, ">="},
		{And, "&&"},
		{Or, "||"},
		{Not, "!"},
		{Ident, "identifier"},
		{Int, "integer"},
		{Number, "number"},
		{String, "string"},
		{True, "true"},
		{False, "false"},
		{Null, "null"},
		{Kind(999), "Kind(999)"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, tc.kind.String())
	}
}

func TestSingleCharTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  Kind
	}{
		{"dollar", "$", Dollar},
		{"at", "@", At},
		{"lbracket", "[", LeftBracket},
		{"rbracket", "]", RightBracket},
		{"lparen", "(", LeftParen},
		{"rparen", ")", RightParen},
		{"star", "*", Star},
		{"question", "?", Question},
		{"comma", ",", Comma},
		{"colon", ":", Colon},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, tc.kind, tok.Kind)
			assert.Equal(t, 0, tok.Start)
			assert.Equal(t, len(tc.input), tok.End)
			assert.Equal(t, tc.input, tok.Val(l.Source()))
			// Next scan should be EOF.
			assert.Equal(t, EOF, l.Scan().Kind)
		})
	}
}

func TestMultiCharOperators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  Kind
	}{
		{"dot", ".", Dot},
		{"dotdot", "..", DotDot},
		{"eqeq", "==", Equal},
		{"noteq", "!=", NotEqual},
		{"not", "!", Not},
		{"less", "<", Less},
		{"lesseq", "<=", LessEqual},
		{"greater", ">", Greater},
		{"greatereq", ">=", GreaterEqual},
		{"and", "&&", And},
		{"or", "||", Or},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, tc.kind, tok.Kind)
			assert.Equal(t, tc.input, tok.Val(l.Source()))
			assert.Equal(t, EOF, l.Scan().Kind)
		})
	}
}

func TestInvalidOperators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"lone_eq", "="},
		{"lone_amp", "&"},
		{"lone_pipe", "|"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := New(tc.input).Scan()
			assert.Equal(t, Invalid, tok.Kind)
			require.Error(t, tok.Err())
		})
	}
}

func TestIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  Kind
		val   string
	}{
		{"simple", "foo", Ident, "foo"},
		{"underscore", "_bar", Ident, "_bar"},
		{"with_digits", "x1y2", Ident, "x1y2"},
		{"unicode", "café", Ident, "café"},
		{"emoji", "say_😀", Ident, "say_😀"},
		{"true", "true", True, "true"},
		{"false", "false", False, "false"},
		{"null", "null", Null, "null"},
		{"truthy", "truthy", Ident, "truthy"},
		{"nullable", "nullable", Ident, "nullable"},
		{"falsetto", "falsetto", Ident, "falsetto"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, tc.kind, tok.Kind)
			assert.Equal(t, tc.val, tok.Val(l.Source()))
		})
	}
}

func TestIntegers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		val   string
	}{
		{"zero", "0", "0"},
		{"one", "1", "1"},
		{"multi", "42", "42"},
		{"large", "9007199254740992", "9007199254740992"},
		{"neg_one", "-1", "-1"},
		{"neg_multi", "-42", "-42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, Int, tok.Kind)
			assert.Equal(t, tc.val, tok.Val(l.Source()))
		})
	}
}

func TestNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		val   string
	}{
		{"frac", "0.1", "0.1"},
		{"frac_more", "42.234853", "42.234853"},
		{"neg_frac", "-42.734", "-42.734"},
		{"neg_zero_frac", "-0.23", "-0.23"},
		{"exp", "0e12", "0e12"},
		{"exp_upper", "42E124", "42E124"},
		{"exp_plus", "99e+123", "99e+123"},
		{"exp_minus", "99e-12", "99e-12"},
		{"frac_exp", "12.32E3", "12.32E3"},
		{"neg_exp", "-42E123", "-42E123"},
		{"sci", "6.67428e-11", "6.67428e-11"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, Number, tok.Kind)
			assert.Equal(t, tc.val, tok.Val(l.Source()))
		})
	}
}

func TestInvalidNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"leading_zero", "032"},
		{"neg_leading_zero", "-032"},
		{"dot_no_digit", "42.x"},
		{"neg_no_digit", "-lol"},
		{"exp_no_digit", "42ex"},
		{"exp_plus_no_digit", "99e+x"},
		{"exp_minus_no_digit", "99e-x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := New(tc.input).Scan()
			assert.Equal(t, Invalid, tok.Kind)
			require.Error(t, tok.Err())
		})
	}
}

func TestStringsDoubleQuoted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		value string
	}{
		{"empty", `""`, ""},
		{"simple", `"hello"`, "hello"},
		{"spaces", `"hello there"`, "hello there"},
		{"utf8", `"hello ø"`, "hello ø"},
		{"emoji", `"hello 👋"`, "hello 👋"},
		{"escape_quote", `"say \"hi\""`, `say "hi"`},
		{"escape_backslash", `"a\\b"`, `a\b`},
		{"escape_slash", `"a\/b"`, "a/b"},
		{"escape_b", `"\b"`, "\b"},
		{"escape_f", `"\f"`, "\f"},
		{"escape_n", `"\n"`, "\n"},
		{"escape_r", `"\r"`, "\r"},
		{"escape_t", `"\t"`, "\t"},
		{"unicode_basic", `"\u00f8"`, "ø"},
		{"unicode_mid", `"fo\u00f8 bar"`, "foø bar"},
		{"surrogate_pair", `"\uD834\uDD1E"`, "\U0001D11E"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, String, tok.Kind, "kind")
			assert.Equal(t, tc.value, tok.Value, "parsed value")
			assert.Equal(t, tc.input, tok.Val(l.Source()), "raw source")
		})
	}
}

func TestStringsSingleQuoted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		value string
	}{
		{"empty", `''`, ""},
		{"simple", `'hello'`, "hello"},
		{"escape_quote", `'say \'hi\''`, "say 'hi'"},
		{"escape_n", `'\n'`, "\n"},
		{"unicode", `'\u00f8'`, "ø"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, String, tok.Kind)
			assert.Equal(t, tc.value, tok.Value)
		})
	}
}

func TestInvalidStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"unterminated_dq", `"hello`},
		{"unterminated_sq", `'hello`},
		{"bad_escape", `"\x"`},
		{"bad_unicode_short", `"\u0f8"`},
		{"invalid_surrogate_low", `"\uD834\uED1E"`},
		{"lone_high_surrogate", `"\uD834 "`},
		{"lone_low_surrogate", `"\uDC00"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := New(tc.input).Scan()
			assert.Equal(t, Invalid, tok.Kind)
			require.Error(t, tok.Err())
		})
	}
}

func TestBlankSpaceSkipping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		{"spaces", "  $  ", []Kind{Dollar}},
		{"tabs", "\t$\t", []Kind{Dollar}},
		{"newlines", "\n$\n", []Kind{Dollar}},
		{"mixed", " \t\r\n $ \t\r\n ", []Kind{Dollar}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanAll(tc.input)
			assertKinds(t, tc.kinds, got)
		})
	}
}

func TestFullExpressions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		{
			name:  "root_dot_member",
			input: "$.foo",
			kinds: []Kind{Dollar, Dot, Ident},
		},
		{
			name:  "root_bracket_string",
			input: `$["foo"]`,
			kinds: []Kind{Dollar, LeftBracket, String, RightBracket},
		},
		{
			name:  "root_bracket_index",
			input: "$[0]",
			kinds: []Kind{Dollar, LeftBracket, Int, RightBracket},
		},
		{
			name:  "descendant",
			input: "$..foo",
			kinds: []Kind{Dollar, DotDot, Ident},
		},
		{
			name:  "wildcard",
			input: "$[*]",
			kinds: []Kind{Dollar, LeftBracket, Star, RightBracket},
		},
		{
			name:  "slice",
			input: "$[1:3:2]",
			kinds: []Kind{Dollar, LeftBracket, Int, Colon, Int, Colon, Int, RightBracket},
		},
		{
			name:  "filter_simple",
			input: "$[?@.active==true]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, Dot, Ident, Equal, True, RightBracket},
		},
		{
			name:  "filter_and",
			input: "$[?@.a && @.b]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, Dot, Ident, And, At, Dot, Ident, RightBracket},
		},
		{
			name:  "filter_or",
			input: "$[?@.a || @.b]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, Dot, Ident, Or, At, Dot, Ident, RightBracket},
		},
		{
			name:  "filter_not",
			input: "$[?!@.a]",
			kinds: []Kind{Dollar, LeftBracket, Question, Not, At, Dot, Ident, RightBracket},
		},
		{
			name:  "filter_comparison",
			input: "$[?@.price < 10]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, Dot, Ident, Less, Int, RightBracket},
		},
		{
			name:  "function_call",
			input: "$[?length(@.a) > 0]",
			kinds: []Kind{Dollar, LeftBracket, Question, Ident, LeftParen, At, Dot, Ident, RightParen, Greater, Int, RightBracket},
		},
		{
			name:  "multiple_selectors",
			input: `$["a","b"]`,
			kinds: []Kind{Dollar, LeftBracket, String, Comma, String, RightBracket},
		},
		{
			name:  "negative_index",
			input: "$[-1]",
			kinds: []Kind{Dollar, LeftBracket, Int, RightBracket},
		},
		{
			name:  "nested_brackets",
			input: "$[?@[0]==1]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, LeftBracket, Int, RightBracket, Equal, Int, RightBracket},
		},
		{
			name:  "comparison_ops",
			input: "$[?@.a!=1 && @.b>=2 && @.c<=3]",
			kinds: []Kind{
				Dollar, LeftBracket, Question,
				At, Dot, Ident, NotEqual, Int,
				And,
				At, Dot, Ident, GreaterEqual, Int,
				And,
				At, Dot, Ident, LessEqual, Int,
				RightBracket,
			},
		},
		{
			name:  "null_literal",
			input: "$[?@.a==null]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, Dot, Ident, Equal, Null, RightBracket},
		},
		{
			name:  "false_literal",
			input: "$[?@.a==false]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, Dot, Ident, Equal, False, RightBracket},
		},
		{
			name:  "float_comparison",
			input: "$[?@.price < 9.99]",
			kinds: []Kind{Dollar, LeftBracket, Question, At, Dot, Ident, Less, Number, RightBracket},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanAll(tc.input)
			assertKinds(t, tc.kinds, got)
		})
	}
}

func TestEOFRepeatable(t *testing.T) {
	t.Parallel()
	l := New("")
	for range 5 {
		tok := l.Scan()
		assert.Equal(t, EOF, tok.Kind)
	}
}

func TestTokenVal(t *testing.T) {
	t.Parallel()
	src := `$.foo[42]`
	l := New(src)

	dollar := l.Scan()
	assert.Equal(t, "$", dollar.Val(l.Source()))

	dot := l.Scan()
	assert.Equal(t, ".", dot.Val(l.Source()))

	ident := l.Scan()
	assert.Equal(t, "foo", ident.Val(l.Source()))

	lb := l.Scan()
	assert.Equal(t, "[", lb.Val(l.Source()))

	num := l.Scan()
	assert.Equal(t, "42", num.Val(l.Source()))

	rb := l.Scan()
	assert.Equal(t, "]", rb.Val(l.Source()))
}

func TestTokenErr(t *testing.T) {
	t.Parallel()

	// Valid token returns nil error.
	tok := Token{Kind: Dollar, Start: 0, End: 1}
	assert.NoError(t, tok.Err())

	// Invalid token returns error.
	tok = Token{Kind: Invalid, Start: 5, End: 6, Value: "bad character"}
	err := tok.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad character")
	assert.Contains(t, err.Error(), "position 5")
}

func TestPeek(t *testing.T) {
	t.Parallel()
	l := New("ab")
	assert.Equal(t, 'a', l.r)
	assert.Equal(t, 'b', l.peek())
	l.next()
	assert.Equal(t, 'b', l.r)
	assert.Equal(t, rune(-1), l.peek())
	l.next()
	assert.Equal(t, rune(-1), l.r)
	assert.Equal(t, rune(-1), l.peek())
}

func TestUnexpectedCharacter(t *testing.T) {
	t.Parallel()
	tok := New("~").Scan()
	assert.Equal(t, Invalid, tok.Kind)
	require.Error(t, tok.Err())
}

func TestZeroCopyVal(t *testing.T) {
	t.Parallel()
	// Verify Val returns a substring of the original source (no allocation).
	src := "$.foo"
	l := New(src)
	l.Scan()        // $
	l.Scan()        // .
	tok := l.Scan() // foo
	val := tok.Val(l.Source())
	assert.Equal(t, "foo", val)
	// The returned string should share memory with src.
	assert.Equal(t, src[2:5], val)
}

// scanAll returns all non-EOF tokens from input.
func scanAll(input string) []Token {
	l := New(input)
	var tokens []Token
	for {
		tok := l.Scan()
		if tok.Kind == EOF {
			break
		}
		tokens = append(tokens, tok)
		if tok.Kind == Invalid {
			break
		}
	}
	return tokens
}

// assertKinds checks that tokens have the expected kinds in order.
func assertKinds(t *testing.T, want []Kind, got []Token) {
	t.Helper()
	kinds := make([]Kind, len(got))
	for i, tok := range got {
		kinds[i] = tok.Kind
	}
	if diff := cmp.Diff(want, kinds); diff != "" {
		t.Errorf("token kinds mismatch (-want +got):\n%s", diff)
	}
}

// TestStringEscapeEdgeCases tests additional escape sequence edge cases.
func TestStringEscapeEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		value string
	}{
		{"all_escapes", `"\b\f\n\r\t\/\\"`, "\b\f\n\r\t/\\"},
		{"mixed_escapes", `"a\nb\tc"`, "a\nb\tc"},
		{"escape_at_start", `"\nhello"`, "\nhello"},
		{"escape_at_end", `"hello\n"`, "hello\n"},
		{"consecutive_escapes", `"\n\n\n"`, "\n\n\n"},
		{"unicode_null", `"\u0000"`, "\x00"},
		{"unicode_space", `"\u0020"`, " "},
		{"unicode_max_bmp", `"\uFFFF"`, "\uFFFF"},
		{"multiple_unicode", `"\u0041\u0042\u0043"`, "ABC"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, String, tok.Kind)
			assert.Equal(t, tc.value, tok.Value)
		})
	}
}

// TestSurrogatePairEdgeCases tests surrogate pair handling edge cases.
func TestSurrogatePairEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		value string
	}{
		{"emoji_grinning", `"\uD83D\uDE00"`, "😀"},
		{"emoji_party", `"\uD83C\uDF89"`, "🎉"},
		{"min_supplementary", `"\uD800\uDC00"`, "\U00010000"},
		{"max_supplementary", `"\uDBFF\uDFFF"`, "\U0010FFFF"},
		{"musical_note", `"\uD834\uDD1E"`, "\U0001D11E"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, String, tok.Kind)
			assert.Equal(t, tc.value, tok.Value)
		})
	}
}

// TestInvalidSurrogatePairs tests invalid surrogate pair combinations.
func TestInvalidSurrogatePairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"high_without_low", `"\uD800"`},
		{"high_with_space", `"\uD800 "`},
		{"high_with_regular", `"\uD800\u0041"`},
		{"low_alone", `"\uDC00"`},
		{"low_first", `"\uDC00\uD800"`},
		{"high_high", `"\uD800\uD800"`},
		{"high_no_backslash", `"\uD800u"`},
		{"high_wrong_escape", `"\uD800\n"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := New(tc.input).Scan()
			assert.Equal(t, Invalid, tok.Kind)
			require.Error(t, tok.Err())
		})
	}
}

// TestNumberBoundaries tests number parsing at boundaries.
func TestNumberBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  Kind
	}{
		{"zero", "0", Int},
		{"neg_zero", "-0", Int},
		{"zero_frac", "0.0", Number},
		{"zero_exp", "0e0", Number},
		{"zero_exp_plus", "0e+0", Number},
		{"zero_exp_minus", "0e-0", Number},
		{"max_safe_int", "9007199254740991", Int},
		{"neg_max_safe_int", "-9007199254740991", Int},
		{"large_exp", "1e308", Number},
		{"small_exp", "1e-308", Number},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, tc.kind, tok.Kind)
			assert.Equal(t, tc.input, tok.Val(l.Source()))
		})
	}
}

// TestNumberFollowedByOperator tests numbers followed by operators.
func TestNumberFollowedByOperator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		{"int_less", "42<10", []Kind{Int, Less, Int}},
		{"int_greater", "42>10", []Kind{Int, Greater, Int}},
		{"int_eq", "42==10", []Kind{Int, Equal, Int}},
		{"float_less", "3.14<10", []Kind{Number, Less, Int}},
		{"int_bracket", "42]", []Kind{Int, RightBracket}},
		{"int_comma", "42,", []Kind{Int, Comma}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanAll(tc.input)
			assertKinds(t, tc.kinds, got)
		})
	}
}

// TestDotDotEdgeCases tests .. vs . disambiguation.
func TestDotDotEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		{"two_dots", "..", []Kind{DotDot}},
		{"three_dots", "...", []Kind{DotDot, Dot}},
		{"four_dots", "....", []Kind{DotDot, DotDot}},
		{"five_dots", ".....", []Kind{DotDot, DotDot, Dot}},
		{"dot_space_dot", ". .", []Kind{Dot, Dot}},
		{"dotdot_space_dot", ".. .", []Kind{DotDot, Dot}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanAll(tc.input)
			assertKinds(t, tc.kinds, got)
		})
	}
}

// TestInvalidEscapeSequences tests various invalid escape sequences.
func TestInvalidEscapeSequences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"backslash_x", `"\x41"`},
		{"backslash_a", `"\a"`},
		{"backslash_v", `"\v"`},
		{"backslash_0", `"\0"`},
		{"backslash_digit", `"\1"`},
		{"unicode_short_1", `"\u"`},
		{"unicode_short_2", `"\u1"`},
		{"unicode_short_3", `"\u12"`},
		{"unicode_short_4", `"\u123"`},
		{"unicode_invalid_hex", `"\uGGGG"`},
		{"unicode_mixed_case_invalid", `"\u00GG"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := New(tc.input).Scan()
			assert.Equal(t, Invalid, tok.Kind)
			require.Error(t, tok.Err())
		})
	}
}

// TestStringControlCharacters tests that control characters are rejected.
func TestStringControlCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"null", "\"\x00\""},
		{"soh", "\"\x01\""},
		{"tab_literal", "\"\x09\""}, // tab must be escaped
		{"lf_literal", "\"\x0A\""},  // newline must be escaped
		{"cr_literal", "\"\x0D\""},  // carriage return must be escaped
		{"us", "\"\x1F\""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := New(tc.input).Scan()
			assert.Equal(t, Invalid, tok.Kind)
			require.Error(t, tok.Err())
		})
	}
}

// TestMixedQuoteTypes tests strings with different quote types.
func TestMixedQuoteTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		value string
	}{
		{"double_with_single", `"It's working"`, "It's working"},
		{"single_with_double", `'He said "hello"'`, `He said "hello"`},
		{"double_escape_double", `"She said \"hi\""`, `She said "hi"`},
		{"single_escape_single", `'It\'s escaped'`, "It's escaped"},
		{"double_no_escape_single", `"don't"`, "don't"},
		{"single_no_escape_double", `'say "hi"'`, `say "hi"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, String, tok.Kind)
			assert.Equal(t, tc.value, tok.Value)
		})
	}
}

// TestErrorAfterInvalid tests that scanning stops after an error.
func TestErrorAfterInvalid(t *testing.T) {
	t.Parallel()

	l := New("$ = @")
	tok := l.Scan()
	assert.Equal(t, Dollar, tok.Kind)

	tok = l.Scan()
	assert.Equal(t, Invalid, tok.Kind)

	// After error, should return EOF.
	for range 3 {
		tok = l.Scan()
		assert.Equal(t, EOF, tok.Kind)
	}
}

// TestTokenPositions tests that token start/end positions are accurate.
func TestTokenPositions(t *testing.T) {
	t.Parallel()

	input := "$ . foo"
	l := New(input)

	tok := l.Scan()
	assert.Equal(t, Dollar, tok.Kind)
	assert.Equal(t, 0, tok.Start)
	assert.Equal(t, 1, tok.End)

	tok = l.Scan()
	assert.Equal(t, Dot, tok.Kind)
	assert.Equal(t, 2, tok.Start)
	assert.Equal(t, 3, tok.End)

	tok = l.Scan()
	assert.Equal(t, Ident, tok.Kind)
	assert.Equal(t, 4, tok.Start)
	assert.Equal(t, 7, tok.End)
}

// TestUTF8MultibytePositions tests positions with multibyte UTF-8 characters.
func TestUTF8MultibytePositions(t *testing.T) {
	t.Parallel()

	input := "café"
	l := New(input)
	tok := l.Scan()
	assert.Equal(t, Ident, tok.Kind)
	assert.Equal(t, 0, tok.Start)
	assert.Equal(t, 5, tok.End) // é is 2 bytes in UTF-8
	assert.Equal(t, "café", tok.Val(l.Source()))
}

// TestConsecutiveOperators tests scanning consecutive operators without spaces.
func TestConsecutiveOperators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		{"eq_noteq", "==!=", []Kind{Equal, NotEqual}},
		{"less_lesseq", "<<<=", []Kind{Less, Less, LessEqual}},
		{"greater_greatereq", ">>>=", []Kind{Greater, Greater, GreaterEqual}},
		{"and_or", "&&||", []Kind{And, Or}},
		{"not_noteq", "!!=", []Kind{Not, NotEqual}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanAll(tc.input)
			assertKinds(t, tc.kinds, got)
		})
	}
}

// TestIdentifierEdgeCases tests edge cases in identifier parsing.
func TestIdentifierEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  Kind
		val   string
	}{
		{"underscore_only", "_", Ident, "_"},
		{"double_underscore", "__", Ident, "__"},
		{"underscore_digit", "_123", Ident, "_123"},
		{"digit_not_first", "a1b2c3", Ident, "a1b2c3"},
		{"all_caps", "CONSTANT", Ident, "CONSTANT"},
		{"camel_case", "camelCase", Ident, "camelCase"},
		{"pascal_case", "PascalCase", Ident, "PascalCase"},
		{"snake_case", "snake_case", Ident, "snake_case"},
		{"true_prefix", "truthy", Ident, "truthy"},
		{"false_prefix", "falsehood", Ident, "falsehood"},
		{"null_prefix", "nullable", Ident, "nullable"},
		{"true_suffix", "untrue", Ident, "untrue"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New(tc.input)
			tok := l.Scan()
			assert.Equal(t, tc.kind, tok.Kind)
			assert.Equal(t, tc.val, tok.Val(l.Source()))
		})
	}
}

// TestNumberFollowedByDot tests numbers followed by dots.
func TestNumberFollowedByDot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		// "42.foo" - lexer sees "42." as a number (invalid, no digit after dot)
		{"int_dot_ident", "42.foo", []Kind{Invalid}},
		// "42.." - lexer sees "42." as invalid number (no digit after dot)
		{"int_dotdot", "42..", []Kind{Invalid}},
		// "3.14.foo" - lexer sees "3.14" as Number, then "." as Dot, then "foo" as Ident
		{"float_dot", "3.14.foo", []Kind{Number, Dot, Ident}},
		// "42 .foo" - with space, 42 is Int, then Dot, then Ident
		{"int_space_dot_ident", "42 .foo", []Kind{Int, Dot, Ident}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanAll(tc.input)
			assertKinds(t, tc.kinds, got)
		})
	}
}

// TestNegativeNumberEdgeCases tests edge cases with negative numbers.
func TestNegativeNumberEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		{"neg_int", "-1", []Kind{Int}},
		{"neg_float", "-3.14", []Kind{Number}},
		{"neg_exp", "-1e10", []Kind{Number}},
		{"minus_space", "- 1", []Kind{Invalid}},
		{"minus_alone", "-", []Kind{Invalid}},
		{"double_minus", "--1", []Kind{Invalid}},
		{"minus_ident", "-foo", []Kind{Invalid}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanAll(tc.input)
			assertKinds(t, tc.kinds, got)
		})
	}
}

// TestLeadingZeros tests that leading zeros are rejected.
func TestLeadingZeros(t *testing.T) {
	t.Parallel()

	tests := []string{
		"01",
		"00",
		"001",
		"0123",
		"-01",
		"-00",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			tok := New(input).Scan()
			assert.Equal(t, Invalid, tok.Kind)
			require.Error(t, tok.Err())
		})
	}
}

// TestExponentEdgeCases tests edge cases in exponent parsing.
func TestExponentEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"exp_lower", "1e10", true},
		{"exp_upper", "1E10", true},
		{"exp_plus", "1e+10", true},
		{"exp_minus", "1e-10", true},
		{"exp_zero", "1e0", true},
		{"exp_no_digit", "1e", false},
		{"exp_plus_no_digit", "1e+", false},
		{"exp_minus_no_digit", "1e-", false},
		{"exp_letter", "1ex", false},
		{"frac_exp", "1.5e10", true},
		{"zero_exp", "0e0", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := New(tc.input).Scan()
			if tc.valid {
				assert.Equal(t, Number, tok.Kind)
			} else {
				assert.Equal(t, Invalid, tok.Kind)
			}
		})
	}
}
