package layouts

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

const (
	maximumLayouts         = 64
	maximumDockDepth       = 64
	maximumDockNodes       = 4096
	maximumPanels          = 4096
	maximumLinkGroups      = 256
	maximumTextBytes       = 4096
	maximumIdentifierBytes = 256
	maximumColumns         = 1024
	minimumSplitRatio      = 0.05
	maximumSplitRatio      = 0.95
)

var (
	// ErrInvalidSnapshot identifies a layout snapshot that violates structural invariants.
	ErrInvalidSnapshot = errors.New("invalid layout snapshot")
	// ErrInvalidDockTree identifies malformed dock-node structure.
	ErrInvalidDockTree = errors.New("invalid dock tree")
)

// Validate checks every layout, dock node, panel, and link reference without
// mutating the supplied snapshot.
func Validate(snapshot Snapshot) error {
	if len(snapshot.Layouts) == 0 {
		return fmt.Errorf("%w: no workspace layouts", ErrInvalidSnapshot)
	}
	if len(snapshot.Layouts) > maximumLayouts {
		return fmt.Errorf("%w: %d layouts exceeds limit %d", ErrInvalidSnapshot, len(snapshot.Layouts), maximumLayouts)
	}
	workspaces := make(map[appstate.Workspace]struct{}, len(snapshot.Layouts))
	for index, layout := range snapshot.Layouts {
		if !layout.Workspace.Valid() {
			return fmt.Errorf("%w: layout %d has unknown workspace %q", ErrInvalidSnapshot, index, layout.Workspace)
		}
		if _, exists := workspaces[layout.Workspace]; exists {
			return fmt.Errorf("%w: duplicate workspace %q", ErrInvalidSnapshot, layout.Workspace)
		}
		workspaces[layout.Workspace] = struct{}{}
		if err := validateLayout(layout); err != nil {
			return fmt.Errorf("%w: workspace %q: %w", ErrInvalidSnapshot, layout.Workspace, err)
		}
	}
	return nil
}

type layoutValidation struct {
	dockIDs        map[appstate.DockNodeID]struct{}
	panelIDs       map[appstate.PanelID]struct{}
	visiblePanels  map[appstate.PanelID]struct{}
	linkGroups     map[appstate.LinkGroupID]struct{}
	singletonTypes map[appstate.PanelType]appstate.PanelID
	dockNodes      int
	panels         int
}

func validateLayout(layout appstate.WorkspaceLayout) error {
	if layout.SchemaVersion != appstate.LayoutSchemaVersion {
		return fmt.Errorf("schema version %d, want %d", layout.SchemaVersion, appstate.LayoutSchemaVersion)
	}
	if layout.Root == nil {
		return fmt.Errorf("%w: root is nil", ErrInvalidDockTree)
	}
	if len(layout.LinkGroups) == 0 {
		return errors.New("no link groups")
	}
	if len(layout.LinkGroups) > maximumLinkGroups {
		return fmt.Errorf("%d link groups exceeds limit %d", len(layout.LinkGroups), maximumLinkGroups)
	}
	validation := layoutValidation{
		dockIDs:        make(map[appstate.DockNodeID]struct{}),
		panelIDs:       make(map[appstate.PanelID]struct{}),
		visiblePanels:  make(map[appstate.PanelID]struct{}),
		linkGroups:     make(map[appstate.LinkGroupID]struct{}),
		singletonTypes: make(map[appstate.PanelType]appstate.PanelID),
	}
	for index, group := range layout.LinkGroups {
		if err := validation.validateLinkGroup(group); err != nil {
			return fmt.Errorf("link group %d: %w", index, err)
		}
	}
	if err := validation.validateNode(layout.Root, 0); err != nil {
		return err
	}
	for index, panel := range layout.HiddenPanels {
		if err := validation.validatePanel(panel, false); err != nil {
			return fmt.Errorf("hidden panel %d: %w", index, err)
		}
	}
	if layout.Maximized != "" {
		if _, exists := validation.visiblePanels[layout.Maximized]; !exists {
			return fmt.Errorf("maximized panel %q is not visible", layout.Maximized)
		}
	}
	return nil
}

func (validation *layoutValidation) validateLinkGroup(group appstate.LinkGroup) error {
	if err := validateIdentifier(string(group.ID), "link-group ID", false); err != nil {
		return err
	}
	if _, exists := validation.linkGroups[group.ID]; exists {
		return fmt.Errorf("duplicate link-group ID %q", group.ID)
	}
	validation.linkGroups[group.ID] = struct{}{}
	if err := validateText(group.Name, "link-group name", false); err != nil {
		return err
	}
	if !group.Color.Valid() {
		return fmt.Errorf("unknown color %q", group.Color)
	}
	if err := validateLinkContext(group.Context); err != nil {
		return err
	}
	return nil
}

func (validation *layoutValidation) validateNode(node appstate.DockNode, depth int) error {
	if depth > maximumDockDepth {
		return fmt.Errorf("%w: depth exceeds %d", ErrInvalidDockTree, maximumDockDepth)
	}
	validation.dockNodes++
	if validation.dockNodes > maximumDockNodes {
		return fmt.Errorf("%w: node count exceeds %d", ErrInvalidDockTree, maximumDockNodes)
	}
	switch node := node.(type) {
	case appstate.SplitNode:
		return validation.validateSplitNode(node, depth)
	case *appstate.SplitNode:
		if node == nil {
			return fmt.Errorf("%w: nil split node", ErrInvalidDockTree)
		}
		return validation.validateSplitNode(*node, depth)
	case appstate.TabStackNode:
		return validation.validateTabStackNode(node, depth)
	case *appstate.TabStackNode:
		if node == nil {
			return fmt.Errorf("%w: nil tab-stack node", ErrInvalidDockTree)
		}
		return validation.validateTabStackNode(*node, depth)
	default:
		return fmt.Errorf("%w: unknown node variant %T", ErrInvalidDockTree, node)
	}
}

func (validation *layoutValidation) validateSplitNode(node appstate.SplitNode, depth int) error {
	if err := validation.addDockID(node.ID); err != nil {
		return err
	}
	switch node.Orientation {
	case appstate.SplitHorizontal, appstate.SplitVertical:
	default:
		return fmt.Errorf("%w: split %q has unknown orientation %q", ErrInvalidDockTree, node.ID, node.Orientation)
	}
	if math.IsNaN(node.Ratio) || math.IsInf(node.Ratio, 0) || node.Ratio < minimumSplitRatio || node.Ratio > maximumSplitRatio {
		return fmt.Errorf(
			"%w: split %q has ratio %v outside [%v,%v]",
			ErrInvalidDockTree,
			node.ID,
			node.Ratio,
			minimumSplitRatio,
			maximumSplitRatio,
		)
	}
	if node.First == nil || node.Second == nil {
		return fmt.Errorf("%w: split %q has a nil child", ErrInvalidDockTree, node.ID)
	}
	if err := validation.validateNode(node.First, depth+1); err != nil {
		return err
	}
	return validation.validateNode(node.Second, depth+1)
}

func (validation *layoutValidation) validateTabStackNode(node appstate.TabStackNode, depth int) error {
	if err := validation.addDockID(node.ID); err != nil {
		return err
	}
	if len(node.Panels) == 0 {
		if depth == 0 && node.Active == "" {
			return nil
		}
		return fmt.Errorf("%w: tab stack %q has no panels", ErrInvalidDockTree, node.ID)
	}
	activeFound := false
	for index, panel := range node.Panels {
		if err := validation.validatePanel(panel, true); err != nil {
			return fmt.Errorf("tab stack %q panel %d: %w", node.ID, index, err)
		}
		if panel.ID == node.Active {
			activeFound = true
		}
	}
	if !activeFound {
		return fmt.Errorf("%w: tab stack %q active panel %q is absent", ErrInvalidDockTree, node.ID, node.Active)
	}
	return nil
}

func (validation *layoutValidation) addDockID(id appstate.DockNodeID) error {
	if err := validateIdentifier(string(id), "dock-node ID", false); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDockTree, err)
	}
	if _, exists := validation.dockIDs[id]; exists {
		return fmt.Errorf("%w: duplicate dock-node ID %q", ErrInvalidDockTree, id)
	}
	validation.dockIDs[id] = struct{}{}
	return nil
}

func (validation *layoutValidation) validatePanel(panel appstate.PanelInstanceState, visible bool) error {
	validation.panels++
	if validation.panels > maximumPanels {
		return fmt.Errorf("panel count exceeds %d", maximumPanels)
	}
	if err := validateIdentifier(string(panel.ID), "panel ID", false); err != nil {
		return err
	}
	if _, exists := validation.panelIDs[panel.ID]; exists {
		return fmt.Errorf("duplicate panel ID %q", panel.ID)
	}
	validation.panelIDs[panel.ID] = struct{}{}
	if visible {
		validation.visiblePanels[panel.ID] = struct{}{}
	}
	descriptor, err := appstate.PanelDescriptorFor(panel.Type)
	if err != nil {
		return fmt.Errorf("unknown panel type %q", panel.Type)
	}
	if descriptor.Multiplicity == appstate.PanelSingleton {
		if previous, exists := validation.singletonTypes[panel.Type]; exists {
			return fmt.Errorf("singleton panel type %q occurs in %q and %q", panel.Type, previous, panel.ID)
		}
		validation.singletonTypes[panel.Type] = panel.ID
	}
	if err := validateText(panel.Title, "panel title", false); err != nil {
		return err
	}
	if _, exists := validation.linkGroups[panel.LinkGroup]; !exists {
		return fmt.Errorf("panel %q references unknown link group %q", panel.ID, panel.LinkGroup)
	}
	if panel.Settings.View.Version <= 0 {
		return fmt.Errorf("panel %q has nonpositive view version %d", panel.ID, panel.Settings.View.Version)
	}
	if panel.Settings.View.CursorRow < 0 {
		return fmt.Errorf("panel %q has negative cursor row", panel.ID)
	}
	if err := validateText(panel.Settings.View.SortKey, "sort key", true); err != nil {
		return err
	}
	if err := validateText(panel.Settings.View.Filter, "filter", true); err != nil {
		return err
	}
	if err := validateTimeRange(panel.Settings.View.TimeRange); err != nil {
		return fmt.Errorf("panel %q view range: %w", panel.ID, err)
	}
	if len(panel.Settings.View.SelectedColumns) > maximumColumns {
		return fmt.Errorf("panel %q selected-column count exceeds %d", panel.ID, maximumColumns)
	}
	columns := make(map[string]struct{}, len(panel.Settings.View.SelectedColumns))
	for _, column := range panel.Settings.View.SelectedColumns {
		if err := validateText(column, "selected column", false); err != nil {
			return err
		}
		if _, exists := columns[column]; exists {
			return fmt.Errorf("panel %q has duplicate selected column %q", panel.ID, column)
		}
		columns[column] = struct{}{}
	}
	return nil
}

func validateLinkContext(context appstate.LinkContext) error {
	if err := validateIdentifier(string(context.DatasetID), "dataset ID", true); err != nil {
		return err
	}
	if err := validateIdentifier(string(context.ExperimentID), "experiment ID", true); err != nil {
		return err
	}
	if err := validateIdentifier(string(context.RunID), "run ID", true); err != nil {
		return err
	}
	if err := validateIdentifier(string(context.ModelID), "model ID", true); err != nil {
		return err
	}
	switch context.Interval {
	case appstate.IntervalDaily, appstate.IntervalHourly:
	default:
		return fmt.Errorf("unknown interval %q", context.Interval)
	}
	if err := validateTimeRange(context.TimeRange); err != nil {
		return fmt.Errorf("link time range: %w", err)
	}
	symbols := make(map[appstate.Symbol]struct{}, len(context.Symbols))
	for _, symbol := range context.Symbols {
		if err := validateIdentifier(string(symbol), "symbol", false); err != nil {
			return err
		}
		if _, exists := symbols[symbol]; exists {
			return fmt.Errorf("duplicate symbol %q", symbol)
		}
		symbols[symbol] = struct{}{}
	}
	return nil
}

func validateTimeRange(timeRange appstate.TimeRange) error {
	startZero := timeRange.Start.IsZero()
	endZero := timeRange.End.IsZero()
	if startZero != endZero {
		return errors.New("only one endpoint is set")
	}
	if !startZero && timeRange.End.Before(timeRange.Start) {
		return errors.New("end precedes start")
	}
	return nil
}

func validateIdentifier(value string, field string, optional bool) error {
	if value == "" {
		if optional {
			return nil
		}
		return fmt.Errorf("%s is empty", field)
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("%s has surrounding whitespace", field)
	}
	if len(value) > maximumIdentifierBytes {
		return fmt.Errorf("%s exceeds %d bytes", field, maximumIdentifierBytes)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", field)
	}
	return nil
}

func validateText(value string, field string, optional bool) error {
	if value == "" && !optional {
		return fmt.Errorf("%s is empty", field)
	}
	if len(value) > maximumTextBytes {
		return fmt.Errorf("%s exceeds %d bytes", field, maximumTextBytes)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", field)
	}
	return nil
}
