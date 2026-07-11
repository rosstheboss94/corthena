package controls

import (
	"errors"
	"testing"
)

func TestWidgetIDHierarchyIsStableAndUnambiguous(t *testing.T) {
	t.Parallel()

	root := NewWidgetID("workspace/data")
	child := root.Child("panel:chart").ChildIndex(7)
	other := NewWidgetID("workspace", "data/panel:chart").ChildIndex(7)

	if !root.Valid() {
		t.Fatal("root ID is invalid")
	}
	if child == other {
		t.Fatalf("distinct hierarchies collided: %q", child)
	}
	if !child.DescendsFrom(root) {
		t.Fatalf("%q does not descend from %q", child, root)
	}
	if root.DescendsFrom(root) {
		t.Fatal("an ID must not be its own descendant")
	}
	if NewWidgetID().Valid() {
		t.Fatal("an ID with no segments must be invalid")
	}
	if NewWidgetID("") == (WidgetID("").ChildIndex(0)) {
		t.Fatal("empty string and numeric segments collided")
	}
}

func TestRectContainsAndIntersectUseHalfOpenLogicalBounds(t *testing.T) {
	t.Parallel()

	rect := Rect{X: 10, Y: 20, Width: 30, Height: 40}
	for _, point := range []Point{{X: 10, Y: 20}, {X: 39.999, Y: 59.999}} {
		if !rect.Contains(point) {
			t.Fatalf("rect does not contain interior point %+v", point)
		}
	}
	for _, point := range []Point{{X: 40, Y: 20}, {X: 10, Y: 60}, {X: 9.999, Y: 20}} {
		if rect.Contains(point) {
			t.Fatalf("rect contains point outside half-open bounds %+v", point)
		}
	}

	intersection, ok := rect.Intersect(Rect{X: 25, Y: 5, Width: 40, Height: 25})
	want := Rect{X: 25, Y: 20, Width: 15, Height: 10}
	if !ok || intersection != want {
		t.Fatalf("intersection = (%+v, %t), want (%+v, true)", intersection, ok, want)
	}

	edge, ok := rect.Intersect(Rect{X: 40, Y: 20, Width: 5, Height: 5})
	if ok || !edge.Empty() {
		t.Fatalf("edge-only intersection = (%+v, %t), want empty", edge, ok)
	}
}

func TestClipStackIntersectsNestedClipsAndChecksBalance(t *testing.T) {
	t.Parallel()

	var stack ClipStack
	if !stack.Balanced() || stack.Depth() != 0 {
		t.Fatal("zero stack is not balanced")
	}
	if _, active := stack.Current().Bounds(); active {
		t.Fatal("empty stack unexpectedly has an active clip")
	}

	root := stack.Push(Rect{X: 0, Y: 0, Width: 100, Height: 80})
	rootBounds, active := root.Bounds()
	if !active || rootBounds != (Rect{X: 0, Y: 0, Width: 100, Height: 80}) {
		t.Fatalf("root clip = (%+v, %t)", rootBounds, active)
	}
	child := stack.Push(Rect{X: 75, Y: 50, Width: 50, Height: 50})
	childBounds, active := child.Bounds()
	wantChild := Rect{X: 75, Y: 50, Width: 25, Height: 30}
	if !active || childBounds != wantChild {
		t.Fatalf("child clip = (%+v, %t), want (%+v, true)", childBounds, active, wantChild)
	}
	if stack.Depth() != 2 || stack.Balanced() {
		t.Fatalf("depth = %d, balanced = %t", stack.Depth(), stack.Balanced())
	}
	if !errors.Is(stack.CheckBalanced(), ErrUnbalancedClipStack) {
		t.Fatalf("CheckBalanced error = %v", stack.CheckBalanced())
	}
	if err := stack.Pop(); err != nil {
		t.Fatalf("pop child: %v", err)
	}
	if err := stack.Pop(); err != nil {
		t.Fatalf("pop root: %v", err)
	}
	if err := stack.CheckBalanced(); err != nil {
		t.Fatalf("balanced stack: %v", err)
	}
	if err := stack.Pop(); !errors.Is(err, ErrClipStackUnderflow) {
		t.Fatalf("underflow error = %v", err)
	}
}

func TestClipStackPropagatesEmptyIntersection(t *testing.T) {
	t.Parallel()

	var stack ClipStack
	stack.Push(Rect{X: 0, Y: 0, Width: 10, Height: 10})
	clip := stack.Push(Rect{X: 20, Y: 20, Width: 5, Height: 5})
	if clip.Contains(Point{X: 20, Y: 20}) {
		t.Fatal("empty nested clip accepted a point")
	}
	bounds, active := clip.Bounds()
	if !active || !bounds.Empty() {
		t.Fatalf("disjoint clip = (%+v, %t), want active and empty", bounds, active)
	}
}
