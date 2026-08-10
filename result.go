package jsonpath

import (
	"iter"
	"slices"
)

// LocatedNode pairs a value with the [NormalizedPath] for its location within
// a JSON query argument.
type LocatedNode struct {
	Value any            // Value is the selected JSON value.
	Path  NormalizedPath // Path is the normalized path to Value.
}

// NodeList is a list of nodes selected by a JSONPath query.
type NodeList []any

// All returns an iterator over all the nodes in list.
func (l NodeList) All() iter.Seq[any] { return slices.Values(l) }

// LocatedNodeList is a list of nodes and their normalized locations.
type LocatedNodeList []LocatedNode

// All returns an iterator over all the located nodes in list.
func (l LocatedNodeList) All() iter.Seq[LocatedNode] { return slices.Values(l) }

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

// Paths returns an iterator over all the normalized paths in list.
func (l LocatedNodeList) Paths() iter.Seq[NormalizedPath] {
	return func(yield func(NormalizedPath) bool) {
		for _, n := range l {
			if !yield(n.Path) {
				return
			}
		}
	}
}

// Deduplicate removes duplicate paths in place and keeps the first node.
func (l LocatedNodeList) Deduplicate() LocatedNodeList {
	return l.deduplicateWithHasher(func(path NormalizedPath) uint64 { return path.hash() })
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
		for _, idx := range collisions[hash] {
			if n.Path.Compare(uniq[idx].Path) == 0 {
				duplicate = true
				break
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

// Sort orders list by normalized path.
func (l LocatedNodeList) Sort() {
	slices.SortFunc(l, func(a, b LocatedNode) int { return a.Path.Compare(b.Path) })
}
