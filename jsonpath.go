// Package jsonpath implements RFC 9535 JSONPath queries for Go values.
package jsonpath

import (
	"slices"

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
