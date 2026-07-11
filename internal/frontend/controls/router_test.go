package controls

import (
	"errors"
	"slices"
	"testing"
)

func TestRouteUsesTopmostClippingAwareHitAndStableActionOrder(t *testing.T) {
	t.Parallel()

	back := Widget{
		ID:        NewWidgetID("back"),
		Bounds:    Rect{Width: 100, Height: 100},
		Pointer:   PointerAction,
		Focusable: true,
	}
	front := Widget{
		ID:        NewWidgetID("front"),
		Bounds:    Rect{Width: 100, Height: 100},
		Clip:      ClipTo(Rect{X: 50, Width: 50, Height: 100}),
		Pointer:   PointerAction,
		Focusable: true,
	}

	pressed, err := Route(State{}, FrameInput{Pointer: PointerInput{
		Position: Point{X: 25, Y: 25},
		Pressed:  true,
		Down:     true,
	}}, []Widget{back, front})
	if err != nil {
		t.Fatalf("route press: %v", err)
	}
	if pressed.State.Hot != back.ID || pressed.State.Active != back.ID || pressed.State.Focused != back.ID {
		t.Fatalf("press state = %+v", pressed.State)
	}
	if pressed.State.PointerCapture.Valid() {
		t.Fatalf("action widget captured pointer: %q", pressed.State.PointerCapture)
	}
	requireEventKinds(t, pressed.Events,
		EventPointerEnter,
		EventFocusGained,
		EventPointerPressed,
	)

	released, err := Route(pressed.State, FrameInput{Pointer: PointerInput{
		Position: Point{X: 25, Y: 25},
		Released: true,
	}}, []Widget{back, front})
	if err != nil {
		t.Fatalf("route release: %v", err)
	}
	requireEventKinds(t, released.Events, EventPointerReleased, EventActivated)
	if released.Events[1].Source != ActivationPointer || !released.Events[0].Inside {
		t.Fatalf("release events = %+v", released.Events)
	}
	if released.State.Active.Valid() || released.State.PointerCapture.Valid() {
		t.Fatalf("release retained pointer state: %+v", released.State)
	}

	frontHit, err := Route(State{}, FrameInput{Pointer: PointerInput{
		Position: Point{X: 75, Y: 25},
	}}, []Widget{back, front})
	if err != nil {
		t.Fatalf("route front hit: %v", err)
	}
	if frontHit.State.Hot != front.ID {
		t.Fatalf("topmost clipped hit = %q, want %q", frontHit.State.Hot, front.ID)
	}
}

func TestPointerCaptureDeliversCompleteDragOutsideClip(t *testing.T) {
	t.Parallel()

	id := NewWidgetID("dock", "splitter")
	widget := Widget{
		ID:        id,
		Bounds:    Rect{X: 10, Y: 10, Width: 20, Height: 80},
		Clip:      ClipTo(Rect{X: 0, Y: 0, Width: 100, Height: 100}),
		Pointer:   PointerDrag,
		Focusable: true,
	}

	started, err := Route(State{}, FrameInput{Pointer: PointerInput{
		Position: Point{X: 15, Y: 20},
		Pressed:  true,
		Down:     true,
	}}, []Widget{widget})
	if err != nil {
		t.Fatalf("route drag start: %v", err)
	}
	requireEventKinds(t, started.Events,
		EventPointerEnter,
		EventFocusGained,
		EventPointerPressed,
		EventDragStarted,
	)
	if started.State.Active != id || started.State.PointerCapture != id {
		t.Fatalf("drag start state = %+v", started.State)
	}

	moved, err := Route(started.State, FrameInput{Pointer: PointerInput{
		Position: Point{X: 150, Y: 120},
		Down:     true,
		Movement: Vector{X: 135, Y: 100},
	}}, []Widget{widget})
	if err != nil {
		t.Fatalf("route captured move: %v", err)
	}
	requireEventKinds(t, moved.Events,
		EventPointerLeave,
		EventPointerMoved,
		EventDragMoved,
	)
	if moved.Events[1].Widget != id || moved.Events[2].Movement != (Vector{X: 135, Y: 100}) {
		t.Fatalf("captured movement events = %+v", moved.Events)
	}
	if moved.State.Hot.Valid() || moved.State.PointerCapture != id {
		t.Fatalf("captured move state = %+v", moved.State)
	}

	ended, err := Route(moved.State, FrameInput{Pointer: PointerInput{
		Position: Point{X: 150, Y: 120},
		Released: true,
	}}, []Widget{widget})
	if err != nil {
		t.Fatalf("route drag end: %v", err)
	}
	requireEventKinds(t, ended.Events, EventPointerReleased, EventDragEnded)
	if ended.Events[0].Inside || ended.State.Active.Valid() || ended.State.PointerCapture.Valid() {
		t.Fatalf("drag end = events %+v, state %+v", ended.Events, ended.State)
	}
}

func TestPointerCaptureCancelsWhenButtonLevelDropsWithoutRelease(t *testing.T) {
	t.Parallel()

	id := NewWidgetID("drag")
	widget := Widget{ID: id, Bounds: Rect{Width: 20, Height: 20}, Pointer: PointerDrag}
	started, err := Route(State{}, FrameInput{Pointer: PointerInput{
		Position: Point{X: 5, Y: 5},
		Pressed:  true,
		Down:     true,
	}}, []Widget{widget})
	if err != nil {
		t.Fatalf("route start: %v", err)
	}

	canceled, err := Route(started.State, FrameInput{Pointer: PointerInput{
		Position: Point{X: 5, Y: 5},
	}}, []Widget{widget})
	if err != nil {
		t.Fatalf("route cancel: %v", err)
	}
	requireEventKinds(t, canceled.Events, EventDragCanceled, EventCanceled)
	if canceled.State.Active.Valid() || canceled.State.PointerCapture.Valid() {
		t.Fatalf("cancel retained pointer state: %+v", canceled.State)
	}
}

func TestKeyboardFocusWrapActivationAndCancelAreDeterministic(t *testing.T) {
	t.Parallel()

	first := Widget{ID: NewWidgetID("first"), Focusable: true, Pointer: PointerAction}
	disabled := Widget{ID: NewWidgetID("disabled"), Focusable: true, Disabled: true, Pointer: PointerAction}
	second := Widget{ID: NewWidgetID("second"), Focusable: true, Pointer: PointerAction}
	widgets := []Widget{first, disabled, second}

	one, err := Route(State{}, FrameInput{Keyboard: KeyboardInput{Focus: FocusNext}}, widgets)
	if err != nil {
		t.Fatalf("route first focus: %v", err)
	}
	requireEventKinds(t, one.Events, EventFocusGained)
	if one.State.Focused != first.ID {
		t.Fatalf("first focus = %q", one.State.Focused)
	}

	two, err := Route(one.State, FrameInput{Keyboard: KeyboardInput{
		Focus:    FocusNext,
		Activate: true,
	}}, widgets)
	if err != nil {
		t.Fatalf("route second focus: %v", err)
	}
	requireEventKinds(t, two.Events, EventFocusLost, EventFocusGained, EventActivated)
	if two.State.Focused != second.ID || two.Events[2].Source != ActivationKeyboard {
		t.Fatalf("second focus result = %+v", two)
	}

	previous, err := Route(two.State, FrameInput{Keyboard: KeyboardInput{Focus: FocusPrevious}}, widgets)
	if err != nil {
		t.Fatalf("route previous focus: %v", err)
	}
	if previous.State.Focused != first.ID {
		t.Fatalf("previous focus = %q", previous.State.Focused)
	}

	canceled, err := Route(previous.State, FrameInput{Keyboard: KeyboardInput{
		Activate: true,
		Cancel:   true,
	}}, widgets)
	if err != nil {
		t.Fatalf("route keyboard cancel: %v", err)
	}
	requireEventKinds(t, canceled.Events, EventCanceled)
	if canceled.State.Focused != first.ID {
		t.Fatalf("cancel cleared focus: %+v", canceled.State)
	}
}

func TestWheelAndMovementRouteToHotWidgetInDeclarationOrder(t *testing.T) {
	t.Parallel()

	back := Widget{ID: NewWidgetID("back"), Bounds: Rect{Width: 50, Height: 50}, Pointer: PointerHover}
	front := Widget{ID: NewWidgetID("front"), Bounds: Rect{Width: 50, Height: 50}, Pointer: PointerHover}
	result, err := Route(State{}, FrameInput{Pointer: PointerInput{
		Position: Point{X: 10, Y: 10},
		Movement: Vector{X: 2, Y: 3},
		Wheel:    Vector{X: 1, Y: -4},
	}}, []Widget{back, front})
	if err != nil {
		t.Fatalf("route hover: %v", err)
	}
	requireEventKinds(t, result.Events, EventPointerEnter, EventPointerMoved, EventWheel)
	for _, event := range result.Events {
		if event.Widget != front.ID {
			t.Fatalf("event routed to %q, want %q", event.Widget, front.ID)
		}
	}
}

func TestReplayProducesIdenticalStatesAndEventOrder(t *testing.T) {
	t.Parallel()

	id := NewWidgetID("replay", "action")
	widget := Widget{
		ID:        id,
		Bounds:    Rect{Width: 40, Height: 20},
		Pointer:   PointerAction,
		Focusable: true,
	}
	frames := []ReplayFrame{
		{
			Input: FrameInput{Pointer: PointerInput{
				Position: Point{X: 10, Y: 10},
				Pressed:  true,
				Down:     true,
			}},
			Widgets: []Widget{widget},
		},
		{
			Input: FrameInput{Pointer: PointerInput{
				Position: Point{X: 10, Y: 10},
				Released: true,
			}},
			Widgets: []Widget{widget},
		},
		{
			Input:   FrameInput{Keyboard: KeyboardInput{Activate: true}},
			Widgets: []Widget{widget},
		},
	}

	first, err := Replay(State{}, frames)
	if err != nil {
		t.Fatalf("first replay: %v", err)
	}
	second, err := Replay(State{}, frames)
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}
	if first.Final != second.Final || len(first.Frames) != len(second.Frames) {
		t.Fatalf("replay summaries differ: first %+v, second %+v", first, second)
	}
	for index := range first.Frames {
		if first.Frames[index].State != second.Frames[index].State ||
			!slices.Equal(first.Frames[index].Events, second.Frames[index].Events) {
			t.Fatalf("replay frame %d differs: first %+v, second %+v", index, first.Frames[index], second.Frames[index])
		}
	}
	requireEventKinds(t, first.Frames[1].Events, EventPointerReleased, EventActivated)
	requireEventKinds(t, first.Frames[2].Events, EventActivated)
}

func TestRouteRejectsInvalidTypedFrames(t *testing.T) {
	t.Parallel()

	id := NewWidgetID("duplicate")
	widget := Widget{ID: id, Pointer: PointerAction}
	_, err := Route(State{}, FrameInput{}, []Widget{widget, widget})
	if !errors.Is(err, ErrDuplicateWidgetID) {
		t.Fatalf("duplicate error = %v", err)
	}

	_, err = Route(State{}, FrameInput{}, []Widget{{Pointer: PointerAction}})
	if !errors.Is(err, ErrInvalidWidget) {
		t.Fatalf("empty ID error = %v", err)
	}

	_, err = Route(State{}, FrameInput{Keyboard: KeyboardInput{Focus: FocusDirection(99)}}, []Widget{widget})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid focus error = %v", err)
	}
}

func TestRemovedCapturedWidgetEmitsStableCancellation(t *testing.T) {
	t.Parallel()

	id := NewWidgetID("removed-drag")
	state := State{Active: id, PointerCapture: id}
	result, err := Route(state, FrameInput{Pointer: PointerInput{Down: true}}, nil)
	if err != nil {
		t.Fatalf("route removed widget: %v", err)
	}
	requireEventKinds(t, result.Events, EventDragCanceled, EventCanceled)
	if result.State.Active.Valid() || result.State.PointerCapture.Valid() {
		t.Fatalf("removed capture retained state: %+v", result.State)
	}
}

func requireEventKinds(t *testing.T, events []Event, want ...EventKind) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d: %+v", len(events), len(want), events)
	}
	for index, kind := range want {
		if events[index].Kind != kind {
			t.Fatalf("event %d kind = %d, want %d: %+v", index, events[index].Kind, kind, events)
		}
	}
}
