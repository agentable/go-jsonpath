package functions

import (
	"fmt"
	"unicode/utf8"
)

type iRegexpChecker struct {
	pattern     string
	offset      int
	groups      []int
	canQuantify bool
}

func checkIRegexp(pattern string) error {
	if !utf8.ValidString(pattern) {
		return fmt.Errorf("invalid I-Regexp: pattern is not valid UTF-8")
	}
	checker := iRegexpChecker{pattern: pattern}
	return checker.check()
}

func (c *iRegexpChecker) check() error {
	for c.offset < len(c.pattern) {
		start := c.offset
		r, size := utf8.DecodeRuneInString(c.pattern[c.offset:])
		switch {
		case r == '(':
			c.groups = append(c.groups, start)
			c.offset += size
			c.canQuantify = false
		case r == ')':
			if len(c.groups) == 0 {
				return c.errorf(start, "unbalanced ')'")
			}
			c.groups = c.groups[:len(c.groups)-1]
			c.offset += size
			c.canQuantify = true
		case r == '|':
			c.offset += size
			c.canQuantify = false
		case r == '[':
			if err := c.checkCharacterClass(); err != nil {
				return err
			}
			c.canQuantify = true
		case r == '\\':
			if err := c.checkEscape(); err != nil {
				return err
			}
			c.canQuantify = true
		case r == '.':
			c.offset += size
			c.canQuantify = true
		case r == '*' || r == '+' || r == '?' || r == '{':
			if err := c.checkQuantifier(r); err != nil {
				return err
			}
		case isNormalIRegexpChar(r):
			c.offset += size
			c.canQuantify = true
		default:
			return c.errorf(start, "invalid character %q", r)
		}
	}

	if len(c.groups) != 0 {
		return c.errorf(c.groups[len(c.groups)-1], "unclosed group")
	}
	return nil
}

func (c *iRegexpChecker) checkEscape() error {
	start := c.offset
	c.offset++
	if c.offset >= len(c.pattern) {
		return c.errorf(start, "trailing escape")
	}

	r, size := utf8.DecodeRuneInString(c.pattern[c.offset:])
	c.offset += size
	if isIRegexpSingleCharEscape(r) {
		return nil
	}
	if r == 'p' || r == 'P' {
		return c.checkUnicodeProperty(start)
	}
	return c.errorf(start, "unsupported escape \\%c", r)
}

func (c *iRegexpChecker) checkUnicodeProperty(start int) error {
	if c.offset >= len(c.pattern) || c.pattern[c.offset] != '{' {
		return c.errorf(start, "Unicode property is missing '{'")
	}
	c.offset++
	propertyStart := c.offset
	for c.offset < len(c.pattern) && c.pattern[c.offset] != '}' {
		c.offset++
	}
	if c.offset >= len(c.pattern) {
		return c.errorf(start, "unclosed Unicode property")
	}
	property := c.pattern[propertyStart:c.offset]
	c.offset++
	if !isIRegexpUnicodeProperty(property) {
		return c.errorf(propertyStart, "unsupported Unicode property %q", property)
	}
	return nil
}

func (c *iRegexpChecker) checkQuantifier(quantifier rune) error {
	start := c.offset
	if !c.canQuantify {
		return c.errorf(start, "quantifier has no atom")
	}

	c.offset++
	if quantifier != '{' {
		c.canQuantify = false
		return nil
	}

	digitStart := c.offset
	for c.offset < len(c.pattern) && isASCIIDigit(c.pattern[c.offset]) {
		c.offset++
	}
	if c.offset == digitStart {
		return c.errorf(start, "range quantifier requires a lower bound")
	}
	if c.offset < len(c.pattern) && c.pattern[c.offset] == ',' {
		c.offset++
		for c.offset < len(c.pattern) && isASCIIDigit(c.pattern[c.offset]) {
			c.offset++
		}
	}
	if c.offset >= len(c.pattern) || c.pattern[c.offset] != '}' {
		return c.errorf(start, "unclosed range quantifier")
	}
	c.offset++
	c.canQuantify = false
	return nil
}

func (c *iRegexpChecker) checkCharacterClass() error {
	start := c.offset
	c.offset++
	if c.offset < len(c.pattern) && c.pattern[c.offset] == '^' {
		c.offset++
	}
	if c.offset >= len(c.pattern) {
		return c.errorf(start, "unclosed character class")
	}
	if c.pattern[c.offset] == ']' {
		return c.errorf(start, "empty character class")
	}

	if c.pattern[c.offset] == '-' {
		c.offset++
	} else if err := c.checkCharacterClassElement(); err != nil {
		return err
	}

	for c.offset < len(c.pattern) {
		switch c.pattern[c.offset] {
		case ']':
			c.offset++
			return nil
		case '-':
			hyphen := c.offset
			c.offset++
			if c.offset < len(c.pattern) && c.pattern[c.offset] == ']' {
				c.offset++
				return nil
			}
			return c.errorf(hyphen, "hyphen is not first, last, or part of a range")
		default:
			if err := c.checkCharacterClassElement(); err != nil {
				return err
			}
		}
	}
	return c.errorf(start, "unclosed character class")
}

func (c *iRegexpChecker) checkCharacterClassElement() error {
	if c.pattern[c.offset] == '\\' {
		start := c.offset
		c.offset++
		if c.offset >= len(c.pattern) {
			return c.errorf(start, "trailing escape in character class")
		}
		r, size := utf8.DecodeRuneInString(c.pattern[c.offset:])
		c.offset += size
		if r == 'p' || r == 'P' {
			return c.checkUnicodeProperty(start)
		}
		if !isIRegexpSingleCharEscape(r) {
			return c.errorf(start, "unsupported escape \\%c in character class", r)
		}
	} else if err := c.checkCharacterClassChar(); err != nil {
		return err
	}

	if c.offset < len(c.pattern) && c.pattern[c.offset] == '-' {
		if c.offset+1 < len(c.pattern) && c.pattern[c.offset+1] == ']' {
			return nil
		}
		c.offset++
		return c.checkCharacterClassChar()
	}
	return nil
}

func (c *iRegexpChecker) checkCharacterClassChar() error {
	start := c.offset
	if c.offset >= len(c.pattern) {
		return c.errorf(start, "missing character class range endpoint")
	}
	if c.pattern[c.offset] == '\\' {
		c.offset++
		if c.offset >= len(c.pattern) {
			return c.errorf(start, "trailing escape in character class")
		}
		r, size := utf8.DecodeRuneInString(c.pattern[c.offset:])
		c.offset += size
		if !isIRegexpSingleCharEscape(r) {
			return c.errorf(start, "invalid character class range endpoint")
		}
		return nil
	}

	r, size := utf8.DecodeRuneInString(c.pattern[c.offset:])
	if !isIRegexpCharacterClassChar(r) {
		return c.errorf(start, "invalid character %q in character class", r)
	}
	c.offset += size
	return nil
}

func (c *iRegexpChecker) errorf(offset int, format string, args ...any) error {
	return fmt.Errorf("invalid I-Regexp at byte %d: %s", offset, fmt.Sprintf(format, args...))
}

func isNormalIRegexpChar(r rune) bool {
	return r <= 0x27 || r == ',' || r == '-' ||
		(r >= 0x2f && r <= 0x3e) ||
		(r >= 0x40 && r <= 0x5a) ||
		(r >= 0x5e && r <= 0x7a) ||
		(r >= 0x7e && r <= 0xd7ff) ||
		(r >= 0xe000 && r <= utf8.MaxRune)
}

func isIRegexpSingleCharEscape(r rune) bool {
	return (r >= '(' && r <= '+') || r == '-' || r == '.' || r == '?' ||
		(r >= '[' && r <= '^') || r == 'n' || r == 'r' || r == 't' ||
		(r >= '{' && r <= '}')
}

func isIRegexpCharacterClassChar(r rune) bool {
	return r <= 0x2c ||
		(r >= 0x2e && r <= 0x5a) ||
		(r >= 0x5e && r <= 0xd7ff) ||
		(r >= 0xe000 && r <= utf8.MaxRune)
}

func isIRegexpUnicodeProperty(property string) bool {
	switch property {
	case "L", "Ll", "Lm", "Lo", "Lt", "Lu",
		"M", "Mc", "Me", "Mn",
		"N", "Nd", "Nl", "No",
		"P", "Pc", "Pd", "Pe", "Pf", "Pi", "Po", "Ps",
		"Z", "Zl", "Zp", "Zs",
		"S", "Sc", "Sk", "Sm", "So",
		"C", "Cc", "Cf", "Cn", "Co":
		return true
	default:
		return false
	}
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
