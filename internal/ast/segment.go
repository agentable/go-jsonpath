package ast

import (
	"slices"
	"strings"
)

// Segment represents a child or descendant segment as defined in
// RFC 9535 §1.4.2. A segment holds one or more selectors.
type Segment struct {
	selectors  []Selector
	descendant bool
}

// Child creates a child [Segment] that applies selectors to direct children.
func Child(sel ...Selector) Segment {
	return Segment{selectors: sel}
}

// Descendant creates a descendant [Segment] that applies selectors recursively
// to all descendants.
func Descendant(sel ...Selector) Segment {
	return Segment{selectors: sel, descendant: true}
}

// Selectors returns the segment's selectors.
func (s *Segment) Selectors() []Selector { return s.selectors }

// IsDescendant reports whether the segment is a descendant segment.
func (s *Segment) IsDescendant() bool { return s.descendant }

// IsSingular reports whether the segment selects at most one node.
// A segment is singular only if it is a child segment with exactly one
// singular selector.
func (s *Segment) IsSingular() bool {
	if s.descendant || len(s.selectors) != 1 {
		return false
	}
	return s.selectors[0].IsSingular()
}

// writeTo writes the canonical string representation of the segment to buf.
// Child segments format as [<selectors>]; descendant segments as ..[<selectors>].
func (s *Segment) writeTo(buf *strings.Builder) {
	if s.descendant {
		buf.WriteString("..")
	}
	buf.WriteByte('[')
	for i := range s.selectors {
		if i > 0 {
			buf.WriteByte(',')
		}
		s.selectors[i].writeTo(buf)
	}
	buf.WriteByte(']')
}

// String returns the canonical string representation of the segment.
func (s *Segment) String() string {
	var buf strings.Builder
	s.writeTo(&buf)
	return buf.String()
}

// Apply applies the segment to a list of nodes and returns the result.
func (s *Segment) Apply(nodes []any, root any) []any {
	if len(nodes) == 0 {
		return nodes
	}

	result := make([]any, 0, len(nodes))
	if s.descendant {
		for _, node := range nodes {
			result = appendDescendant(result, s.selectors, node, root)
		}
		return result
	}

	for _, node := range nodes {
		result = appendSelectors(result, s.selectors, node, root)
	}
	return result
}

// appendSelectors applies selectors to a single node and appends results.
func appendSelectors(out []any, selectors []Selector, node, root any) []any {
	for i := range selectors {
		out = selectors[i].Apply(out, node, root)
	}
	return out
}

// appendDescendant applies selectors to node and all descendants.
func appendDescendant(out []any, selectors []Selector, node, root any) []any {
	stack := []any{node}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		out = appendSelectors(out, selectors, current, root)

		stack = slices.Grow(stack, ChildCount(current))
		ForEachChildReverse(current, func(child ChildNode) bool {
			stack = append(stack, child.Value)
			return true
		})
	}
	return out
}
