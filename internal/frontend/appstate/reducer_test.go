package appstate

import (
	"reflect"
	"testing"
	"time"
)

func TestInitialStateIsDeterministic(t *testing.T) {
	t.Parallel()

	clock := FixedClock{Time: fixedTime()}
	first, firstEffects, err := NewInitialState(clock, NewSequentialIDSource("test"))
	if err != nil {
		t.Fatal(err)
	}
	second, secondEffects, err := NewInitialState(clock, NewSequentialIDSource("test"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("initial states differ\nfirst: %#v\nsecond: %#v", first, second)
	}
	if !reflect.DeepEqual(firstEffects, secondEffects) {
		t.Fatalf("initial effects differ\nfirst: %#v\nsecond: %#v", firstEffects, secondEffects)
	}
}

func TestSameActionSequenceProducesIdenticalState(t *testing.T) {
	t.Parallel()

	firstState, firstEffects := reduceSequence(t)
	secondState, secondEffects := reduceSequence(t)
	if !reflect.DeepEqual(firstState, secondState) {
		t.Fatalf("states differ\nfirst: %#v\nsecond: %#v", firstState, secondState)
	}
	if !reflect.DeepEqual(firstEffects, secondEffects) {
		t.Fatalf("effects differ\nfirst: %#v\nsecond: %#v", firstEffects, secondEffects)
	}
}

func TestSnapshotMessageIsCopiedIntoState(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("copy"))
	if err != nil {
		t.Fatal(err)
	}
	message := testSnapshot()
	next, _, err := Reduce(state, ClientMessageAction{Message: message})
	if err != nil {
		t.Fatal(err)
	}

	message.Datasets[0].Symbols[0] = "BROKEN"
	message.Models[0].FeatureNames[0] = "broken_feature"
	if got := next.Datasets[0].Symbols[0]; got != "AAPL" {
		t.Fatalf("dataset symbol mutated through message: %s", got)
	}
	if got := next.Models[0].FeatureNames[0]; got != "ret_5" {
		t.Fatalf("model feature mutated through message: %s", got)
	}
}

func TestOpeningExistingSingletonFocusesWithoutDuplicate(t *testing.T) {
	t.Parallel()

	clock := FixedClock{Time: fixedTime()}
	ids := NewSequentialIDSource("singleton")
	state, _, err := NewInitialState(clock, ids)
	if err != nil {
		t.Fatal(err)
	}
	layoutIndex, err := state.layoutIndex(WorkspaceData)
	if err != nil {
		t.Fatal(err)
	}
	before := panelCount(state.Layouts[layoutIndex].Root)
	panel, err := NewPanelInstance(ids.NewPanelID(), PanelCatalogTable, "link-default-data")
	if err != nil {
		t.Fatal(err)
	}

	next, effects, err := Reduce(state, OpenPanelAction{
		Workspace:       WorkspaceData,
		Panel:           panel,
		PersistEffectID: ids.NewEffectID(),
		RequestedAt:     clock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	layoutIndex, err = next.layoutIndex(WorkspaceData)
	if err != nil {
		t.Fatal(err)
	}
	after := panelCount(next.Layouts[layoutIndex].Root)
	if after != before {
		t.Fatalf("panel count = %d, want %d", after, before)
	}
	if len(effects) != 1 {
		t.Fatalf("effects = %d, want 1", len(effects))
	}
}

func TestCommandPaletteActionIsDeterministicOverlayState(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("palette"))
	if err != nil {
		t.Fatal(err)
	}
	open, effects, err := Reduce(state, SetCommandPaletteAction{Open: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 0 {
		t.Fatalf("effects = %d, want 0", len(effects))
	}
	if !open.Overlays.CommandPaletteOpen {
		t.Fatal("command palette is closed after open action")
	}
	closed, effects, err := Reduce(open, SetCommandPaletteAction{Open: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 0 {
		t.Fatalf("effects = %d, want 0", len(effects))
	}
	if closed.Overlays.CommandPaletteOpen {
		t.Fatal("command palette is open after close action")
	}
	if state.Overlays.CommandPaletteOpen {
		t.Fatal("original state was mutated")
	}
}

func TestDismissToastActionRemovesOnlyMatchingToast(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("toast"))
	if err != nil {
		t.Fatal(err)
	}
	state.Overlays.Toasts = []Toast{
		{ID: "toast-a", Kind: ToastInfo, Message: "first", CreatedAt: fixedTime()},
		{ID: "toast-b", Kind: ToastWarning, Message: "second", CreatedAt: fixedTime()},
	}
	next, effects, err := Reduce(state, DismissToastAction{ToastID: "toast-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 0 {
		t.Fatalf("effects = %d, want 0", len(effects))
	}
	if got := len(next.Overlays.Toasts); got != 1 {
		t.Fatalf("toasts = %d, want 1", got)
	}
	if got := next.Overlays.Toasts[0].ID; got != "toast-b" {
		t.Fatalf("remaining toast = %q, want toast-b", got)
	}
	if got := len(state.Overlays.Toasts); got != 2 {
		t.Fatalf("original toast count = %d, want 2", got)
	}
}

func reduceSequence(t *testing.T) (AppState, []UIEffect) {
	t.Helper()

	clock := FixedClock{Time: fixedTime()}
	ids := NewSequentialIDSource("seq")
	state, startupEffects, err := NewInitialState(clock, ids)
	if err != nil {
		t.Fatal(err)
	}
	collected := cloneEffects(startupEffects)
	actions := []UIAction{
		ClientMessageAction{Message: testSnapshot()},
		SelectWorkspaceAction{Workspace: WorkspaceResearch},
		SetLinkContextAction{Context: LinkContext{
			DatasetID: "dataset-test",
			Symbols:   []Symbol{"AAPL", "MSFT"},
			Interval:  IntervalDaily,
			TimeRange: TimeRange{
				Start: fixedTime().AddDate(0, -1, 0),
				End:   fixedTime(),
			},
			RunID:   "run-test",
			ModelID: "model-test",
		}},
	}
	for _, action := range actions {
		next, effects, err := Reduce(state, action)
		if err != nil {
			t.Fatal(err)
		}
		state = next
		collected = append(collected, effects...)
	}
	panel, err := NewPanelInstance(ids.NewPanelID(), PanelDistributions, "link-default-research")
	if err != nil {
		t.Fatal(err)
	}
	next, effects, err := Reduce(state, OpenPanelAction{
		Workspace:       WorkspaceResearch,
		Panel:           panel,
		PersistEffectID: ids.NewEffectID(),
		RequestedAt:     clock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	state = next
	collected = append(collected, effects...)
	return state, collected
}

func testSnapshot() SnapshotMessage {
	now := fixedTime()
	return SnapshotMessage{
		Event: EventEnvelope{
			ID:            "evt-test-snapshot",
			Type:          "snapshot",
			SchemaVersion: 1,
			Timestamp:     now,
		},
		Connection: ConnectionSnapshot{
			State:     ConnectionConnected,
			UpdatedAt: now,
			Detail:    "test",
		},
		Components: []ComponentSnapshot{
			{
				Component: ComponentCoordinator,
				State:     ComponentHealthy,
				Detail:    "test",
				UpdatedAt: now,
			},
		},
		Cache: CacheSnapshot{
			Generation: 1,
			BytesUsed:  128,
			UpdatedAt:  now,
		},
		Datasets: []DatasetSummary{
			{
				ID:          "dataset-test",
				Name:        "Test dataset",
				Status:      DatasetReady,
				Symbols:     []Symbol{"AAPL", "MSFT"},
				Interval:    IntervalDaily,
				Rows:        100,
				Start:       now.AddDate(-1, 0, 0),
				End:         now,
				Revision:    3,
				Fingerprint: "fingerprint",
			},
		},
		Jobs: []JobSummary{
			{
				ID:             "job-test",
				ExperimentID:   "experiment-test",
				RunID:          "run-test",
				State:          JobRunning,
				ProgressPermil: 500,
				Stage:          "fold 1",
				CPUSlots:       2,
				UpdatedAt:      now,
			},
		},
		Results: []RunResultSummary{
			{
				ID:           "run-test",
				ExperimentID: "experiment-test",
				JobID:        "job-test",
				DatasetID:    "dataset-test",
				ModelID:      "model-test",
				Metrics:      []MetricSummary{{Name: "test_ic", Value: 0.04}},
				CompletedAt:  now,
				Immutable:    true,
			},
		},
		Models: []ModelSummary{
			{
				ID:           "model-test",
				RunID:        "run-test",
				Kind:         ModelRandomForest,
				Alias:        "candidate",
				FeatureNames: []FeatureName{"ret_5", "volatility_20"},
				CreatedAt:    now,
				Immutable:    true,
			},
		},
	}
}

func panelCount(root DockNode) int {
	switch node := root.(type) {
	case TabStackNode:
		return len(node.Panels)
	case SplitNode:
		return panelCount(node.First) + panelCount(node.Second)
	default:
		return 0
	}
}

func cloneEffects(input []UIEffect) []UIEffect {
	if len(input) == 0 {
		return nil
	}
	output := make([]UIEffect, len(input))
	copy(output, input)
	return output
}

func fixedTime() time.Time {
	return time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)
}
