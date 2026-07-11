package chart

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestTransformForwardInverseAndClip(t *testing.T) {
	t.Parallel()
	transform, err := NewTransform(Rect{MinX: 0, MinY: -10, MaxX: 100, MaxY: 10}, Rect{MinX: 10, MinY: 20, MaxX: 210, MaxY: 120})
	if err != nil {
		t.Fatal(err)
	}
	screen, err := transform.Forward(Point{X: 25, Y: 5})
	if err != nil {
		t.Fatal(err)
	}
	if screen != (Point{X: 60, Y: 45}) {
		t.Fatalf("forward = %+v, want {60 45}", screen)
	}
	data, err := transform.Inverse(screen)
	if err != nil {
		t.Fatal(err)
	}
	if data != (Point{X: 25, Y: 5}) {
		t.Fatalf("inverse = %+v, want {25 5}", data)
	}
	start, end, visible, err := ClipSegment(Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}, Point{X: -5, Y: 5}, Point{X: 15, Y: 5})
	if err != nil || !visible || start != (Point{X: 0, Y: 5}) || end != (Point{X: 10, Y: 5}) {
		t.Fatalf("clip = %+v %+v %t, err=%v", start, end, visible, err)
	}
}

func TestGeometryRejectsDegenerateAndNonFiniteValues(t *testing.T) {
	t.Parallel()
	if _, err := NewTransform(Rect{}, Rect{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1}); !errors.Is(err, ErrInvalidGeometry) {
		t.Fatalf("degenerate error = %v", err)
	}
	if _, err := ToPoint32(Point{X: math.Inf(1)}); !errors.Is(err, ErrInvalidGeometry) {
		t.Fatalf("non-finite error = %v", err)
	}
	if _, err := ToPoint32(Point{X: math.MaxFloat64}); !errors.Is(err, ErrInvalidGeometry) {
		t.Fatalf("out-of-range error = %v", err)
	}
	_, _, visible, err := ClipSegment(Rect{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1}, Point{X: -2, Y: -2}, Point{X: -1, Y: -1})
	if err != nil || visible {
		t.Fatalf("outside clip visible=%t err=%v", visible, err)
	}
}

func TestTicksUseDeterministicNiceStep(t *testing.T) {
	t.Parallel()
	got, err := Ticks(-1.2, 8.7, 6)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{0, 2, 4, 6, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ticks = %v, want %v", got, want)
	}
}
