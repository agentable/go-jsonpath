// Package jsonpath implements RFC 9535 JSONPath queries for Go values.
package jsonpath

import (
	stdjson "encoding/json"
	"errors"
	"io"
	"slices"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/agentable/go-jsonpath/internal/ast"
)

// Path is a compiled RFC 9535 JSONPath query. Safe for concurrent use.
type Path struct {
	query *ast.PathQuery
}

type locatedDescendantFrame struct {
	node any
	path NormalizedPath
}

func estimateSelectorsOutput(selectors []ast.Selector, node any) int {
	n := 0
	for i := range selectors {
		n += selectors[i].OutputEstimate(node)
	}
	return n
}

// Select returns all nodes matched by p in input.
// input must be a decoded JSON value (any / []any / map[string]any / primitive).
func (p Path) Select(input any) NodeList {
	if p.query == nil {
		return nil
	}
	return NodeList(p.query.Select(input, input))
}

// SelectLocated returns matched nodes paired with their normalized paths.
func (p Path) SelectLocated(input any) LocatedNodeList {
	if p.query == nil {
		return nil
	}
	res := []LocatedNode{{Value: input}}
	segments := p.query.Segments()
	for i := range segments {
		res = applySegmentLocated(&segments[i], res, input)
	}
	return LocatedNodeList(res)
}

// String returns the canonical string representation of p.
func (p Path) String() string {
	if p.query == nil {
		return ""
	}
	return p.query.String()
}

// MarshalText implements encoding.TextMarshaler.
func (p Path) MarshalText() ([]byte, error) {
	if p.query == nil {
		return nil, ErrInvalidPath
	}
	return []byte(p.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *Path) UnmarshalText(text []byte) error {
	if p == nil {
		return ErrInvalidPath
	}
	path, err := Parse(string(text))
	if err != nil {
		return err
	}
	*p = path
	return nil
}

// Parse compiles a JSONPath expression. Returns ErrPathParse on failure.
func Parse(expr string) (Path, error) {
	p, err := NewParser()
	if err != nil {
		return Path{}, err
	}
	return p.Parse(expr)
}

// MustParse compiles a JSONPath expression. Panics on failure.
func MustParse(expr string) Path {
	path, err := Parse(expr)
	if err != nil {
		panic(err)
	}
	return path
}

// Valid reports whether expr is a syntactically valid JSONPath expression.
func Valid(expr string) bool {
	_, err := Parse(expr)
	return err == nil
}

// QueryJSON unmarshals src, preserving JSON numbers as [encoding/json.Number],
// and evaluates path against it.
func QueryJSON(src []byte, path Path) (NodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSON(src)
	if err != nil {
		return nil, err
	}
	return path.Select(v), nil
}

// QueryJSONRead unmarshals JSON from r, preserving numbers as
// [encoding/json.Number], and evaluates path.
func QueryJSONRead(r io.Reader, path Path) (NodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSONRead(r)
	if err != nil {
		return nil, err
	}
	return path.Select(v), nil
}

// QueryJSONLocated unmarshals src, preserving JSON numbers as
// [encoding/json.Number], and evaluates path against it, returning matched nodes
// together with their normalized paths.
func QueryJSONLocated(src []byte, path Path) (LocatedNodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSON(src)
	if err != nil {
		return nil, err
	}
	return path.SelectLocated(v), nil
}

// QueryJSONReadLocated unmarshals JSON from r, preserving numbers as
// [encoding/json.Number], and evaluates path, returning matched nodes together
// with their normalized paths.
func QueryJSONReadLocated(r io.Reader, path Path) (LocatedNodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSONRead(r)
	if err != nil {
		return nil, err
	}
	return path.SelectLocated(v), nil
}

func unmarshalJSON(src []byte) (any, error) {
	var v any
	if err := json.Unmarshal(src, &v, preserveJSONNumbers); err != nil {
		return nil, errors.Join(ErrUnmarshal, err)
	}
	return v, nil
}

func unmarshalJSONRead(r io.Reader) (any, error) {
	var v any
	if err := json.UnmarshalRead(r, &v, preserveJSONNumbers); err != nil {
		return nil, errors.Join(ErrUnmarshal, err)
	}
	return v, nil
}

var preserveJSONNumbers = json.WithUnmarshalers(json.UnmarshalFromFunc(
	func(dec *jsontext.Decoder, dst *any) error {
		if dec.PeekKind() != '0' {
			return errors.ErrUnsupported
		}
		src, err := dec.ReadValue()
		if err != nil {
			return err
		}
		*dst = stdjson.Number(src)
		return nil
	},
))

func extendPath(path NormalizedPath, elem pathStep) NormalizedPath {
	return path.appendStep(elem)
}

func applySegmentLocated(seg *ast.Segment, nodes []LocatedNode, root any) []LocatedNode {
	if len(nodes) == 0 {
		return nodes
	}
	out := make([]LocatedNode, 0, len(nodes))
	if seg.IsDescendant() {
		for _, n := range nodes {
			out = appendDescendantLocated(out, seg, n.Value, n.Path, root)
		}
		return out
	}

	selectors := seg.Selectors()
	if len(selectors) == 1 {
		sel := &selectors[0]
		for _, n := range nodes {
			if sel.Kind != ast.Name && sel.Kind != ast.Index {
				if growth := sel.OutputEstimate(n.Value); growth > 0 {
					out = slices.Grow(out, growth)
				}
			}
			out = appendSelectorLocated(out, sel, n.Value, n.Path, root)
		}
	} else {
		for _, n := range nodes {
			if growth := estimateSelectorsOutput(selectors, n.Value); growth > 0 {
				out = slices.Grow(out, growth)
			}
			out = appendSelectorsLocated(out, selectors, n.Value, n.Path, root)
		}
	}
	return out
}

func appendDescendantLocated(out []LocatedNode, seg *ast.Segment, node any, path NormalizedPath, root any) []LocatedNode {
	stack := make([]locatedDescendantFrame, 1, 8)
	stack[0] = locatedDescendantFrame{node: node, path: path}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		out = appendSelectorsLocated(out, seg.Selectors(), frame.node, frame.path, root)

		stack = slices.Grow(stack, ast.ChildCount(frame.node))
		ast.ForEachChildReverse(frame.node, func(child ast.ChildNode) bool {
			stack = append(stack, locatedDescendantFrame{
				node: child.Value,
				path: extendPath(frame.path, childPathElement(child)),
			})
			return true
		})
	}
	return out
}

func appendSelectorsLocated(out []LocatedNode, selectors []ast.Selector, node any, path NormalizedPath, root any) []LocatedNode {
	for i := range selectors {
		out = appendSelectorLocated(out, &selectors[i], node, path, root)
	}
	return out
}

func appendSelectorLocated(out []LocatedNode, sel *ast.Selector, node any, path NormalizedPath, root any) []LocatedNode {
	switch sel.Kind {
	case ast.Name:
		if m, ok := node.(map[string]any); ok {
			if v, ok := m[sel.Name]; ok {
				out = append(out, LocatedNode{Value: v, Path: extendPath(path, namePathStep(sel.Name))})
			}
		}
	case ast.Index:
		if arr, ok := node.([]any); ok {
			if idx, ok := ast.NormalizeIndex(sel.Index, len(arr)); ok {
				out = append(out, LocatedNode{Value: arr[idx], Path: extendPath(path, indexPathStep(idx))})
			}
		}
	case ast.Slice:
		if arr, ok := node.([]any); ok {
			out = appendSliceLocated(out, arr, path, sel.Slice)
		}
	case ast.Wildcard:
		out = slices.Grow(out, ast.ChildCount(node))
		ast.ForEachChild(node, func(child ast.ChildNode) bool {
			out = append(out, LocatedNode{Value: child.Value, Path: extendPath(path, childPathElement(child))})
			return true
		})
	case ast.Filter:
		out = slices.Grow(out, ast.ChildCount(node))
		ast.ForEachChild(node, func(child ast.ChildNode) bool {
			if sel.Filter.Eval(child.Value, root) {
				out = append(out, LocatedNode{Value: child.Value, Path: extendPath(path, childPathElement(child))})
			}
			return true
		})
	}
	return out
}

func appendSliceLocated(out []LocatedNode, arr []any, path NormalizedPath, args ast.SliceArgs) []LocatedNode {
	bounds, ok := ast.ResolveSliceBounds(args, len(arr))
	if !ok {
		return out
	}

	n := bounds.Count()
	if n == 0 {
		return out
	}

	out = slices.Grow(out, n)
	bounds.ForEachSliceIndex(func(index int) bool {
		out = append(out, LocatedNode{Value: arr[index], Path: extendPath(path, indexPathStep(index))})
		return true
	})
	return out
}

func childPathElement(child ast.ChildNode) pathStep {
	if child.Array {
		return indexPathStep(child.Index)
	}
	return namePathStep(child.Name)
}
