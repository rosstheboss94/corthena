package effects_test

import (
	"context"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/effects"
	"github.com/rosstheboss94/corthena/internal/frontend/simulator"
)

func TestRuntimeLoadsSnapshot(t *testing.T) {
	t.Parallel()

	runtime, cleanup := startRuntime(t, simulator.FailureProfile{})
	defer cleanup()
	ok := runtime.Enqueue(appstate.LoadSnapshotEffect{
		ID:            "effect-load",
		CorrelationID: "corr-load",
		RequestedAt:   fixedTime(),
	})
	if !ok {
		t.Fatal("Enqueue returned false")
	}

	action := waitAction(t, runtime.Actions())
	clientAction, ok := action.(appstate.ClientMessageAction)
	if !ok {
		t.Fatalf("action = %T, want ClientMessageAction", action)
	}
	snapshot, ok := clientAction.Message.(appstate.SnapshotMessage)
	if !ok {
		t.Fatalf("message = %T, want SnapshotMessage", clientAction.Message)
	}
	if snapshot.Event.CorrelationID != "corr-load" {
		t.Fatalf("correlation ID = %q, want corr-load", snapshot.Event.CorrelationID)
	}
}

func TestRuntimePersistsLayout(t *testing.T) {
	t.Parallel()

	client, err := simulator.NewDemoCoordinator(simulator.Options{
		Seed:  3,
		Clock: appstate.FixedClock{Time: fixedTime()},
	})
	if err != nil {
		t.Fatal(err)
	}
	store := effects.NewMemoryLayoutStore()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runtime, err := effects.Start(ctx, client, store, effects.Config{
		Clock: appstate.FixedClock{Time: fixedTime()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	layout, err := appstate.DefaultWorkspaceLayout(appstate.WorkspaceResearch, appstate.NewSequentialIDSource("layout"))
	if err != nil {
		t.Fatal(err)
	}
	ok := runtime.Enqueue(appstate.PersistLayoutEffect{
		ID:        "effect-layout",
		Workspace: layout.Workspace,
		Layout:    layout,
		Requested: fixedTime(),
	})
	if !ok {
		t.Fatal("Enqueue returned false")
	}

	action := waitAction(t, runtime.Actions())
	persisted, ok := action.(appstate.LayoutPersistedAction)
	if !ok {
		t.Fatalf("action = %T, want LayoutPersistedAction", action)
	}
	if persisted.EffectID != "effect-layout" {
		t.Fatalf("effect ID = %q, want effect-layout", persisted.EffectID)
	}
	if got := len(store.Layouts()); got != 1 {
		t.Fatalf("stored layouts = %d, want 1", got)
	}
}

func TestRuntimeReportsTypedSnapshotFailure(t *testing.T) {
	t.Parallel()

	runtime, cleanup := startRuntime(t, simulator.FailureProfile{Snapshot: true})
	defer cleanup()
	ok := runtime.Enqueue(appstate.LoadSnapshotEffect{
		ID:            "effect-fail",
		CorrelationID: "corr-fail",
		RequestedAt:   fixedTime(),
	})
	if !ok {
		t.Fatal("Enqueue returned false")
	}

	action := waitAction(t, runtime.Actions())
	failed, ok := action.(appstate.EffectFailedAction)
	if !ok {
		t.Fatalf("action = %T, want EffectFailedAction", action)
	}
	if failed.Error.CorrelationID != "corr-fail" {
		t.Fatalf("correlation ID = %q, want corr-fail", failed.Error.CorrelationID)
	}
	if failed.Error.Code != appstate.ErrorClientUnavailable {
		t.Fatalf("error code = %q, want client_unavailable", failed.Error.Code)
	}
}

func TestRuntimeSupersedesResearchGeneration(t *testing.T) {
	client, err := simulator.NewDemoCoordinator(simulator.Options{
		Seed: 19, Clock: appstate.FixedClock{Time: fixedTime()},
		Delays: simulator.DelayProfile{Research: 80 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runtime, err := effects.Start(ctx, client, effects.NewMemoryLayoutStore(), effects.Config{
		Clock: appstate.FixedClock{Time: fixedTime()}, MaxConcurrent: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	query := runtimeResearchQuery()
	if !runtime.Enqueue(appstate.QueryResearchEffect{ID: "research-1", Query: query}) {
		t.Fatal("first Research enqueue failed")
	}
	query.Generation = 2
	query.CorrelationID = "research-2"
	if !runtime.Enqueue(appstate.QueryResearchEffect{ID: "research-2", Query: query}) {
		t.Fatal("second Research enqueue failed")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case action := <-runtime.Actions():
			clientAction, ok := action.(appstate.ClientMessageAction)
			if !ok {
				continue
			}
			message, ok := clientAction.Message.(appstate.ResearchResponseMessage)
			if !ok {
				continue
			}
			if message.Snapshot.Query.Generation != 2 {
				t.Fatalf("published stale Research generation %d", message.Snapshot.Query.Generation)
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for current Research generation")
		}
	}
}

func TestRuntimeExplicitResearchCancellationPublishesTypedAction(t *testing.T) {
	client, err := simulator.NewDemoCoordinator(simulator.Options{
		Seed: 29, Clock: appstate.FixedClock{Time: fixedTime()},
		Delays: simulator.DelayProfile{Research: time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runtime, err := effects.Start(ctx, client, effects.NewMemoryLayoutStore(), effects.Config{
		Clock: appstate.FixedClock{Time: fixedTime()}, MaxConcurrent: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	query := runtimeResearchQuery()
	if !runtime.Enqueue(appstate.QueryResearchEffect{ID: "research-cancel", Query: query}) ||
		!runtime.Enqueue(appstate.CancelResearchEffect{ID: "cancel-research", GroupID: query.GroupID, Generation: query.Generation}) {
		t.Fatal("Research cancellation effects were not enqueued")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case action := <-runtime.Actions():
			cancelled, ok := action.(appstate.ResearchQueryCancelledAction)
			if ok && cancelled.GroupID == query.GroupID && cancelled.Generation == query.Generation {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for typed Research cancellation")
		}
	}
}

func TestRuntimeReportsResearchQueueSaturation(t *testing.T) {
	client, err := simulator.NewDemoCoordinator(simulator.Options{Seed: 37, Clock: appstate.FixedClock{Time: fixedTime()}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runtime, err := effects.Start(ctx, client, effects.NewMemoryLayoutStore(), effects.Config{
		Clock: appstate.FixedClock{Time: fixedTime()}, MaxConcurrent: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	blocking := runtimeResearchQuery()
	blocking.Scenario = appstate.ResearchScenarioLoading
	if !runtime.Enqueue(appstate.QueryResearchEffect{ID: "research-loading", Query: blocking}) {
		t.Fatal("loading Research enqueue failed")
	}
	queued := runtimeResearchQuery()
	queued.GroupID = "link-compare-research"
	queued.CorrelationID = "research-queued"
	if !runtime.Enqueue(appstate.QueryResearchEffect{ID: "research-queued", Query: queued}) {
		t.Fatal("queued Research enqueue failed")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case action := <-runtime.Actions():
			failed, ok := action.(appstate.ResearchQueryFailedAction)
			if ok && failed.GroupID == queued.GroupID {
				if failed.Error.Code != appstate.ErrorEffectBusy || !failed.Error.Retryable {
					t.Fatalf("queue saturation error = %+v", failed.Error)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for Research queue saturation")
		}
	}
}

func runtimeResearchQuery() appstate.ResearchQuery {
	return appstate.ResearchQuery{
		CorrelationID: "research-1", GroupID: "link-default-research", Generation: 1,
		DatasetID: "dataset-us-equities", Symbols: []appstate.Symbol{"AAPL"}, Interval: appstate.IntervalDaily,
		TimeRange:        appstate.TimeRange{Start: fixedTime().AddDate(-1, 0, 0), End: fixedTime()},
		SelectedFeatures: []appstate.FeatureName{"ret_5"}, Target: appstate.TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
		Resolution: 640, PageSize: 80, Sort: appstate.ResearchSortTimeAscending, Scenario: appstate.ResearchScenarioNormal,
	}
}

func startRuntime(
	t *testing.T,
	failures simulator.FailureProfile,
) (*effects.Runtime, func()) {
	t.Helper()

	client, err := simulator.NewDemoCoordinator(simulator.Options{
		Seed:     3,
		Clock:    appstate.FixedClock{Time: fixedTime()},
		Failures: failures,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	runtime, err := effects.Start(
		ctx,
		client,
		effects.NewMemoryLayoutStore(),
		effects.Config{Clock: appstate.FixedClock{Time: fixedTime()}},
	)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cleanup := func() {
		if err := runtime.Close(); err != nil {
			t.Fatal(err)
		}
		cancel()
	}
	return runtime, cleanup
}

func waitAction(t *testing.T, actions <-chan appstate.UIAction) appstate.UIAction {
	t.Helper()

	select {
	case action, ok := <-actions:
		if !ok {
			t.Fatal("action channel closed")
		}
		return action
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for action")
		return nil
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)
}
