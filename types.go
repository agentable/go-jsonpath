package jsonpath

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// ErrPathParse is returned when a JSONPath expression cannot be parsed.
	ErrPathParse = errors.New("jsonpath: parse error")
	// ErrFunction is returned for function registration or expression failures.
	ErrFunction = errors.New("jsonpath: function error")
	// ErrUnmarshal is returned when JSON unmarshaling fails in QueryJSON functions.
	ErrUnmarshal = errors.New("jsonpath: unmarshal error")
	// ErrInvalidPath is returned for an invalid compiled or normalized path.
	ErrInvalidPath = errors.New("jsonpath: invalid path")
)

// ParseError describes a JSONPath parse failure in a program-readable form.
type ParseError struct {
	// Offset is the byte offset in the original expression where parsing failed.
	Offset int
	// Reason is a short human-readable reason for the failure.
	Reason string
	// Snippet is a short source excerpt around Offset.
	Snippet string
	// Cause is the underlying lexer, parser, or function validation error.
	Cause error
}

// Error returns a human-readable parse error message.
func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	if e.Snippet != "" {
		return e.Reason + " at position " + strconv.Itoa(e.Offset) + " near " + e.Snippet
	}
	if e.Offset >= 0 {
		return e.Reason + " at position " + strconv.Itoa(e.Offset)
	}
	return e.Reason
}

// Unwrap returns the underlying parse failure cause.
func (e *ParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

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

// LocatedNode pairs a value with the [NormalizedPath] for its location within
// a JSON query argument.
type LocatedNode struct {
	Value any            // Value is the selected JSON value.
	Path  NormalizedPath // Path is the normalized path to Value.
}

// NodeList is a list of nodes selected by a JSONPath query. Each node
// represents a single JSON value selected from the JSON query argument.
type NodeList []any

// All returns an iterator over all the nodes in list.
func (l NodeList) All() iter.Seq[any] {
	return slices.Values(l)
}

// LocatedNodeList is a list of nodes selected by a JSONPath query, along with
// their [NormalizedPath] locations.
type LocatedNodeList []LocatedNode

// All returns an iterator over all the located nodes in list.
func (l LocatedNodeList) All() iter.Seq[LocatedNode] {
	return slices.Values(l)
}

// Values returns an iterator over all the node values in list.
func (l LocatedNodeList) Values() iter.Seq[any] {
	return func(yield func(any) bool) {
		for _, n := range l {
			if !yield(n.Value) {
				return
			}
		}
	}
}

// Paths returns an iterator over all the [NormalizedPath] values in list.
func (l LocatedNodeList) Paths() iter.Seq[NormalizedPath] {
	return func(yield func(NormalizedPath) bool) {
		for _, n := range l {
			if !yield(n.Path) {
				return
			}
		}
	}
}

// Deduplicate deduplicates the nodes in list based on their [NormalizedPath]
// values, modifying the contents of list. It returns the modified list, which
// may have a shorter length, and zeroes the elements between the new length
// and the original length.
func (l LocatedNodeList) Deduplicate() LocatedNodeList {
	return l.deduplicateWithHasher(func(path NormalizedPath) uint64 {
		return path.hash()
	})
}

func (l LocatedNodeList) deduplicateWithHasher(hashFn func(NormalizedPath) uint64) LocatedNodeList {
	if len(l) <= 1 {
		return l
	}

	seen := make(map[uint64]int, len(l))
	var collisions map[uint64][]int
	uniq := l[:0]
	for _, n := range l {
		hash := hashFn(n.Path)
		first, ok := seen[hash]
		if !ok {
			seen[hash] = len(uniq)
			uniq = append(uniq, n)
			continue
		}

		if n.Path.Compare(uniq[first].Path) == 0 {
			continue
		}

		duplicate := false
		if collisions != nil {
			for _, idx := range collisions[hash] {
				if n.Path.Compare(uniq[idx].Path) == 0 {
					duplicate = true
					break
				}
			}
		}
		if duplicate {
			continue
		}

		if collisions == nil {
			collisions = make(map[uint64][]int)
		}
		collisions[hash] = append(collisions[hash], len(uniq))
		uniq = append(uniq, n)
	}
	clear(l[len(uniq):])
	return slices.Clip(uniq)
}

// Sort sorts list by the [NormalizedPath] of each node.
func (l LocatedNodeList) Sort() {
	slices.SortFunc(l, func(a, b LocatedNode) int {
		return a.Path.Compare(b.Path)
	})
}
