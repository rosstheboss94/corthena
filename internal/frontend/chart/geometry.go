// Package chart provides first-party, Raylib-independent chart preparation
// and interaction kernels. All numerical work uses float64 until checked
// conversion into render-ready float32 values.
package chart

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrInvalidGeometry reports a non-finite, degenerate, or out-of-range
	// chart geometry value.
	ErrInvalidGeometry = errors.New("invalid chart geometry")
	// ErrInvalidData reports malformed or unordered source data.
	ErrInvalidData = errors.New("invalid chart data")
)

// Point is a double-precision chart coordinate.
type Point struct {
	X float64
	Y float64
}

// Rect is an axis-aligned double-precision rectangle.
type Rect struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

// Valid reports whether all coordinates are finite and both dimensions are
// positive.
func (rect Rect) Valid() bool {
	return finite(rect.MinX) && finite(rect.MinY) && finite(rect.MaxX) && finite(rect.MaxY) &&
		rect.MaxX > rect.MinX && rect.MaxY > rect.MinY
}

// Width returns the rectangle width.
func (rect Rect) Width() float64 { return rect.MaxX - rect.MinX }

// Height returns the rectangle height.
func (rect Rect) Height() float64 { return rect.MaxY - rect.MinY }

// Contains reports whether the inclusive rectangle contains point.
func (rect Rect) Contains(point Point) bool {
	return rect.Valid() && finitePoint(point) && point.X >= rect.MinX && point.X <= rect.MaxX &&
		point.Y >= rect.MinY && point.Y <= rect.MaxY
}

// Transform maps a data rectangle to a screen rectangle. Screen Y grows
// downward, so the Y axis is inverted.
type Transform struct {
	data   Rect
	screen Rect
}

// NewTransform validates and constructs a double-precision transform.
func NewTransform(data Rect, screen Rect) (Transform, error) {
	if !data.Valid() || !screen.Valid() {
		return Transform{}, fmt.Errorf("%w: data and screen rectangles must be finite and non-degenerate", ErrInvalidGeometry)
	}
	for _, coordinate := range []float64{screen.MinX, screen.MinY, screen.MaxX, screen.MaxY} {
		if _, err := checkedFloat32(coordinate); err != nil {
			return Transform{}, err
		}
	}
	return Transform{data: data, screen: screen}, nil
}

// DataRect returns the immutable data rectangle.
func (transform Transform) DataRect() Rect { return transform.data }

// ScreenRect returns the immutable screen rectangle.
func (transform Transform) ScreenRect() Rect { return transform.screen }

// Forward maps a data coordinate into screen space.
func (transform Transform) Forward(point Point) (Point, error) {
	if !transform.data.Valid() || !transform.screen.Valid() || !finitePoint(point) {
		return Point{}, fmt.Errorf("%w: cannot transform non-finite point", ErrInvalidGeometry)
	}
	x := transform.screen.MinX + (point.X-transform.data.MinX)*transform.screen.Width()/transform.data.Width()
	y := transform.screen.MaxY - (point.Y-transform.data.MinY)*transform.screen.Height()/transform.data.Height()
	if !finite(x) || !finite(y) {
		return Point{}, fmt.Errorf("%w: transformed point is non-finite", ErrInvalidGeometry)
	}
	return Point{X: x, Y: y}, nil
}

// Inverse maps a screen coordinate into data space.
func (transform Transform) Inverse(point Point) (Point, error) {
	if !transform.data.Valid() || !transform.screen.Valid() || !finitePoint(point) {
		return Point{}, fmt.Errorf("%w: cannot invert non-finite point", ErrInvalidGeometry)
	}
	x := transform.data.MinX + (point.X-transform.screen.MinX)*transform.data.Width()/transform.screen.Width()
	y := transform.data.MinY + (transform.screen.MaxY-point.Y)*transform.data.Height()/transform.screen.Height()
	if !finite(x) || !finite(y) {
		return Point{}, fmt.Errorf("%w: inverted point is non-finite", ErrInvalidGeometry)
	}
	return Point{X: x, Y: y}, nil
}

// Point32 is a checked render-ready coordinate.
type Point32 struct {
	X float32
	Y float32
}

// ToPoint32 converts a finite in-range point for a native draw adapter.
func ToPoint32(point Point) (Point32, error) {
	x, err := checkedFloat32(point.X)
	if err != nil {
		return Point32{}, err
	}
	y, err := checkedFloat32(point.Y)
	if err != nil {
		return Point32{}, err
	}
	return Point32{X: x, Y: y}, nil
}

// ClipSegment clips a line segment to bounds using the Liang-Barsky method.
func ClipSegment(bounds Rect, start Point, end Point) (Point, Point, bool, error) {
	if !bounds.Valid() || !finitePoint(start) || !finitePoint(end) {
		return Point{}, Point{}, false, fmt.Errorf("%w: invalid clipping input", ErrInvalidGeometry)
	}
	dx := end.X - start.X
	dy := end.Y - start.Y
	p := [4]float64{-dx, dx, -dy, dy}
	q := [4]float64{start.X - bounds.MinX, bounds.MaxX - start.X, start.Y - bounds.MinY, bounds.MaxY - start.Y}
	u1, u2 := 0.0, 1.0
	for index := range p {
		if p[index] == 0 {
			if q[index] < 0 {
				return Point{}, Point{}, false, nil
			}
			continue
		}
		ratio := q[index] / p[index]
		if p[index] < 0 {
			if ratio > u2 {
				return Point{}, Point{}, false, nil
			}
			u1 = max(u1, ratio)
		} else {
			if ratio < u1 {
				return Point{}, Point{}, false, nil
			}
			u2 = min(u2, ratio)
		}
	}
	return Point{X: start.X + u1*dx, Y: start.Y + u1*dy},
		Point{X: start.X + u2*dx, Y: start.Y + u2*dy}, true, nil
}

// Ticks returns deterministic "nice" ticks covering the requested range.
func Ticks(minimum float64, maximum float64, target int) ([]float64, error) {
	if !finite(minimum) || !finite(maximum) || maximum <= minimum || target < 2 {
		return nil, fmt.Errorf("%w: invalid tick range or target", ErrInvalidGeometry)
	}
	raw := (maximum - minimum) / float64(target-1)
	power := math.Pow(10, math.Floor(math.Log10(raw)))
	fraction := raw / power
	step := power
	switch {
	case fraction <= 1:
		step = power
	case fraction <= 2:
		step = 2 * power
	case fraction <= 5:
		step = 5 * power
	default:
		step = 10 * power
	}
	first := math.Ceil(minimum/step) * step
	last := math.Floor(maximum/step) * step
	if !finite(first) || !finite(last) || last < first {
		return nil, fmt.Errorf("%w: tick calculation overflow", ErrInvalidGeometry)
	}
	count := int(math.Round((last-first)/step)) + 1
	if count < 0 || count > 1_000_000 {
		return nil, fmt.Errorf("%w: unreasonable tick count", ErrInvalidGeometry)
	}
	result := make([]float64, 0, count)
	for index := 0; index < count; index++ {
		value := first + float64(index)*step
		if math.Abs(value) < step*1e-12 {
			value = 0
		}
		result = append(result, value)
	}
	return result, nil
}

func checkedFloat32(value float64) (float32, error) {
	if !finite(value) || value > math.MaxFloat32 || value < -math.MaxFloat32 {
		return 0, fmt.Errorf("%w: coordinate %g cannot be represented as float32", ErrInvalidGeometry, value)
	}
	converted := float32(value)
	if math.IsInf(float64(converted), 0) || math.IsNaN(float64(converted)) {
		return 0, fmt.Errorf("%w: coordinate %g converted to a non-finite float32", ErrInvalidGeometry, value)
	}
	return converted, nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func finitePoint(point Point) bool { return finite(point.X) && finite(point.Y) }
