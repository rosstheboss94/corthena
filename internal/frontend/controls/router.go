package controls

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidWidget identifies a malformed widget declaration.
	ErrInvalidWidget = errors.New("invalid widget")
	// ErrDuplicateWidgetID identifies repeated IDs in one widget frame.
	ErrDuplicateWidgetID = errors.New("duplicate widget ID")
	// ErrInvalidInput identifies an unsupported typed input value.
	ErrInvalidInput = errors.New("invalid controls input")
)

// State is the complete persistent interaction state between frames.
type State struct {
	Hot            WidgetID
	Active         WidgetID
	Focused        WidgetID
	PointerCapture WidgetID
}

// EventKind identifies one routed widget transition.
type EventKind uint8

const (
	// EventFocusLost reports that a widget lost keyboard focus.
	EventFocusLost EventKind = iota + 1
	// EventFocusGained reports that a widget gained keyboard focus.
	EventFocusGained
	// EventPointerLeave reports that the pointer left a widget's hit region.
	EventPointerLeave
	// EventPointerEnter reports that the pointer entered a widget's hit region.
	EventPointerEnter
	// EventPointerPressed reports a primary-pointer press.
	EventPointerPressed
	// EventDragStarted begins a pointer-captured drag lifecycle.
	EventDragStarted
	// EventPointerMoved reports routed pointer movement.
	EventPointerMoved
	// EventDragMoved reports captured drag movement.
	EventDragMoved
	// EventWheel reports routed horizontal or vertical wheel movement.
	EventWheel
	// EventPointerReleased reports a primary-pointer release.
	EventPointerReleased
	// EventDragEnded completes a captured drag lifecycle.
	EventDragEnded
	// EventDragCanceled terminates a drag without a release.
	EventDragCanceled
	// EventCanceled reports cancellation of an active or focused interaction.
	EventCanceled
	// EventActivated reports pointer or keyboard activation.
	EventActivated
)

// ActivationSource identifies the input path that activated a widget.
type ActivationSource uint8

const (
	// ActivationNone is used by non-activation events.
	ActivationNone ActivationSource = iota
	// ActivationPointer identifies release-inside pointer activation.
	ActivationPointer
	// ActivationKeyboard identifies focused keyboard activation.
	ActivationKeyboard
)

// Event is an immutable, typed widget input event. Fields unrelated to Kind
// retain their zero values.
type Event struct {
	Kind     EventKind
	Widget   WidgetID
	Position Point
	Movement Vector
	Wheel    Vector
	Inside   bool
	Source   ActivationSource
}

// FrameResult contains state and events after routing one frame.
type FrameResult struct {
	State  State
	Events []Event
}

// ReplayFrame is one immutable input and widget declaration in a replay.
type ReplayFrame struct {
	Input   FrameInput
	Widgets []Widget
}

// ReplayResult contains every frame result and the final replay state.
type ReplayResult struct {
	Frames []FrameResult
	Final  State
}

// Route deterministically routes one input frame. Widgets are treated as an
// immutable back-to-front declaration. Events have stable phase order:
// invalidation, cancel, focus traversal, hot transition, press, movement,
// wheel, release, then keyboard activation. Within transitions, loss/cancel
// precedes gain/start/end/activation.
func Route(previous State, input FrameInput, widgets []Widget) (FrameResult, error) {
	widgetIndexes, err := validateFrame(input, widgets)
	if err != nil {
		return FrameResult{}, err
	}

	state := previous
	events := make([]Event, 0, 8)
	state, events = invalidateState(state, widgets, widgetIndexes, input.Pointer.Position, events)

	if input.Keyboard.Cancel {
		state, events = cancelInteraction(state, input.Pointer.Position, events)
	}

	if input.Keyboard.Focus != FocusUnchanged {
		state, events = moveFocus(state, widgets, input.Keyboard.Focus, input.Pointer.Position, events)
	}

	hot := hitTest(widgets, input.Pointer.Position)
	if state.Hot != hot {
		if state.Hot.Valid() {
			events = append(events, eventAt(EventPointerLeave, state.Hot, input.Pointer.Position))
		}
		if hot.Valid() {
			events = append(events, eventAt(EventPointerEnter, hot, input.Pointer.Position))
		}
	}
	state.Hot = hot

	if input.Pointer.Pressed {
		state, events = pressPointer(state, widgets, widgetIndexes, input.Pointer.Position, events)
	}

	if !input.Pointer.Movement.IsZero() {
		target := state.PointerCapture
		if !target.Valid() {
			target = state.Hot
		}
		if target.Valid() {
			event := eventAt(EventPointerMoved, target, input.Pointer.Position)
			event.Movement = input.Pointer.Movement
			events = append(events, event)
			if state.PointerCapture == target && (input.Pointer.Down || input.Pointer.Pressed) {
				event.Kind = EventDragMoved
				events = append(events, event)
			}
		}
	}

	if !input.Pointer.Wheel.IsZero() && state.Hot.Valid() {
		event := eventAt(EventWheel, state.Hot, input.Pointer.Position)
		event.Wheel = input.Pointer.Wheel
		events = append(events, event)
	}

	if input.Pointer.Released {
		state, events = releasePointer(state, widgets, widgetIndexes, input.Pointer.Position, events)
	} else if !input.Pointer.Pressed && !input.Pointer.Down && state.Active.Valid() {
		state, events = cancelInteraction(state, input.Pointer.Position, events)
	}

	if input.Keyboard.Activate && !input.Keyboard.Cancel && state.Focused.Valid() {
		if index, found := widgetIndexes[state.Focused]; found && widgets[index].canFocus() {
			event := eventAt(EventActivated, state.Focused, input.Pointer.Position)
			event.Source = ActivationKeyboard
			events = append(events, event)
		}
	}

	return FrameResult{State: state, Events: events}, nil
}

// Replay applies typed frames sequentially without mutating the inputs. The
// same initial state and frame sequence always produce the same result.
func Replay(initial State, frames []ReplayFrame) (ReplayResult, error) {
	result := ReplayResult{
		Frames: make([]FrameResult, 0, len(frames)),
		Final:  initial,
	}
	for frameIndex, frame := range frames {
		frameResult, err := Route(result.Final, frame.Input, frame.Widgets)
		if err != nil {
			return result, fmt.Errorf("replay frame %d: %w", frameIndex, err)
		}
		result.Frames = append(result.Frames, frameResult)
		result.Final = frameResult.State
	}
	return result, nil
}

func validateFrame(input FrameInput, widgets []Widget) (map[WidgetID]int, error) {
	if !input.Keyboard.Focus.Valid() {
		return nil, fmt.Errorf("%w: focus direction %d", ErrInvalidInput, input.Keyboard.Focus)
	}
	indexes := make(map[WidgetID]int, len(widgets))
	for index, widget := range widgets {
		if !widget.ID.Valid() {
			return nil, fmt.Errorf("%w at index %d: empty ID", ErrInvalidWidget, index)
		}
		if !widget.Pointer.Valid() {
			return nil, fmt.Errorf("%w %q: pointer behavior %d", ErrInvalidWidget, widget.ID, widget.Pointer)
		}
		if _, duplicate := indexes[widget.ID]; duplicate {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateWidgetID, widget.ID)
		}
		indexes[widget.ID] = index
	}
	return indexes, nil
}

func invalidateState(
	state State,
	widgets []Widget,
	indexes map[WidgetID]int,
	position Point,
	events []Event,
) (State, []Event) {
	if state.PointerCapture.Valid() && !validDragOwner(state.PointerCapture, widgets, indexes) {
		events = append(events,
			eventAt(EventDragCanceled, state.PointerCapture, position),
			eventAt(EventCanceled, state.PointerCapture, position),
		)
		if state.Active == state.PointerCapture {
			state.Active = ""
		}
		state.PointerCapture = ""
	}
	if state.Active.Valid() && !validActiveOwner(state.Active, widgets, indexes) {
		events = append(events, eventAt(EventCanceled, state.Active, position))
		state.Active = ""
	}
	if state.Focused.Valid() && !validFocusOwner(state.Focused, widgets, indexes) {
		events = append(events, eventAt(EventFocusLost, state.Focused, position))
		state.Focused = ""
	}
	return state, events
}

func cancelInteraction(
	state State,
	position Point,
	events []Event,
) (State, []Event) {
	target := state.PointerCapture
	if target.Valid() {
		events = append(events, eventAt(EventDragCanceled, target, position))
	} else {
		target = state.Active
	}
	if !target.Valid() {
		target = state.Focused
	}
	if target.Valid() {
		events = append(events, eventAt(EventCanceled, target, position))
	}
	state.Active = ""
	state.PointerCapture = ""
	return state, events
}

func moveFocus(
	state State,
	widgets []Widget,
	direction FocusDirection,
	position Point,
	events []Event,
) (State, []Event) {
	focusable := make([]WidgetID, 0, len(widgets))
	for _, widget := range widgets {
		if widget.canFocus() {
			focusable = append(focusable, widget.ID)
		}
	}
	if len(focusable) == 0 {
		if state.Focused.Valid() {
			events = append(events, eventAt(EventFocusLost, state.Focused, position))
			state.Focused = ""
		}
		return state, events
	}

	current := -1
	for index, id := range focusable {
		if id == state.Focused {
			current = index
			break
		}
	}
	next := 0
	if direction == FocusPrevious {
		next = len(focusable) - 1
		if current >= 0 {
			next = (current - 1 + len(focusable)) % len(focusable)
		}
	} else if current >= 0 {
		next = (current + 1) % len(focusable)
	}

	return setFocus(state, focusable[next], position, events)
}

func hitTest(widgets []Widget, position Point) WidgetID {
	for index := len(widgets) - 1; index >= 0; index-- {
		if widgets[index].HitTest(position) {
			return widgets[index].ID
		}
	}
	return ""
}

func pressPointer(
	state State,
	widgets []Widget,
	indexes map[WidgetID]int,
	position Point,
	events []Event,
) (State, []Event) {
	if state.Active.Valid() {
		state, events = cancelInteraction(state, position, events)
	}
	index, found := indexes[state.Hot]
	if !found {
		return state, events
	}
	widget := widgets[index]
	if !widget.canOwnActivePointer() {
		return state, events
	}
	if widget.canFocus() {
		state, events = setFocus(state, widget.ID, position, events)
	}
	state.Active = widget.ID
	events = append(events, eventAt(EventPointerPressed, widget.ID, position))
	if widget.Pointer == PointerDrag {
		state.PointerCapture = widget.ID
		events = append(events, eventAt(EventDragStarted, widget.ID, position))
	}
	return state, events
}

func releasePointer(
	state State,
	widgets []Widget,
	indexes map[WidgetID]int,
	position Point,
	events []Event,
) (State, []Event) {
	target := state.PointerCapture
	if !target.Valid() {
		target = state.Active
	}
	if !target.Valid() {
		return state, events
	}
	inside := state.Hot == target
	event := eventAt(EventPointerReleased, target, position)
	event.Inside = inside
	events = append(events, event)

	index, found := indexes[target]
	if state.PointerCapture == target {
		event.Kind = EventDragEnded
		events = append(events, event)
	} else if found && widgets[index].Pointer == PointerAction && inside {
		event.Kind = EventActivated
		event.Source = ActivationPointer
		events = append(events, event)
	}
	state.Active = ""
	state.PointerCapture = ""
	return state, events
}

func setFocus(state State, target WidgetID, position Point, events []Event) (State, []Event) {
	if state.Focused == target {
		return state, events
	}
	if state.Focused.Valid() {
		events = append(events, eventAt(EventFocusLost, state.Focused, position))
	}
	state.Focused = target
	if target.Valid() {
		events = append(events, eventAt(EventFocusGained, target, position))
	}
	return state, events
}

func validDragOwner(id WidgetID, widgets []Widget, indexes map[WidgetID]int) bool {
	index, found := indexes[id]
	return found && !widgets[index].Disabled && widgets[index].Pointer == PointerDrag
}

func validActiveOwner(id WidgetID, widgets []Widget, indexes map[WidgetID]int) bool {
	index, found := indexes[id]
	return found && widgets[index].canOwnActivePointer()
}

func validFocusOwner(id WidgetID, widgets []Widget, indexes map[WidgetID]int) bool {
	index, found := indexes[id]
	return found && widgets[index].canFocus()
}

func eventAt(kind EventKind, widget WidgetID, position Point) Event {
	return Event{Kind: kind, Widget: widget, Position: position}
}
