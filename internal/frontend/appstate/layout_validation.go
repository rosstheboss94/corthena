package appstate

import (
	"fmt"
	"math"
)

const (
	maxDockDepth      = 64
	minimumSplitRatio = 0.05
	maximumSplitRatio = 0.95
)

// CloneWorkspaceLayouts returns an independent copy suitable for an
// asynchronous persistence boundary.
func CloneWorkspaceLayouts(layouts []WorkspaceLayout) []WorkspaceLayout {
	return cloneLayouts(layouts)
}

// ValidateWorkspaceLayouts validates a nonempty set with unique workspaces.
func ValidateWorkspaceLayouts(layouts []WorkspaceLayout) error {
	if len(layouts) == 0 {
		return fmt.Errorf("%w: layout set is empty", ErrInvariant)
	}
	workspaces := make(map[Workspace]struct{}, len(layouts))
	for index, layout := range layouts {
		if _, exists := workspaces[layout.Workspace]; exists {
			return fmt.Errorf("%w: duplicate workspace %q at layout %d", ErrInvariant, layout.Workspace, index)
		}
		if err := ValidateWorkspaceLayout(layout); err != nil {
			return fmt.Errorf("validate layout %d: %w", index, err)
		}
		workspaces[layout.Workspace] = struct{}{}
	}
	return nil
}

// ValidateWorkspaceLayout validates all dock, panel, and link references in a
// current-schema layout before it enters frontend state.
func ValidateWorkspaceLayout(layout WorkspaceLayout) error {
	if layout.SchemaVersion != LayoutSchemaVersion {
		return fmt.Errorf(
			"%w: workspace %q schema version %d, want %d",
			ErrInvariant,
			layout.Workspace,
			layout.SchemaVersion,
			LayoutSchemaVersion,
		)
	}
	if !layout.Workspace.Valid() {
		return fmt.Errorf("%w: unknown workspace %q", ErrInvariant, layout.Workspace)
	}
	if layout.Root == nil {
		return fmt.Errorf("%w: workspace %q has nil dock root", ErrInvariant, layout.Workspace)
	}

	groupIDs := make(map[LinkGroupID]struct{}, len(layout.LinkGroups))
	for _, group := range layout.LinkGroups {
		if group.ID == "" {
			return fmt.Errorf("%w: workspace %q has empty link-group ID", ErrInvariant, layout.Workspace)
		}
		if group.Name == "" {
			return fmt.Errorf("%w: link group %q has empty name", ErrInvariant, group.ID)
		}
		if !group.Color.Valid() {
			return fmt.Errorf("%w: link group %q has invalid color %q", ErrInvariant, group.ID, group.Color)
		}
		if _, exists := groupIDs[group.ID]; exists {
			return fmt.Errorf("%w: duplicate link-group ID %q", ErrInvariant, group.ID)
		}
		if err := validateLinkContext(group.Context); err != nil {
			return fmt.Errorf("validate link group %q: %w", group.ID, err)
		}
		groupIDs[group.ID] = struct{}{}
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("%w: workspace %q has no link groups", ErrInvariant, layout.Workspace)
	}

	validation := dockValidation{
		nodeIDs:       make(map[DockNodeID]struct{}),
		panelIDs:      make(map[PanelID]struct{}),
		panelTypes:    make(map[PanelType]int),
		visiblePanels: make(map[PanelID]struct{}),
		linkGroupIDs:  groupIDs,
	}
	if _, err := validation.validateNode(layout.Root, 0, true); err != nil {
		return err
	}
	for _, panel := range layout.HiddenPanels {
		if err := validation.validatePanel(panel, false); err != nil {
			return fmt.Errorf("validate hidden panel %q: %w", panel.ID, err)
		}
	}
	if layout.Maximized != "" {
		if _, exists := validation.visiblePanels[layout.Maximized]; !exists {
			return fmt.Errorf("%w: maximized panel %q is not visible", ErrInvariant, layout.Maximized)
		}
	}
	for panelType, count := range validation.panelTypes {
		descriptor, err := PanelDescriptorFor(panelType)
		if err != nil {
			return err
		}
		if descriptor.Multiplicity == PanelSingleton && count > 1 {
			return fmt.Errorf("%w: singleton panel type %q appears %d times", ErrInvariant, panelType, count)
		}
	}
	return nil
}

type dockValidation struct {
	nodeIDs       map[DockNodeID]struct{}
	panelIDs      map[PanelID]struct{}
	panelTypes    map[PanelType]int
	visiblePanels map[PanelID]struct{}
	linkGroupIDs  map[LinkGroupID]struct{}
}

func (validation *dockValidation) validateNode(node DockNode, depth int, root bool) (bool, error) {
	if depth > maxDockDepth {
		return false, fmt.Errorf("%w: dock tree exceeds depth %d", ErrInvariant, maxDockDepth)
	}
	switch node := node.(type) {
	case SplitNode:
		if err := validation.addNodeID(node.ID); err != nil {
			return false, err
		}
		if node.Orientation != SplitHorizontal && node.Orientation != SplitVertical {
			return false, fmt.Errorf("%w: split %q has invalid orientation %q", ErrInvariant, node.ID, node.Orientation)
		}
		if math.IsNaN(node.Ratio) || math.IsInf(node.Ratio, 0) ||
			node.Ratio < minimumSplitRatio || node.Ratio > maximumSplitRatio {
			return false, fmt.Errorf("%w: split %q has invalid ratio %v", ErrInvariant, node.ID, node.Ratio)
		}
		if node.First == nil || node.Second == nil {
			return false, fmt.Errorf("%w: split %q has a nil child", ErrInvariant, node.ID)
		}
		firstEmpty, err := validation.validateNode(node.First, depth+1, false)
		if err != nil {
			return false, err
		}
		secondEmpty, err := validation.validateNode(node.Second, depth+1, false)
		if err != nil {
			return false, err
		}
		if firstEmpty || secondEmpty {
			return false, fmt.Errorf("%w: split %q contains an empty stack", ErrInvariant, node.ID)
		}
		return false, nil
	case TabStackNode:
		if err := validation.addNodeID(node.ID); err != nil {
			return false, err
		}
		if len(node.Panels) == 0 {
			if !root || node.Active != "" {
				return false, fmt.Errorf("%w: stack %q is empty or has a dangling active panel", ErrInvariant, node.ID)
			}
			return true, nil
		}
		activeFound := false
		for _, panel := range node.Panels {
			if err := validation.validatePanel(panel, true); err != nil {
				return false, fmt.Errorf("validate stack %q: %w", node.ID, err)
			}
			if panel.ID == node.Active {
				activeFound = true
			}
		}
		if !activeFound {
			return false, fmt.Errorf("%w: stack %q active panel %q is missing", ErrInvariant, node.ID, node.Active)
		}
		return false, nil
	default:
		return false, fmt.Errorf("%w: unhandled dock node %T", ErrInvariant, node)
	}
}

func (validation *dockValidation) addNodeID(id DockNodeID) error {
	if id == "" {
		return fmt.Errorf("%w: dock node ID is empty", ErrInvariant)
	}
	if _, exists := validation.nodeIDs[id]; exists {
		return fmt.Errorf("%w: duplicate dock node ID %q", ErrInvariant, id)
	}
	validation.nodeIDs[id] = struct{}{}
	return nil
}

func (validation *dockValidation) validatePanel(panel PanelInstanceState, visible bool) error {
	if err := validatePanel(panel); err != nil {
		return err
	}
	if _, exists := validation.panelIDs[panel.ID]; exists {
		return fmt.Errorf("%w: duplicate panel ID %q", ErrInvariant, panel.ID)
	}
	if _, exists := validation.linkGroupIDs[panel.LinkGroup]; !exists {
		return fmt.Errorf("%w: panel %q references unknown link group %q", ErrInvariant, panel.ID, panel.LinkGroup)
	}
	if panel.Settings.View.Version <= 0 {
		return fmt.Errorf("%w: panel %q has invalid view-state version %d", ErrInvariant, panel.ID, panel.Settings.View.Version)
	}
	validation.panelIDs[panel.ID] = struct{}{}
	validation.panelTypes[panel.Type]++
	if visible {
		validation.visiblePanels[panel.ID] = struct{}{}
	}
	return nil
}

func validateLinkContext(context LinkContext) error {
	if !context.Interval.Valid() {
		return fmt.Errorf("%w: invalid link interval %q", ErrInvariant, context.Interval)
	}
	if context.TimeRange.Start.IsZero() != context.TimeRange.End.IsZero() {
		return fmt.Errorf("%w: link time range has only one endpoint", ErrInvariant)
	}
	if !context.TimeRange.Start.IsZero() &&
		context.TimeRange.Start.After(context.TimeRange.End) {
		return fmt.Errorf("%w: link time range starts after it ends", ErrInvariant)
	}
	return nil
}
