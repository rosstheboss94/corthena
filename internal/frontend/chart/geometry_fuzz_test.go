package chart

import (
	"math"
	"testing"
)

func FuzzTransformRoundTrip(f *testing.F) {
	f.Add(0.0, 10.0, -5.0, 5.0, 2.5, 1.25)
	f.Add(-1e100, 1e100, -1e50, 1e50, 0.0, 0.0)
	f.Add(-1.6666666666666666e100, 2e100, -1e50, 1e50, 62.0, 0.0)
	f.Add(math.NaN(), 1.0, 0.0, 1.0, 0.5, 0.5)
	f.Fuzz(func(t *testing.T, minimumX float64, maximumX float64, minimumY float64, maximumY float64, x float64, y float64) {
		transform, err := NewTransform(
			Rect{MinX: minimumX, MinY: minimumY, MaxX: maximumX, MaxY: maximumY},
			Rect{MinX: 0, MinY: 0, MaxX: 1920, MaxY: 1080},
		)
		if err != nil || !finite(x) || !finite(y) {
			return
		}
		screen, err := transform.Forward(Point{X: x, Y: y})
		if err != nil {
			return
		}
		roundTrip, err := transform.Inverse(screen)
		if err != nil {
			t.Fatal(err)
		}
		if !approximatelyEqual(roundTrip.X, x, transform.DataRect().Width()) || !approximatelyEqual(roundTrip.Y, y, transform.DataRect().Height()) {
			t.Fatalf("round trip (%g,%g) -> (%g,%g)", x, y, roundTrip.X, roundTrip.Y)
		}
	})
}

func approximatelyEqual(left float64, right float64, span float64) bool {
	difference := math.Abs(left - right)
	scale := max(1, math.Abs(left), math.Abs(right), math.Abs(span))
	return difference <= scale*1e-10
}
