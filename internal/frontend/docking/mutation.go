package docking

import (
	"fmt"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

// ActivatePanel returns a cloned layout with panel active in its current stack.
func ActivatePanel(layout appstate.WorkspaceLayout, panelID appstate.PanelID) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	if panelID == "" {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel ID is empty", ErrInvalidMutation)
	}
	root, found := activatePanel(next.Root, panelID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q", ErrNotFound, panelID)
	}
	next.Root = root
	return finishMutation(next)
}

// ReorderPanel moves a panel to a final zero-based index within its current
// stack while preserving that stack's active panel.
func ReorderPanel(layout appstate.WorkspaceLayout, panelID appstate.PanelID, index int) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	location, found := findPanel(next.Root, panelID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q", ErrNotFound, panelID)
	}
	if index < 0 || index >= location.stackLength {
		return appstate.WorkspaceLayout{}, fmt.Errorf(
			"%w: reorder index %d is outside [0, %d)",
			ErrInvalidMutation,
			index,
			location.stackLength,
		)
	}
	root, changed := reorderPanel(next.Root, location.stackID, panelID, index, false)
	if !changed {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: stack %q", ErrNotFound, location.stackID)
	}
	next.Root = root
	return finishMutation(next)
}

// MovePanel moves a visible panel into a target tab stack. For another stack,
// index is an insertion index in [0, target length]. For the current stack it
// is the panel's final index in [0, stack length). The moved panel becomes
// active and an emptied source split is collapsed.
func MovePanel(
	layout appstate.WorkspaceLayout,
	panelID appstate.PanelID,
	targetStackID appstate.DockNodeID,
	index int,
) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	location, found := findPanel(next.Root, panelID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q", ErrNotFound, panelID)
	}
	target, found := findStack(next.Root, targetStackID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
	}
	if location.stackID == targetStackID {
		if index < 0 || index >= len(target.Panels) {
			return appstate.WorkspaceLayout{}, fmt.Errorf("%w: move index %d is outside [0, %d)", ErrInvalidMutation, index, len(target.Panels))
		}
		root, changed := reorderPanel(next.Root, targetStackID, panelID, index, true)
		if !changed {
			return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
		}
		next.Root = root
		return finishMutation(next)
	}
	if index < 0 || index > len(target.Panels) {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: move index %d is outside [0, %d]", ErrInvalidMutation, index, len(target.Panels))
	}
	root, panel, _, found := detachPanel(next.Root, panelID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q", ErrNotFound, panelID)
	}
	if root == nil {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: moving panel %q removed the target tree", ErrInvalidMutation, panelID)
	}
	root, inserted := insertPanel(root, targetStackID, panel, index, true)
	if !inserted {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q disappeared", ErrInvalidMutation, targetStackID)
	}
	next.Root = root
	return finishMutation(next)
}

// DockPanel moves a visible panel to a center tab target or creates a new
// directional split around targetStackID. newPaneRatio is the share assigned
// to the moved panel; it is clamped before storage. Directional docking
// requires unique, non-empty new stack and split IDs. Center docking appends
// the panel and ignores those IDs.
func DockPanel(
	layout appstate.WorkspaceLayout,
	panelID appstate.PanelID,
	targetStackID appstate.DockNodeID,
	position DockPosition,
	newStackID appstate.DockNodeID,
	newSplitID appstate.DockNodeID,
	newPaneRatio float64,
) (appstate.WorkspaceLayout, error) {
	if !position.Valid() {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: dock position %q", ErrInvalidMutation, position)
	}
	if position == DockCenter {
		if err := ValidateLayout(layout); err != nil {
			return appstate.WorkspaceLayout{}, err
		}
		location, found := findPanel(layout.Root, panelID)
		if !found {
			return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q", ErrNotFound, panelID)
		}
		target, found := findStack(layout.Root, targetStackID)
		if !found {
			return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
		}
		if location.stackID == targetStackID {
			return ActivatePanel(layout, panelID)
		}
		return MovePanel(layout, panelID, targetStackID, len(target.Panels))
	}

	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	location, found := findPanel(next.Root, panelID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q", ErrNotFound, panelID)
	}
	target, found := findStack(next.Root, targetStackID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
	}
	if location.stackID == targetStackID && len(target.Panels) == 1 {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: a stack cannot be split against its only panel", ErrInvalidMutation)
	}
	if newStackID == "" || newSplitID == "" {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: new stack and split IDs are required", ErrInvalidMutation)
	}
	if newStackID == newSplitID {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: %w: new stack and split IDs are both %q", ErrInvalidMutation, ErrDuplicateID, newStackID)
	}
	ids := collectNodeIDs(next.Root)
	if _, duplicate := ids[newStackID]; duplicate {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: %w: dock node ID %q", ErrInvalidMutation, ErrDuplicateID, newStackID)
	}
	if _, duplicate := ids[newSplitID]; duplicate {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: %w: dock node ID %q", ErrInvalidMutation, ErrDuplicateID, newSplitID)
	}

	root, panel, _, found := detachPanel(next.Root, panelID)
	if !found || root == nil {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q cannot be detached for splitting", ErrInvalidMutation, panelID)
	}
	newStack := appstate.TabStackNode{
		ID:     newStackID,
		Active: panel.ID,
		Panels: []appstate.PanelInstanceState{panel.Clone()},
	}
	paneRatio := ClampSplitRatio(newPaneRatio)
	root, replaced := replaceStackWithSplit(root, targetStackID, position, newStack, newSplitID, paneRatio)
	if !replaced {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q disappeared", ErrInvalidMutation, targetStackID)
	}
	next.Root = root
	return finishMutation(next)
}

// SplitPanel is the directional-only form of DockPanel.
func SplitPanel(
	layout appstate.WorkspaceLayout,
	panelID appstate.PanelID,
	targetStackID appstate.DockNodeID,
	position DockPosition,
	newStackID appstate.DockNodeID,
	newSplitID appstate.DockNodeID,
	newPaneRatio float64,
) (appstate.WorkspaceLayout, error) {
	if position == DockCenter {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: SplitPanel requires a directional position", ErrInvalidMutation)
	}
	return DockPanel(layout, panelID, targetStackID, position, newStackID, newSplitID, newPaneRatio)
}

// ClosePanel removes a visible panel, appends it to HiddenPanels, and collapses
// an emptied split. Closing the final root panel preserves its empty stack ID
// so a later reopen has a stable target.
func ClosePanel(layout appstate.WorkspaceLayout, panelID appstate.PanelID) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	root, panel, sourceStackID, found := detachPanel(next.Root, panelID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: panel %q", ErrNotFound, panelID)
	}
	if root == nil {
		root = appstate.TabStackNode{ID: sourceStackID}
	}
	next.Root = root
	next.HiddenPanels = append(next.HiddenPanels, panel.Clone())
	if next.Maximized == panelID {
		next.Maximized = ""
	}
	return finishMutation(next)
}

// ReopenPanel moves one hidden panel into a target stack at index and makes it
// active. The panel's stable ID, settings, and link group are preserved.
func ReopenPanel(
	layout appstate.WorkspaceLayout,
	panelID appstate.PanelID,
	targetStackID appstate.DockNodeID,
	index int,
) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	target, found := findStack(next.Root, targetStackID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
	}
	if index < 0 || index > len(target.Panels) {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: reopen index %d is outside [0, %d]", ErrInvalidMutation, index, len(target.Panels))
	}
	hiddenIndex := -1
	var panel appstate.PanelInstanceState
	for candidateIndex, candidate := range next.HiddenPanels {
		if candidate.ID == panelID {
			hiddenIndex = candidateIndex
			panel = candidate.Clone()
			break
		}
	}
	if hiddenIndex < 0 {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: hidden panel %q", ErrNotFound, panelID)
	}
	next.HiddenPanels = removePanelAt(next.HiddenPanels, hiddenIndex)
	root, inserted := insertPanel(next.Root, targetStackID, panel, index, true)
	if !inserted {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
	}
	next.Root = root
	return finishMutation(next)
}

// AddPanel inserts a newly-created panel into a visible stack. Validation
// rejects duplicate stable IDs, missing link groups, and singleton duplicates.
func AddPanel(
	layout appstate.WorkspaceLayout,
	panel appstate.PanelInstanceState,
	targetStackID appstate.DockNodeID,
	index int,
) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	target, found := findStack(next.Root, targetStackID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
	}
	if index < 0 || index > len(target.Panels) {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: add index %d is outside [0, %d]", ErrInvalidMutation, index, len(target.Panels))
	}
	root, inserted := insertPanel(next.Root, targetStackID, panel.Clone(), index, true)
	if !inserted {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: target stack %q", ErrNotFound, targetStackID)
	}
	next.Root = root
	return finishMutation(next)
}

// MaximizePanel marks one visible panel for temporary full-host display.
func MaximizePanel(layout appstate.WorkspaceLayout, panelID appstate.PanelID) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	if _, found := findPanel(next.Root, panelID); !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: visible panel %q", ErrNotFound, panelID)
	}
	next.Maximized = panelID
	return finishMutation(next)
}

// RestorePanel clears temporary panel maximization.
func RestorePanel(layout appstate.WorkspaceLayout) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	next.Maximized = ""
	return finishMutation(next)
}

// ResizeSplit stores a finite, clamped first-child ratio for one split.
func ResizeSplit(
	layout appstate.WorkspaceLayout,
	splitID appstate.DockNodeID,
	ratio float64,
) (appstate.WorkspaceLayout, error) {
	next, err := validatedClone(layout)
	if err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	root, found := resizeSplit(next.Root, splitID, ClampSplitRatio(ratio))
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: split %q", ErrNotFound, splitID)
	}
	next.Root = root
	return finishMutation(next)
}

type panelLocation struct {
	stackID     appstate.DockNodeID
	stackLength int
}

func validatedClone(layout appstate.WorkspaceLayout) (appstate.WorkspaceLayout, error) {
	if err := ValidateLayout(layout); err != nil {
		return appstate.WorkspaceLayout{}, err
	}
	return layout.Clone(), nil
}

func finishMutation(layout appstate.WorkspaceLayout) (appstate.WorkspaceLayout, error) {
	if err := ValidateLayout(layout); err != nil {
		return appstate.WorkspaceLayout{}, fmt.Errorf("%w: mutation produced invalid layout: %w", ErrInvalidMutation, err)
	}
	return layout, nil
}

func findPanel(root appstate.DockNode, panelID appstate.PanelID) (panelLocation, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		for _, panel := range node.Panels {
			if panel.ID == panelID {
				return panelLocation{stackID: node.ID, stackLength: len(node.Panels)}, true
			}
		}
	case appstate.SplitNode:
		if location, found := findPanel(node.First, panelID); found {
			return location, true
		}
		return findPanel(node.Second, panelID)
	}
	return panelLocation{}, false
}

func findStack(root appstate.DockNode, stackID appstate.DockNodeID) (appstate.TabStackNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return node, node.ID == stackID
	case appstate.SplitNode:
		if stack, found := findStack(node.First, stackID); found {
			return stack, true
		}
		return findStack(node.Second, stackID)
	default:
		return appstate.TabStackNode{}, false
	}
}

func activatePanel(root appstate.DockNode, panelID appstate.PanelID) (appstate.DockNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		for _, panel := range node.Panels {
			if panel.ID == panelID {
				node.Active = panelID
				return node, true
			}
		}
		return node, false
	case appstate.SplitNode:
		first, found := activatePanel(node.First, panelID)
		if found {
			node.First = first
			return node, true
		}
		second, found := activatePanel(node.Second, panelID)
		node.Second = second
		return node, found
	default:
		return root, false
	}
}

func reorderPanel(
	root appstate.DockNode,
	stackID appstate.DockNodeID,
	panelID appstate.PanelID,
	index int,
	activate bool,
) (appstate.DockNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		if node.ID != stackID {
			return node, false
		}
		current := -1
		for candidateIndex, panel := range node.Panels {
			if panel.ID == panelID {
				current = candidateIndex
				break
			}
		}
		if current < 0 {
			return node, false
		}
		panel := node.Panels[current].Clone()
		panels := removePanelAt(node.Panels, current)
		node.Panels = insertPanelAt(panels, panel, index)
		if activate {
			node.Active = panelID
		}
		return node, true
	case appstate.SplitNode:
		first, changed := reorderPanel(node.First, stackID, panelID, index, activate)
		if changed {
			node.First = first
			return node, true
		}
		second, changed := reorderPanel(node.Second, stackID, panelID, index, activate)
		node.Second = second
		return node, changed
	default:
		return root, false
	}
}

func detachPanel(
	root appstate.DockNode,
	panelID appstate.PanelID,
) (appstate.DockNode, appstate.PanelInstanceState, appstate.DockNodeID, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		for index, panel := range node.Panels {
			if panel.ID != panelID {
				continue
			}
			detached := panel.Clone()
			node.Panels = removePanelAt(node.Panels, index)
			if len(node.Panels) == 0 {
				return nil, detached, node.ID, true
			}
			if node.Active == panelID {
				node.Active = node.Panels[0].ID
			}
			return node, detached, node.ID, true
		}
		return node, appstate.PanelInstanceState{}, "", false
	case appstate.SplitNode:
		first, panel, sourceStackID, found := detachPanel(node.First, panelID)
		if found {
			if first == nil {
				return node.Second, panel, sourceStackID, true
			}
			node.First = first
			return node, panel, sourceStackID, true
		}
		second, panel, sourceStackID, found := detachPanel(node.Second, panelID)
		if found {
			if second == nil {
				return node.First, panel, sourceStackID, true
			}
			node.Second = second
			return node, panel, sourceStackID, true
		}
		return node, appstate.PanelInstanceState{}, "", false
	default:
		return root, appstate.PanelInstanceState{}, "", false
	}
}

func insertPanel(
	root appstate.DockNode,
	stackID appstate.DockNodeID,
	panel appstate.PanelInstanceState,
	index int,
	activate bool,
) (appstate.DockNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		if node.ID != stackID {
			return node, false
		}
		node.Panels = insertPanelAt(node.Panels, panel.Clone(), index)
		if activate {
			node.Active = panel.ID
		}
		return node, true
	case appstate.SplitNode:
		first, inserted := insertPanel(node.First, stackID, panel, index, activate)
		if inserted {
			node.First = first
			return node, true
		}
		second, inserted := insertPanel(node.Second, stackID, panel, index, activate)
		node.Second = second
		return node, inserted
	default:
		return root, false
	}
}

func replaceStackWithSplit(
	root appstate.DockNode,
	stackID appstate.DockNodeID,
	position DockPosition,
	newStack appstate.TabStackNode,
	newSplitID appstate.DockNodeID,
	newPaneRatio float64,
) (appstate.DockNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		if node.ID != stackID {
			return node, false
		}
		split := appstate.SplitNode{ID: newSplitID}
		switch position {
		case DockLeft:
			split.Orientation = appstate.SplitHorizontal
			split.Ratio = newPaneRatio
			split.First = newStack
			split.Second = node
		case DockRight:
			split.Orientation = appstate.SplitHorizontal
			split.Ratio = ClampSplitRatio(1 - newPaneRatio)
			split.First = node
			split.Second = newStack
		case DockTop:
			split.Orientation = appstate.SplitVertical
			split.Ratio = newPaneRatio
			split.First = newStack
			split.Second = node
		case DockBottom:
			split.Orientation = appstate.SplitVertical
			split.Ratio = ClampSplitRatio(1 - newPaneRatio)
			split.First = node
			split.Second = newStack
		default:
			return node, false
		}
		return split, true
	case appstate.SplitNode:
		first, replaced := replaceStackWithSplit(node.First, stackID, position, newStack, newSplitID, newPaneRatio)
		if replaced {
			node.First = first
			return node, true
		}
		second, replaced := replaceStackWithSplit(node.Second, stackID, position, newStack, newSplitID, newPaneRatio)
		node.Second = second
		return node, replaced
	default:
		return root, false
	}
}

func resizeSplit(root appstate.DockNode, splitID appstate.DockNodeID, ratio float64) (appstate.DockNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return node, false
	case appstate.SplitNode:
		if node.ID == splitID {
			node.Ratio = ratio
			return node, true
		}
		first, found := resizeSplit(node.First, splitID, ratio)
		if found {
			node.First = first
			return node, true
		}
		second, found := resizeSplit(node.Second, splitID, ratio)
		node.Second = second
		return node, found
	default:
		return root, false
	}
}

func collectNodeIDs(root appstate.DockNode) map[appstate.DockNodeID]struct{} {
	ids := make(map[appstate.DockNodeID]struct{})
	var visit func(appstate.DockNode)
	visit = func(current appstate.DockNode) {
		switch node := current.(type) {
		case appstate.TabStackNode:
			ids[node.ID] = struct{}{}
		case appstate.SplitNode:
			ids[node.ID] = struct{}{}
			visit(node.First)
			visit(node.Second)
		}
	}
	visit(root)
	return ids
}

func removePanelAt(panels []appstate.PanelInstanceState, index int) []appstate.PanelInstanceState {
	result := make([]appstate.PanelInstanceState, 0, len(panels)-1)
	for candidateIndex, panel := range panels {
		if candidateIndex != index {
			result = append(result, panel.Clone())
		}
	}
	return result
}

func insertPanelAt(
	panels []appstate.PanelInstanceState,
	panel appstate.PanelInstanceState,
	index int,
) []appstate.PanelInstanceState {
	result := make([]appstate.PanelInstanceState, 0, len(panels)+1)
	for candidateIndex, candidate := range panels {
		if candidateIndex == index {
			result = append(result, panel.Clone())
		}
		result = append(result, candidate.Clone())
	}
	if index == len(panels) {
		result = append(result, panel.Clone())
	}
	return result
}
