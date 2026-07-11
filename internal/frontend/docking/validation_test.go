package docking

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestValidateLayoutRejectsMalformedState(t *testing.T) {
	t.Parallel()
	singletonDuplicate := testPanel(t, "panel-new", appstate.PanelCoverageTimeline)

	tests := []struct {
		name   string
		mutate func(appstate.WorkspaceLayout) appstate.WorkspaceLayout
		cause  error
	}{
		{name: "schema", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.SchemaVersion++
			return layout
		}},
		{name: "workspace", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Workspace = "unknown"
			return layout
		}},
		{name: "nil root", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = nil
			return layout
		}},
		{name: "no link groups", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.LinkGroups = nil
			return layout
		}},
		{name: "empty link group ID", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.LinkGroups[0].ID = ""
			return layout
		}},
		{name: "empty link group name", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.LinkGroups[0].Name = ""
			return layout
		}},
		{name: "invalid link group color", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.LinkGroups[0].Color = "invalid"
			return layout
		}},
		{name: "duplicate link group", cause: ErrDuplicateID, mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.LinkGroups = append(layout.LinkGroups, layout.LinkGroups[0].Clone())
			return layout
		}},
		{name: "reversed link range", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.LinkGroups[0].Context.TimeRange.Start = time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
			layout.LinkGroups[0].Context.TimeRange.End = time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
			return layout
		}},
		{name: "duplicate node ID", cause: ErrDuplicateID, mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			second := root.Second.(appstate.TabStackNode)
			second.ID = "stack-a"
			root.Second = second
			layout.Root = root
			return layout
		}},
		{name: "empty node ID", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			root.ID = ""
			layout.Root = root
			return layout
		}},
		{name: "orientation", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			root.Orientation = "diagonal"
			layout.Root = root
			return layout
		}},
		{name: "NaN ratio", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			root.Ratio = math.NaN()
			layout.Root = root
			return layout
		}},
		{name: "low ratio", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			root.Ratio = 0.01
			layout.Root = root
			return layout
		}},
		{name: "high ratio", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			root.Ratio = 0.99
			layout.Root = root
			return layout
		}},
		{name: "nil child", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			root.Second = nil
			layout.Root = root
			return layout
		}},
		{name: "empty non-root stack", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			root := layout.Root.(appstate.SplitNode)
			root.Second = appstate.TabStackNode{ID: "stack-b"}
			layout.Root = root
			return layout
		}},
		{name: "active panel missing", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Active = "missing"
				return stack
			})
			return layout
		}},
		{name: "duplicate panel ID", cause: ErrDuplicateID, mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[1].ID = stack.Panels[0].ID
				return stack
			})
			return layout
		}},
		{name: "empty panel ID", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[0].ID = ""
				return stack
			})
			return layout
		}},
		{name: "unknown panel type", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[0].Type = "unknown"
				return stack
			})
			return layout
		}},
		{name: "empty panel title", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[0].Title = ""
				return stack
			})
			return layout
		}},
		{name: "missing link group", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[0].LinkGroup = "missing"
				return stack
			})
			return layout
		}},
		{name: "empty link group reference", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[0].LinkGroup = ""
				return stack
			})
			return layout
		}},
		{name: "view state version", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[0].Settings.View.Version = 0
				return stack
			})
			return layout
		}},
		{name: "singleton duplicate", cause: ErrSingleton, mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Root = mutateStack(layout.Root, "stack-a", func(stack appstate.TabStackNode) appstate.TabStackNode {
				stack.Panels[0] = singletonDuplicate.Clone()
				stack.Active = singletonDuplicate.ID
				return stack
			})
			return layout
		}},
		{name: "hidden duplicate ID", cause: ErrDuplicateID, mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			stack, _ := stackByID(layout.Root, "stack-a")
			layout.HiddenPanels = []appstate.PanelInstanceState{stack.Panels[0].Clone()}
			return layout
		}},
		{name: "maximized missing", mutate: func(layout appstate.WorkspaceLayout) appstate.WorkspaceLayout {
			layout.Maximized = "missing"
			return layout
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			layout := test.mutate(testWorkspaceLayout(t).Clone())
			err := ValidateLayout(layout)
			if !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("ValidateLayout error = %v, want ErrInvalidLayout", err)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("ValidateLayout error = %v, want cause %v", err, test.cause)
			}
		})
	}
}

func TestValidateLayoutCapsDockDepth(t *testing.T) {
	t.Parallel()

	root := appstate.DockNode(testStack(t, 0))
	for index := 1; index <= MaximumDockDepth+2; index++ {
		root = appstate.SplitNode{
			ID:          appstate.DockNodeID(fmt.Sprintf("split-%03d", index)),
			Orientation: appstate.SplitHorizontal,
			Ratio:       0.5,
			First:       root,
			Second:      testStack(t, index),
		}
	}
	layout := appstate.WorkspaceLayout{
		SchemaVersion: appstate.LayoutSchemaVersion,
		Workspace:     appstate.WorkspaceData,
		Root:          root,
		LinkGroups: []appstate.LinkGroup{{
			ID: "link-a", Name: "Group A", Color: appstate.LinkGroupCyan,
		}},
	}
	if err := ValidateLayout(layout); !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("ValidateLayout deep-tree error = %v, want ErrInvalidLayout", err)
	}
	if _, err := MinimumSize(root, DefaultSplitterThickness); !errors.Is(err, ErrInvalidGeometry) {
		t.Fatalf("MinimumSize deep-tree error = %v, want ErrInvalidGeometry", err)
	}
}

func TestValidatePanelDescriptor(t *testing.T) {
	t.Parallel()

	valid, err := appstate.PanelDescriptorFor(appstate.PanelOHLCVChart)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePanelDescriptor(valid); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(appstate.PanelDescriptor) appstate.PanelDescriptor
	}{
		{name: "type", mutate: func(descriptor appstate.PanelDescriptor) appstate.PanelDescriptor {
			descriptor.Type = "unknown"
			return descriptor
		}},
		{name: "title", mutate: func(descriptor appstate.PanelDescriptor) appstate.PanelDescriptor {
			descriptor.Title = ""
			return descriptor
		}},
		{name: "icon", mutate: func(descriptor appstate.PanelDescriptor) appstate.PanelDescriptor {
			descriptor.Icon = ""
			return descriptor
		}},
		{name: "multiplicity", mutate: func(descriptor appstate.PanelDescriptor) appstate.PanelDescriptor {
			descriptor.Multiplicity = "unknown"
			return descriptor
		}},
		{name: "minimum width", mutate: func(descriptor appstate.PanelDescriptor) appstate.PanelDescriptor {
			descriptor.MinimumSize.Width = 0
			return descriptor
		}},
		{name: "unknown link", mutate: func(descriptor appstate.PanelDescriptor) appstate.PanelDescriptor {
			descriptor.SupportedLinks = append(descriptor.SupportedLinks, appstate.LinkScope("unknown"))
			return descriptor
		}},
		{name: "duplicate link", mutate: func(descriptor appstate.PanelDescriptor) appstate.PanelDescriptor {
			descriptor.SupportedLinks = append(descriptor.SupportedLinks, descriptor.SupportedLinks[0])
			return descriptor
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidatePanelDescriptor(test.mutate(valid.Clone())); !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("ValidatePanelDescriptor error = %v, want ErrInvalidLayout", err)
			}
		})
	}
}

func mutateStack(
	root appstate.DockNode,
	id appstate.DockNodeID,
	mutate func(appstate.TabStackNode) appstate.TabStackNode,
) appstate.DockNode {
	switch node := root.(type) {
	case appstate.TabStackNode:
		if node.ID == id {
			return mutate(node)
		}
		return node
	case appstate.SplitNode:
		node.First = mutateStack(node.First, id, mutate)
		node.Second = mutateStack(node.Second, id, mutate)
		return node
	default:
		return root
	}
}

func testStack(t *testing.T, index int) appstate.TabStackNode {
	t.Helper()
	id := appstate.PanelID(fmt.Sprintf("panel-depth-%03d", index))
	panel := testPanel(t, id, appstate.PanelDatasetInspector)
	return appstate.TabStackNode{
		ID:     appstate.DockNodeID(fmt.Sprintf("stack-depth-%03d", index)),
		Active: panel.ID,
		Panels: []appstate.PanelInstanceState{panel},
	}
}
