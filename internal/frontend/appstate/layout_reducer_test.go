package appstate

import (
	"reflect"
	"testing"
	"time"
)

func TestCloseAndReopenSingletonPreservesStablePanelID(t *testing.T) {
	t.Parallel()

	clock := FixedClock{Time: fixedTime()}
	ids := NewSequentialIDSource("reopen")
	state, _, err := NewInitialState(clock, ids)
	if err != nil {
		t.Fatal(err)
	}
	layoutIndex, err := state.layoutIndex(WorkspaceData)
	if err != nil {
		t.Fatal(err)
	}
	original, ok := findPanelByTypeForTest(state.Layouts[layoutIndex].Root, PanelCatalogTable)
	if !ok {
		t.Fatal("default catalog panel is missing")
	}

	closed, effects, err := Reduce(state, ClosePanelAction{
		Workspace:       WorkspaceData,
		PanelID:         original.ID,
		PersistEffectID: "close-effect",
		RequestedAt:     clock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 1 {
		t.Fatalf("close effects = %d, want 1", len(effects))
	}
	if _, ok := effects[0].(PersistLayoutsEffect); !ok {
		t.Fatalf("close effect = %T, want PersistLayoutsEffect", effects[0])
	}
	if got := closed.Layouts[layoutIndex].HiddenPanels; len(got) != 1 || got[0].ID != original.ID {
		t.Fatalf("hidden panels = %#v, want original panel", got)
	}

	replacement, err := NewPanelInstance(ids.NewPanelID(), PanelCatalogTable, original.LinkGroup)
	if err != nil {
		t.Fatal(err)
	}
	reopened, _, err := Reduce(closed, OpenPanelAction{
		Workspace:       WorkspaceData,
		Panel:           replacement,
		PersistEffectID: "reopen-effect",
		RequestedAt:     clock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Layouts[layoutIndex].HiddenPanels) != 0 {
		t.Fatalf("hidden panels = %#v, want none", reopened.Layouts[layoutIndex].HiddenPanels)
	}
	visible, ok := findPanelByTypeForTest(reopened.Layouts[layoutIndex].Root, PanelCatalogTable)
	if !ok {
		t.Fatal("reopened catalog panel is missing")
	}
	if visible.ID != original.ID {
		t.Fatalf("reopened ID = %q, want stable ID %q", visible.ID, original.ID)
	}
}

func TestLateLayoutLoadDoesNotOverwriteLocalMutation(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("late"))
	if err != nil {
		t.Fatal(err)
	}
	defaults := CloneWorkspaceLayouts(state.Layouts)
	mutated, _, err := Reduce(state, SetLinkContextAction{Context: LinkContext{
		DatasetID: "local-dataset",
		Interval:  IntervalDaily,
	}})
	if err != nil {
		t.Fatal(err)
	}

	loaded, effects, err := Reduce(mutated, LayoutsLoadedAction{
		EffectID:     "load-effect",
		BaseRevision: 0,
		Revision:     99,
		Layouts:      defaults,
		LoadedAt:     fixedTime(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 0 {
		t.Fatalf("effects = %d, want 0", len(effects))
	}
	if loaded.LayoutRevision != mutated.LayoutRevision {
		t.Fatalf("revision = %d, want %d", loaded.LayoutRevision, mutated.LayoutRevision)
	}
	if !reflect.DeepEqual(loaded.Layouts, mutated.Layouts) {
		t.Fatal("late load overwrote locally mutated layouts")
	}
}

func TestStalePersistenceResultCannotClearNewerFailure(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("stale"))
	if err != nil {
		t.Fatal(err)
	}
	state.Persistence.PendingRevision = 2
	state.Persistence.LastErrorRevision = 2
	state.Persistence.LastError = ErrorSnapshot{Code: ErrorPersistence, Message: "newer failure"}

	next, _, err := Reduce(state, LayoutPersistedAction{
		EffectID: "stale-save",
		Revision: 1,
		SavedAt:  fixedTime().Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if next.Persistence.PendingRevision != 2 {
		t.Fatalf("pending revision = %d, want 2", next.Persistence.PendingRevision)
	}
	if next.Persistence.LastError.Message != "newer failure" {
		t.Fatalf("last error = %#v, want newer failure", next.Persistence.LastError)
	}
}

func TestLinkGroupUpdateCopiesOnlySourcePanelScopes(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("links"))
	if err != nil {
		t.Fatal(err)
	}
	layoutIndex, err := state.layoutIndex(WorkspaceResearch)
	if err != nil {
		t.Fatal(err)
	}
	panel, ok := findPanelByTypeForTest(state.Layouts[layoutIndex].Root, PanelOHLCVChart)
	if !ok {
		t.Fatal("default OHLCV panel is missing")
	}
	incoming := LinkContext{
		DatasetID: "dataset-linked",
		Symbols:   []Symbol{"MSFT"},
		Interval:  IntervalHourly,
		TimeRange: TimeRange{
			Start: fixedTime().Add(-time.Hour),
			End:   fixedTime(),
		},
		RunID: "run-must-not-propagate",
	}

	next, _, err := Reduce(state, UpdateLinkGroupContextAction{
		Workspace:     WorkspaceResearch,
		GroupID:       panel.LinkGroup,
		SourcePanelID: panel.ID,
		Context:       incoming,
	})
	if err != nil {
		t.Fatal(err)
	}
	context := next.Layouts[layoutIndex].LinkGroups[0].Context
	if context.DatasetID != incoming.DatasetID || !reflect.DeepEqual(context.Symbols, incoming.Symbols) {
		t.Fatalf("linked context = %#v, want dataset and symbols from %#v", context, incoming)
	}
	if context.RunID != "" {
		t.Fatalf("run ID propagated through unsupported scope: %q", context.RunID)
	}
	if &context.Symbols[0] == &incoming.Symbols[0] {
		t.Fatal("linked symbols share backing storage with action input")
	}
}

func findPanelByTypeForTest(root DockNode, panelType PanelType) (PanelInstanceState, bool) {
	switch node := root.(type) {
	case TabStackNode:
		for _, panel := range node.Panels {
			if panel.Type == panelType {
				return panel, true
			}
		}
	case SplitNode:
		if panel, ok := findPanelByTypeForTest(node.First, panelType); ok {
			return panel, true
		}
		return findPanelByTypeForTest(node.Second, panelType)
	}
	return PanelInstanceState{}, false
}
