package nativeui

import (
	"context"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/assets"
	"github.com/rosstheboss94/corthena/internal/frontend/simulator"
)

func TestDrawDataAndExperimentsPanelsUseTypedPreparedState(t *testing.T) {
	clock := appstate.FixedClock{Time: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}
	client, err := simulator.NewDemoCoordinator(simulator.Options{Seed: 101, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	state, _, err := appstate.NewInitialState(clock, appstate.NewSequentialIDSource("phase7-native"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.Snapshot(context.Background(), appstate.SnapshotRequest{CorrelationID: "snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	state, effects, err := appstate.Reduce(state, appstate.ClientMessageAction{Message: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	for _, effect := range effects {
		switch effect := effect.(type) {
		case appstate.QueryDataWorkspaceEffect:
			message, queryErr := client.DataWorkspace(context.Background(), effect.Query)
			if queryErr != nil {
				t.Fatal(queryErr)
			}
			state, _, err = appstate.Reduce(state, appstate.ClientMessageAction{Message: message})
		case appstate.EvaluateExperimentEffect:
			message, queryErr := client.EvaluateExperiment(context.Background(), effect.Request)
			if queryErr != nil {
				t.Fatal(queryErr)
			}
			state, _, err = appstate.Reduce(state, appstate.ClientMessageAction{Message: message})
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	drawPhase7State(t, state, appstate.WorkspaceData)
	state, effects, err = appstate.Reduce(state, appstate.SelectWorkspaceAction{Workspace: appstate.WorkspaceExperiments})
	if err != nil {
		t.Fatal(err)
	}
	for _, effect := range effects {
		query, ok := effect.(appstate.QueryExperimentsEffect)
		if !ok {
			continue
		}
		message, queryErr := client.Experiments(context.Background(), query.Query)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		state, _, err = appstate.Reduce(state, appstate.ClientMessageAction{Message: message})
		if err != nil {
			t.Fatal(err)
		}
	}
	drawPhase7State(t, state, appstate.WorkspaceExperiments)
}

func TestDataAndExperimentsScaleMatrix(t *testing.T) {
	state := testShellState(t)
	state.Datasets = []appstate.DatasetSummary{{
		ID: "dataset-matrix", Name: "Matrix dataset", Status: appstate.DatasetReady,
		Symbols: []appstate.Symbol{"AAPL"}, Interval: appstate.IntervalDaily, Rows: 1000,
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Revision: 1, Fingerprint: "matrix", Adjustment: "split_dividend_adjusted",
	}}
	state.Data.State = appstate.WorkspaceReady
	state.Data.Snapshot.Catalog = state.Datasets
	state.Data.SelectedDatasetID = state.Datasets[0].ID
	state.Experiments.State = appstate.WorkspaceReady
	state.Experiments.Draft = appstate.ExperimentDraft{Revision: 1, Name: "matrix"}
	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, workspace := range []appstate.Workspace{appstate.WorkspaceData, appstate.WorkspaceExperiments} {
		state.ActiveWorkspace = workspace
		for _, size := range []struct{ width, height int32 }{{1280, 720}, {1920, 1080}} {
			for _, scale := range appstate.UIScalePresets() {
				backend := newFakeBackend()
				backend.width, backend.height = size.width, size.height
				window, openErr := openWithBackend(Config{Width: size.width, Height: size.height, Title: "phase7-matrix", Hidden: true}, assetSet, allowGuard{}, backend)
				if openErr != nil {
					t.Fatal(openErr)
				}
				state.Preferences.UIScale = scale
				if _, drawErr := window.DrawShellFrame(state); drawErr != nil {
					t.Fatalf("draw %s %dx%d %d%%: %v", workspace, size.width, size.height, scale, drawErr)
				}
				if closeErr := window.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
			}
		}
	}
}

func drawPhase7State(t *testing.T, state appstate.AppState, workspace appstate.Workspace) {
	t.Helper()
	state.ActiveWorkspace = workspace
	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	backend.width, backend.height = 1920, 1080
	window, err := openWithBackend(Config{Width: 1920, Height: 1080, Title: "phase7", Hidden: true}, assetSet, allowGuard{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	if _, err := window.DrawShellFrame(state); err != nil {
		t.Fatal(err)
	}
	if countCalls(backend.calls, "drawText") == 0 || countCalls(backend.calls, "drawRectangle") == 0 {
		t.Fatalf("%s did not draw prepared panel primitives", workspace)
	}
}
