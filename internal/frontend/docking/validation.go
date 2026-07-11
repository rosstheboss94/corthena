package docking

import (
	"fmt"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

// MaximumDockDepth bounds recursive layout parsing and tree operations.
const MaximumDockDepth = 64

// ValidatePanelDescriptor checks the closed descriptor fields used by docking
// geometry and singleton enforcement.
func ValidatePanelDescriptor(descriptor appstate.PanelDescriptor) error {
	if descriptor.Type == "" {
		return fmt.Errorf("%w: panel descriptor type is empty", ErrInvalidLayout)
	}
	if _, err := appstate.PanelDescriptorFor(descriptor.Type); err != nil {
		return fmt.Errorf("%w: descriptor type %q: %v", ErrInvalidLayout, descriptor.Type, err)
	}
	if descriptor.Title == "" {
		return fmt.Errorf("%w: descriptor %q title is empty", ErrInvalidLayout, descriptor.Type)
	}
	if descriptor.Icon == "" {
		return fmt.Errorf("%w: descriptor %q icon is empty", ErrInvalidLayout, descriptor.Type)
	}
	switch descriptor.Multiplicity {
	case appstate.PanelSingleton, appstate.PanelMultiple:
	default:
		return fmt.Errorf("%w: descriptor %q has multiplicity %q", ErrInvalidLayout, descriptor.Type, descriptor.Multiplicity)
	}
	if descriptor.MinimumSize.Width <= 0 || descriptor.MinimumSize.Height <= 0 {
		return fmt.Errorf("%w: descriptor %q minimum size must be positive", ErrInvalidLayout, descriptor.Type)
	}
	seenLinks := make(map[appstate.LinkScope]struct{}, len(descriptor.SupportedLinks))
	for _, scope := range descriptor.SupportedLinks {
		if !validLinkScope(scope) {
			return fmt.Errorf("%w: descriptor %q has link scope %q", ErrInvalidLayout, descriptor.Type, scope)
		}
		if _, duplicate := seenLinks[scope]; duplicate {
			return fmt.Errorf("%w: descriptor %q repeats link scope %q", ErrInvalidLayout, descriptor.Type, scope)
		}
		seenLinks[scope] = struct{}{}
	}
	return nil
}

// ValidateLayout checks a complete current-version workspace layout. It
// validates all node, panel, active-tab, maximize, singleton, and link-group
// references before the layout is used or persisted.
func ValidateLayout(layout appstate.WorkspaceLayout) error {
	if layout.SchemaVersion != appstate.LayoutSchemaVersion {
		return fmt.Errorf(
			"%w: schema version %d, want %d",
			ErrInvalidLayout,
			layout.SchemaVersion,
			appstate.LayoutSchemaVersion,
		)
	}
	if !layout.Workspace.Valid() {
		return fmt.Errorf("%w: workspace %q is unknown", ErrInvalidLayout, layout.Workspace)
	}
	if layout.Root == nil {
		return fmt.Errorf("%w: dock root is nil", ErrInvalidLayout)
	}

	validator := layoutValidator{
		nodeIDs:       make(map[appstate.DockNodeID]struct{}),
		panelIDs:      make(map[appstate.PanelID]struct{}),
		visiblePanels: make(map[appstate.PanelID]struct{}),
		panelTypes:    make(map[appstate.PanelType]int),
		linkGroups:    make(map[appstate.LinkGroupID]struct{}),
	}
	if len(layout.LinkGroups) == 0 {
		return fmt.Errorf("%w: workspace %q has no link groups", ErrInvalidLayout, layout.Workspace)
	}
	for _, group := range layout.LinkGroups {
		if group.ID == "" {
			return fmt.Errorf("%w: link-group ID is empty", ErrInvalidLayout)
		}
		if _, duplicate := validator.linkGroups[group.ID]; duplicate {
			return fmt.Errorf("%w: %w: link-group ID %q", ErrInvalidLayout, ErrDuplicateID, group.ID)
		}
		if group.Name == "" {
			return fmt.Errorf("%w: link group %q has an empty name", ErrInvalidLayout, group.ID)
		}
		if !group.Color.Valid() {
			return fmt.Errorf("%w: link group %q has invalid color %q", ErrInvalidLayout, group.ID, group.Color)
		}
		if !group.Context.TimeRange.Start.IsZero() && !group.Context.TimeRange.End.IsZero() &&
			group.Context.TimeRange.Start.After(group.Context.TimeRange.End) {
			return fmt.Errorf("%w: link group %q time range starts after it ends", ErrInvalidLayout, group.ID)
		}
		validator.linkGroups[group.ID] = struct{}{}
	}
	if err := validator.validateNode(layout.Root, 0, true); err != nil {
		return err
	}
	for _, panel := range layout.HiddenPanels {
		if err := validator.validatePanel(panel, false); err != nil {
			return err
		}
	}
	for panelType, count := range validator.panelTypes {
		descriptor, err := appstate.PanelDescriptorFor(panelType)
		if err != nil {
			return fmt.Errorf("%w: panel type %q: %v", ErrInvalidLayout, panelType, err)
		}
		if descriptor.Multiplicity == appstate.PanelSingleton && count > 1 {
			return fmt.Errorf("%w: %w: panel type %q has %d instances", ErrInvalidLayout, ErrSingleton, panelType, count)
		}
	}
	if layout.Maximized != "" {
		if _, visible := validator.visiblePanels[layout.Maximized]; !visible {
			return fmt.Errorf("%w: maximized panel %q is not visible", ErrInvalidLayout, layout.Maximized)
		}
	}
	return nil
}

type layoutValidator struct {
	nodeIDs       map[appstate.DockNodeID]struct{}
	panelIDs      map[appstate.PanelID]struct{}
	visiblePanels map[appstate.PanelID]struct{}
	panelTypes    map[appstate.PanelType]int
	linkGroups    map[appstate.LinkGroupID]struct{}
}

func (validator *layoutValidator) validateNode(root appstate.DockNode, depth int, isRoot bool) error {
	if depth > MaximumDockDepth {
		return fmt.Errorf("%w: dock tree exceeds depth %d", ErrInvalidLayout, MaximumDockDepth)
	}
	switch node := root.(type) {
	case appstate.TabStackNode:
		if err := validator.recordNodeID(node.ID); err != nil {
			return err
		}
		if !isRoot && len(node.Panels) == 0 {
			return fmt.Errorf("%w: non-root tab stack %q is empty", ErrInvalidLayout, node.ID)
		}
		if len(node.Panels) == 0 {
			if node.Active != "" {
				return fmt.Errorf("%w: empty tab stack %q has active panel %q", ErrInvalidLayout, node.ID, node.Active)
			}
			return nil
		}
		activeFound := false
		for _, panel := range node.Panels {
			if err := validator.validatePanel(panel, true); err != nil {
				return err
			}
			if panel.ID == node.Active {
				activeFound = true
			}
		}
		if node.Active == "" || !activeFound {
			return fmt.Errorf("%w: tab stack %q active panel %q is not in the stack", ErrInvalidLayout, node.ID, node.Active)
		}
		return nil
	case appstate.SplitNode:
		if err := validator.recordNodeID(node.ID); err != nil {
			return err
		}
		switch node.Orientation {
		case appstate.SplitHorizontal, appstate.SplitVertical:
		default:
			return fmt.Errorf("%w: split %q has orientation %q", ErrInvalidLayout, node.ID, node.Orientation)
		}
		if !finite(node.Ratio) || node.Ratio < MinimumSplitRatio || node.Ratio > MaximumSplitRatio {
			return fmt.Errorf(
				"%w: split %q ratio %v is outside [%v, %v]",
				ErrInvalidLayout,
				node.ID,
				node.Ratio,
				MinimumSplitRatio,
				MaximumSplitRatio,
			)
		}
		if node.First == nil || node.Second == nil {
			return fmt.Errorf("%w: split %q has a nil child", ErrInvalidLayout, node.ID)
		}
		if err := validator.validateNode(node.First, depth+1, false); err != nil {
			return err
		}
		return validator.validateNode(node.Second, depth+1, false)
	default:
		return fmt.Errorf("%w: unsupported dock node %T", ErrInvalidLayout, root)
	}
}

func (validator *layoutValidator) recordNodeID(id appstate.DockNodeID) error {
	if id == "" {
		return fmt.Errorf("%w: dock node ID is empty", ErrInvalidLayout)
	}
	if _, duplicate := validator.nodeIDs[id]; duplicate {
		return fmt.Errorf("%w: %w: dock node ID %q", ErrInvalidLayout, ErrDuplicateID, id)
	}
	validator.nodeIDs[id] = struct{}{}
	return nil
}

func (validator *layoutValidator) validatePanel(panel appstate.PanelInstanceState, visible bool) error {
	if panel.ID == "" {
		return fmt.Errorf("%w: panel ID is empty", ErrInvalidLayout)
	}
	if _, duplicate := validator.panelIDs[panel.ID]; duplicate {
		return fmt.Errorf("%w: %w: panel ID %q", ErrInvalidLayout, ErrDuplicateID, panel.ID)
	}
	descriptor, err := appstate.PanelDescriptorFor(panel.Type)
	if err != nil {
		return fmt.Errorf("%w: panel %q: %v", ErrInvalidLayout, panel.ID, err)
	}
	if err := ValidatePanelDescriptor(descriptor); err != nil {
		return err
	}
	if panel.Title == "" {
		return fmt.Errorf("%w: panel %q title is empty", ErrInvalidLayout, panel.ID)
	}
	if _, exists := validator.linkGroups[panel.LinkGroup]; !exists {
		return fmt.Errorf("%w: panel %q references missing link group %q", ErrInvalidLayout, panel.ID, panel.LinkGroup)
	}
	if panel.Settings.View.Version <= 0 {
		return fmt.Errorf("%w: panel %q has invalid view-state version %d", ErrInvalidLayout, panel.ID, panel.Settings.View.Version)
	}
	validator.panelIDs[panel.ID] = struct{}{}
	validator.panelTypes[panel.Type]++
	if visible {
		validator.visiblePanels[panel.ID] = struct{}{}
	}
	return nil
}

func validLinkScope(scope appstate.LinkScope) bool {
	switch scope {
	case appstate.LinkDataset,
		appstate.LinkSymbols,
		appstate.LinkInterval,
		appstate.LinkTimeRange,
		appstate.LinkExperiment,
		appstate.LinkRun,
		appstate.LinkModel:
		return true
	default:
		return false
	}
}
