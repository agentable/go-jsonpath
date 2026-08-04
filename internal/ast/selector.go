package ast

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// SelectorKind identifies the variant stored in a [Selector].
type SelectorKind uint8

// SelectorKind values.
const (
	Name     SelectorKind = iota // member name selector
	Index                        // array index selector
	Slice                        // array slice selector
	Wildcard                     // wildcard selector
	Filter                       // filter selector
)

// Selector is a tagged union representing one of the five RFC 9535 selector
// types. Using a concrete struct instead of an interface keeps selector slices
// contiguous in memory for cache efficiency.
type Selector struct {
	Kind   SelectorKind // Kind identifies which selector field is active.
	Name   string       // Name is the member name for [Name] selectors.
	Index  int64        // Index is the array index for [Index] selectors.
	Slice  SliceArgs    // Slice holds the bounds for [Slice] selectors.
	Filter *FilterExpr  // Filter holds the predicate for [Filter] selectors.
}

// SliceArgs holds the optional start, end, and step for a slice selector.
type SliceArgs struct {
	Start    int64 // Start is the explicit slice start when [SliceArgs.HasStart] is true.
	End      int64 // End is the explicit slice end when [SliceArgs.HasEnd] is true.
	Step     int64 // Step is the explicit slice step when [SliceArgs.HasStep] is true.
	HasStart bool  // HasStart reports whether Start was present in the query.
	HasEnd   bool  // HasEnd reports whether End was present in the query.
	HasStep  bool  // HasStep reports whether Step was present in the query.
}

// NameSelector returns a Selector for a member name.
func NameSelector(name string) Selector {
	return Selector{Kind: Name, Name: name}
}

// IndexSelector returns a Selector for an array index.
func IndexSelector(idx int64) Selector {
	return Selector{Kind: Index, Index: idx}
}

// SliceSelector returns a Selector for an array slice.
func SliceSelector(args SliceArgs) Selector {
	return Selector{Kind: Slice, Slice: args}
}

// WildcardSelector returns a wildcard Selector.
func WildcardSelector() Selector {
	return Selector{Kind: Wildcard}
}

// FilterSelector returns a filter Selector.
func FilterSelector(expr *FilterExpr) Selector {
	return Selector{Kind: Filter, Filter: expr}
}

// IsSingular reports whether the selector can select at most one node.
// Only name and index selectors are singular.
func (s *Selector) IsSingular() bool {
	return s.Kind == Name || s.Kind == Index
}

// OutputEstimate returns an upper-bound estimate for how many nodes s can
// append when applied to node.
func (s *Selector) OutputEstimate(node any) int {
	switch s.Kind {
	case Name, Index:
		return 1
	case Wildcard, Filter:
		return ChildCount(node)
	case Slice:
		if arr, ok := node.([]any); ok {
			bounds, ok := ResolveSliceBounds(s.Slice, len(arr))
			if ok {
				return bounds.Count()
			}
		}
	}
	return 0
}

// writeTo writes the canonical string representation of s to buf.
func (s *Selector) writeTo(buf *strings.Builder) {
	switch s.Kind {
	case Name:
		writeStringLiteral(buf, s.Name)
	case Index:
		buf.WriteString(strconv.FormatInt(s.Index, 10))
	case Slice:
		s.Slice.writeTo(buf)
	case Wildcard:
		buf.WriteByte('*')
	case Filter:
		buf.WriteString("?")
		s.Filter.writeTo(buf)
	}
}

func writeStringLiteral(buf *strings.Builder, s string) {
	buf.WriteByte('"')
	for i := 0; i < len(s); {
		if s[i] < utf8.RuneSelf {
			switch s[i] {
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
			case '"':
				buf.WriteString(`\"`)
			case '\\':
				buf.WriteString(`\\`)
			default:
				if s[i] < ' ' {
					buf.WriteString(`\u00`)
					buf.WriteByte(hexDigit(s[i] >> 4))
					buf.WriteByte(hexDigit(s[i] & 0x0f))
				} else {
					buf.WriteByte(s[i])
				}
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		buf.WriteRune(r)
		i += size
	}
	buf.WriteByte('"')
}

func hexDigit(v byte) byte {
	if v < 10 {
		return '0' + v
	}
	return 'a' + (v - 10)
}

// String returns the canonical string representation of s.
func (s *Selector) String() string {
	var buf strings.Builder
	s.writeTo(&buf)
	return buf.String()
}

// Apply applies the selector to a node and appends matching results to out.
func (s *Selector) Apply(out []any, node, root any) []any {
	switch s.Kind {
	case Name:
		if m, ok := node.(map[string]any); ok {
			if v, ok := m[s.Name]; ok {
				out = append(out, v)
			}
		}
	case Index:
		if arr, ok := node.([]any); ok {
			if idx, ok := NormalizeIndex(s.Index, len(arr)); ok {
				out = append(out, arr[idx])
			}
		}
	case Slice:
		if arr, ok := node.([]any); ok {
			out = s.applySlice(out, arr)
		}
	case Wildcard:
		out = slices.Grow(out, s.OutputEstimate(node))
		ForEachChild(node, func(child ChildNode) bool {
			out = append(out, child.Value)
			return true
		})
	case Filter:
		if out != nil {
			out = slices.Grow(out, s.OutputEstimate(node))
		}
		ForEachChild(node, func(child ChildNode) bool {
			if s.Filter.Eval(child.Value, root) {
				out = append(out, child.Value)
			}
			return true
		})
	}
	return out
}

// applySlice applies a slice selector to an array.
func (s *Selector) applySlice(out []any, arr []any) []any {
	bounds, ok := ResolveSliceBounds(s.Slice, len(arr))
	if !ok {
		return out
	}

	n := bounds.Count()
	if n == 0 {
		return out
	}

	out = slices.Grow(out, n)
	bounds.ForEachSliceIndex(func(index int) bool {
		out = append(out, arr[index])
		return true
	})
	return out
}

// writeTo writes the canonical slice notation (e.g. "1:5:2") to buf.
func (a *SliceArgs) writeTo(buf *strings.Builder) {
	if a.HasStart {
		buf.WriteString(strconv.FormatInt(a.Start, 10))
	}
	buf.WriteByte(':')
	if a.HasEnd {
		buf.WriteString(strconv.FormatInt(a.End, 10))
	}
	if a.HasStep {
		buf.WriteByte(':')
		buf.WriteString(strconv.FormatInt(a.Step, 10))
	}
}
