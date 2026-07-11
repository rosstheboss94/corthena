package chart

import (
	"reflect"
	"testing"
)

func TestInteractionReplayIsDeterministic(t *testing.T) {
	t.Parallel()
	viewport := Rect{MinX: 0, MinY: 0, MaxX: 100, MaxY: 100}
	initial := InteractionState{View: Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}, Fit: Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}}
	inputs := []InteractionInput{
		{Pointer: Point{X: 50, Y: 50}, WheelY: 1},
		{Pointer: Point{X: 40, Y: 40}, PointerPressed: true, PointerDown: true},
		{Pointer: Point{X: 50, Y: 60}, PointerDown: true},
		{Pointer: Point{X: 50, Y: 60}, PointerReleased: true},
		{Pointer: Point{X: 20, Y: 20}, PointerPressed: true, PointerDown: true, SelectModifier: true},
		{Pointer: Point{X: 80, Y: 70}, PointerReleased: true, SelectModifier: true},
		{Pointer: Point{X: 10, Y: 10}, ToggleSeriesID: "prediction"},
		{Pointer: Point{X: 10, Y: 10}, Reset: true},
	}
	replay := func() (InteractionState, []InteractionEvent) {
		state := initial
		var events []InteractionEvent
		for _, input := range inputs {
			var reduced []InteractionEvent
			var err error
			state, reduced, err = ReduceInteraction(state, viewport, input)
			if err != nil {
				t.Fatal(err)
			}
			events = append(events, reduced...)
		}
		return state, events
	}
	firstState, firstEvents := replay()
	secondState, secondEvents := replay()
	if !reflect.DeepEqual(firstState, secondState) || !reflect.DeepEqual(firstEvents, secondEvents) {
		t.Fatalf("replay differs:\n%+v\n%+v", firstState, secondState)
	}
	if firstState.View != firstState.Fit || firstState.Selected || len(firstState.Series) != 1 || firstState.Series[0].Visible {
		t.Fatalf("final state = %+v", firstState)
	}
}

func TestBoxSelectionConvertsScreenToData(t *testing.T) {
	t.Parallel()
	viewport := Rect{MinX: 0, MinY: 0, MaxX: 100, MaxY: 100}
	state := InteractionState{View: Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}, Fit: Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}}
	state, _, _ = ReduceInteraction(state, viewport, InteractionInput{Pointer: Point{X: 20, Y: 30}, PointerPressed: true, PointerDown: true, SelectModifier: true})
	state, events, err := ReduceInteraction(state, viewport, InteractionInput{Pointer: Point{X: 80, Y: 90}, PointerReleased: true, SelectModifier: true})
	if err != nil {
		t.Fatal(err)
	}
	want := Rect{MinX: 2, MinY: 1, MaxX: 8, MaxY: 7}
	if !state.Selected || state.Selection != want || len(events) == 0 || events[len(events)-1].Kind != EventRangeSelected {
		t.Fatalf("selection = %+v, events=%+v", state.Selection, events)
	}
}
