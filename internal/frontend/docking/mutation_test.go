package docking

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestActivateReorderAndMovePanelArePure(t *testing.T) {
	t.Parallel()

	layout := testWorkspaceLayout(t)
	original := layout.Clone()

	activated, err := ActivatePanel(layout, "panel-b")
	if err != nil {
		t.Fatal(err)
	}
	stackA, _ := stackByID(activated.Root, "stack-a")
	if stackA.Active != "panel-b" {
		t.Fatalf("active panel = %q, want panel-b", stackA.Active)
	}

	reordered, err := ReorderPanel(layout, "panel-b", 0)
	if err != nil {
		t.Fatal(err)
	}
	stackA, _ = stackByID(reordered.Root, "stack-a")
	assertPanelIDs(t, stackA.Panels, "panel-b", "panel-a")
	if stackA.Active != "panel-a" {
		t.Fatalf("reorder changed active panel to %q", stackA.Active)
	}

	moved, err := MovePanel(layout, "panel-b", "stack-b", 1)
	if err != nil {
		t.Fatal(err)
	}
	stackA, _ = stackByID(moved.Root, "stack-a")
	stackB, _ := stackByID(moved.Root, "stack-b")
	assertPanelIDs(t, stackA.Panels, "panel-a")
	assertPanelIDs(t, stackB.Panels, "panel-c", "panel-b")
	if stackB.Active != "panel-b" {
		t.Fatalf("target active panel = %q, want panel-b", stackB.Active)
	}

	closed, err := ClosePanel(layout, "panel-b")
	if err != nil {
		t.Fatal(err)
	}
	collapsed, err := MovePanel(closed, "panel-a", "stack-b", 1)
	if err != nil {
		t.Fatal(err)
	}
	rootStack, ok := collapsed.Root.(appstate.TabStackNode)
	if !ok || rootStack.ID != "stack-b" {
		t.Fatalf("collapsed root = %#v, want stack-b", collapsed.Root)
	}
	assertPanelIDs(t, rootStack.Panels, "panel-c", "panel-a")

	if !reflect.DeepEqual(layout, original) {
		t.Fatalf("original layout mutated\ngot:  %#v\nwant: %#v", layout, original)
	}
}

func TestDockPanelDirectionalTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		position    DockPosition
		orientation appstate.SplitOrientation
		wantRatio   float64
		newFirst    bool
	}{
		{position: DockLeft, orientation: appstate.SplitHorizontal, wantRatio: 0.2, newFirst: true},
		{position: DockRight, orientation: appstate.SplitHorizontal, wantRatio: 0.8, newFirst: false},
		{position: DockTop, orientation: appstate.SplitVertical, wantRatio: 0.2, newFirst: true},
		{position: DockBottom, orientation: appstate.SplitVertical, wantRatio: 0.8, newFirst: false},
	}
	for _, test := range tests {
		test := test
		t.Run(string(test.position), func(t *testing.T) {
			t.Parallel()
			layout := testWorkspaceLayout(t)
			next, err := DockPanel(layout, "panel-b", "stack-b", test.position, "stack-new", "split-new", 0.2)
			if err != nil {
				t.Fatal(err)
			}
			split, found := splitByID(next.Root, "split-new")
			if !found {
				t.Fatal("new split not found")
			}
			if split.Orientation != test.orientation {
				t.Fatalf("orientation = %q, want %q", split.Orientation, test.orientation)
			}
			assertClose(t, split.Ratio, test.wantRatio)
			firstID := nodeID(split.First)
			secondID := nodeID(split.Second)
			if test.newFirst {
				if firstID != "stack-new" || secondID != "stack-b" {
					t.Fatalf("children = (%q, %q), want (stack-new, stack-b)", firstID, secondID)
				}
			} else if firstID != "stack-b" || secondID != "stack-new" {
				t.Fatalf("children = (%q, %q), want (stack-b, stack-new)", firstID, secondID)
			}
			newStack, found := stackByID(next.Root, "stack-new")
			if !found {
				t.Fatal("new stack not found")
			}
			assertPanelIDs(t, newStack.Panels, "panel-b")
			if newStack.Active != "panel-b" {
				t.Fatalf("new stack active = %q, want panel-b", newStack.Active)
			}
		})
	}
}

func TestDockPanelCenterAppendsAndActivates(t *testing.T) {
	t.Parallel()

	layout := testWorkspaceLayout(t)
	next, err := DockPanel(layout, "panel-b", "stack-b", DockCenter, "", "", math.NaN())
	if err != nil {
		t.Fatal(err)
	}
	stack, found := stackByID(next.Root, "stack-b")
	if !found {
		t.Fatal("target stack not found")
	}
	assertPanelIDs(t, stack.Panels, "panel-c", "panel-b")
	if stack.Active != "panel-b" {
		t.Fatalf("active panel = %q, want panel-b", stack.Active)
	}
}

func TestMoveAndSplitWithinCurrentStack(t *testing.T) {
	t.Parallel()

	layout := testWorkspaceLayout(t)
	moved, err := MovePanel(layout, "panel-b", "stack-a", 0)
	if err != nil {
		t.Fatal(err)
	}
	stack, _ := stackByID(moved.Root, "stack-a")
	assertPanelIDs(t, stack.Panels, "panel-b", "panel-a")
	if stack.Active != "panel-b" {
		t.Fatalf("same-stack move active = %q, want panel-b", stack.Active)
	}

	splitLayout, err := DockPanel(layout, "panel-b", "stack-a", DockRight, "stack-detached", "split-detached", 0.3)
	if err != nil {
		t.Fatal(err)
	}
	split, found := splitByID(splitLayout.Root, "split-detached")
	if !found {
		t.Fatal("same-stack split not found")
	}
	if nodeID(split.First) != "stack-a" || nodeID(split.Second) != "stack-detached" {
		t.Fatalf("same-stack split children = (%q, %q)", nodeID(split.First), nodeID(split.Second))
	}
	originalStack, _ := stackByID(splitLayout.Root, "stack-a")
	newStack, _ := stackByID(splitLayout.Root, "stack-detached")
	assertPanelIDs(t, originalStack.Panels, "panel-a")
	assertPanelIDs(t, newStack.Panels, "panel-b")

	if _, err := DockPanel(singlePanelLayout(t), "panel-a", "stack-a", DockLeft, "stack-new", "split-new", 0.3); !errors.Is(err, ErrInvalidMutation) {
		t.Fatalf("single-panel self split error = %v, want ErrInvalidMutation", err)
	}
}

func TestCloseCollapseReopenAndFinalRoot(t *testing.T) {
	t.Parallel()

	layout := testWorkspaceLayout(t)
	maximized, err := MaximizePanel(layout, "panel-c")
	if err != nil {
		t.Fatal(err)
	}
	closed, err := ClosePanel(maximized, "panel-c")
	if err != nil {
		t.Fatal(err)
	}
	rootStack, ok := closed.Root.(appstate.TabStackNode)
	if !ok || rootStack.ID != "stack-a" {
		t.Fatalf("root after close = %#v, want stack-a", closed.Root)
	}
	if closed.Maximized != "" {
		t.Fatalf("maximized = %q after closing that panel", closed.Maximized)
	}
	assertPanelIDs(t, closed.HiddenPanels, "panel-c")

	reopened, err := ReopenPanel(closed, "panel-c", "stack-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	rootStack, ok = reopened.Root.(appstate.TabStackNode)
	if !ok {
		t.Fatalf("reopened root = %T, want TabStackNode", reopened.Root)
	}
	assertPanelIDs(t, rootStack.Panels, "panel-a", "panel-c", "panel-b")
	if rootStack.Active != "panel-c" {
		t.Fatalf("reopened active = %q, want panel-c", rootStack.Active)
	}
	if len(reopened.HiddenPanels) != 0 {
		t.Fatalf("hidden panels = %d, want 0", len(reopened.HiddenPanels))
	}

	single := singlePanelLayout(t)
	lastClosed, err := ClosePanel(single, "panel-a")
	if err != nil {
		t.Fatal(err)
	}
	emptyRoot, ok := lastClosed.Root.(appstate.TabStackNode)
	if !ok || emptyRoot.ID != "stack-a" || len(emptyRoot.Panels) != 0 || emptyRoot.Active != "" {
		t.Fatalf("final close root = %#v, want stable empty stack-a", lastClosed.Root)
	}
	lastReopened, err := ReopenPanel(lastClosed, "panel-a", "stack-a", 0)
	if err != nil {
		t.Fatal(err)
	}
	restoredRoot := lastReopened.Root.(appstate.TabStackNode)
	assertPanelIDs(t, restoredRoot.Panels, "panel-a")
}

func TestMaximizeRestoreAndResizeSplit(t *testing.T) {
	t.Parallel()

	layout := testWorkspaceLayout(t)
	maximized, err := MaximizePanel(layout, "panel-b")
	if err != nil {
		t.Fatal(err)
	}
	if maximized.Maximized != "panel-b" {
		t.Fatalf("maximized = %q, want panel-b", maximized.Maximized)
	}
	restored, err := RestorePanel(maximized)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Maximized != "" {
		t.Fatalf("restored maximized = %q, want empty", restored.Maximized)
	}

	nanRatio, err := ResizeSplit(layout, "split-root", math.NaN())
	if err != nil {
		t.Fatal(err)
	}
	split, _ := splitByID(nanRatio.Root, "split-root")
	assertClose(t, split.Ratio, 0.5)
	maxRatio, err := ResizeSplit(layout, "split-root", math.Inf(1))
	if err != nil {
		t.Fatal(err)
	}
	split, _ = splitByID(maxRatio.Root, "split-root")
	assertClose(t, split.Ratio, MaximumSplitRatio)
}

func TestAddAndDockPanelSafeguards(t *testing.T) {
	t.Parallel()

	layout := testWorkspaceLayout(t)
	singleton := testPanel(t, "panel-new-singleton", appstate.PanelCoverageTimeline)
	if _, err := AddPanel(layout, singleton, "stack-a", 2); !errors.Is(err, ErrSingleton) {
		t.Fatalf("AddPanel singleton error = %v, want ErrSingleton", err)
	}
	duplicateID := testPanel(t, "panel-a", appstate.PanelDatasetInspector)
	if _, err := AddPanel(layout, duplicateID, "stack-a", 2); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("AddPanel duplicate error = %v, want ErrDuplicateID", err)
	}
	valid := testPanel(t, "panel-new", appstate.PanelDatasetInspector)
	added, err := AddPanel(layout, valid, "stack-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	stack, _ := stackByID(added.Root, "stack-a")
	assertPanelIDs(t, stack.Panels, "panel-a", "panel-new", "panel-b")

	if _, err := DockPanel(layout, "panel-b", "stack-b", DockLeft, "stack-a", "split-new", 0.2); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("DockPanel duplicate stack error = %v, want ErrDuplicateID", err)
	}
	if _, err := DockPanel(layout, "panel-b", "stack-b", DockLeft, "stack-new", "stack-new", 0.2); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("DockPanel equal new IDs error = %v, want ErrDuplicateID", err)
	}
}

func testWorkspaceLayout(t *testing.T) appstate.WorkspaceLayout {
	t.Helper()
	panelA := testPanel(t, "panel-a", appstate.PanelDatasetInspector)
	panelB := testPanel(t, "panel-b", appstate.PanelCoverageTimeline)
	panelC := testPanel(t, "panel-c", appstate.PanelImportLogs)
	layout := appstate.WorkspaceLayout{
		SchemaVersion: appstate.LayoutSchemaVersion,
		Workspace:     appstate.WorkspaceData,
		Root: appstate.SplitNode{
			ID:          "split-root",
			Orientation: appstate.SplitHorizontal,
			Ratio:       0.5,
			First: appstate.TabStackNode{
				ID: "stack-a", Active: panelA.ID, Panels: []appstate.PanelInstanceState{panelA, panelB},
			},
			Second: appstate.TabStackNode{
				ID: "stack-b", Active: panelC.ID, Panels: []appstate.PanelInstanceState{panelC},
			},
		},
		LinkGroups: []appstate.LinkGroup{{
			ID: "link-a", Name: "Group A", Color: appstate.LinkGroupCyan,
		}},
	}
	if err := ValidateLayout(layout); err != nil {
		t.Fatalf("test layout is invalid: %v", err)
	}
	return layout
}

func singlePanelLayout(t *testing.T) appstate.WorkspaceLayout {
	t.Helper()
	panel := testPanel(t, "panel-a", appstate.PanelDatasetInspector)
	return appstate.WorkspaceLayout{
		SchemaVersion: appstate.LayoutSchemaVersion,
		Workspace:     appstate.WorkspaceData,
		Root:          appstate.TabStackNode{ID: "stack-a", Active: panel.ID, Panels: []appstate.PanelInstanceState{panel}},
		LinkGroups: []appstate.LinkGroup{{
			ID: "link-a", Name: "Group A", Color: appstate.LinkGroupCyan,
		}},
	}
}

func stackByID(root appstate.DockNode, id appstate.DockNodeID) (appstate.TabStackNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return node, node.ID == id
	case appstate.SplitNode:
		if stack, found := stackByID(node.First, id); found {
			return stack, true
		}
		return stackByID(node.Second, id)
	default:
		return appstate.TabStackNode{}, false
	}
}

func splitByID(root appstate.DockNode, id appstate.DockNodeID) (appstate.SplitNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return appstate.SplitNode{}, false
	case appstate.SplitNode:
		if node.ID == id {
			return node, true
		}
		if split, found := splitByID(node.First, id); found {
			return split, true
		}
		return splitByID(node.Second, id)
	default:
		return appstate.SplitNode{}, false
	}
}

func nodeID(root appstate.DockNode) appstate.DockNodeID {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return node.ID
	case appstate.SplitNode:
		return node.ID
	default:
		return ""
	}
}

func assertPanelIDs(t *testing.T, panels []appstate.PanelInstanceState, wants ...appstate.PanelID) {
	t.Helper()
	if len(panels) != len(wants) {
		t.Fatalf("panel count = %d, want %d", len(panels), len(wants))
	}
	for index, want := range wants {
		if panels[index].ID != want {
			t.Fatalf("panel %d = %q, want %q", index, panels[index].ID, want)
		}
	}
}
