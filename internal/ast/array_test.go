package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		idx    int64
		length int
		want   int
		wantOK bool
	}{
		{"positive in bounds", 2, 5, 2, true},
		{"zero", 0, 5, 0, true},
		{"last element", 4, 5, 4, true},
		{"negative -1", -1, 5, 4, true},
		{"negative -2", -2, 5, 3, true},
		{"negative all", -5, 5, 0, true},
		{"out of bounds positive", 10, 5, 0, false},
		{"out of bounds negative", -10, 5, 0, false},
		{"empty array", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := NormalizeIndex(tt.idx, tt.length)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
