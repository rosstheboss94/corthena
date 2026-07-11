package controls

// Point is a position in logical UI coordinates.
type Point struct {
	X float64
	Y float64
}

// Vector is a movement or wheel delta in logical UI coordinates.
type Vector struct {
	X float64
	Y float64
}

// IsZero reports whether both vector components are zero.
func (vector Vector) IsZero() bool {
	return vector.X == 0 && vector.Y == 0
}

// Rect is an axis-aligned rectangle in logical UI coordinates. Width and
// Height must be positive for the rectangle to contain points. Rectangles use
// half-open bounds: the left and top edges are included; right and bottom are
// excluded.
type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// Right returns the rectangle's exclusive right edge.
func (rect Rect) Right() float64 {
	return rect.X + rect.Width
}

// Bottom returns the rectangle's exclusive bottom edge.
func (rect Rect) Bottom() float64 {
	return rect.Y + rect.Height
}

// Empty reports whether the rectangle contains no area.
func (rect Rect) Empty() bool {
	return rect.Width <= 0 || rect.Height <= 0
}

// Contains reports whether point lies within the rectangle's half-open bounds.
func (rect Rect) Contains(point Point) bool {
	return !rect.Empty() &&
		point.X >= rect.X && point.X < rect.Right() &&
		point.Y >= rect.Y && point.Y < rect.Bottom()
}

// Intersect returns the positive-area intersection of rect and other. When
// they do not overlap, the returned rectangle has zero area and ok is false.
func (rect Rect) Intersect(other Rect) (intersection Rect, ok bool) {
	left := maxFloat(rect.X, other.X)
	top := maxFloat(rect.Y, other.Y)
	right := minFloat(rect.Right(), other.Right())
	bottom := minFloat(rect.Bottom(), other.Bottom())

	intersection = Rect{
		X:      left,
		Y:      top,
		Width:  right - left,
		Height: bottom - top,
	}
	if rect.Empty() || other.Empty() || intersection.Empty() {
		intersection.Width = 0
		intersection.Height = 0
		return intersection, false
	}
	return intersection, true
}

func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
