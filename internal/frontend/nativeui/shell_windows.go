package nativeui

import (
	"fmt"
	"strings"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/chart"
	virtualtable "github.com/rosstheboss94/corthena/internal/frontend/table"
)

const (
	topNavHeight     = float32(36)
	contextBarHeight = float32(40)
	panelHeaderSize  = float32(32)
	statusBarHeight  = float32(26)
	gridUnit         = float32(4)
	fontCaption      = float32(12)
	fontBody         = float32(13)
	fontControl      = float32(14)
	fontHeading      = float32(18)
)

var (
	tokenBackground = rgb(0x0B, 0x0D, 0x10)
	tokenPanel      = rgb(0x11, 0x15, 0x1A)
	tokenRaised     = rgb(0x17, 0x1C, 0x22)
	tokenDivider    = rgb(0x25, 0x2B, 0x33)
	tokenText       = rgb(0xD6, 0xDC, 0xE5)
	tokenMuted      = rgb(0x7E, 0x88, 0x96)
	tokenCyan       = rgb(0x3C, 0xC8, 0xC8)
	tokenPurple     = rgb(0x9B, 0x7C, 0xF6)
	tokenPositive   = rgb(0x4C, 0xC3, 0x8A)
	tokenNegative   = rgb(0xEF, 0x6B, 0x73)
	tokenWarning    = rgb(0xD8, 0xB4, 0x5A)
)

type shellInput struct {
	width                int32
	height               int32
	fps                  int32
	dpi                  point
	mouse                point
	mouseDelta           point
	leftMousePressed     bool
	leftMouseDown        bool
	leftMouseReleased    bool
	mouseWheel           float32
	openCommandPressed   bool
	openSettingsPressed  bool
	increaseScalePressed bool
	decreaseScalePressed bool
	resetScalePressed    bool
	escapePressed        bool
	enterPressed         bool
	tabPressed           bool
	shiftDown            bool
	upPressed            bool
	downPressed          bool
}

type shellRenderer struct {
	window  *Window
	input   shellInput
	scale   float32
	actions []appstate.UIAction
	err     error
}

type shellCommand struct {
	title   string
	detail  string
	enabled bool
	action  appstate.UIAction
}

// DrawShellFrame draws one application-shell frame and returns typed
// user actions for the workstation render loop to reduce on the UI thread.
func (window *Window) DrawShellFrame(state appstate.AppState) ([]appstate.UIAction, error) {
	if err := window.requireOpen("draw shell frame"); err != nil {
		return nil, err
	}
	input, err := window.readShellInput()
	if err != nil {
		return nil, err
	}
	if input.width <= 0 || input.height <= 0 {
		return nil, fmt.Errorf("draw shell frame: invalid screen size %dx%d", input.width, input.height)
	}
	renderer := &shellRenderer{
		window: window,
		input:  input,
		scale:  shellScale(input.dpi, state.Preferences.UIScale),
	}
	if input.openCommandPressed && !state.Overlays.CommandPaletteOpen && state.Overlays.CriticalError.Message == "" {
		renderer.actions = append(renderer.actions, appstate.SetCommandPaletteAction{Open: true})
	}
	if input.openSettingsPressed && !state.Overlays.SettingsOpen && state.Overlays.CriticalError.Message == "" {
		renderer.actions = append(renderer.actions, appstate.SetSettingsOpenAction{Open: true})
	}
	if input.increaseScalePressed {
		renderer.actions = append(renderer.actions, appstate.SetUIScaleAction{Scale: appstate.StepUIScale(state.Preferences.UIScale, 1)})
	}
	if input.decreaseScalePressed {
		renderer.actions = append(renderer.actions, appstate.SetUIScaleAction{Scale: appstate.StepUIScale(state.Preferences.UIScale, -1)})
	}
	if input.resetScalePressed {
		renderer.actions = append(renderer.actions, appstate.SetUIScaleAction{Scale: appstate.DefaultUIScale})
	}

	if err := window.check("begin Raylib drawing"); err != nil {
		return nil, err
	}
	window.backend.beginDrawing()

	if err := window.check("clear Raylib background"); err != nil {
		return renderer.actions, err
	}
	window.backend.clearBackground(tokenBackground)

	renderer.drawApplicationShell(state)
	drawErr := renderer.err
	if err := window.check("end Raylib drawing"); err != nil {
		return renderer.actions, err
	}
	window.backend.endDrawing()
	if drawErr != nil {
		return renderer.actions, drawErr
	}
	return renderer.actions, nil
}

func (window *Window) readShellInput() (shellInput, error) {
	if err := window.check("read Raylib screen width"); err != nil {
		return shellInput{}, err
	}
	width := window.backend.screenWidth()
	if err := window.check("read Raylib screen height"); err != nil {
		return shellInput{}, err
	}
	height := window.backend.screenHeight()
	if err := window.check("read Raylib frame rate"); err != nil {
		return shellInput{}, err
	}
	fps := window.backend.fps()
	if window.fixedFPS > 0 {
		fps = window.fixedFPS
	}
	if err := window.check("read Raylib window DPI scale"); err != nil {
		return shellInput{}, err
	}
	dpi := window.backend.windowScaleDPI()
	if err := window.check("read Raylib mouse position"); err != nil {
		return shellInput{}, err
	}
	mouse := window.backend.mousePosition()
	if err := window.check("read Raylib mouse delta"); err != nil {
		return shellInput{}, err
	}
	mouseDelta := window.backend.mouseDelta()
	if err := window.check("read Raylib mouse button"); err != nil {
		return shellInput{}, err
	}
	leftMousePressed := window.backend.leftMousePressed()
	if err := window.check("read Raylib held mouse button"); err != nil {
		return shellInput{}, err
	}
	leftMouseDown := window.backend.leftMouseDown()
	if err := window.check("read Raylib released mouse button"); err != nil {
		return shellInput{}, err
	}
	leftMouseReleased := window.backend.leftMouseReleased()
	if err := window.check("read Raylib mouse wheel"); err != nil {
		return shellInput{}, err
	}
	mouseWheel := window.backend.mouseWheelMove()
	if err := window.check("read Raylib command shortcut"); err != nil {
		return shellInput{}, err
	}
	openCommandPressed := window.backend.openCommandPalettePressed()
	if err := window.check("read Raylib settings shortcut"); err != nil {
		return shellInput{}, err
	}
	openSettingsPressed := window.backend.openSettingsPressed()
	if err := window.check("read Raylib increase UI scale shortcut"); err != nil {
		return shellInput{}, err
	}
	increaseScalePressed := window.backend.increaseUIScalePressed()
	if err := window.check("read Raylib decrease UI scale shortcut"); err != nil {
		return shellInput{}, err
	}
	decreaseScalePressed := window.backend.decreaseUIScalePressed()
	if err := window.check("read Raylib reset UI scale shortcut"); err != nil {
		return shellInput{}, err
	}
	resetScalePressed := window.backend.resetUIScalePressed()
	if err := window.check("read Raylib escape key"); err != nil {
		return shellInput{}, err
	}
	escapePressed := window.backend.escapePressed()
	if err := window.check("read Raylib enter key"); err != nil {
		return shellInput{}, err
	}
	enterPressed := window.backend.enterPressed()
	if err := window.check("read Raylib tab key"); err != nil {
		return shellInput{}, err
	}
	tabPressed := window.backend.tabPressed()
	if err := window.check("read Raylib shift key"); err != nil {
		return shellInput{}, err
	}
	shiftDown := window.backend.shiftDown()
	if err := window.check("read Raylib up key"); err != nil {
		return shellInput{}, err
	}
	upPressed := window.backend.upPressed()
	if err := window.check("read Raylib down key"); err != nil {
		return shellInput{}, err
	}
	downPressed := window.backend.downPressed()
	return shellInput{
		width:                width,
		height:               height,
		fps:                  fps,
		dpi:                  dpi,
		mouse:                mouse,
		mouseDelta:           mouseDelta,
		leftMousePressed:     leftMousePressed,
		leftMouseDown:        leftMouseDown,
		leftMouseReleased:    leftMouseReleased,
		mouseWheel:           mouseWheel,
		openCommandPressed:   openCommandPressed,
		openSettingsPressed:  openSettingsPressed,
		increaseScalePressed: increaseScalePressed,
		decreaseScalePressed: decreaseScalePressed,
		resetScalePressed:    resetScalePressed,
		escapePressed:        escapePressed,
		enterPressed:         enterPressed,
		tabPressed:           tabPressed,
		shiftDown:            shiftDown,
		upPressed:            upPressed,
		downPressed:          downPressed,
	}, nil
}

func shellScale(dpi point, preset appstate.UIScalePreset) float32 {
	scale := maxFloat32(dpi.x, dpi.y)
	if scale != scale || scale < 1 {
		scale = 1
	}
	if !preset.Valid() {
		preset = appstate.DefaultUIScale
	}
	scale *= float32(preset) / 100
	if scale > 2 {
		return 2
	}
	if scale < 1 {
		return 1
	}
	return scale
}

func (renderer *shellRenderer) drawApplicationShell(state appstate.AppState) {
	width := float32(renderer.input.width)
	height := float32(renderer.input.height)
	topHeight := renderer.scaleValue(topNavHeight + contextBarHeight)
	statusHeight := renderer.scaleValue(statusBarHeight)
	content := rectangle{
		x:      0,
		y:      topHeight,
		width:  width,
		height: maxFloat32(0, height-topHeight-statusHeight),
	}

	renderer.drawTopNavigation(state)
	renderer.drawContextBar(state)
	renderer.drawContent(state, content)
	renderer.drawStatusBar(state)
	renderer.drawToasts(state)
	renderer.drawModalLayer(state)
}

func (renderer *shellRenderer) drawTopNavigation(state appstate.AppState) {
	width := float32(renderer.input.width)
	height := renderer.scaleValue(topNavHeight)
	compact := width < renderer.scaleValue(1500)
	bounds := rectangle{x: 0, y: 0, width: width, height: height}
	renderer.rect(bounds, tokenPanel)
	renderer.line(point{x: 0, y: height - 1}, point{x: width, y: height - 1}, 1, tokenDivider)
	brand := "Corthena"
	start := float32(116)
	if compact {
		brand = "C"
		start = 56
	}
	renderer.text(renderer.window.interFont, brand, point{x: renderer.scaleValue(12), y: renderer.scaleValue(8)}, 14, tokenText)

	x := renderer.scaleValue(start)
	for _, workspace := range appstate.Workspaces() {
		label := workspaceTitle(workspace)
		tabWidth := renderer.tabWidth(label)
		if compact {
			label = compactWorkspaceTitle(workspace)
			tabWidth = renderer.scaleValue(44)
		}
		tab := rectangle{
			x:      x,
			y:      renderer.scaleValue(4),
			width:  tabWidth,
			height: renderer.scaleValue(24),
		}
		active := state.ActiveWorkspace == workspace
		if renderer.navButton(tab, label, active, tokenCyan) && !modalOpen(state) {
			renderer.actions = append(renderer.actions, appstate.SelectWorkspaceAction{Workspace: workspace})
		}
		x += tabWidth + renderer.scaleValue(4)
	}

	right := width - renderer.scaleValue(12)
	settingsLabel := "Settings"
	settingsWidth := renderer.scaleValue(92)
	paletteLabel := "Ctrl+K Command"
	paletteWidth := renderer.scaleValue(132)
	if compact {
		settingsLabel = "Set"
		settingsWidth = renderer.scaleValue(52)
		paletteLabel = "Cmd"
		paletteWidth = renderer.scaleValue(52)
	}
	right -= settingsWidth
	settingsBounds := rectangle{x: right, y: renderer.scaleValue(4), width: settingsWidth, height: renderer.scaleValue(28)}
	if renderer.navButton(settingsBounds, settingsLabel, state.Overlays.SettingsOpen, tokenCyan) && !modalOpen(state) {
		renderer.actions = append(renderer.actions, appstate.SetSettingsOpenAction{Open: true})
	}
	right -= renderer.scaleValue(8)
	right -= paletteWidth
	if renderer.navButton(
		rectangle{x: right, y: renderer.scaleValue(4), width: paletteWidth, height: renderer.scaleValue(28)},
		paletteLabel,
		state.Overlays.CommandPaletteOpen,
		tokenPurple,
	) && !modalOpen(state) {
		renderer.actions = append(renderer.actions, appstate.SetCommandPaletteAction{Open: true})
	}

	detailRight := right - renderer.scaleValue(8)
	detailWidth := renderer.scaleValue(86 + 8 + 154)
	if !compact && detailRight-detailWidth > x+renderer.scaleValue(8) {
		jobWidth := renderer.scaleValue(86)
		detailRight -= jobWidth
		renderer.statusChip(rectangle{x: detailRight, y: renderer.scaleValue(6), width: jobWidth, height: renderer.scaleValue(24)}, fmt.Sprintf("%d active", activeJobCount(state.Jobs)), tokenPurple)
		detailRight -= renderer.scaleValue(8)
		connWidth := renderer.scaleValue(154)
		detailRight -= connWidth
		renderer.statusChip(rectangle{x: detailRight, y: renderer.scaleValue(6), width: connWidth, height: renderer.scaleValue(24)}, connectionLabel(state.Connection), connectionColor(state.Connection.State))
	}
}

func (renderer *shellRenderer) drawContextBar(state appstate.AppState) {
	y := renderer.scaleValue(topNavHeight)
	width := float32(renderer.input.width)
	height := renderer.scaleValue(contextBarHeight)
	renderer.rect(rectangle{x: 0, y: y, width: width, height: height}, tokenBackground)
	renderer.line(point{x: 0, y: y + height - 1}, point{x: width, y: y + height - 1}, 1, tokenDivider)

	dataset := selectedDatasetName(state)
	symbols := symbolsLabel(state.LinkContext.Symbols)
	interval := string(state.LinkContext.Interval)
	if interval == "" {
		interval = "--"
	}
	rangeLabel := "--"
	if !state.LinkContext.TimeRange.Start.IsZero() && !state.LinkContext.TimeRange.End.IsZero() {
		rangeLabel = state.LinkContext.TimeRange.Start.Format("2006-01-02") + " to " +
			state.LinkContext.TimeRange.End.Format("2006-01-02")
	}

	x := renderer.scaleValue(12)
	datasetX := x
	x = renderer.contextItem(x, y, "Dataset", clipText(dataset, 28), tokenCyan)
	datasetBounds := rectangle{x: datasetX, y: y, width: x - datasetX, height: height}
	symbolX := x
	x = renderer.contextItem(x, y, "Symbols", clipText(symbols, 24), tokenPurple)
	symbolBounds := rectangle{x: symbolX, y: y, width: x - symbolX, height: height}
	intervalX := x
	x = renderer.contextItem(x, y, "Interval", interval, tokenText)
	intervalBounds := rectangle{x: intervalX, y: y, width: x - intervalX, height: height}
	if width >= renderer.scaleValue(1100) {
		renderer.contextItem(x, y, "Range", rangeLabel, tokenText)
	}
	if renderer.input.leftMousePressed {
		switch {
		case pointInRectangle(renderer.input.mouse, datasetBounds):
			if context, found := nextDatasetContext(state); found {
				renderer.actions = append(renderer.actions, appstate.SetLinkContextAction{Context: context})
			}
		case pointInRectangle(renderer.input.mouse, symbolBounds):
			if context, found := nextSymbolContext(state); found {
				renderer.actions = append(renderer.actions, appstate.SetLinkContextAction{Context: context})
			}
		case pointInRectangle(renderer.input.mouse, intervalBounds):
			if context, found := nextIntervalContext(state); found {
				renderer.actions = append(renderer.actions, appstate.SetLinkContextAction{Context: context})
			}
		}
	}
}

func nextDatasetContext(state appstate.AppState) (appstate.LinkContext, bool) {
	if len(state.Datasets) == 0 {
		return appstate.LinkContext{}, false
	}
	index := 0
	for candidate, dataset := range state.Datasets {
		if dataset.ID == state.LinkContext.DatasetID {
			index = (candidate + 1) % len(state.Datasets)
			break
		}
	}
	return contextForDataset(state.LinkContext, state.Datasets[index]), true
}

func nextIntervalContext(state appstate.AppState) (appstate.LinkContext, bool) {
	for _, dataset := range state.Datasets {
		if dataset.Interval != state.LinkContext.Interval {
			return contextForDataset(state.LinkContext, dataset), true
		}
	}
	return appstate.LinkContext{}, false
}

func nextSymbolContext(state appstate.AppState) (appstate.LinkContext, bool) {
	for _, dataset := range state.Datasets {
		if dataset.ID != state.LinkContext.DatasetID || len(dataset.Symbols) == 0 {
			continue
		}
		context := state.LinkContext.Clone()
		if len(context.Symbols) != 1 {
			context.Symbols = []appstate.Symbol{dataset.Symbols[0]}
			return context, true
		}
		for index, symbol := range dataset.Symbols {
			if symbol != context.Symbols[0] {
				continue
			}
			if index+1 < len(dataset.Symbols) {
				context.Symbols = []appstate.Symbol{dataset.Symbols[index+1]}
			} else {
				context.Symbols = append([]appstate.Symbol(nil), dataset.Symbols...)
			}
			return context, true
		}
		context.Symbols = []appstate.Symbol{dataset.Symbols[0]}
		return context, true
	}
	return appstate.LinkContext{}, false
}

func contextForDataset(current appstate.LinkContext, dataset appstate.DatasetSummary) appstate.LinkContext {
	current.DatasetID = dataset.ID
	current.Symbols = append([]appstate.Symbol(nil), dataset.Symbols...)
	current.Interval = dataset.Interval
	current.TimeRange = appstate.TimeRange{Start: dataset.Start, End: dataset.End}
	return current
}

func (renderer *shellRenderer) contextItem(x float32, y float32, label string, value string, accent colorValue) float32 {
	top := y + renderer.scaleValue(6)
	renderer.text(renderer.window.interFont, label, point{x: x, y: top + renderer.scaleValue(1)}, 11, tokenMuted)
	valueX := x + renderer.scaleValue(72)
	renderer.text(renderer.window.interFont, value, point{x: valueX, y: top}, 12, accent)
	return valueX + renderer.scaleValue(float32(len(value))*7+32)
}

func (renderer *shellRenderer) drawContent(state appstate.AppState, content rectangle) {
	leftWidth := renderer.scaleValue(260)
	if content.width < renderer.scaleValue(1100) {
		leftWidth = renderer.scaleValue(218)
	}
	left := rectangle{x: 0, y: content.y, width: leftWidth, height: content.height}
	main := rectangle{
		x:      leftWidth + renderer.scaleValue(1),
		y:      content.y,
		width:  maxFloat32(0, content.width-leftWidth-renderer.scaleValue(1)),
		height: content.height,
	}
	renderer.drawLeftRail(state, left)
	renderer.drawDockHost(state, main)
}

func (renderer *shellRenderer) drawLeftRail(state appstate.AppState, bounds rectangle) {
	renderer.rect(bounds, tokenPanel)
	renderer.line(point{x: bounds.x + bounds.width - 1, y: bounds.y}, point{x: bounds.x + bounds.width - 1, y: bounds.y + bounds.height}, 1, tokenDivider)
	padding := renderer.scaleValue(10)
	y := bounds.y + renderer.scaleValue(10)
	renderer.sectionTitle(bounds.x+padding, y, "Workspace Panels")
	y += renderer.scaleValue(24)

	layout, ok := activeLayout(state)
	if ok {
		panels := flattenPanels(layout.Root)
		active := activePanelID(layout.Root)
		for index, panel := range panels {
			if index >= 9 {
				break
			}
			row := rectangle{
				x:      bounds.x + padding,
				y:      y,
				width:  bounds.width - 2*padding,
				height: renderer.scaleValue(22),
			}
			if panel.ID == active {
				renderer.rect(row, tokenRaised)
				renderer.rect(rectangle{x: row.x, y: row.y, width: renderer.scaleValue(2), height: row.height}, tokenCyan)
			}
			if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) &&
				!modalOpen(state) {
				if next, activated := activateDockLayout(layout, panel.ID); activated {
					renderer.actions = append(renderer.actions, appstate.ApplyWorkspaceLayoutAction{Layout: next})
				}
			}
			renderer.text(renderer.window.interFont, clipText(panel.Title, 24), point{x: row.x + renderer.scaleValue(8), y: row.y + renderer.scaleValue(5)}, 11, tokenText)
			y += renderer.scaleValue(24)
		}
		if len(layout.HiddenPanels) > 0 && y < bounds.y+bounds.height-renderer.scaleValue(230) {
			y += renderer.scaleValue(8)
			renderer.sectionTitle(bounds.x+padding, y, "Hidden Panels")
			y += renderer.scaleValue(24)
			for index, panel := range layout.HiddenPanels {
				if index >= 4 || y >= bounds.y+bounds.height-renderer.scaleValue(210) {
					break
				}
				row := rectangle{
					x: bounds.x + padding, y: y,
					width: bounds.width - 2*padding, height: renderer.scaleValue(22),
				}
				if pointInRectangle(renderer.input.mouse, row) {
					renderer.rect(row, tokenRaised)
				}
				if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) &&
					!modalOpen(state) {
					renderer.actions = append(renderer.actions, appstate.OpenPanelAction{
						Workspace: layout.Workspace,
						Panel:     panel.Clone(),
					})
				}
				renderer.text(renderer.window.interFont, "+ "+clipText(panel.Title, 22), point{x: row.x + renderer.scaleValue(8), y: row.y + renderer.scaleValue(5)}, 11, tokenMuted)
				y += renderer.scaleValue(24)
			}
		}
	}

	bottom := bounds.y + bounds.height
	globalHeight := renderer.scaleValue(104)
	globalY := bottom - globalHeight
	showGlobal := y+renderer.scaleValue(10) <= globalY
	componentBottom := globalY
	if !showGlobal {
		componentBottom = bottom
	}
	componentY := y + renderer.scaleValue(10)
	if componentY+renderer.scaleValue(54) <= componentBottom {
		renderer.sectionTitle(bounds.x+padding, componentY, "Component Status")
		componentY += renderer.scaleValue(24)
		for _, component := range state.Components {
			if componentY+renderer.scaleValue(30) > componentBottom {
				break
			}
			renderer.componentRow(bounds.x+padding, componentY, bounds.width-2*padding, component)
			componentY += renderer.scaleValue(30)
		}
	}
	if showGlobal {
		y = globalY
		renderer.sectionTitle(bounds.x+padding, y, "Global Context")
		y += renderer.scaleValue(24)
		renderer.smallLine(bounds.x+padding, y, "Dataset", string(state.LinkContext.DatasetID))
		y += renderer.scaleValue(20)
		renderer.smallLine(bounds.x+padding, y, "Run", string(state.LinkContext.RunID))
		y += renderer.scaleValue(20)
		renderer.smallLine(bounds.x+padding, y, "Model", string(state.LinkContext.ModelID))
	}
}

func (renderer *shellRenderer) drawPanelBody(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	if panel.ID == "" {
		renderer.emptyDock(bounds, "No active panel")
		return
	}
	title := panel.Title
	if title == "" {
		title = string(panel.Type)
	}
	renderer.text(renderer.window.interFont, title, point{x: bounds.x + renderer.scaleValue(14), y: bounds.y + renderer.scaleValue(12)}, 16, tokenText)
	renderer.text(renderer.window.interFont, "Deterministic demo data", point{x: bounds.x + renderer.scaleValue(14), y: bounds.y + renderer.scaleValue(34)}, 11, tokenMuted)

	content := rectangle{
		x:      bounds.x + renderer.scaleValue(14),
		y:      bounds.y + renderer.scaleValue(58),
		width:  bounds.width - renderer.scaleValue(28),
		height: bounds.height - renderer.scaleValue(72),
	}
	if isResearchPanel(panel.Type) {
		renderer.drawResearchPanel(state, panel, content)
		return
	}
	if isDataPanel(panel.Type) {
		renderer.drawDataPanel(state, panel, content)
		return
	}
	if isExperimentPanel(panel.Type) {
		renderer.drawExperimentPanel(state, panel, content)
		return
	}
	if isJobsPanel(panel.Type) {
		renderer.drawJobsPanel(state, panel, content)
		return
	}
	if isResultsPanel(panel.Type) {
		renderer.drawResultsPanel(state, panel, content)
		return
	}
	if isModelsPanel(panel.Type) {
		renderer.drawModelsPanel(state, panel, content)
		return
	}
	if isInferencePanel(panel.Type) {
		renderer.drawInferencePanel(state, panel, content)
		return
	}
	switch state.ActiveWorkspace {
	case appstate.WorkspaceData:
		renderer.drawDatasetRows(state, content)
	case appstate.WorkspaceResearch:
		renderer.drawResearchPreview(state, content)
	case appstate.WorkspaceExperiments:
		renderer.drawExperimentPreview(state, content)
	case appstate.WorkspaceJobs:
		renderer.drawJobPreview(state, content)
	case appstate.WorkspaceResults:
		renderer.drawResultPreview(state, content)
	case appstate.WorkspaceModels:
		renderer.drawModelPreview(state, content)
	case appstate.WorkspaceInference:
		renderer.drawInferencePreview(state, content)
	default:
		renderer.emptyDock(content, "Unknown workspace")
	}
}

func (renderer *shellRenderer) drawDatasetRows(state appstate.AppState, bounds rectangle) {
	if len(state.Datasets) == 0 {
		renderer.emptyDock(bounds, "Waiting for dataset snapshot")
		return
	}
	columns := []virtualtable.Column{
		{ID: "dataset", Title: "Dataset", Kind: virtualtable.CellString, Width: float64(bounds.width) * 0.46, MinWidth: 80, MaxWidth: 1200, Pinned: true, Sortable: true},
		{ID: "status", Title: "Status", Kind: virtualtable.CellString, Width: float64(bounds.width) * 0.20, MinWidth: 70, MaxWidth: 400, Sortable: true},
		{ID: "rows", Title: "Rows", Kind: virtualtable.CellInteger, Width: float64(bounds.width) * 0.18, MinWidth: 60, MaxWidth: 400, Sortable: true},
		{ID: "revision", Title: "Revision", Kind: virtualtable.CellInteger, Width: float64(bounds.width) * 0.16, MinWidth: 60, MaxWidth: 300, Sortable: true},
	}
	rows := make([]virtualtable.Row, len(state.Datasets))
	for index, dataset := range state.Datasets {
		rows[index] = virtualtable.Row{
			ID: virtualtable.RowID(dataset.ID), SourceIndex: uint64(index + 1),
			Cells: []virtualtable.Cell{
				{Kind: virtualtable.CellString, String: dataset.Name},
				{Kind: virtualtable.CellString, String: string(dataset.Status)},
				{Kind: virtualtable.CellInteger, Integer: int64(dataset.Rows)},
				{Kind: virtualtable.CellInteger, Integer: int64(dataset.Revision)},
			},
		}
	}
	model, err := virtualtable.NewModel(virtualtable.Dataset{Columns: columns, Rows: rows})
	if err != nil {
		renderer.emptyDock(bounds, "Invalid dataset table")
		return
	}
	window, err := model.Virtualize(virtualtable.WindowRequest{
		OriginX: float64(bounds.x), OriginY: float64(bounds.y), Width: float64(bounds.width), Height: float64(bounds.height),
		HeaderHeight: float64(renderer.scaleValue(24)), RowHeight: float64(renderer.scaleValue(24)), OverscanRows: 1, OverscanColumns: 1,
	})
	if err != nil {
		renderer.emptyDock(bounds, "Invalid table viewport")
		return
	}
	renderer.drawTableWindow(window, virtualtable.Selection{})
}

func (renderer *shellRenderer) drawResearchPreview(state appstate.AppState, bounds rectangle) {
	renderer.rect(bounds, tokenBackground)
	renderer.outline(bounds, tokenDivider)
	gridRows := 6
	for index := 1; index < gridRows; index++ {
		y := bounds.y + bounds.height*float32(index)/float32(gridRows)
		renderer.line(point{x: bounds.x, y: y}, point{x: bounds.x + bounds.width, y: y}, 1, withAlpha(tokenDivider, 160))
	}
	gridCols := 8
	for index := 1; index < gridCols; index++ {
		x := bounds.x + bounds.width*float32(index)/float32(gridCols)
		renderer.line(point{x: x, y: bounds.y}, point{x: x, y: bounds.y + bounds.height}, 1, withAlpha(tokenDivider, 120))
	}
	points := []point{
		{x: bounds.x + bounds.width*0.04, y: bounds.y + bounds.height*0.70},
		{x: bounds.x + bounds.width*0.16, y: bounds.y + bounds.height*0.62},
		{x: bounds.x + bounds.width*0.28, y: bounds.y + bounds.height*0.68},
		{x: bounds.x + bounds.width*0.40, y: bounds.y + bounds.height*0.48},
		{x: bounds.x + bounds.width*0.54, y: bounds.y + bounds.height*0.52},
		{x: bounds.x + bounds.width*0.68, y: bounds.y + bounds.height*0.36},
		{x: bounds.x + bounds.width*0.82, y: bounds.y + bounds.height*0.42},
		{x: bounds.x + bounds.width*0.96, y: bounds.y + bounds.height*0.28},
	}
	segments := make([]chart.Segment32, 0, len(points)-1)
	for index := 0; index+1 < len(points); index++ {
		segments = append(segments, chart.Segment32{
			Start: chart.Point32{X: points[index].x, Y: points[index].y},
			End:   chart.Point32{X: points[index+1].x, Y: points[index+1].y},
			Style: chart.StylePrimary,
		})
	}
	renderer.drawChartFrame(chart.Frame{Layers: []chart.PreparedLayer{{ID: "research-preview", Kind: chart.LayerLine, Segments: segments}}})
	renderer.text(renderer.window.monoFont, symbolsLabel(state.LinkContext.Symbols), point{x: bounds.x + renderer.scaleValue(10), y: bounds.y + renderer.scaleValue(10)}, 12, tokenMuted)
}

func (renderer *shellRenderer) drawExperimentPreview(state appstate.AppState, bounds rectangle) {
	lines := []string{
		"Experiment editor scaffold",
		"Dataset: " + string(state.LinkContext.DatasetID),
		"Target: next-bar log return",
		"Model: deterministic random forest",
		"Validation: coordinator unavailable commands disabled until backend swap",
	}
	renderer.textBlock(bounds, lines)
}

func (renderer *shellRenderer) drawJobPreview(state appstate.AppState, bounds rectangle) {
	if len(state.Jobs) == 0 {
		renderer.emptyDock(bounds, "Waiting for job snapshot")
		return
	}
	y := bounds.y
	for _, job := range state.Jobs {
		row := rectangle{x: bounds.x, y: y, width: bounds.width, height: renderer.scaleValue(46)}
		renderer.rect(row, tokenRaised)
		renderer.outline(row, tokenDivider)
		renderer.text(renderer.window.interFont, string(job.ID), point{x: row.x + renderer.scaleValue(10), y: row.y + renderer.scaleValue(7)}, 12, tokenText)
		renderer.text(renderer.window.interFont, job.Stage, point{x: row.x + renderer.scaleValue(180), y: row.y + renderer.scaleValue(7)}, 12, tokenMuted)
		renderer.progressBar(rectangle{x: row.x + renderer.scaleValue(10), y: row.y + renderer.scaleValue(28), width: row.width - renderer.scaleValue(20), height: renderer.scaleValue(8)}, job.ProgressPermil, jobStateColor(job.State))
		y += renderer.scaleValue(52)
		if y > bounds.y+bounds.height-renderer.scaleValue(46) {
			break
		}
	}
}

func (renderer *shellRenderer) drawResultPreview(state appstate.AppState, bounds rectangle) {
	if len(state.Results) == 0 {
		renderer.emptyDock(bounds, "Waiting for result snapshot")
		return
	}
	result := state.Results[0]
	lines := []string{
		"Run: " + string(result.ID),
		"Immutable: " + boolLabel(result.Immutable),
		"Validation: " + result.ValidationStart.Format("2006-01-02") + " to " + result.ValidationEnd.Format("2006-01-02"),
		"Test: " + result.TestStart.Format("2006-01-02") + " to " + result.TestEnd.Format("2006-01-02"),
	}
	for _, metric := range result.Metrics {
		lines = append(lines, metric.Name+": "+fmt.Sprintf("%.4f", metric.Value))
	}
	renderer.textBlock(bounds, lines)
}

func (renderer *shellRenderer) drawModelPreview(state appstate.AppState, bounds rectangle) {
	if len(state.Models) == 0 {
		renderer.emptyDock(bounds, "Waiting for model snapshot")
		return
	}
	model := state.Models[0]
	lines := []string{
		"Model: " + string(model.ID),
		"Alias: " + model.Alias,
		"Kind: " + string(model.Kind),
		"Immutable: " + boolLabel(model.Immutable),
		"Features: " + featureNamesLabel(model.FeatureNames),
		"Fingerprint: " + model.ArtifactFingerprint,
	}
	renderer.textBlock(bounds, lines)
}

func (renderer *shellRenderer) drawInferencePreview(state appstate.AppState, bounds rectangle) {
	if len(state.Inferences) == 0 {
		renderer.emptyDock(bounds, "Waiting for inference snapshot")
		return
	}
	inference := state.Inferences[0]
	renderer.tableHeader(bounds, []string{"Rank", "Symbol", "Score", "State"})
	y := bounds.y + renderer.scaleValue(28)
	for _, score := range inference.Scores {
		renderer.tableText(bounds.x+renderer.scaleValue(10), y, fmt.Sprintf("%d", score.Rank), tokenText)
		renderer.tableText(bounds.x+bounds.width*0.22, y, string(score.Symbol), tokenCyan)
		renderer.tableText(bounds.x+bounds.width*0.44, y, fmt.Sprintf("%.3f", score.Score), tokenText)
		renderer.tableText(bounds.x+bounds.width*0.66, y, string(inference.State), inferenceStateColor(inference.State))
		y += renderer.scaleValue(24)
	}
}

func (renderer *shellRenderer) drawStatusBar(state appstate.AppState) {
	height := float32(renderer.input.height)
	width := float32(renderer.input.width)
	barHeight := renderer.scaleValue(statusBarHeight)
	y := height - barHeight
	renderer.rect(rectangle{x: 0, y: y, width: width, height: barHeight}, tokenPanel)
	renderer.line(point{x: 0, y: y}, point{x: width, y: y}, 1, tokenDivider)

	worker := componentDetail(state.Components, appstate.ComponentWorkerPool)
	if worker == "" {
		worker = "workers pending"
	}
	parts := []string{
		"health " + string(state.Connection.State),
		fmt.Sprintf("UI %d%%", state.Preferences.UIScale),
		"selection " + selectionLabel(state),
		"cache " + cacheLabel(state.Cache),
		fmt.Sprintf("CPU %d slots", activeCPUSlots(state.Jobs)),
		worker,
		fmt.Sprintf("FPS %d", renderer.input.fps),
		"Ctrl+K command  Ctrl+, settings",
	}
	x := renderer.scaleValue(10)
	for _, part := range parts {
		renderer.text(renderer.window.interFont, clipText(part, 34), point{x: x, y: y + renderer.scaleValue(5)}, 10, tokenMuted)
		x += renderer.scaleValue(float32(len(part))*6 + 26)
		if x > width-renderer.scaleValue(120) {
			break
		}
	}
}

func (renderer *shellRenderer) drawToasts(state appstate.AppState) {
	if len(state.Overlays.Toasts) == 0 {
		return
	}
	width := float32(renderer.input.width)
	toastWidth := renderer.scaleValue(360)
	x := width - toastWidth - renderer.scaleValue(12)
	y := renderer.scaleValue(topNavHeight+contextBarHeight) + renderer.scaleValue(12)
	start := maxInt(0, len(state.Overlays.Toasts)-3)
	for _, toast := range state.Overlays.Toasts[start:] {
		color := toastColor(toast.Kind)
		bounds := rectangle{x: x, y: y, width: toastWidth, height: renderer.scaleValue(42)}
		renderer.rect(bounds, withAlpha(tokenRaised, 242))
		renderer.outline(bounds, color)
		renderer.rect(rectangle{x: bounds.x, y: bounds.y, width: renderer.scaleValue(3), height: bounds.height}, color)
		renderer.text(renderer.window.interFont, clipText(toast.Message, 42), point{x: bounds.x + renderer.scaleValue(12), y: bounds.y + renderer.scaleValue(13)}, 11, tokenText)
		y += renderer.scaleValue(48)
	}
}

func (renderer *shellRenderer) drawModalLayer(state appstate.AppState) {
	if state.Overlays.CriticalError.Message != "" {
		renderer.drawCriticalErrorModal(state.Overlays.CriticalError)
		return
	}
	if state.Overlays.SettingsOpen {
		renderer.drawSettingsModal(state)
		return
	}
	if state.Overlays.CommandPaletteOpen {
		renderer.drawCommandPalette(state)
	}
}

func (renderer *shellRenderer) drawCriticalErrorModal(err appstate.ErrorSnapshot) {
	renderer.scrim()
	width := float32(renderer.input.width)
	height := float32(renderer.input.height)
	bounds := renderer.modalBounds(width, height, 520, 220)
	renderer.rect(bounds, tokenPanel)
	renderer.outline(bounds, tokenNegative)
	renderer.text(renderer.window.interFont, "Critical Error", point{x: bounds.x + renderer.scaleValue(18), y: bounds.y + renderer.scaleValue(18)}, 16, tokenNegative)
	renderer.text(renderer.window.interFont, clipText(err.Message, 58), point{x: bounds.x + renderer.scaleValue(18), y: bounds.y + renderer.scaleValue(52)}, 12, tokenText)
	renderer.text(renderer.window.interFont, "Coordinator actions are disabled until the error clears.", point{x: bounds.x + renderer.scaleValue(18), y: bounds.y + renderer.scaleValue(78)}, 11, tokenMuted)
	renderer.disabledButton(rectangle{x: bounds.x + renderer.scaleValue(18), y: bounds.y + bounds.height - renderer.scaleValue(42), width: renderer.scaleValue(130), height: renderer.scaleValue(24)}, "Reconnect")
	renderer.disabledButton(rectangle{x: bounds.x + renderer.scaleValue(156), y: bounds.y + bounds.height - renderer.scaleValue(42), width: renderer.scaleValue(130), height: renderer.scaleValue(24)}, "Restart")
}

func (renderer *shellRenderer) drawSettingsModal(state appstate.AppState) {
	renderer.scrim()
	if renderer.input.escapePressed {
		renderer.actions = append(renderer.actions, appstate.SetSettingsOpenAction{Open: false})
	}
	width := float32(renderer.input.width)
	height := float32(renderer.input.height)
	bounds := renderer.modalBounds(width, height, 620, 350)
	renderer.rect(bounds, tokenPanel)
	renderer.outline(bounds, tokenCyan)
	padding := renderer.scaleValue(20)
	renderer.text(renderer.window.interFont, "Settings", point{x: bounds.x + padding, y: bounds.y + padding}, fontHeading, tokenText)
	closeBounds := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(88), y: bounds.y + renderer.scaleValue(14), width: renderer.scaleValue(68), height: renderer.scaleValue(32)}
	if renderer.navButton(closeBounds, "Close", false, tokenCyan) && renderer.clicked(closeBounds) {
		renderer.actions = append(renderer.actions, appstate.SetSettingsOpenAction{Open: false})
	}

	y := bounds.y + renderer.scaleValue(72)
	renderer.text(renderer.window.interFont, "Appearance", point{x: bounds.x + padding, y: y}, fontControl, tokenCyan)
	y += renderer.scaleValue(32)
	renderer.text(renderer.window.interFont, "Interface size", point{x: bounds.x + padding, y: y}, fontBody, tokenText)
	y += renderer.scaleValue(24)
	renderer.text(renderer.window.interFont, "Scales text, controls, docking, spacing, and pointer targets together.", point{x: bounds.x + padding, y: y}, fontCaption, tokenMuted)
	y += renderer.scaleValue(38)

	presets := appstate.UIScalePresets()
	gap := renderer.scaleValue(8)
	available := bounds.width - 2*padding - gap*float32(len(presets)-1)
	buttonWidth := available / float32(len(presets))
	for index, preset := range presets {
		button := rectangle{x: bounds.x + padding + float32(index)*(buttonWidth+gap), y: y, width: buttonWidth, height: renderer.scaleValue(38)}
		selected := preset == state.Preferences.UIScale
		if renderer.navButton(button, fmt.Sprintf("%d%%", preset), selected, tokenCyan) && renderer.clicked(button) && !selected {
			renderer.actions = append(renderer.actions, appstate.SetUIScaleAction{Scale: preset})
		}
	}

	y += renderer.scaleValue(62)
	windowsScale := maxFloat32(renderer.input.dpi.x, renderer.input.dpi.y)
	if windowsScale != windowsScale || windowsScale < 1 {
		windowsScale = 1
	}
	renderer.text(renderer.window.interFont, fmt.Sprintf("Windows scale %.0f%%   Effective UI scale %.0f%%", windowsScale*100, renderer.scale*100), point{x: bounds.x + padding, y: y}, fontCaption, tokenMuted)
	y += renderer.scaleValue(30)
	renderer.text(renderer.window.monoFont, "Ctrl+Plus / Ctrl+Minus adjust size   Ctrl+0 restores 125%", point{x: bounds.x + padding, y: y}, fontCaption, tokenMuted)
	if state.PreferencePersistence.LastError.Message != "" {
		y += renderer.scaleValue(30)
		renderer.text(renderer.window.interFont, clipText(state.PreferencePersistence.LastError.Message, 72), point{x: bounds.x + padding, y: y}, fontCaption, tokenNegative)
	}
}

func (renderer *shellRenderer) drawCommandPalette(state appstate.AppState) {
	renderer.scrim()
	commands := commandPaletteCommands(state)
	if len(commands) == 0 {
		renderer.window.commandIndex = 0
	} else {
		if renderer.window.commandIndex < 0 {
			renderer.window.commandIndex = 0
		}
		if renderer.window.commandIndex >= len(commands) {
			renderer.window.commandIndex = len(commands) - 1
		}
		if renderer.input.downPressed {
			renderer.window.commandIndex = nextCommandIndex(commands, renderer.window.commandIndex, 1)
		}
		if renderer.input.upPressed {
			renderer.window.commandIndex = nextCommandIndex(commands, renderer.window.commandIndex, -1)
		}
		if renderer.input.enterPressed {
			renderer.actions = append(renderer.actions, commandActions(commands[renderer.window.commandIndex])...)
		}
	}
	if renderer.input.escapePressed {
		renderer.actions = append(renderer.actions, appstate.SetCommandPaletteAction{Open: false})
	}

	width := float32(renderer.input.width)
	height := float32(renderer.input.height)
	bounds := renderer.modalBounds(width, height, 620, 420)
	renderer.rect(bounds, tokenPanel)
	renderer.outline(bounds, tokenPurple)
	renderer.text(renderer.window.interFont, "Command Palette", point{x: bounds.x + renderer.scaleValue(16), y: bounds.y + renderer.scaleValue(14)}, 16, tokenText)
	renderer.text(renderer.window.interFont, "Navigate workspaces and inspect available shell commands", point{x: bounds.x + renderer.scaleValue(16), y: bounds.y + renderer.scaleValue(38)}, 11, tokenMuted)
	input := rectangle{x: bounds.x + renderer.scaleValue(16), y: bounds.y + renderer.scaleValue(62), width: bounds.width - renderer.scaleValue(32), height: renderer.scaleValue(28)}
	renderer.rect(input, tokenBackground)
	renderer.outline(input, tokenDivider)
	renderer.text(renderer.window.monoFont, "> workspace", point{x: input.x + renderer.scaleValue(9), y: input.y + renderer.scaleValue(7)}, 11, tokenMuted)

	y := bounds.y + renderer.scaleValue(102)
	for index, command := range commands {
		row := rectangle{x: bounds.x + renderer.scaleValue(10), y: y, width: bounds.width - renderer.scaleValue(20), height: renderer.scaleValue(38)}
		selected := index == renderer.window.commandIndex
		if selected {
			renderer.rect(row, tokenRaised)
			renderer.rect(rectangle{x: row.x, y: row.y, width: renderer.scaleValue(3), height: row.height}, tokenPurple)
		}
		textColor := tokenText
		detailColor := tokenMuted
		if !command.enabled {
			textColor = tokenMuted
			detailColor = withAlpha(tokenMuted, 150)
		}
		renderer.text(renderer.window.interFont, command.title, point{x: row.x + renderer.scaleValue(12), y: row.y + renderer.scaleValue(7)}, 12, textColor)
		renderer.text(renderer.window.interFont, command.detail, point{x: row.x + renderer.scaleValue(220), y: row.y + renderer.scaleValue(7)}, 11, detailColor)
		if command.enabled && renderer.clicked(row) {
			renderer.window.commandIndex = index
			renderer.actions = append(renderer.actions, commandActions(command)...)
		}
		y += renderer.scaleValue(40)
		if y > bounds.y+bounds.height-renderer.scaleValue(44) {
			break
		}
	}
}

func (renderer *shellRenderer) rect(bounds rectangle, color colorValue) {
	renderer.call("draw Raylib rectangle", func() {
		renderer.window.backend.drawRectangle(bounds, color)
	})
}

func (renderer *shellRenderer) outline(bounds rectangle, color colorValue) {
	renderer.call("draw Raylib rectangle outline", func() {
		renderer.window.backend.drawRectangleLines(bounds, 1, color)
	})
}

func (renderer *shellRenderer) line(start point, end point, thickness float32, color colorValue) {
	renderer.call("draw Raylib line", func() {
		renderer.window.backend.drawLine(start, end, thickness, color)
	})
}

func (renderer *shellRenderer) text(font fontHandle, text string, position point, size float32, color colorValue) {
	renderer.call("draw Raylib text", func() {
		renderer.window.backend.drawText(font, text, position, renderer.scaleValue(readableTextSize(size)), 0, color)
	})
}

func (renderer *shellRenderer) call(operation string, fn func()) {
	if renderer.err != nil {
		return
	}
	if err := renderer.window.check(operation); err != nil {
		renderer.err = err
		return
	}
	fn()
}

func (renderer *shellRenderer) scaleValue(value float32) float32 {
	return value * renderer.scale
}

func (renderer *shellRenderer) modalBounds(width float32, height float32, logicalWidth float32, logicalHeight float32) rectangle {
	margin := renderer.scaleValue(16)
	modalWidth := minFloat32(renderer.scaleValue(logicalWidth), maxFloat32(0, width-2*margin))
	modalHeight := minFloat32(renderer.scaleValue(logicalHeight), maxFloat32(0, height-2*margin))
	return rectangle{x: (width - modalWidth) / 2, y: (height - modalHeight) / 2, width: modalWidth, height: modalHeight}
}

func (renderer *shellRenderer) clicked(bounds rectangle) bool {
	return renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, bounds)
}

func (renderer *shellRenderer) navButton(bounds rectangle, label string, active bool, accent colorValue) bool {
	bg := tokenPanel
	textColor := tokenMuted
	if pointInRectangle(renderer.input.mouse, bounds) {
		bg = tokenRaised
		textColor = tokenText
	}
	if active {
		bg = tokenRaised
		textColor = tokenText
	}
	renderer.rect(bounds, bg)
	if active {
		renderer.rect(rectangle{x: bounds.x, y: bounds.y + bounds.height - renderer.scaleValue(2), width: bounds.width, height: renderer.scaleValue(2)}, accent)
	}
	renderer.text(renderer.window.interFont, label, point{x: bounds.x + renderer.scaleValue(10), y: bounds.y + renderer.scaleValue(5)}, 11, textColor)
	return renderer.clicked(bounds)
}

func (renderer *shellRenderer) statusChip(bounds rectangle, label string, color colorValue) {
	renderer.rect(bounds, withAlpha(color, 26))
	renderer.outline(bounds, withAlpha(color, 180))
	renderer.text(renderer.window.interFont, clipText(label, 20), point{x: bounds.x + renderer.scaleValue(8), y: bounds.y + renderer.scaleValue(5)}, 10, tokenText)
}

func (renderer *shellRenderer) componentRow(x float32, y float32, width float32, component appstate.ComponentSnapshot) {
	color := componentStateColor(component.State)
	renderer.rect(rectangle{x: x, y: y, width: width, height: renderer.scaleValue(24)}, tokenRaised)
	renderer.rect(rectangle{x: x, y: y, width: renderer.scaleValue(3), height: renderer.scaleValue(24)}, color)
	renderer.text(renderer.window.interFont, componentTitle(component.Component), point{x: x + renderer.scaleValue(10), y: y + renderer.scaleValue(5)}, 11, tokenText)
	renderer.text(renderer.window.interFont, clipText(component.Detail, 18), point{x: x + width*0.52, y: y + renderer.scaleValue(5)}, 10, tokenMuted)
}

func (renderer *shellRenderer) sectionTitle(x float32, y float32, title string) {
	renderer.text(renderer.window.interFont, title, point{x: x, y: y}, 11, tokenMuted)
}

func (renderer *shellRenderer) smallLine(x float32, y float32, label string, value string) {
	if value == "" {
		value = "--"
	}
	renderer.text(renderer.window.interFont, label, point{x: x, y: y}, 10, tokenMuted)
	renderer.text(renderer.window.interFont, clipText(value, 20), point{x: x + renderer.scaleValue(58), y: y}, 10, tokenText)
}

func (renderer *shellRenderer) tableHeader(bounds rectangle, columns []string) {
	header := rectangle{x: bounds.x, y: bounds.y, width: bounds.width, height: renderer.scaleValue(24)}
	renderer.rect(header, tokenRaised)
	renderer.outline(header, tokenDivider)
	for index, column := range columns {
		x := bounds.x + renderer.scaleValue(10) + bounds.width*float32(index)/float32(len(columns))
		renderer.text(renderer.window.interFont, column, point{x: x, y: header.y + renderer.scaleValue(6)}, 10, tokenMuted)
	}
}

func (renderer *shellRenderer) tableText(x float32, y float32, value string, color colorValue) {
	renderer.text(renderer.window.monoFont, value, point{x: x, y: y + renderer.scaleValue(5)}, 11, color)
}

func (renderer *shellRenderer) progressBar(bounds rectangle, permil int, color colorValue) {
	renderer.rect(bounds, tokenBackground)
	renderer.outline(bounds, tokenDivider)
	if permil < 0 {
		permil = 0
	}
	if permil > 1000 {
		permil = 1000
	}
	fill := bounds
	fill.width = bounds.width * float32(permil) / 1000
	renderer.rect(fill, color)
}

func (renderer *shellRenderer) textBlock(bounds rectangle, lines []string) {
	y := bounds.y
	for _, line := range lines {
		renderer.text(renderer.window.monoFont, clipText(line, 72), point{x: bounds.x, y: y}, 12, tokenText)
		y += renderer.scaleValue(24)
		if y > bounds.y+bounds.height-renderer.scaleValue(20) {
			return
		}
	}
}

func (renderer *shellRenderer) emptyDock(bounds rectangle, message string) {
	renderer.rect(bounds, tokenBackground)
	renderer.outline(bounds, tokenDivider)
	renderer.text(renderer.window.interFont, message, point{x: bounds.x + renderer.scaleValue(16), y: bounds.y + renderer.scaleValue(16)}, 12, tokenMuted)
}

func (renderer *shellRenderer) disabledButton(bounds rectangle, label string) {
	renderer.rect(bounds, tokenBackground)
	renderer.outline(bounds, tokenDivider)
	renderer.text(renderer.window.interFont, label, point{x: bounds.x + renderer.scaleValue(12), y: bounds.y + renderer.scaleValue(6)}, 10, tokenMuted)
}

func (renderer *shellRenderer) scrim() {
	renderer.rect(
		rectangle{x: 0, y: 0, width: float32(renderer.input.width), height: float32(renderer.input.height)},
		colorValue{red: 0, green: 0, blue: 0, alpha: 168},
	)
}

func (renderer *shellRenderer) tabWidth(label string) float32 {
	return renderer.scaleValue(float32(len(label))*7 + 36)
}

func commandPaletteCommands(state appstate.AppState) []shellCommand {
	commands := make([]shellCommand, 0, len(appstate.Workspaces())+1)
	for _, workspace := range appstate.Workspaces() {
		active := state.ActiveWorkspace == workspace
		detail := "Switch workspace"
		if active {
			detail = "Current workspace"
		}
		commands = append(commands, shellCommand{
			title:   "Go to " + workspaceTitle(workspace),
			detail:  detail,
			enabled: true,
			action:  appstate.SelectWorkspaceAction{Workspace: workspace},
		})
	}
	commands = append(commands, shellCommand{
		title:   "Reset " + workspaceTitle(state.ActiveWorkspace) + " layout",
		detail:  "Restore default docking",
		enabled: true,
		action:  appstate.ResetWorkspaceLayoutAction{Workspace: state.ActiveWorkspace},
	})
	return commands
}

func modalOpen(state appstate.AppState) bool {
	return state.Overlays.CommandPaletteOpen || state.Overlays.SettingsOpen || state.Overlays.CriticalError.Message != ""
}

func readableTextSize(size float32) float32 {
	switch {
	case size <= 10:
		return fontCaption
	case size <= 12:
		return fontBody
	case size <= 14:
		return fontControl
	default:
		return maxFloat32(size, fontHeading)
	}
}

func commandActions(command shellCommand) []appstate.UIAction {
	if !command.enabled || command.action == nil {
		return nil
	}
	return []appstate.UIAction{
		command.action,
		appstate.SetCommandPaletteAction{Open: false},
	}
}

func nextCommandIndex(commands []shellCommand, current int, direction int) int {
	if len(commands) == 0 {
		return 0
	}
	index := current
	for range len(commands) {
		index += direction
		if index < 0 {
			index = len(commands) - 1
		}
		if index >= len(commands) {
			index = 0
		}
		if commands[index].enabled {
			return index
		}
	}
	return current
}

func activeLayout(state appstate.AppState) (appstate.WorkspaceLayout, bool) {
	for _, layout := range state.Layouts {
		if layout.Workspace == state.ActiveWorkspace {
			return layout, true
		}
	}
	return appstate.WorkspaceLayout{}, false
}

func flattenPanels(root appstate.DockNode) []appstate.PanelInstanceState {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return append([]appstate.PanelInstanceState(nil), node.Panels...)
	case appstate.SplitNode:
		first := flattenPanels(node.First)
		second := flattenPanels(node.Second)
		return append(first, second...)
	default:
		return nil
	}
}

func activePanelID(root appstate.DockNode) appstate.PanelID {
	switch node := root.(type) {
	case appstate.TabStackNode:
		return node.Active
	case appstate.SplitNode:
		if active := activePanelID(node.First); active != "" {
			return active
		}
		return activePanelID(node.Second)
	default:
		return ""
	}
}

func selectedDatasetName(state appstate.AppState) string {
	for _, dataset := range state.Datasets {
		if dataset.ID == state.LinkContext.DatasetID {
			return dataset.Name
		}
	}
	if state.LinkContext.DatasetID != "" {
		return string(state.LinkContext.DatasetID)
	}
	return "No dataset selected"
}

func symbolsLabel(symbols []appstate.Symbol) string {
	if len(symbols) == 0 {
		return "--"
	}
	parts := make([]string, len(symbols))
	for index, symbol := range symbols {
		parts[index] = string(symbol)
	}
	return strings.Join(parts, ", ")
}

func featureNamesLabel(features []appstate.FeatureName) string {
	if len(features) == 0 {
		return "--"
	}
	parts := make([]string, len(features))
	for index, feature := range features {
		parts[index] = string(feature)
	}
	return strings.Join(parts, ", ")
}

func selectionLabel(state appstate.AppState) string {
	dataset := string(state.LinkContext.DatasetID)
	if dataset == "" {
		dataset = "--"
	}
	return dataset + " " + symbolsLabel(state.LinkContext.Symbols)
}

func cacheLabel(cache appstate.CacheSnapshot) string {
	if cache.BytesUsed == 0 {
		return "--"
	}
	return fmt.Sprintf("%d MB gen %d", cache.BytesUsed/(1024*1024), cache.Generation)
}

func componentDetail(components []appstate.ComponentSnapshot, component appstate.Component) string {
	for _, snapshot := range components {
		if snapshot.Component == component {
			return snapshot.Detail
		}
	}
	return ""
}

func activeJobCount(jobs []appstate.JobSummary) int {
	count := 0
	for _, job := range jobs {
		switch job.State {
		case appstate.JobQueued,
			appstate.JobRunning,
			appstate.JobPauseRequested,
			appstate.JobPaused,
			appstate.JobInterrupted:
			count++
		}
	}
	return count
}

func activeCPUSlots(jobs []appstate.JobSummary) int {
	total := 0
	for _, job := range jobs {
		switch job.State {
		case appstate.JobQueued,
			appstate.JobRunning,
			appstate.JobPauseRequested:
			total += job.CPUSlots
		}
	}
	return total
}

func workspaceTitle(workspace appstate.Workspace) string {
	switch workspace {
	case appstate.WorkspaceData:
		return "Data"
	case appstate.WorkspaceResearch:
		return "Research"
	case appstate.WorkspaceExperiments:
		return "Experiments"
	case appstate.WorkspaceJobs:
		return "Jobs"
	case appstate.WorkspaceResults:
		return "Results"
	case appstate.WorkspaceModels:
		return "Models"
	case appstate.WorkspaceInference:
		return "Inference"
	default:
		return string(workspace)
	}
}

func compactWorkspaceTitle(workspace appstate.Workspace) string {
	switch workspace {
	case appstate.WorkspaceData:
		return "D"
	case appstate.WorkspaceResearch:
		return "R"
	case appstate.WorkspaceExperiments:
		return "E"
	case appstate.WorkspaceJobs:
		return "J"
	case appstate.WorkspaceResults:
		return "Rs"
	case appstate.WorkspaceModels:
		return "M"
	case appstate.WorkspaceInference:
		return "I"
	default:
		return "?"
	}
}

func componentTitle(component appstate.Component) string {
	switch component {
	case appstate.ComponentCoordinator:
		return "Coordinator"
	case appstate.ComponentCatalog:
		return "Catalog"
	case appstate.ComponentScheduler:
		return "Scheduler"
	case appstate.ComponentWorkerPool:
		return "Workers"
	case appstate.ComponentCache:
		return "Cache"
	default:
		return string(component)
	}
}

func connectionLabel(snapshot appstate.ConnectionSnapshot) string {
	if snapshot.Detail == "" {
		return string(snapshot.State)
	}
	return string(snapshot.State) + " " + snapshot.Detail
}

func connectionColor(state appstate.ConnectionState) colorValue {
	switch state {
	case appstate.ConnectionConnected:
		return tokenPositive
	case appstate.ConnectionStarting:
		return tokenCyan
	case appstate.ConnectionDegraded:
		return tokenWarning
	case appstate.ConnectionDisconnected:
		return tokenNegative
	default:
		return tokenMuted
	}
}

func componentStateColor(state appstate.ComponentState) colorValue {
	switch state {
	case appstate.ComponentHealthy:
		return tokenPositive
	case appstate.ComponentStarting:
		return tokenCyan
	case appstate.ComponentDegraded:
		return tokenWarning
	case appstate.ComponentStopping:
		return tokenPurple
	case appstate.ComponentFailed:
		return tokenNegative
	default:
		return tokenMuted
	}
}

func jobStateColor(state appstate.JobState) colorValue {
	switch state {
	case appstate.JobCompleted:
		return tokenPositive
	case appstate.JobFailed,
		appstate.JobCancelled:
		return tokenNegative
	case appstate.JobPaused,
		appstate.JobPauseRequested,
		appstate.JobInterrupted:
		return tokenWarning
	default:
		return tokenCyan
	}
}

func inferenceStateColor(state appstate.InferenceState) colorValue {
	switch state {
	case appstate.InferenceCompleted:
		return tokenPositive
	case appstate.InferenceFailed:
		return tokenNegative
	case appstate.InferenceQueued:
		return tokenWarning
	default:
		return tokenCyan
	}
}

func toastColor(kind appstate.ToastKind) colorValue {
	switch kind {
	case appstate.ToastError:
		return tokenNegative
	case appstate.ToastWarning:
		return tokenWarning
	default:
		return tokenCyan
	}
}

func pointInRectangle(value point, bounds rectangle) bool {
	return value.x >= bounds.x &&
		value.x <= bounds.x+bounds.width &&
		value.y >= bounds.y &&
		value.y <= bounds.y+bounds.height
}

func inset(bounds rectangle, amount float32) rectangle {
	return rectangle{
		x:      bounds.x + amount,
		y:      bounds.y + amount,
		width:  maxFloat32(0, bounds.width-2*amount),
		height: maxFloat32(0, bounds.height-2*amount),
	}
}

func clipText(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func rgb(red uint8, green uint8, blue uint8) colorValue {
	return colorValue{red: red, green: green, blue: blue, alpha: 255}
}

func withAlpha(value colorValue, alpha uint8) colorValue {
	value.alpha = alpha
	return value
}

func minFloat32(first float32, second float32) float32 {
	if first < second {
		return first
	}
	return second
}

func maxFloat32(first float32, second float32) float32 {
	if first > second {
		return first
	}
	return second
}

func maxInt(first int, second int) int {
	if first > second {
		return first
	}
	return second
}
