package nativeui

import (
	"context"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/assets"
	"github.com/rosstheboss94/corthena/internal/frontend/simulator"
)

func TestDrawResearchVerticalSliceUsesPreparedChartAndVirtualRows(t *testing.T) {
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
	clock := appstate.FixedClock{Time: time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)}
	state, _, err := appstate.NewInitialState(clock, appstate.NewSequentialIDSource("research-native"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := simulator.NewDemoCoordinator(simulator.Options{Seed: 11, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	snapshot, err := client.Snapshot(context.Background(), appstate.SnapshotRequest{CorrelationID: "snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = appstate.Reduce(state, appstate.ClientMessageAction{Message: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	state, effects, err := appstate.Reduce(state, appstate.SelectWorkspaceAction{Workspace: appstate.WorkspaceResearch})
	if err != nil {
		t.Fatal(err)
	}
	var query appstate.ResearchQuery
	for _, effect := range effects {
		if research, ok := effect.(appstate.QueryResearchEffect); ok {
			query = research.Query
		}
	}
	if query.Generation == 0 {
		t.Fatalf("Research selection effects = %#v", effects)
	}
	message, err := client.Research(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = appstate.Reduce(state, appstate.ClientMessageAction{Message: message})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := window.DrawShellFrame(state); err != nil {
		t.Fatal(err)
	}
	if countCalls(backend.calls, "drawLine") == 0 || countCalls(backend.calls, "drawRectangle") == 0 || countCalls(backend.calls, "drawText") == 0 {
		t.Fatalf("Research frame did not draw chart/table primitives: %v", backend.calls)
	}
}

func TestResearchResponsiveLayoutAndScaleMatrix(t *testing.T) {
	state := testShellState(t)
	state.ActiveWorkspace = appstate.WorkspaceResearch
	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct{ width, height int32 }{{1280, 720}, {1920, 1080}} {
		for _, scale := range appstate.UIScalePresets() {
			backend := newFakeBackend()
			backend.width, backend.height = size.width, size.height
			window, err := openWithBackend(Config{Width: size.width, Height: size.height, Title: "matrix", Hidden: true}, assetSet, allowGuard{}, backend)
			if err != nil {
				t.Fatalf("open %dx%d %d%%: %v", size.width, size.height, scale, err)
			}
			state.Preferences.UIScale = scale
			if _, err := window.DrawShellFrame(state); err != nil {
				t.Fatalf("draw %dx%d %d%%: %v", size.width, size.height, scale, err)
			}
			if err := window.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	layout, found := activeLayout(state)
	if !found {
		t.Fatal("Research layout is missing")
	}
	wide := (&shellRenderer{scale: 1}).responsiveDockLayout(layout, rectangle{width: 1400, height: 800})
	if _, ok := wide.Root.(appstate.SplitNode); !ok {
		t.Fatalf("wide Research root = %T, want split", wide.Root)
	}
	narrow := (&shellRenderer{scale: 2}).responsiveDockLayout(layout, rectangle{width: 1280, height: 720})
	stack, ok := narrow.Root.(appstate.TabStackNode)
	if !ok || len(stack.Panels) != 6 {
		t.Fatalf("narrow Research root = %T panels %d", narrow.Root, len(stack.Panels))
	}
}

func TestContextBarEmitsTypedDatasetSelection(t *testing.T) {
	state := testShellState(t)
	state.Datasets = []appstate.DatasetSummary{
		{ID: "first", Name: "First", Symbols: []appstate.Symbol{"AAPL"}, Interval: appstate.IntervalDaily, Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "second", Name: "Second", Symbols: []appstate.Symbol{"SPY"}, Interval: appstate.IntervalHourly, Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	state.LinkContext = contextForDataset(state.LinkContext, state.Datasets[0])
	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.mouse = point{x: 30, y: 55}
	backend.leftPressed = true
	window, err := openWithBackend(testConfig(), assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	actions, err := window.DrawShellFrame(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range actions {
		selection, ok := action.(appstate.SetLinkContextAction)
		if ok && selection.Context.DatasetID == "second" && selection.Context.Interval == appstate.IntervalHourly {
			return
		}
	}
	t.Fatalf("dataset context action missing: %#v", actions)
}
