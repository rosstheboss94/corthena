package controls

import "errors"

var (
	// ErrClipStackUnderflow identifies a pop without a matching push.
	ErrClipStackUnderflow = errors.New("clip stack underflow")
	// ErrUnbalancedClipStack identifies outstanding clips at a frame boundary.
	ErrUnbalancedClipStack = errors.New("clip stack is unbalanced")
)

// Clip is an optional logical clipping rectangle. The zero value is
// unclipped.
type Clip struct {
	rect   Rect
	active bool
}

// ClipTo constructs an active clip rectangle.
func ClipTo(rect Rect) Clip {
	return Clip{rect: rect, active: true}
}

// Bounds returns the clip rectangle and whether clipping is active.
func (clip Clip) Bounds() (Rect, bool) {
	return clip.rect, clip.active
}

// Contains reports whether a point passes the clip. An inactive clip accepts
// every point; an active empty clip accepts none.
func (clip Clip) Contains(point Point) bool {
	return !clip.active || clip.rect.Contains(point)
}

// ClipStack intersects nested logical clips. The zero value is ready for use
// and must have depth zero at a frame boundary.
type ClipStack struct {
	clips []Rect
}

// Push adds rect and returns the effective intersection with its parent clip.
func (stack *ClipStack) Push(rect Rect) Clip {
	effective := rect
	if len(stack.clips) > 0 {
		var ok bool
		effective, ok = stack.clips[len(stack.clips)-1].Intersect(rect)
		if !ok {
			effective.Width = 0
			effective.Height = 0
		}
	}
	stack.clips = append(stack.clips, effective)
	return ClipTo(effective)
}

// Pop removes the current clip. It returns ErrClipStackUnderflow when no clip
// is active.
func (stack *ClipStack) Pop() error {
	if len(stack.clips) == 0 {
		return ErrClipStackUnderflow
	}
	stack.clips = stack.clips[:len(stack.clips)-1]
	return nil
}

// Current returns the effective current clip. The zero value is returned when
// the stack is empty.
func (stack *ClipStack) Current() Clip {
	if len(stack.clips) == 0 {
		return Clip{}
	}
	return ClipTo(stack.clips[len(stack.clips)-1])
}

// Depth returns the number of unmatched pushes.
func (stack *ClipStack) Depth() int {
	return len(stack.clips)
}

// Balanced reports whether every push has a matching pop.
func (stack *ClipStack) Balanced() bool {
	return len(stack.clips) == 0
}

// CheckBalanced returns ErrUnbalancedClipStack when clips remain active.
func (stack *ClipStack) CheckBalanced() error {
	if !stack.Balanced() {
		return ErrUnbalancedClipStack
	}
	return nil
}
