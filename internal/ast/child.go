package ast

import (
	"maps"
	"slices"
)

// ChildNode is one direct child of a JSON object or array.
type ChildNode struct {
	Value any
	Name  string
	Index int
	Array bool
}

// ChildCount returns the number of direct children in node.
func ChildCount(node any) int {
	switch v := node.(type) {
	case map[string]any:
		return len(v)
	case []any:
		return len(v)
	default:
		return 0
	}
}

// ForEachChild calls yield for each direct child of node.
func ForEachChild(node any, yield func(ChildNode) bool) {
	switch v := node.(type) {
	case map[string]any:
		for _, name := range sortedChildNames(v) {
			if !yield(ChildNode{Value: v[name], Name: name}) {
				return
			}
		}
	case []any:
		for index, child := range v {
			if !yield(ChildNode{Value: child, Index: index, Array: true}) {
				return
			}
		}
	}
}

// ForEachChildReverse calls yield for direct children in stack-push order.
func ForEachChildReverse(node any, yield func(ChildNode) bool) {
	switch v := node.(type) {
	case map[string]any:
		names := sortedChildNames(v)
		for _, name := range slices.Backward(names) {
			if !yield(ChildNode{Value: v[name], Name: name}) {
				return
			}
		}
	case []any:
		for index, v0 := range slices.Backward(v) {
			if !yield(ChildNode{Value: v0, Index: index, Array: true}) {
				return
			}
		}
	}
}

func sortedChildNames(node map[string]any) []string {
	return slices.Sorted(maps.Keys(node))
}
