package controls

// PointerInput is the complete primary-pointer snapshot for one frame.
// Pressed and Released are edge events; Down is the current level state.
type PointerInput struct {
	Position Point
	Pressed  bool
	Down     bool
	Released bool
	Movement Vector
	Wheel    Vector
}

// FocusDirection requests one deterministic keyboard focus transition.
type FocusDirection uint8

const (
	// FocusUnchanged leaves keyboard focus unchanged.
	FocusUnchanged FocusDirection = iota
	// FocusNext advances focus in widget declaration order and wraps.
	FocusNext
	// FocusPrevious moves focus backward in widget declaration order and wraps.
	FocusPrevious
)

// Valid reports whether direction is a supported focus request.
func (direction FocusDirection) Valid() bool {
	switch direction {
	case FocusUnchanged, FocusNext, FocusPrevious:
		return true
	default:
		return false
	}
}

// KeyboardInput is the keyboard command snapshot for one frame. Cancel wins
// over Activate when both are set.
type KeyboardInput struct {
	Focus    FocusDirection
	Activate bool
	Cancel   bool
}

// FrameInput groups all routed input for one frame.
type FrameInput struct {
	Pointer  PointerInput
	Keyboard KeyboardInput
}
