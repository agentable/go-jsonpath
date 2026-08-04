// Package lexer provides a hand-written, zero-copy lexer for RFC 9535 JSONPath
// expressions.
//
// Tokens store byte offsets (start, end) into the source string rather than
// copied substrings, enabling zero-allocation access via [Token.Val]. String
// tokens additionally store a parsed value with escape sequences resolved.
package lexer

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Kind identifies a lexical token type.
type Kind int16

// Kind values.
const (
	Invalid      Kind = iota // error token; Value holds error message
	EOF                      // end of input
	Dollar                   // $
	At                       // @
	Dot                      // .
	DotDot                   // ..
	LeftBracket              // [
	RightBracket             // ]
	LeftParen                // (
	RightParen               // )
	Star                     // *
	Question                 // ?
	Comma                    // ,
	Colon                    // :
	Equal                    // ==
	NotEqual                 // !=
	Less                     // <
	LessEqual                // <=
	Greater                  // >
	GreaterEqual             // >=
	And                      // &&
	Or                       // ||
	Not                      // !
	Ident                    // identifier (member-name-shorthand / function name)
	Int                      // integer literal
	Number                   // number (float) literal
	String                   // single- or double-quoted string; Value holds parsed content
	True                     // true
	False                    // false
	Null                     // null
)

var kindNames = [...]string{
	Invalid:      "invalid",
	EOF:          "EOF",
	Dollar:       "$",
	At:           "@",
	Dot:          ".",
	DotDot:       "..",
	LeftBracket:  "[",
	RightBracket: "]",
	LeftParen:    "(",
	RightParen:   ")",
	Star:         "*",
	Question:     "?",
	Comma:        ",",
	Colon:        ":",
	Equal:        "==",
	NotEqual:     "!=",
	Less:         "<",
	LessEqual:    "<=",
	Greater:      ">",
	GreaterEqual: ">=",
	And:          "&&",
	Or:           "||",
	Not:          "!",
	Ident:        "identifier",
	Int:          "integer",
	Number:       "number",
	String:       "string",
	True:         "true",
	False:        "false",
	Null:         "null",
}

// String returns the human-readable name of k.
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", k)
}

// Token represents a single lexical token. Use [Token.Val] for zero-copy
// access to the raw source text. For [String] tokens, [Token.Value] holds the
// parsed string with escape sequences resolved.
type Token struct {
	Kind  Kind   // Kind is the token type.
	Start int    // byte offset in source (inclusive)
	End   int    // byte offset in source (exclusive)
	Value string // parsed value for String; error message for Invalid
}

// Val returns the raw source substring — no allocation.
func (t Token) Val(src string) string { return src[t.Start:t.End] }

// ErrSyntax is the sentinel error returned by [Token.Err] for invalid tokens.
var ErrSyntax = errors.New("jsonpath")

// Err returns a parse error for [Invalid] tokens and nil for all others.
func (t Token) Err() error {
	if t.Kind != Invalid {
		return nil
	}
	return fmt.Errorf("%w: %s at position %d", ErrSyntax, t.Value, t.Start)
}

// Lexer tokenizes an RFC 9535 JSONPath expression. Create with [New] and
// call [Lexer.Scan] repeatedly to get tokens.
type Lexer struct {
	src     string // source input
	r       rune   // current rune; -1 means EOF
	rPos    int    // byte offset of current rune
	nextPos int    // byte offset after current rune
}

// New creates a Lexer for src.
func New(src string) *Lexer {
	l := &Lexer{src: src, r: -1}
	l.next() // prime
	return l
}

// Source returns the original source string.
func (l *Lexer) Source() string { return l.src }

// next advances to the next rune. EOF is represented by -1.
func (l *Lexer) next() {
	if l.nextPos >= len(l.src) {
		l.rPos = len(l.src)
		l.r = -1
		return
	}

	l.rPos = l.nextPos
	r, w := rune(l.src[l.nextPos]), 1
	if r >= utf8.RuneSelf {
		r, w = utf8.DecodeRuneInString(l.src[l.nextPos:])
	}
	l.nextPos += w
	l.r = r
}

// peek returns the next rune without advancing. Returns -1 at EOF.
func (l *Lexer) peek() rune {
	if l.nextPos < len(l.src) {
		r := rune(l.src[l.nextPos])
		if r >= utf8.RuneSelf {
			r, _ = utf8.DecodeRuneInString(l.src[l.nextPos:])
		}
		return r
	}
	return -1
}

// errToken creates an [Invalid] token and halts the lexer.
func (l *Lexer) errToken(start int, msg string) Token {
	l.r = -1 // halt further scanning
	return Token{Kind: Invalid, Start: start, End: l.rPos, Value: msg}
}

// Scan returns the next token. After [EOF] is returned, subsequent calls
// continue returning EOF.
func (l *Lexer) Scan() Token {
	for isBlankSpace(l.r) {
		l.next()
	}

	if l.r < 0 {
		return Token{Kind: EOF, Start: l.rPos, End: l.rPos}
	}

	start := l.rPos

	switch l.r {
	case '$':
		l.next()
		return Token{Kind: Dollar, Start: start, End: l.rPos}
	case '@':
		l.next()
		return Token{Kind: At, Start: start, End: l.rPos}
	case '[':
		l.next()
		return Token{Kind: LeftBracket, Start: start, End: l.rPos}
	case ']':
		l.next()
		return Token{Kind: RightBracket, Start: start, End: l.rPos}
	case '(':
		l.next()
		return Token{Kind: LeftParen, Start: start, End: l.rPos}
	case ')':
		l.next()
		return Token{Kind: RightParen, Start: start, End: l.rPos}
	case '*':
		l.next()
		return Token{Kind: Star, Start: start, End: l.rPos}
	case '?':
		l.next()
		return Token{Kind: Question, Start: start, End: l.rPos}
	case ',':
		l.next()
		return Token{Kind: Comma, Start: start, End: l.rPos}
	case ':':
		l.next()
		return Token{Kind: Colon, Start: start, End: l.rPos}

	case '.':
		if l.peek() == '.' {
			l.next()
			l.next()
			return Token{Kind: DotDot, Start: start, End: l.rPos}
		}
		l.next()
		return Token{Kind: Dot, Start: start, End: l.rPos}
	case '=':
		if l.peek() == '=' {
			l.next()
			l.next()
			return Token{Kind: Equal, Start: start, End: l.rPos}
		}
		l.next()
		return l.errToken(start, "unexpected '='")
	case '!':
		if l.peek() == '=' {
			l.next()
			l.next()
			return Token{Kind: NotEqual, Start: start, End: l.rPos}
		}
		l.next()
		return Token{Kind: Not, Start: start, End: l.rPos}
	case '<':
		if l.peek() == '=' {
			l.next()
			l.next()
			return Token{Kind: LessEqual, Start: start, End: l.rPos}
		}
		l.next()
		return Token{Kind: Less, Start: start, End: l.rPos}
	case '>':
		if l.peek() == '=' {
			l.next()
			l.next()
			return Token{Kind: GreaterEqual, Start: start, End: l.rPos}
		}
		l.next()
		return Token{Kind: Greater, Start: start, End: l.rPos}
	case '&':
		if l.peek() == '&' {
			l.next()
			l.next()
			return Token{Kind: And, Start: start, End: l.rPos}
		}
		l.next()
		return l.errToken(start, "unexpected '&'")
	case '|':
		if l.peek() == '|' {
			l.next()
			l.next()
			return Token{Kind: Or, Start: start, End: l.rPos}
		}
		l.next()
		return l.errToken(start, "unexpected '|'")

	case '"', '\'':
		return l.scanString()

	case '-':
		return l.scanNumber()

	default:
		if isDigit(l.r) {
			return l.scanNumber()
		}
		if isNameFirst(l.r) {
			return l.scanIdent()
		}
		ch := l.r
		l.next()
		return l.errToken(start, fmt.Sprintf("unexpected character %q", ch))
	}
}

// scanIdent scans an identifier, including the keywords true, false, null.
// l.r must be a valid name-first character on entry.
func (l *Lexer) scanIdent() Token {
	start := l.rPos
	for isNameChar(l.r) {
		l.next()
	}
	raw := l.src[start:l.rPos]
	kind := Ident
	switch raw {
	case "true":
		kind = True
	case "false":
		kind = False
	case "null":
		kind = Null
	}
	return Token{Kind: kind, Start: start, End: l.rPos}
}

// scanNumber scans an integer or number (float) literal per RFC 9535.
// l.r must be '-' or a digit on entry.
func (l *Lexer) scanNumber() Token {
	start := l.rPos

	if l.r == '-' {
		l.next()
		if !isDigit(l.r) {
			return l.errToken(start, "expected digit after '-'")
		}
	}

	if l.r == '0' {
		l.next()
		if isDigit(l.r) {
			return l.errToken(start, "leading zeros not allowed")
		}
	} else {
		for isDigit(l.r) {
			l.next()
		}
	}

	kind := Int

	if l.r == '.' {
		kind = Number
		l.next()
		if !isDigit(l.r) {
			return l.errToken(start, "expected digit after '.'")
		}
		for isDigit(l.r) {
			l.next()
		}
	}

	if l.r == 'e' || l.r == 'E' {
		kind = Number
		l.next()
		if l.r == '+' || l.r == '-' {
			l.next()
		}
		if !isDigit(l.r) {
			return l.errToken(start, "expected digit in exponent")
		}
		for isDigit(l.r) {
			l.next()
		}
	}

	return Token{Kind: kind, Start: start, End: l.rPos}
}

// scanString scans a single- or double-quoted string literal per RFC 9535.
// On entry, l.r must be a quote rune. The parsed value with escapes resolved
// is stored in Token.Value.
func (l *Lexer) scanString() Token {
	start := l.rPos
	quote := l.r
	l.next() // consume opening quote

	var buf strings.Builder

	for l.r >= 0 {
		switch {
		case l.r == quote:
			l.next() // consume closing quote
			return Token{Kind: String, Start: start, End: l.rPos, Value: buf.String()}
		case l.r == '\\':
			if !l.scanEscape(quote, &buf) {
				return l.errToken(start, "invalid escape sequence")
			}
		case isUnescaped(l.r, quote):
			buf.WriteRune(l.r)
			l.next()
		default:
			return l.errToken(start, fmt.Sprintf("invalid character %U in string", l.r))
		}
	}

	return l.errToken(start, "unterminated string")
}

// scanEscape handles a single escape sequence starting at '\\'. On entry l.r
// must be '\\'. Returns false if the escape is invalid.
func (l *Lexer) scanEscape(quote rune, buf *strings.Builder) bool {
	l.next() // consume '\'

	switch l.r {
	case quote:
		buf.WriteRune(quote)
	case 'b':
		buf.WriteByte('\b')
	case 'f':
		buf.WriteByte('\f')
	case 'n':
		buf.WriteByte('\n')
	case 'r':
		buf.WriteByte('\r')
	case 't':
		buf.WriteByte('\t')
	case '/':
		buf.WriteByte('/')
	case '\\':
		buf.WriteByte('\\')
	case 'u':
		return l.scanUnicodeEscape(buf)
	default:
		return false
	}
	l.next()
	return true
}

// scanUnicodeEscape handles a \uXXXX escape (including surrogate pairs).
// On entry l.r must be 'u'. Writes the decoded rune to buf.
func (l *Lexer) scanUnicodeEscape(buf *strings.Builder) bool {
	l.next() // consume 'u'

	r := l.scanHex4()
	if r < 0 {
		return false
	}

	if !utf16.IsSurrogate(r) {
		buf.WriteRune(r)
		return true
	}

	// Must be a high surrogate (D800-DBFF).
	if r >= 0xDC00 {
		return false
	}

	// Expect \uXXXX for low surrogate.
	if l.r != '\\' {
		return false
	}
	l.next()
	if l.r != 'u' {
		return false
	}
	l.next()

	low := l.scanHex4()
	if low < 0 {
		return false
	}

	decoded := utf16.DecodeRune(r, low)
	if decoded == unicode.ReplacementChar {
		return false
	}

	buf.WriteRune(decoded)
	return true
}

// scanHex4 scans exactly 4 hex digits and returns the code point.
// Returns -1 if fewer than 4 valid hex digits are found.
func (l *Lexer) scanHex4() rune {
	var r rune
	for range 4 {
		h := hexVal(l.r)
		if h < 0 {
			return -1
		}
		r = r*16 + h
		l.next()
	}
	return r
}

// hexVal returns the numeric value of hex digit r, or -1 if not a hex digit.
func hexVal(r rune) rune {
	switch {
	case '0' <= r && r <= '9':
		return r - '0'
	case 'a' <= r && r <= 'f':
		return r - 'a' + 10
	case 'A' <= r && r <= 'F':
		return r - 'A' + 10
	default:
		return -1
	}
}

// isBlankSpace reports whether r is RFC 9535 blank space (SP / HTAB / LF / CR).
func isBlankSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// isDigit reports whether r is an ASCII digit.
func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

// isNameFirst reports whether r is valid as the first character of a member
// name per RFC 9535 §2.5.1.1.
//
//	name-first = ALPHA / "_" / %x80-D7FF / %xE000-10FFFF
func isNameFirst(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		r == '_' ||
		(r >= 0x80 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0x10FFFF)
}

// isNameChar reports whether r is valid in a member name after the first
// character per RFC 9535 §2.5.1.1.
//
//	name-char = name-first / DIGIT
func isNameChar(r rune) bool {
	return isNameFirst(r) || isDigit(r)
}

// isUnescaped reports whether r is an unescaped character valid in a string
// with the given quote character, per RFC 9535 §2.3.1.
func isUnescaped(r, quote rune) bool {
	if r == quote {
		return false
	}
	// %x20-5B / %x5D-D7FF / %xE000-10FFFF
	// (0x5C = backslash, excluded by the gap between ranges)
	return (r >= 0x20 && r <= 0x5B) ||
		(r >= 0x5D && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0x10FFFF)
}
