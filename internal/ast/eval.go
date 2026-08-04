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
		for i := len(names) - 1; i >= 0; i-- {
			name := names[i]
			if !yield(ChildNode{Value: v[name], Name: name}) {
				return
			}
		}
	case []any:
		for index := len(v) - 1; index >= 0; index-- {
			if !yield(ChildNode{Value: v[index], Index: index, Array: true}) {
				return
			}
		}
	}
}

func sortedChildNames(node map[string]any) []string {
	return slices.Sorted(maps.Keys(node))
}

// NormalizeIndex converts a JSONPath array index to a Go slice index.
func NormalizeIndex(index int64, length int) (int, bool) {
	if index < 0 {
		index += int64(length)
	}
	if index < 0 || index >= int64(length) {
		return 0, false
	}
	return int(index), true
}

// SliceBounds holds normalized slice selector bounds.
type SliceBounds struct {
	Start int64
	End   int64
	Step  int64
}

// ResolveSliceBounds normalizes a slice selector for an array length.
func ResolveSliceBounds(args SliceArgs, length int) (SliceBounds, bool) {
	if length == 0 {
		return SliceBounds{}, false
	}

	step := int64(1)
	if args.HasStep {
		step = args.Step
	}
	if step == 0 {
		return SliceBounds{}, false
	}

	var start, end int64
	if step > 0 {
		start = 0
		if args.HasStart {
			start = args.Start
		}
		end = int64(length)
		if args.HasEnd {
			end = args.End
		}
	} else {
		start = int64(length - 1)
		if args.HasStart {
			start = args.Start
		}
		end = -int64(length) - 1
		if args.HasEnd {
			end = args.End
		}
	}

	start, end = normalizeSliceBounds(start, end, step, length)
	return SliceBounds{Start: start, End: end, Step: step}, true
}

func normalizeSliceBounds(start, end, step int64, length int) (int64, int64) {
	if start < 0 {
		start += int64(length)
		if start < 0 && step > 0 {
			start = 0
		}
	} else if start >= int64(length) {
		if step < 0 {
			start = int64(length - 1)
		}
	}

	if end < 0 {
		end += int64(length)
		if end < 0 && step < 0 {
			end = -1
		}
	} else if end > int64(length) {
		end = int64(length)
	}

	return start, end
}

// Count returns the number of indexes selected by b.
func (b SliceBounds) Count() int {
	if b.Step > 0 {
		if b.End <= b.Start {
			return 0
		}
		return int((b.End - b.Start + b.Step - 1) / b.Step)
	}
	if b.Start <= b.End {
		return 0
	}
	return int((b.Start - b.End - b.Step - 1) / -b.Step)
}

// ForEachSliceIndex calls yield for each index selected by b.
func (b SliceBounds) ForEachSliceIndex(yield func(int) bool) {
	if b.Step > 0 {
		for i := b.Start; i < b.End; i += b.Step {
			if !yield(int(i)) {
				return
			}
		}
		return
	}

	for i := b.Start; i > b.End; i += b.Step {
		if !yield(int(i)) {
			return
		}
	}
}
