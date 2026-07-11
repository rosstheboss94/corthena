package nativeui

import (
	"reflect"
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/assets"
)

func TestNestedDockRenderingBalancesScissorCalls(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := window.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	state := nestedDockShellState(t)
	if _, err := window.DrawShellFrame(state); err != nil {
		t.Fatal(err)
	}
	if got := countCalls(backend.calls, "beginScissor"); got != 2 {
		t.Fatalf("beginScissor calls = %d, want 2; calls: %v", got, backend.calls)
	}
	if got := countCalls(backend.calls, "endScissor"); got != 2 {
		t.Fatalf("endScissor calls = %d, want 2; calls: %v", got, backend.calls)
	}
}

func TestShellInputReadsDPIAndDragLifecycle(t *testing.T) {
	t.Parallel()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.dpi = point{x: 1.5, y: 1.5}
	backend.leftPressed = true
	backend.leftDown = true
	backend.delta = point{x: 4, y: 0}
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := window.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	if _, err := window.DrawShellFrame(testShellState(t)); err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{
		"windowScaleDPI",
		"mouseDelta",
		"leftMouseDown",
		"leftMouseReleased",
		"mouseWheelMove",
		"tabPressed",
		"shiftDown",
	} {
		if got := countCalls(backend.calls, call); got != 1 {
			t.Fatalf("%s calls = %d, want 1; calls: %v", call, got, backend.calls)
		}
	}
}

func TestDockInputReplayIsDeterministic(t *testing.T) {
	t.Parallel()

	first := replayDockInput(t)
	second := replayDockInput(t)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("dock replay differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	split, ok := first.Root.(appstate.SplitNode)
	if !ok {
		t.Fatalf("replayed root = %T, want SplitNode", first.Root)
	}
	left, ok := split.First.(appstate.TabStackNode)
	if !ok || len(left.Panels) != 1 {
		t.Fatalf("replayed left child = %#v, want one-panel tab stack", split.First)
	}
}

func replayDockInput(t *testing.T) appstate.WorkspaceLayout {
	t.Helper()

	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := window.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	state := testShellState(t)
	layout, found := activeLayout(state)
	if !found {
		t.Fatal("active workspace layout is missing")
	}
	stack, ok := layout.Root.(appstate.TabStackNode)
	if !ok || len(stack.Panels) < 2 {
		t.Fatalf("dock root = %T with %d panels, want tab stack with at least two", layout.Root, len(stack.Panels))
	}

	scale := shellScale(backend.dpi, state.Preferences.UIScale)
	contentY := scale * (topNavHeight + contextBarHeight)
	contentHeight := float32(backend.height) - contentY - scale*statusBarHeight
	main := rectangle{
		x:      scale*260 + scale,
		y:      contentY,
		width:  float32(backend.width) - scale*260 - scale,
		height: contentHeight,
	}
	host := inset(main, scale*8)
	visual := (&shellRenderer{window: window, scale: scale}).makeDockStackVisual(stack, host)
	source := point{
		x: visual.tabs[1].bounds.x + visual.tabs[1].bounds.width/2,
		y: visual.tabs[1].bounds.y + visual.tabs[1].bounds.height/2,
	}
	target := point{x: host.x + host.width*0.1, y: host.y + host.height/2}
	frames := []struct {
		mouse    point
		delta    point
		pressed  bool
		down     bool
		released bool
	}{
		{mouse: source, pressed: true, down: true},
		{mouse: target, delta: point{x: target.x - source.x, y: target.y - source.y}, down: true},
		{mouse: target, released: true},
	}

	for frameIndex, frame := range frames {
		backend.mouse = frame.mouse
		backend.delta = frame.delta
		backend.leftPressed = frame.pressed
		backend.leftDown = frame.down
		backend.leftReleased = frame.released
		actions, err := window.DrawShellFrame(state)
		if err != nil {
			t.Fatalf("draw replay frame %d: %v", frameIndex, err)
		}
		for _, action := range actions {
			state, _, err = appstate.Reduce(state, action)
			if err != nil {
				t.Fatalf("reduce replay frame %d action %T: %v", frameIndex, action, err)
			}
		}
	}

	layout, found = activeLayout(state)
	if !found {
		t.Fatal("replayed workspace layout is missing")
	}
	return layout
}

func nestedDockShellState(t *testing.T) appstate.AppState {
	t.Helper()

	state := testShellState(t)
	layout := state.Layouts[0].Clone()
	stack, ok := layout.Root.(appstate.TabStackNode)
	if !ok || len(stack.Panels) < 4 {
		t.Fatalf("default data root = %T with %d panels, want stack with at least 4", layout.Root, len(stack.Panels))
	}
	firstPanels := append([]appstate.PanelInstanceState(nil), stack.Panels[:2]...)
	secondPanels := append([]appstate.PanelInstanceState(nil), stack.Panels[2:]...)
	layout.Root = appstate.SplitNode{
		ID:          "test-split",
		Orientation: appstate.SplitHorizontal,
		Ratio:       0.5,
		First: appstate.TabStackNode{
			ID: "test-left", Active: firstPanels[0].ID, Panels: firstPanels,
		},
		Second: appstate.TabStackNode{
			ID: "test-right", Active: secondPanels[0].ID, Panels: secondPanels,
		},
	}
	if err := appstate.ValidateWorkspaceLayout(layout); err != nil {
		t.Fatal(err)
	}
	state.Layouts[0] = layout
	return state
}
