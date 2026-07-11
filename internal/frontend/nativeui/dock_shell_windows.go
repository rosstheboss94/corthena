package nativeui

import (
	"fmt"
	"math"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/controls"
	"github.com/rosstheboss94/corthena/internal/frontend/docking"
)

type dockWidgetKind uint8

const (
	dockWidgetTab dockWidgetKind = iota + 1
	dockWidgetClose
	dockWidgetMaximize
	dockWidgetLinkGroup
	dockWidgetSplitter
)

type dockWidgetBinding struct {
	kind    dockWidgetKind
	panelID appstate.PanelID
	stackID appstate.DockNodeID
	splitID appstate.DockNodeID
	tooltip string
}

type dockTabVisual struct {
	panel  appstate.PanelInstanceState
	bounds rectangle
	widget controls.WidgetID
}

type dockStackVisual struct {
	stack          appstate.TabStackNode
	bounds         rectangle
	header         rectangle
	body           rectangle
	tabs           []dockTabVisual
	closeBounds    rectangle
	maximizeBounds rectangle
	linkBounds     rectangle
	closeWidget    controls.WidgetID
	maximizeWidget controls.WidgetID
	linkWidget     controls.WidgetID
}

type dockUIState struct {
	controls   controls.State
	dragPanel  appstate.PanelID
	dragTravel float64
	sequence   uint64
}

func (renderer *shellRenderer) drawDockHost(state appstate.AppState, bounds rectangle) {
	renderer.rect(bounds, tokenBackground)
	layout, ok := activeLayout(state)
	if !ok {
		renderer.emptyDock(bounds, "No layout for workspace")
		return
	}
	host := inset(bounds, renderer.scaleValue(8))
	if host.width <= 0 || host.height <= 0 {
		return
	}
	layout = renderer.responsiveDockLayout(layout, host)

	var visuals []dockStackVisual
	var geometry docking.Geometry
	var err error
	if layout.Maximized != "" {
		visuals, err = renderer.maximizedDockVisual(layout, host)
	} else {
		geometry, err = renderer.calculateDockGeometry(layout.Root, host)
		if err == nil {
			visuals, err = renderer.dockStackVisuals(layout.Root, geometry)
		}
	}
	if err != nil {
		renderer.err = fmt.Errorf("draw dock host: %w", err)
		return
	}

	widgets, bindings := renderer.dockWidgets(state, layout, visuals, geometry)
	input := renderer.dockFrameInput(modalOpen(state))
	result, err := controls.Route(renderer.window.dockUI.controls, input, widgets)
	if err != nil {
		renderer.err = fmt.Errorf("route dock input: %w", err)
		return
	}
	renderer.window.dockUI.controls = result.State
	renderer.reduceDockEvents(layout, visuals, geometry, bindings, result.Events)

	for _, visual := range visuals {
		renderer.drawDockStack(state, layout, visual, result.State)
	}
	if layout.Maximized == "" {
		renderer.drawDockSplitters(geometry, result.State, bindings)
	}
	renderer.drawDockDropTargets(visuals)
	renderer.drawDockTooltip(bindings, result.State.Hot)
}

func (renderer *shellRenderer) responsiveDockLayout(layout appstate.WorkspaceLayout, host rectangle) appstate.WorkspaceLayout {
	if layout.Workspace != appstate.WorkspaceResearch || renderer.scale <= 0 ||
		float64(host.width)/float64(renderer.scale) >= 1100 {
		return layout
	}
	panels := collectDockPanels(layout.Root)
	if len(panels) == 0 {
		return layout
	}
	active := panels[0].ID
	for _, panel := range panels {
		if panel.Type == appstate.PanelOHLCVChart {
			active = panel.ID
			break
		}
	}
	layout.Root = appstate.TabStackNode{
		ID: appstate.DockNodeID("dock-responsive-research"), Active: active, Panels: panels,
	}
	if layout.Maximized != "" {
		layout.Maximized = ""
	}
	return layout
}

func collectDockPanels(root appstate.DockNode) []appstate.PanelInstanceState {
	switch node := root.(type) {
	case appstate.TabStackNode:
		panels := make([]appstate.PanelInstanceState, len(node.Panels))
		for index, panel := range node.Panels {
			panels[index] = panel.Clone()
		}
		return panels
	case appstate.SplitNode:
		first := collectDockPanels(node.First)
		return append(first, collectDockPanels(node.Second)...)
	default:
		return nil
	}
}

func (renderer *shellRenderer) calculateDockGeometry(
	root appstate.DockNode,
	host rectangle,
) (docking.Geometry, error) {
	scale := float64(renderer.scale)
	options := docking.DefaultGeometryOptions()
	options.DPIScale = scale
	return docking.CalculateGeometry(root, docking.Rect{
		X:      float64(host.x) / scale,
		Y:      float64(host.y) / scale,
		Width:  float64(host.width) / scale,
		Height: float64(host.height) / scale,
	}, options)
}

func (renderer *shellRenderer) dockStackVisuals(
	root appstate.DockNode,
	geometry docking.Geometry,
) ([]dockStackVisual, error) {
	visuals := make([]dockStackVisual, 0)
	for _, node := range geometry.Nodes {
		if node.Kind != docking.NodeTabStack {
			continue
		}
		stack, found := dockStackByID(root, node.ID)
		if !found {
			return nil, fmt.Errorf("stack %q is missing from dock tree", node.ID)
		}
		visuals = append(visuals, renderer.makeDockStackVisual(stack, nativeRectangle(node.Bounds.Physical)))
	}
	return visuals, nil
}

func (renderer *shellRenderer) maximizedDockVisual(
	layout appstate.WorkspaceLayout,
	host rectangle,
) ([]dockStackVisual, error) {
	panel, found := dockPanelByID(layout.Root, layout.Maximized)
	if !found {
		return nil, fmt.Errorf("maximized panel %q is missing", layout.Maximized)
	}
	stack := appstate.TabStackNode{
		ID:     appstate.DockNodeID("maximized-" + string(panel.ID)),
		Active: panel.ID,
		Panels: []appstate.PanelInstanceState{panel},
	}
	return []dockStackVisual{renderer.makeDockStackVisual(stack, host)}, nil
}

func (renderer *shellRenderer) makeDockStackVisual(
	stack appstate.TabStackNode,
	bounds rectangle,
) dockStackVisual {
	headerHeight := minFloat32(bounds.height, renderer.scaleValue(panelHeaderSize))
	header := rectangle{x: bounds.x, y: bounds.y, width: bounds.width, height: headerHeight}
	body := rectangle{
		x:      bounds.x + renderer.scaleValue(1),
		y:      bounds.y + headerHeight,
		width:  maxFloat32(0, bounds.width-renderer.scaleValue(2)),
		height: maxFloat32(0, bounds.height-headerHeight-renderer.scaleValue(1)),
	}
	buttonSize := minFloat32(renderer.scaleValue(20), header.height)
	buttonGap := renderer.scaleValue(2)
	right := header.x + header.width - renderer.scaleValue(3)
	closeBounds := rectangle{x: right - buttonSize, y: header.y + renderer.scaleValue(3), width: buttonSize, height: maxFloat32(0, buttonSize-renderer.scaleValue(2))}
	right = closeBounds.x - buttonGap
	maximizeBounds := rectangle{x: right - buttonSize, y: closeBounds.y, width: buttonSize, height: closeBounds.height}
	right = maximizeBounds.x - buttonGap
	linkWidth := minFloat32(renderer.scaleValue(96), maxFloat32(0, header.width*0.28))
	linkBounds := rectangle{x: right - linkWidth, y: closeBounds.y, width: linkWidth, height: closeBounds.height}
	right = linkBounds.x - buttonGap

	tabCount := len(stack.Panels)
	tabs := make([]dockTabVisual, 0, tabCount)
	available := maxFloat32(0, right-header.x-renderer.scaleValue(3))
	tabWidth := renderer.scaleValue(112)
	if tabCount > 0 {
		tabWidth = available / float32(tabCount)
		tabWidth = minFloat32(renderer.scaleValue(132), maxFloat32(renderer.scaleValue(56), tabWidth))
	}
	x := header.x + renderer.scaleValue(3)
	for _, panel := range stack.Panels {
		if x >= right {
			break
		}
		width := minFloat32(tabWidth, maxFloat32(0, right-x))
		tabs = append(tabs, dockTabVisual{
			panel: panel.Clone(),
			bounds: rectangle{
				x: x, y: header.y + renderer.scaleValue(3), width: width, height: maxFloat32(0, header.height-renderer.scaleValue(5)),
			},
		})
		x += tabWidth + renderer.scaleValue(1)
	}
	return dockStackVisual{
		stack:          stack,
		bounds:         bounds,
		header:         header,
		body:           body,
		tabs:           tabs,
		closeBounds:    closeBounds,
		maximizeBounds: maximizeBounds,
		linkBounds:     linkBounds,
	}
}

func (renderer *shellRenderer) dockWidgets(
	state appstate.AppState,
	layout appstate.WorkspaceLayout,
	visuals []dockStackVisual,
	geometry docking.Geometry,
) ([]controls.Widget, map[controls.WidgetID]dockWidgetBinding) {
	rootID := controls.NewWidgetID("dock", string(state.ActiveWorkspace))
	widgets := make([]controls.Widget, 0, len(visuals)*8+len(geometry.Splitters))
	bindings := make(map[controls.WidgetID]dockWidgetBinding)
	for visualIndex := range visuals {
		visual := &visuals[visualIndex]
		stackID := rootID.Child("stack").Child(string(visual.stack.ID))
		headerClip := controls.ClipTo(controlRect(visual.header))
		for tabIndex := range visual.tabs {
			tab := &visual.tabs[tabIndex]
			tab.widget = stackID.Child("tab").Child(string(tab.panel.ID))
			widgets = append(widgets, controls.Widget{
				ID: tab.widget, Bounds: controlRect(tab.bounds), Clip: headerClip,
				Pointer: controls.PointerDrag, Focusable: true,
			})
			bindings[tab.widget] = dockWidgetBinding{
				kind: dockWidgetTab, panelID: tab.panel.ID, stackID: visual.stack.ID,
				tooltip: tab.panel.Title,
			}
		}

		active, found := panelInStack(visual.stack, visual.stack.Active)
		if !found {
			continue
		}
		visual.maximizeWidget = stackID.Child("maximize")
		widgets = append(widgets, controls.Widget{
			ID: visual.maximizeWidget, Bounds: controlRect(visual.maximizeBounds), Clip: headerClip,
			Pointer: controls.PointerAction, Focusable: true,
		})
		maximizeTooltip := "Maximize panel"
		if layout.Maximized != "" {
			maximizeTooltip = "Restore panel"
		}
		bindings[visual.maximizeWidget] = dockWidgetBinding{
			kind: dockWidgetMaximize, panelID: active.ID, stackID: visual.stack.ID, tooltip: maximizeTooltip,
		}

		visual.closeWidget = stackID.Child("close")
		widgets = append(widgets, controls.Widget{
			ID: visual.closeWidget, Bounds: controlRect(visual.closeBounds), Clip: headerClip,
			Pointer: controls.PointerAction, Focusable: true,
		})
		bindings[visual.closeWidget] = dockWidgetBinding{
			kind: dockWidgetClose, panelID: active.ID, stackID: visual.stack.ID, tooltip: "Close panel",
		}

		descriptor, err := appstate.PanelDescriptorFor(active.Type)
		if err == nil && len(descriptor.SupportedLinks) > 0 && len(layout.LinkGroups) > 1 {
			visual.linkWidget = stackID.Child("link-group")
			widgets = append(widgets, controls.Widget{
				ID: visual.linkWidget, Bounds: controlRect(visual.linkBounds), Clip: headerClip,
				Pointer: controls.PointerAction, Focusable: true,
			})
			bindings[visual.linkWidget] = dockWidgetBinding{
				kind: dockWidgetLinkGroup, panelID: active.ID, stackID: visual.stack.ID,
				tooltip: "Change link group",
			}
		}
	}
	for _, splitter := range geometry.Splitters {
		id := rootID.Child("splitter").Child(string(splitter.SplitID))
		widgets = append(widgets, controls.Widget{
			ID: id, Bounds: controlRect(nativeRectangle(splitter.Bounds.Physical)),
			Pointer: controls.PointerDrag,
		})
		bindings[id] = dockWidgetBinding{kind: dockWidgetSplitter, splitID: splitter.SplitID, tooltip: "Resize panels"}
	}
	return widgets, bindings
}

func (renderer *shellRenderer) dockFrameInput(blocked bool) controls.FrameInput {
	focus := controls.FocusUnchanged
	if renderer.input.tabPressed {
		focus = controls.FocusNext
		if renderer.input.shiftDown {
			focus = controls.FocusPrevious
		}
	}
	input := controls.FrameInput{
		Pointer: controls.PointerInput{
			Position: controls.Point{X: float64(renderer.input.mouse.x), Y: float64(renderer.input.mouse.y)},
			Pressed:  renderer.input.leftMousePressed,
			Down:     renderer.input.leftMouseDown,
			Released: renderer.input.leftMouseReleased,
			Movement: controls.Vector{X: float64(renderer.input.mouseDelta.x), Y: float64(renderer.input.mouseDelta.y)},
			Wheel:    controls.Vector{Y: float64(renderer.input.mouseWheel)},
		},
		Keyboard: controls.KeyboardInput{
			Focus: focus, Activate: renderer.input.enterPressed, Cancel: renderer.input.escapePressed,
		},
	}
	if blocked {
		input = controls.FrameInput{Keyboard: controls.KeyboardInput{Cancel: true}}
	}
	return input
}

func (renderer *shellRenderer) reduceDockEvents(
	layout appstate.WorkspaceLayout,
	visuals []dockStackVisual,
	geometry docking.Geometry,
	bindings map[controls.WidgetID]dockWidgetBinding,
	events []controls.Event,
) {
	for _, event := range events {
		binding, found := bindings[event.Widget]
		if !found {
			continue
		}
		switch event.Kind {
		case controls.EventPointerPressed:
			if binding.kind == dockWidgetTab {
				renderer.window.dockUI.dragPanel = binding.panelID
				renderer.window.dockUI.dragTravel = 0
				next, err := docking.ActivatePanel(layout, binding.panelID)
				if err == nil {
					renderer.actions = append(renderer.actions, appstate.ApplyWorkspaceLayoutAction{Layout: next})
				}
			}
		case controls.EventDragMoved:
			if binding.kind == dockWidgetTab {
				renderer.window.dockUI.dragTravel += math.Abs(event.Movement.X) + math.Abs(event.Movement.Y)
				continue
			}
			if binding.kind != dockWidgetSplitter {
				continue
			}
			next, err := resizeLayoutAtPointer(layout, geometry, binding.splitID, event.Position)
			if err == nil {
				renderer.actions = append(renderer.actions, appstate.ApplyWorkspaceLayoutAction{Layout: next})
			}
		case controls.EventActivated:
			renderer.activateDockWidget(layout, binding)
		case controls.EventDragEnded:
			if binding.kind == dockWidgetTab {
				if renderer.window.dockUI.dragTravel >= float64(renderer.scaleValue(6)) {
					renderer.dropDockPanel(layout, visuals, binding.panelID, event.Position)
				}
				renderer.window.dockUI.dragPanel = ""
				renderer.window.dockUI.dragTravel = 0
			}
		case controls.EventDragCanceled, controls.EventCanceled:
			if binding.kind == dockWidgetTab {
				renderer.window.dockUI.dragPanel = ""
				renderer.window.dockUI.dragTravel = 0
			}
		}
	}
}

func (renderer *shellRenderer) activateDockWidget(
	layout appstate.WorkspaceLayout,
	binding dockWidgetBinding,
) {
	var next appstate.WorkspaceLayout
	var err error
	switch binding.kind {
	case dockWidgetTab:
		next, err = docking.ActivatePanel(layout, binding.panelID)
	case dockWidgetClose:
		next, err = docking.ClosePanel(layout, binding.panelID)
	case dockWidgetMaximize:
		if layout.Maximized == "" {
			next, err = docking.MaximizePanel(layout, binding.panelID)
		} else {
			next, err = docking.RestorePanel(layout)
		}
	case dockWidgetLinkGroup:
		groupID, found := nextLinkGroup(layout, binding.panelID)
		if found {
			renderer.actions = append(renderer.actions, appstate.AssignPanelLinkGroupAction{
				Workspace: layout.Workspace, PanelID: binding.panelID, GroupID: groupID,
			})
		}
		return
	default:
		return
	}
	if err == nil {
		renderer.actions = append(renderer.actions, appstate.ApplyWorkspaceLayoutAction{Layout: next})
	}
}

func (renderer *shellRenderer) dropDockPanel(
	layout appstate.WorkspaceLayout,
	visuals []dockStackVisual,
	panelID appstate.PanelID,
	position controls.Point,
) {
	target, found := stackVisualAt(visuals, position)
	if !found || layout.Maximized != "" {
		return
	}
	logicalPoint := docking.Point{X: position.X / float64(renderer.scale), Y: position.Y / float64(renderer.scale)}
	logicalBounds := docking.Rect{
		X: float64(target.bounds.x) / float64(renderer.scale), Y: float64(target.bounds.y) / float64(renderer.scale),
		Width: float64(target.bounds.width) / float64(renderer.scale), Height: float64(target.bounds.height) / float64(renderer.scale),
	}
	dockPosition, found, err := docking.HitTestDropTarget(logicalBounds, logicalPoint)
	if err != nil || !found {
		return
	}
	var next appstate.WorkspaceLayout
	if dockPosition == docking.DockCenter {
		index := dropTabIndex(target, float32(position.X))
		sourceStack, sourceFound := dockStackForPanel(layout.Root, panelID)
		if sourceFound && sourceStack.ID == target.stack.ID {
			if len(sourceStack.Panels) == 0 {
				return
			}
			if index >= len(sourceStack.Panels) {
				index = len(sourceStack.Panels) - 1
			}
			next, err = docking.ReorderPanel(layout, panelID, index)
		} else {
			next, err = docking.MovePanel(layout, panelID, target.stack.ID, len(target.stack.Panels))
		}
	} else {
		stackID, splitID := renderer.nextDockNodeIDs(layout)
		next, err = docking.DockPanel(
			layout, panelID, target.stack.ID, dockPosition, stackID, splitID, 0.35,
		)
	}
	if err == nil {
		renderer.actions = append(renderer.actions, appstate.ApplyWorkspaceLayoutAction{Layout: next})
	}
}

func (renderer *shellRenderer) nextDockNodeIDs(
	layout appstate.WorkspaceLayout,
) (appstate.DockNodeID, appstate.DockNodeID) {
	for {
		renderer.window.dockUI.sequence++
		base := fmt.Sprintf("dock-ui-%06d", renderer.window.dockUI.sequence)
		stackID := appstate.DockNodeID(base + "-stack")
		splitID := appstate.DockNodeID(base + "-split")
		if !dockNodeIDExists(layout.Root, stackID) && !dockNodeIDExists(layout.Root, splitID) {
			return stackID, splitID
		}
	}
}

func (renderer *shellRenderer) drawDockStack(
	state appstate.AppState,
	layout appstate.WorkspaceLayout,
	visual dockStackVisual,
	inputState controls.State,
) {
	if visual.bounds.width <= 0 || visual.bounds.height <= 0 {
		return
	}
	renderer.rect(visual.bounds, tokenPanel)
	renderer.outline(visual.bounds, tokenDivider)
	renderer.rect(visual.header, tokenRaised)
	renderer.line(
		point{x: visual.header.x, y: visual.header.y + visual.header.height - 1},
		point{x: visual.header.x + visual.header.width, y: visual.header.y + visual.header.height - 1},
		1,
		tokenDivider,
	)
	for _, tab := range visual.tabs {
		active := tab.panel.ID == visual.stack.Active
		color := tokenPanel
		if active || inputState.Hot == tab.widget || inputState.Active == tab.widget {
			color = tokenBackground
		}
		renderer.rect(tab.bounds, color)
		if active {
			renderer.rect(rectangle{x: tab.bounds.x, y: tab.bounds.y + tab.bounds.height - renderer.scaleValue(2), width: tab.bounds.width, height: renderer.scaleValue(2)}, linkGroupColor(layout, tab.panel.LinkGroup))
		}
		if inputState.Focused == tab.widget {
			renderer.outline(tab.bounds, tokenCyan)
		}
		renderer.text(renderer.window.interFont, clipText(tab.panel.Title, 16), point{x: tab.bounds.x + renderer.scaleValue(7), y: tab.bounds.y + renderer.scaleValue(5)}, 10, tokenText)
	}

	active, found := panelInStack(visual.stack, visual.stack.Active)
	if !found {
		renderer.emptyDock(visual.body, "No active panel")
		return
	}
	renderer.drawDockLinkGroup(layout, active, visual)
	renderer.iconButton(
		visual.maximizeBounds,
		"[]",
		inputState.Hot == visual.maximizeWidget || inputState.Focused == visual.maximizeWidget,
	)
	renderer.iconButton(
		visual.closeBounds,
		"x",
		inputState.Hot == visual.closeWidget || inputState.Focused == visual.closeWidget,
	)
	renderer.withScissor(visual.body, func() {
		renderer.drawPanelBody(state, active, visual.body)
	})
}

func (renderer *shellRenderer) drawDockLinkGroup(
	layout appstate.WorkspaceLayout,
	panel appstate.PanelInstanceState,
	visual dockStackVisual,
) {
	group, found := linkGroupByID(layout.LinkGroups, panel.LinkGroup)
	if !found || visual.linkBounds.width <= 0 {
		return
	}
	renderer.rect(visual.linkBounds, tokenPanel)
	renderer.outline(visual.linkBounds, tokenDivider)
	color := linkColor(group.Color)
	swatch := renderer.scaleValue(5)
	renderer.rect(rectangle{
		x: visual.linkBounds.x + renderer.scaleValue(5), y: visual.linkBounds.y + (visual.linkBounds.height-swatch)/2,
		width: swatch, height: swatch,
	}, color)
	renderer.text(
		renderer.window.interFont,
		clipText(group.Name, 10),
		point{x: visual.linkBounds.x + renderer.scaleValue(14), y: visual.linkBounds.y + renderer.scaleValue(4)},
		9,
		tokenText,
	)
}

func (renderer *shellRenderer) iconButton(bounds rectangle, label string, hot bool) {
	color := tokenRaised
	if hot {
		color = tokenDivider
	}
	renderer.rect(bounds, color)
	renderer.outline(bounds, tokenDivider)
	renderer.text(renderer.window.monoFont, label, point{x: bounds.x + renderer.scaleValue(5), y: bounds.y + renderer.scaleValue(3)}, 9, tokenText)
}

func (renderer *shellRenderer) drawDockSplitters(
	geometry docking.Geometry,
	state controls.State,
	bindings map[controls.WidgetID]dockWidgetBinding,
) {
	for _, splitter := range geometry.Splitters {
		bounds := nativeRectangle(splitter.Bounds.Physical)
		color := tokenDivider
		for widgetID, binding := range bindings {
			if binding.kind == dockWidgetSplitter && binding.splitID == splitter.SplitID &&
				(state.Hot == widgetID || state.Active == widgetID) {
				color = tokenCyan
				break
			}
		}
		renderer.rect(bounds, color)
	}
}

func (renderer *shellRenderer) drawDockDropTargets(visuals []dockStackVisual) {
	if renderer.window.dockUI.dragPanel == "" ||
		renderer.window.dockUI.dragTravel < float64(renderer.scaleValue(6)) ||
		!renderer.window.dockUI.controls.PointerCapture.Valid() {
		return
	}
	position := controls.Point{X: float64(renderer.input.mouse.x), Y: float64(renderer.input.mouse.y)}
	target, found := stackVisualAt(visuals, position)
	if !found {
		return
	}
	scale := float64(renderer.scale)
	logical := docking.Rect{
		X: float64(target.bounds.x) / scale, Y: float64(target.bounds.y) / scale,
		Width: float64(target.bounds.width) / scale, Height: float64(target.bounds.height) / scale,
	}
	targets, err := docking.DropTargetRects(logical)
	if err != nil {
		return
	}
	logicalPointer := docking.Point{X: position.X / scale, Y: position.Y / scale}
	hot, _, _ := docking.HitTestDropTarget(logical, logicalPointer)
	for _, dropTarget := range targets {
		bounds := nativeRectangle(docking.Rect{
			X: dropTarget.Rect.X * scale, Y: dropTarget.Rect.Y * scale,
			Width: dropTarget.Rect.Width * scale, Height: dropTarget.Rect.Height * scale,
		})
		color := withAlpha(tokenCyan, 48)
		if dropTarget.Position == hot {
			color = withAlpha(tokenCyan, 112)
		}
		renderer.rect(inset(bounds, renderer.scaleValue(2)), color)
		renderer.outline(inset(bounds, renderer.scaleValue(2)), tokenCyan)
	}
}

func (renderer *shellRenderer) drawDockTooltip(
	bindings map[controls.WidgetID]dockWidgetBinding,
	hot controls.WidgetID,
) {
	binding, found := bindings[hot]
	if !found || binding.tooltip == "" || binding.kind == dockWidgetTab {
		return
	}
	width := renderer.scaleValue(float32(len(binding.tooltip))*6 + 16)
	bounds := rectangle{
		x:     minFloat32(renderer.input.mouse.x+renderer.scaleValue(10), float32(renderer.input.width)-width-renderer.scaleValue(4)),
		y:     minFloat32(renderer.input.mouse.y+renderer.scaleValue(12), float32(renderer.input.height)-renderer.scaleValue(26)),
		width: width, height: renderer.scaleValue(22),
	}
	renderer.rect(bounds, tokenBackground)
	renderer.outline(bounds, tokenDivider)
	renderer.text(renderer.window.interFont, binding.tooltip, point{x: bounds.x + renderer.scaleValue(7), y: bounds.y + renderer.scaleValue(5)}, 9, tokenText)
}

func (renderer *shellRenderer) withScissor(bounds rectangle, draw func()) {
	if renderer.err != nil || bounds.width <= 0 || bounds.height <= 0 {
		return
	}
	if err := renderer.window.check("begin Raylib scissor mode"); err != nil {
		renderer.err = err
		return
	}
	renderer.window.backend.beginScissor(bounds)
	draw()
	if err := renderer.window.check("end Raylib scissor mode"); err != nil {
		if renderer.err == nil {
			renderer.err = err
		}
		return
	}
	renderer.window.backend.endScissor()
}

func resizeLayoutAtPointer(
	layout appstate.WorkspaceLayout,
	geometry docking.Geometry,
	splitID appstate.DockNodeID,
	position controls.Point,
) (appstate.WorkspaceLayout, error) {
	node, found := geometry.Node(splitID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("split %q geometry is missing", splitID)
	}
	splitter, found := geometry.Splitter(splitID)
	if !found {
		return appstate.WorkspaceLayout{}, fmt.Errorf("splitter %q geometry is missing", splitID)
	}
	logicalPointer := docking.Point{X: position.X / geometry.DPIScale, Y: position.Y / geometry.DPIScale}
	var ratio float64
	switch splitter.Orientation {
	case appstate.SplitHorizontal:
		available := node.Bounds.Logical.Width - splitter.LogicalThickness
		ratio = (logicalPointer.X - node.Bounds.Logical.X - splitter.LogicalThickness/2) / available
	case appstate.SplitVertical:
		available := node.Bounds.Logical.Height - splitter.LogicalThickness
		ratio = (logicalPointer.Y - node.Bounds.Logical.Y - splitter.LogicalThickness/2) / available
	default:
		return appstate.WorkspaceLayout{}, fmt.Errorf("split %q has invalid orientation", splitID)
	}
	return docking.ResizeSplit(layout, splitID, ratio)
}

func nativeRectangle(rect docking.Rect) rectangle {
	return rectangle{x: float32(rect.X), y: float32(rect.Y), width: float32(rect.Width), height: float32(rect.Height)}
}

func controlRect(rect rectangle) controls.Rect {
	return controls.Rect{X: float64(rect.x), Y: float64(rect.y), Width: float64(rect.width), Height: float64(rect.height)}
}

func dockStackByID(root appstate.DockNode, stackID appstate.DockNodeID) (appstate.TabStackNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return node, node.ID == stackID
	case appstate.SplitNode:
		if stack, found := dockStackByID(node.First, stackID); found {
			return stack, true
		}
		return dockStackByID(node.Second, stackID)
	default:
		return appstate.TabStackNode{}, false
	}
}

func dockStackForPanel(root appstate.DockNode, panelID appstate.PanelID) (appstate.TabStackNode, bool) {
	switch node := root.(type) {
	case appstate.TabStackNode:
		for _, panel := range node.Panels {
			if panel.ID == panelID {
				return node, true
			}
		}
	case appstate.SplitNode:
		if stack, found := dockStackForPanel(node.First, panelID); found {
			return stack, true
		}
		return dockStackForPanel(node.Second, panelID)
	}
	return appstate.TabStackNode{}, false
}

func dockPanelByID(root appstate.DockNode, panelID appstate.PanelID) (appstate.PanelInstanceState, bool) {
	stack, found := dockStackForPanel(root, panelID)
	if !found {
		return appstate.PanelInstanceState{}, false
	}
	return panelInStack(stack, panelID)
}

func panelInStack(stack appstate.TabStackNode, panelID appstate.PanelID) (appstate.PanelInstanceState, bool) {
	for _, panel := range stack.Panels {
		if panel.ID == panelID {
			return panel.Clone(), true
		}
	}
	return appstate.PanelInstanceState{}, false
}

func stackVisualAt(visuals []dockStackVisual, point controls.Point) (dockStackVisual, bool) {
	for _, visual := range visuals {
		if controlRect(visual.bounds).Contains(point) {
			return visual, true
		}
	}
	return dockStackVisual{}, false
}

func dropTabIndex(visual dockStackVisual, x float32) int {
	for index, tab := range visual.tabs {
		if x < tab.bounds.x+tab.bounds.width/2 {
			return index
		}
	}
	return len(visual.stack.Panels)
}

func dockNodeIDExists(root appstate.DockNode, id appstate.DockNodeID) bool {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return node.ID == id
	case appstate.SplitNode:
		return node.ID == id || dockNodeIDExists(node.First, id) || dockNodeIDExists(node.Second, id)
	default:
		return false
	}
}

func nextLinkGroup(layout appstate.WorkspaceLayout, panelID appstate.PanelID) (appstate.LinkGroupID, bool) {
	panel, found := dockPanelByID(layout.Root, panelID)
	if !found || len(layout.LinkGroups) < 2 {
		return "", false
	}
	for index, group := range layout.LinkGroups {
		if group.ID == panel.LinkGroup {
			return layout.LinkGroups[(index+1)%len(layout.LinkGroups)].ID, true
		}
	}
	return layout.LinkGroups[0].ID, true
}

func linkGroupByID(groups []appstate.LinkGroup, id appstate.LinkGroupID) (appstate.LinkGroup, bool) {
	for _, group := range groups {
		if group.ID == id {
			return group.Clone(), true
		}
	}
	return appstate.LinkGroup{}, false
}

func linkGroupColor(layout appstate.WorkspaceLayout, id appstate.LinkGroupID) colorValue {
	group, found := linkGroupByID(layout.LinkGroups, id)
	if !found {
		return tokenMuted
	}
	return linkColor(group.Color)
}

func linkColor(color appstate.LinkGroupColor) colorValue {
	switch color {
	case appstate.LinkGroupCyan:
		return tokenCyan
	case appstate.LinkGroupPurple:
		return tokenPurple
	case appstate.LinkGroupPositive:
		return tokenPositive
	case appstate.LinkGroupWarning:
		return tokenWarning
	default:
		return tokenMuted
	}
}

func activateDockLayout(
	layout appstate.WorkspaceLayout,
	panelID appstate.PanelID,
) (appstate.WorkspaceLayout, bool) {
	next, err := docking.ActivatePanel(layout, panelID)
	return next, err == nil
}
