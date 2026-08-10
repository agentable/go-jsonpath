package jsonpath

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// PathElement is either a Name (string key) or an Index (array index)
// in a normalized path. Implemented by [NameElement] and [IndexElement].
type PathElement interface {
	pathElement() pathStep
}

type pathStepKind uint8

const (
	pathStepIndex pathStepKind = iota
	pathStepName
)

type pathStep struct {
	kind  pathStepKind
	name  string
	index int
}

// NameElement is a valid UTF-8 string key in a normalized path.
type NameElement string

func (n NameElement) pathElement() pathStep {
	return namePathStep(string(n))
}

// writeNormalizedTo writes n to buf as ['name'] with proper escaping per
// RFC 9535 §2.7.
func (n NameElement) writeNormalizedTo(buf *strings.Builder) {
	s := string(n)
	buf.WriteString("['")
	for i := 0; i < len(n); {
		if n[i] < utf8.RuneSelf {
			switch n[i] {
			case '\b':
				buf.WriteString(`\b`)
			case '\f':
				buf.WriteString(`\f`)
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			case '\t':
				buf.WriteString(`\t`)
			case '\'':
				buf.WriteString(`\'`)
			case '\\':
				buf.WriteString(`\\`)
			default:
				if n[i] < ' ' {
					buf.WriteString(`\u00`)
					buf.WriteByte(lowerHex(n[i] >> 4))
					buf.WriteByte(lowerHex(n[i] & 0x0f))
				} else {
					buf.WriteByte(n[i])
				}
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		buf.WriteRune(r)
		i += size
	}
	buf.WriteString("']")
}

// writePointerTo writes n to buf as an RFC 6901 JSON Pointer reference token,
// escaping ~ as ~0 and / as ~1.
func (n NameElement) writePointerTo(buf *strings.Builder) {
	for i := range len(n) {
		switch n[i] {
		case '~':
			buf.WriteString("~0")
		case '/':
			buf.WriteString("~1")
		default:
			buf.WriteByte(n[i])
		}
	}
}

// IndexElement is an array index in a normalized path.
type IndexElement int

func (i IndexElement) pathElement() pathStep {
	return indexPathStep(int(i))
}

// writeNormalizedTo writes i to buf as [N].
func (i IndexElement) writeNormalizedTo(buf *strings.Builder) {
	buf.WriteByte('[')
	buf.WriteString(strconv.Itoa(int(i)))
	buf.WriteByte(']')
}

// writePointerTo writes i to buf as its decimal string.
func (i IndexElement) writePointerTo(buf *strings.Builder) {
	buf.WriteString(strconv.Itoa(int(i)))
}

func namePathStep(name string) pathStep {
	return pathStep{kind: pathStepName, name: name}
}

func indexPathStep(index int) pathStep {
	return pathStep{kind: pathStepIndex, index: index}
}

func (s pathStep) element() PathElement {
	if s.kind == pathStepName {
		return NameElement(s.name)
	}
	return IndexElement(s.index)
}

func (s pathStep) writeNormalizedTo(buf *strings.Builder) {
	if s.kind == pathStepName {
		NameElement(s.name).writeNormalizedTo(buf)
		return
	}
	IndexElement(s.index).writeNormalizedTo(buf)
}

func (s pathStep) writePointerTo(buf *strings.Builder) {
	if s.kind == pathStepName {
		NameElement(s.name).writePointerTo(buf)
		return
	}
	IndexElement(s.index).writePointerTo(buf)
}

// NormalizedPath is an immutable sequence of Name/Index selectors per
// RFC 9535 §2.7. The zero value is the root path.
type NormalizedPath struct {
	elements []pathStep
}

// NewNormalizedPath returns a normalized path from validated elements.
func NewNormalizedPath(elements ...PathElement) (NormalizedPath, error) {
	if len(elements) == 0 {
		return NormalizedPath{}, nil
	}
	steps := make([]pathStep, len(elements))
	for i := range elements {
		step, err := pathStepFromElement(elements[i])
		if err != nil {
			return NormalizedPath{}, err
		}
		steps[i] = step
	}
	return NormalizedPath{elements: steps}, nil
}

// Len returns the number of path elements.
func (p NormalizedPath) Len() int {
	return len(p.elements)
}

// Element returns the i'th path element.
func (p NormalizedPath) Element(i int) PathElement {
	return p.elements[i].element()
}

// Elements returns a copy of p's path elements.
func (p NormalizedPath) Elements() []PathElement {
	elements := make([]PathElement, len(p.elements))
	for i := range p.elements {
		elements[i] = p.elements[i].element()
	}
	return elements
}

// Append returns a path with a validated element appended.
func (p NormalizedPath) Append(elem PathElement) (NormalizedPath, error) {
	step, err := pathStepFromElement(elem)
	if err != nil {
		return NormalizedPath{}, err
	}
	return p.appendStep(step), nil
}

func pathStepFromElement(elem PathElement) (pathStep, error) {
	var step pathStep
	switch elem := elem.(type) {
	case nil:
		return pathStep{}, fmt.Errorf("%w: nil normalized path element", ErrInvalidPath)
	case NameElement:
		step = namePathStep(string(elem))
	case *NameElement:
		if elem == nil {
			return pathStep{}, fmt.Errorf("%w: nil normalized path element", ErrInvalidPath)
		}
		step = namePathStep(string(*elem))
	case IndexElement:
		step = indexPathStep(int(elem))
	case *IndexElement:
		if elem == nil {
			return pathStep{}, fmt.Errorf("%w: nil normalized path element", ErrInvalidPath)
		}
		step = indexPathStep(int(*elem))
	default:
		return pathStep{}, fmt.Errorf("%w: unsupported normalized path element %T", ErrInvalidPath, elem)
	}
	if step.kind == pathStepName && !utf8.ValidString(step.name) {
		return pathStep{}, fmt.Errorf("%w: normalized path name is not valid UTF-8", ErrInvalidPath)
	}
	if step.kind == pathStepIndex && step.index < 0 {
		return pathStep{}, fmt.Errorf("%w: negative normalized path index %d", ErrInvalidPath, step.index)
	}
	return step, nil
}

func (p NormalizedPath) appendStep(step pathStep) NormalizedPath {
	elements := make([]pathStep, len(p.elements)+1)
	copy(elements, p.elements)
	elements[len(p.elements)] = step
	return NormalizedPath{elements: elements}
}

// String returns the normalized path string, e.g. $['a'][0].
func (p NormalizedPath) String() string {
	var buf strings.Builder
	buf.Grow(p.normalizedLen())
	buf.WriteByte('$')
	for _, e := range p.elements {
		e.writeNormalizedTo(&buf)
	}
	return buf.String()
}

// Pointer returns an RFC 6901 JSON Pointer string, e.g. /a/0.
func (p NormalizedPath) Pointer() string {
	var buf strings.Builder
	buf.Grow(p.pointerLen())
	for _, e := range p.elements {
		buf.WriteByte('/')
		e.writePointerTo(&buf)
	}
	return buf.String()
}

// Compare compares p to q and returns -1, 0, or 1. Indexes are always
// considered less than names.
func (p NormalizedPath) Compare(q NormalizedPath) int {
	minLen := min(len(p.elements), len(q.elements))

	for i := range minLen {
		v1 := p.elements[i]
		v2 := q.elements[i]

		if v1.kind == pathStepName && v2.kind == pathStepName {
			if x := cmp.Compare(v1.name, v2.name); x != 0 {
				return x
			}
			continue
		}

		if v1.kind != v2.kind {
			return cmp.Compare(v1.kind, v2.kind)
		}

		if x := cmp.Compare(v1.index, v2.index); x != 0 {
			return x
		}
	}

	return cmp.Compare(len(p.elements), len(q.elements))
}

// Equal reports whether p and q contain the same path elements.
func (p NormalizedPath) Equal(q NormalizedPath) bool {
	return p.Compare(q) == 0
}

func (p NormalizedPath) normalizedLen() int {
	n := 1
	for _, elem := range p.elements {
		switch elem.kind {
		case pathStepName:
			n += 4
			for _, r := range elem.name {
				switch r {
				case '\b', '\f', '\n', '\r', '\t', '\'', '\\':
					n += 2
				default:
					if r < ' ' {
						n += 6
					} else {
						n += utf8.RuneLen(r)
					}
				}
			}
		case pathStepIndex:
			n += 2 + digits10(elem.index)
		}
	}
	return n
}

func (p NormalizedPath) pointerLen() int {
	n := 0
	for _, elem := range p.elements {
		n++
		switch elem.kind {
		case pathStepName:
			for _, r := range elem.name {
				switch r {
				case '~', '/':
					n += 2
				default:
					n += utf8.RuneLen(r)
				}
			}
		case pathStepIndex:
			n += digits10(elem.index)
		}
	}
	return n
}

func (p NormalizedPath) hash() uint64 {
	h := uint64(14695981039346656037)
	for _, elem := range p.elements {
		switch elem.kind {
		case pathStepName:
			h = hashByte(h, 'n')
			h = hashString(h, elem.name)
		case pathStepIndex:
			h = hashByte(h, 'i')
			h = hashInt64(h, int64(elem.index))
		}
	}
	return h
}

func lowerHex(v byte) byte {
	if v < 10 {
		return '0' + v
	}
	return 'a' + (v - 10)
}

func digits10(v int) int {
	if v == 0 {
		return 1
	}
	n := 0
	if v < 0 {
		n++
		v = -v
	}
	for v > 0 {
		v /= 10
		n++
	}
	return n
}

func hashByte(h uint64, b byte) uint64 {
	h ^= uint64(b)
	return h * 1099511628211
}

func hashString(h uint64, s string) uint64 {
	for i := range len(s) {
		h = hashByte(h, s[i])
	}
	return h
}

func hashInt64(h uint64, v int64) uint64 {
	var buf [20]byte
	digits := strconv.AppendInt(buf[:0], v, 10)
	for _, b := range digits {
		h = hashByte(h, b)
	}
	return h
}

// MarshalText marshals p into its normalized path string. Implements
// [encoding.TextMarshaler].
func (p NormalizedPath) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}
