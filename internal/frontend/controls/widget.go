package controls

// PointerBehavior defines how a widget participates in pointer routing.
type PointerBehavior uint8

const (
	// PointerAction tracks an active press and activates on release inside.
	PointerAction PointerBehavior = iota
	// PointerDrag captures pointer movement until release or cancellation.
	PointerDrag
	// PointerHover receives hot, movement, and wheel routing without presses.
	PointerHover
	// PointerNone removes a widget from pointer hit testing.
	PointerNone
)

// Valid reports whether behavior is a supported pointer behavior.
func (behavior PointerBehavior) Valid() bool {
	switch behavior {
	case PointerAction, PointerDrag, PointerHover, PointerNone:
		return true
	default:
		return false
	}
}

// Widget describes one typed interaction region for a frame. Widgets are
// declared back-to-front; the last matching widget wins pointer hit testing.
type Widget struct {
	ID        WidgetID
	Bounds    Rect
	Clip      Clip
	Pointer   PointerBehavior
	Focusable bool
	Disabled  bool
}

// HitTest reports whether the enabled widget accepts pointer input at point,
// including its effective clip.
func (widget Widget) HitTest(point Point) bool {
	return !widget.Disabled &&
		widget.Pointer != PointerNone &&
		widget.Bounds.Contains(point) &&
		widget.Clip.Contains(point)
}

func (widget Widget) canFocus() bool {
	return !widget.Disabled && widget.Focusable
}

func (widget Widget) canOwnActivePointer() bool {
	return !widget.Disabled &&
		(widget.Pointer == PointerAction || widget.Pointer == PointerDrag)
}
