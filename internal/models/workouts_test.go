package models

import (
	"math"
	"testing"
)

func TestFiniteOrNil(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := finiteOrNil(v); got != nil {
			t.Errorf("finiteOrNil(%v) = %v, want nil", v, *got)
		}
	}
	for _, v := range []float64{0, 3.1, -2} {
		got := finiteOrNil(v)
		if got == nil || *got != v {
			t.Errorf("finiteOrNil(%v) = %v, want %v", v, got, v)
		}
	}
}
