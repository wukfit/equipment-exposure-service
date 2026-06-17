package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPartialExposure(t *testing.T) {
	cases := []struct {
		name      string
		magnitude float64
		minutes   int
		wantA8    float64
		wantPts   float64
	}{
		{"aircat 30min", 2.1, 30, 0.525, 4},
		{"aircat 60min", 2.1, 60, 0.7425, 9},
		{"jcb 30min", 4.0, 30, 1.0, 16},
		{"jcb 2h", 4.0, 120, 2.0, 64},
		{"jcb 8h", 4.0, 480, 4.0, 256},
		{"zero duration", 2.1, 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := NewPartialExposure(c.magnitude, c.minutes)
			assert.InDelta(t, c.wantA8, p.A8(), 0.001)
			assert.Equal(t, c.wantPts, p.Points())
		})
	}
}

func TestAggregate(t *testing.T) {
	t.Run("empty is zero", func(t *testing.T) {
		got := Aggregate(nil)
		assert.Equal(t, 0.0, got.A8())
		assert.Equal(t, 0.0, got.Points())
	})
	t.Run("single is identity", func(t *testing.T) {
		p := NewPartialExposure(2.1, 30)
		got := Aggregate([]PartialExposure{p})
		assert.InDelta(t, p.A8(), got.A8(), 0.0001)
		assert.Equal(t, p.Points(), got.Points())
	})
	t.Run("rss for a8, sum for points", func(t *testing.T) {
		parts := []PartialExposure{NewPartialExposure(2.1, 30), NewPartialExposure(4.0, 120)}
		got := Aggregate(parts)
		assert.InDelta(t, 2.0678, got.A8(), 0.001) // sqrt(0.525^2 + 2.0^2)
		assert.Equal(t, 68.0, got.Points())        // 4 + 64
	})
}

func TestNewPartialExposureExtra(t *testing.T) {
	t.Run("order independence of Aggregate", func(t *testing.T) {
		// RSS is commutative; confirm [a,b] and [b,a] produce identical results.
		a := NewPartialExposure(2.1, 30)
		b := NewPartialExposure(4.0, 120)
		ab := Aggregate([]PartialExposure{a, b})
		ba := Aggregate([]PartialExposure{b, a})
		assert.InDelta(t, ab.A8(), ba.A8(), 0.0001)
		assert.Equal(t, ab.Points(), ba.Points())
	})

	t.Run("points rounding at 0.5 boundary", func(t *testing.T) {
		// mag=3.0, minutes=45 (0.75 h):
		//   raw = (3.0/2.5)^2 * (0.75/8) * 100
		//       = 1.44 * 0.09375 * 100
		//       = 13.5
		// math.Round(13.5) = 14  (Go rounds half away from zero for positive values)
		p := NewPartialExposure(3.0, 45)
		assert.Equal(t, 14.0, p.Points())
	})

	t.Run("large duration jcb 16h", func(t *testing.T) {
		// mag=4.0, minutes=960 (16 h):
		//   A8    = 4.0 * sqrt(16/8) = 4.0 * sqrt(2) ≈ 5.6569
		//   raw   = (4.0/2.5)^2 * (16/8) * 100 = 2.56 * 2 * 100 = 512.0
		//   Points = math.Round(512.0) = 512
		p := NewPartialExposure(4.0, 960)
		assert.InDelta(t, 5.6569, p.A8(), 0.001)
		assert.Equal(t, 512.0, p.Points())
	})

	t.Run("aggregate three elements", func(t *testing.T) {
		// a: mag=2.1, min=30  → A8=0.525,    pts=4
		// b: mag=4.0, min=120 → A8=2.0,      pts=64
		// c: mag=3.0, min=45  → A8=3.0*sqrt(0.75/8)=3.0*0.30619≈0.91856, pts=14
		//
		// RSS A8 = sqrt(0.525^2 + 2.0^2 + 0.91856^2)
		//        = sqrt(0.275625 + 4.0 + 0.84376)
		//        = sqrt(5.11938)
		//        ≈ 2.2627
		// Points = 4 + 64 + 14 = 82
		a := NewPartialExposure(2.1, 30)
		b := NewPartialExposure(4.0, 120)
		c := NewPartialExposure(3.0, 45)
		got := Aggregate([]PartialExposure{a, b, c})
		assert.InDelta(t, 2.2627, got.A8(), 0.001)
		assert.Equal(t, 82.0, got.Points())
	})
}
