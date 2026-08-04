package ast

import "strings"

// PathQuery is the root of a compiled JSONPath expression. It holds a sequence
// of segments and whether the query is rooted ($) or relative (@).
type PathQuery struct {
	segments []Segment
	root     bool
}

// NewPathQuery creates a [PathQuery]. When root is true it indicates a
// root-identifier ($) query; when false it indicates a current-node (@) query
// used in filter sub-expressions.
func NewPathQuery(root bool, segments ...Segment) *PathQuery {
	return &PathQuery{root: root, segments: segments}
}

// Segments returns the query's segments.
func (q *PathQuery) Segments() []Segment { return q.segments }

// IsRoot reports whether the query starts from the root ($).
func (q *PathQuery) IsRoot() bool { return q.root }

// IsSingular reports whether the query always selects at most one node.
// A query is singular when every segment is singular (child segment with
// exactly one name or index selector) and no segment is a descendant segment.
func (q *PathQuery) IsSingular() bool {
	for i := range q.segments {
		if q.segments[i].IsDescendant() || !q.segments[i].IsSingular() {
			return false
		}
	}
	return true
}

// writeTo writes the canonical string representation of q to buf.
func (q *PathQuery) writeTo(buf *strings.Builder) {
	writeQueryRoot(buf, q.root)
	for i := range q.segments {
		q.segments[i].writeTo(buf)
	}
}

// String returns the canonical string representation of the query,
// e.g. $["a"][0] or @["name"].
func (q *PathQuery) String() string {
	var buf strings.Builder
	q.writeTo(&buf)
	return buf.String()
}

// Select evaluates the query against the given current and root nodes.
// For root queries ($), it evaluates against root. For relative queries (@),
// it evaluates against current.
func (q *PathQuery) Select(current, root any) []any {
	start := root
	if !q.root {
		start = current
	}

	result := []any{start}
	for i := range q.segments {
		result = q.segments[i].Apply(result, root)
	}
	return result
}

func writeQueryRoot(buf *strings.Builder, rooted bool) {
	if rooted {
		buf.WriteByte('$')
		return
	}
	buf.WriteByte('@')
}
