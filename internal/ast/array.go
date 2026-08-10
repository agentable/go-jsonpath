package ast

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
