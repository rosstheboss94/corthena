package nativeui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	virtualtable "github.com/rosstheboss94/corthena/internal/frontend/table"
)

func isJobsPanel(panelType appstate.PanelType) bool {
	switch panelType {
	case appstate.PanelJobQueue, appstate.PanelJobProgress, appstate.PanelLiveMetrics,
		appstate.PanelWorkerResources, appstate.PanelProcessStatus, appstate.PanelCheckpointStatus,
		appstate.PanelJobLogs:
		return true
	default:
		return false
	}
}

func isResultsPanel(panelType appstate.PanelType) bool {
	switch panelType {
	case appstate.PanelRunBrowser, appstate.PanelMetricComparison, appstate.PanelEquityChart,
		appstate.PanelFoldTimeline, appstate.PanelPredictionOverlay, appstate.PanelResultDistributions,
		appstate.PanelConfigurationDiff:
		return true
	default:
		return false
	}
}

func (renderer *shellRenderer) drawJobsPanel(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	content := phase8Content(bounds, renderer.scaleValue(24))
	renderer.drawJobsScenarioControl(state.JobsWorkspace, bounds)
	if state.JobsWorkspace.State == appstate.WorkspaceLoading && !state.JobsWorkspace.Stale {
		renderer.emptyDock(content, "Loading deterministic job telemetry...")
		return
	}
	if (state.JobsWorkspace.State == appstate.WorkspaceFailed || state.JobsWorkspace.State == appstate.WorkspaceCancelled || state.JobsWorkspace.State == appstate.WorkspaceBusy) && !state.JobsWorkspace.Stale {
		renderer.drawJobsFailure(state.JobsWorkspace, content)
		return
	}
	switch panel.Type {
	case appstate.PanelJobQueue:
		renderer.drawJobQueue(state.JobsWorkspace, content)
	case appstate.PanelJobProgress:
		renderer.drawJobProgress(state.JobsWorkspace, content)
	case appstate.PanelLiveMetrics:
		renderer.drawJobMetrics(state.JobsWorkspace, content)
	case appstate.PanelWorkerResources:
		renderer.drawWorkerResources(state.JobsWorkspace, content)
	case appstate.PanelProcessStatus:
		renderer.drawProcessStatus(state.JobsWorkspace, content)
	case appstate.PanelCheckpointStatus:
		renderer.drawCheckpointStatus(state.JobsWorkspace, content)
	case appstate.PanelJobLogs:
		renderer.drawJobLogs(state.JobsWorkspace, content)
	}
	if state.JobsWorkspace.Stale {
		renderer.drawPhase7Status("Jobs", true, bounds)
	}
}

func (renderer *shellRenderer) drawResultsPanel(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	content := phase8Content(bounds, renderer.scaleValue(24))
	renderer.drawResultsScenarioControl(state.ResultsWorkspace, bounds)
	if state.ResultsWorkspace.State == appstate.WorkspaceLoading && !state.ResultsWorkspace.Stale {
		renderer.emptyDock(content, "Loading immutable run comparisons...")
		return
	}
	if (state.ResultsWorkspace.State == appstate.WorkspaceFailed || state.ResultsWorkspace.State == appstate.WorkspaceCancelled || state.ResultsWorkspace.State == appstate.WorkspaceBusy) && !state.ResultsWorkspace.Stale {
		renderer.drawResultsFailure(state.ResultsWorkspace, content)
		return
	}
	if state.ResultsWorkspace.State == appstate.WorkspaceEmpty {
		renderer.emptyDock(content, "No immutable results match this view")
		return
	}
	switch panel.Type {
	case appstate.PanelRunBrowser:
		renderer.drawRunBrowser(state.ResultsWorkspace, content)
	case appstate.PanelMetricComparison:
		renderer.drawMetricComparison(state.ResultsWorkspace, content)
	case appstate.PanelEquityChart:
		renderer.drawResultEquity(state.ResultsWorkspace, content)
	case appstate.PanelFoldTimeline:
		renderer.drawFoldTimeline(state.ResultsWorkspace, content)
	case appstate.PanelPredictionOverlay:
		renderer.drawPredictionOverlay(state.ResultsWorkspace, content)
	case appstate.PanelResultDistributions:
		renderer.drawResultDistributions(state.ResultsWorkspace, content)
	case appstate.PanelConfigurationDiff:
		renderer.drawConfigurationDiff(state.ResultsWorkspace, content)
	}
	if state.ResultsWorkspace.Stale || state.ResultsWorkspace.State == appstate.WorkspaceDegraded || state.ResultsWorkspace.State == appstate.WorkspaceRecovered {
		renderer.drawPhase7Status(string(state.ResultsWorkspace.State), state.ResultsWorkspace.Stale, bounds)
	}
}

func phase8Content(bounds rectangle, header float32) rectangle {
	return rectangle{x: bounds.x, y: bounds.y + header, width: bounds.width, height: maxFloat32(0, bounds.height-header)}
}

func (renderer *shellRenderer) drawJobsScenarioControl(state appstate.JobsWorkspaceState, bounds rectangle) {
	button := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(156), y: bounds.y, width: renderer.scaleValue(152), height: renderer.scaleValue(20)}
	if renderer.phase7Button(button, "Scenario: "+string(state.Scenario), true) {
		scenarios := appstate.JobsScenarios()
		next := scenarios[0]
		for index, scenario := range scenarios {
			if scenario == state.Scenario {
				next = scenarios[(index+1)%len(scenarios)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetJobsScenarioAction{Scenario: next})
	}
}

func (renderer *shellRenderer) drawResultsScenarioControl(state appstate.ResultsWorkspaceState, bounds rectangle) {
	button := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(148), y: bounds.y, width: renderer.scaleValue(144), height: renderer.scaleValue(20)}
	if renderer.phase7Button(button, "Scenario: "+string(state.Scenario), true) {
		scenarios := appstate.ResultsScenarios()
		next := scenarios[0]
		for index, scenario := range scenarios {
			if scenario == state.Scenario {
				next = scenarios[(index+1)%len(scenarios)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetResultsScenarioAction{Scenario: next})
	}
}

func (renderer *shellRenderer) drawJobsFailure(state appstate.JobsWorkspaceState, bounds rectangle) {
	message := state.Error.Message
	if message == "" {
		message = "Jobs request failed"
	}
	renderer.textBlock(bounds, []string{string(state.State), message, "Retry reconciles a fresh typed generation."})
	retry := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(72), width: renderer.scaleValue(84), height: renderer.scaleValue(24)}
	if renderer.phase7Button(retry, "Retry", true) {
		generation := state.Generation + 1
		renderer.actions = append(renderer.actions, appstate.RequestJobsWorkspaceAction{Query: appstate.JobsWorkspaceQuery{
			CorrelationID: appstate.CorrelationID(fmt.Sprintf("jobs-%020d", generation)), Generation: generation, Scenario: appstate.JobsScenarioSuccess,
		}})
	}
}

func (renderer *shellRenderer) drawResultsFailure(state appstate.ResultsWorkspaceState, bounds rectangle) {
	message := state.Error.Message
	if message == "" {
		message = "Results request failed"
	}
	renderer.textBlock(bounds, []string{string(state.State), message, "Retry retains the last immutable comparison."})
	retry := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(72), width: renderer.scaleValue(84), height: renderer.scaleValue(24)}
	if renderer.phase7Button(retry, "Retry", true) {
		generation := state.Generation + 1
		renderer.actions = append(renderer.actions, appstate.RequestResultsWorkspaceAction{Query: appstate.ResultsWorkspaceQuery{
			CorrelationID: appstate.CorrelationID(fmt.Sprintf("results-%020d", generation)), Generation: generation,
			Scenario: appstate.ResultsScenarioNormal, Filter: state.Filter, RunIDs: append([]appstate.RunID(nil), state.SelectedRunIDs...),
		}})
	}
}

func (renderer *shellRenderer) drawJobQueue(state appstate.JobsWorkspaceState, bounds rectangle) {
	if len(state.Snapshot.Jobs) == 0 {
		renderer.emptyDock(bounds, "Job queue is empty")
		return
	}
	columns := []virtualtable.Column{
		{ID: "job", Title: "Job", Kind: virtualtable.CellString, Width: maxFloat(150, float64(bounds.width)*0.35), MinWidth: 100, MaxWidth: 1200, Pinned: true, Sortable: true},
		{ID: "state", Title: "State", Kind: virtualtable.CellString, Width: maxFloat(90, float64(bounds.width)*0.19), MinWidth: 70, MaxWidth: 400, Sortable: true},
		{ID: "stage", Title: "Stage", Kind: virtualtable.CellString, Width: maxFloat(170, float64(bounds.width)*0.31), MinWidth: 100, MaxWidth: 900, Sortable: true},
		{ID: "progress", Title: "%", Kind: virtualtable.CellInteger, Width: maxFloat(56, float64(bounds.width)*0.12), MinWidth: 50, MaxWidth: 180, Sortable: true},
	}
	rows := make([]virtualtable.Row, len(state.Snapshot.Jobs))
	for index, detail := range state.Snapshot.Jobs {
		rows[index] = virtualtable.Row{ID: virtualtable.RowID(detail.Summary.ID), SourceIndex: uint64(index + 1), Cells: []virtualtable.Cell{
			{Kind: virtualtable.CellString, String: string(detail.Summary.ID)},
			{Kind: virtualtable.CellString, String: string(detail.Summary.State)},
			{Kind: virtualtable.CellString, String: detail.Summary.Stage},
			{Kind: virtualtable.CellInteger, Integer: int64(detail.Summary.ProgressPermil / 10)},
		}}
	}
	model, err := virtualtable.NewModel(virtualtable.Dataset{Columns: columns, Rows: rows})
	if err != nil {
		renderer.emptyDock(bounds, "Invalid job queue")
		return
	}
	headerHeight, rowHeight := renderer.scaleValue(24), renderer.scaleValue(25)
	window, err := model.Virtualize(virtualtable.WindowRequest{
		OriginX: float64(bounds.x), OriginY: float64(bounds.y), Width: float64(bounds.width), Height: float64(bounds.height),
		HeaderHeight: float64(headerHeight), RowHeight: float64(rowHeight), OverscanRows: 1, OverscanColumns: 1,
	})
	if err != nil {
		renderer.emptyDock(bounds, "Invalid job viewport")
		return
	}
	renderer.drawTableWindow(window, virtualtable.Selection{IDs: []virtualtable.RowID{virtualtable.RowID(state.SelectedJobID)}})
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, bounds) && renderer.input.mouse.y >= bounds.y+headerHeight {
		rowIndex := int((renderer.input.mouse.y - bounds.y - headerHeight) / rowHeight)
		if rowIndex >= window.RowStart && rowIndex < window.RowEnd && rowIndex < len(rows) {
			renderer.actions = append(renderer.actions, appstate.SelectJobAction{JobID: appstate.JobID(rows[rowIndex].ID)})
		}
	}
}

func (renderer *shellRenderer) drawJobProgress(state appstate.JobsWorkspaceState, bounds rectangle) {
	detail, found := selectedJob(state)
	if !found {
		renderer.emptyDock(bounds, "Select a job")
		return
	}
	buttonY := bounds.y
	buttonWidth := renderer.scaleValue(72)
	for index, control := range []appstate.JobControl{appstate.JobControlPause, appstate.JobControlResume, appstate.JobControlCancel} {
		x := bounds.x + float32(index)*(buttonWidth+renderer.scaleValue(6))
		enabled := detail.Summary.State.AllowsControl(control) && state.PendingControl == ""
		if renderer.phase7Button(rectangle{x: x, y: buttonY, width: buttonWidth, height: renderer.scaleValue(23)}, string(control), enabled) {
			commandID := appstate.CorrelationID(fmt.Sprintf("job-%s-%s-%020d", detail.Summary.ID, control, state.Generation))
			renderer.actions = append(renderer.actions, appstate.ControlJobAction{Command: appstate.JobControlCommand{
				CorrelationID: commandID, CommandID: commandID, Generation: state.Generation,
				JobID: detail.Summary.ID, Control: control,
			}})
		}
	}
	y := bounds.y + renderer.scaleValue(35)
	renderer.text(renderer.window.monoFont, string(detail.Summary.State)+"  "+detail.Summary.Stage, point{x: bounds.x, y: y}, 9, jobStateColor(detail.Summary.State))
	y += renderer.scaleValue(22)
	for _, stage := range detail.Stages {
		if y+renderer.scaleValue(37) > bounds.y+bounds.height {
			break
		}
		renderer.text(renderer.window.interFont, clipText(stage.Name, 34), point{x: bounds.x, y: y}, 9, tokenText)
		renderer.text(renderer.window.monoFont, string(stage.State), point{x: bounds.x + bounds.width*0.67, y: y}, 8, phase7StateColor(string(stage.State)))
		y += renderer.scaleValue(16)
		renderer.progressBar(rectangle{x: bounds.x, y: y, width: bounds.width, height: renderer.scaleValue(7)}, stage.ProgressPermil, phase7StateColor(string(stage.State)))
		y += renderer.scaleValue(20)
	}
}

func (renderer *shellRenderer) drawJobMetrics(state appstate.JobsWorkspaceState, bounds rectangle) {
	detail, found := selectedJob(state)
	if !found || len(detail.Metrics) == 0 {
		renderer.emptyDock(bounds, "No live metrics")
		return
	}
	seriesHeight := bounds.height / float32(len(detail.Metrics))
	styles := []colorValue{tokenCyan, tokenPurple, tokenWarning}
	for index, series := range detail.Metrics {
		chartBounds := rectangle{x: bounds.x, y: bounds.y + float32(index)*seriesHeight, width: bounds.width, height: seriesHeight - renderer.scaleValue(5)}
		renderer.drawMetricSeries(chartBounds, series, styles[index%len(styles)])
	}
}

func (renderer *shellRenderer) drawMetricSeries(bounds rectangle, series appstate.JobMetricSeries, color colorValue) {
	if len(series.Points) == 0 {
		return
	}
	minimum, maximum := series.Points[0].Value, series.Points[0].Value
	for _, item := range series.Points {
		minimum = math.Min(minimum, item.Value)
		maximum = math.Max(maximum, item.Value)
	}
	span := maximum - minimum
	if span == 0 {
		span = 1
	}
	plot := inset(bounds, renderer.scaleValue(4))
	renderer.outline(bounds, tokenDivider)
	for index := 0; index+1 < len(series.Points); index++ {
		first, second := series.Points[index], series.Points[index+1]
		x1 := plot.x + plot.width*float32(index)/float32(len(series.Points)-1)
		x2 := plot.x + plot.width*float32(index+1)/float32(len(series.Points)-1)
		y1 := plot.y + plot.height*(1-float32((first.Value-minimum)/span))
		y2 := plot.y + plot.height*(1-float32((second.Value-minimum)/span))
		renderer.line(point{x: x1, y: y1}, point{x: x2, y: y2}, 1.5, color)
	}
	last := series.Points[len(series.Points)-1]
	renderer.text(renderer.window.monoFont, fmt.Sprintf("%s  %.4g", series.Name, last.Value), point{x: bounds.x + renderer.scaleValue(6), y: bounds.y + renderer.scaleValue(5)}, 8, color)
}

func (renderer *shellRenderer) drawWorkerResources(state appstate.JobsWorkspaceState, bounds rectangle) {
	detail, found := selectedJob(state)
	if !found {
		renderer.emptyDock(bounds, "Select a job")
		return
	}
	worker := detail.Worker
	lines := []string{
		"Worker       " + worker.WorkerID,
		"PID          " + strconv.Itoa(worker.PID),
		"State        " + string(worker.State),
		fmt.Sprintf("CPU lease    %d / %d global slots", worker.LeasedSlots, state.Snapshot.TotalCPUSlots),
		"GOMAXPROCS   " + strconv.Itoa(worker.GOMAXPROCS),
		fmt.Sprintf("Tasks        %d active / %d goroutines", worker.ActiveTasks, worker.Goroutines),
		"Memory       " + formatBytes(worker.MemoryBytes),
		"Heartbeat    " + worker.HeartbeatAt.Format("15:04:05"),
		"Detail       " + worker.Degradation,
	}
	renderer.textBlock(bounds, lines)
}

func (renderer *shellRenderer) drawProcessStatus(state appstate.JobsWorkspaceState, bounds rectangle) {
	rowHeight := renderer.scaleValue(34)
	for index, process := range state.Snapshot.Processes {
		if float32(index+1)*rowHeight > bounds.height {
			break
		}
		row := rectangle{x: bounds.x, y: bounds.y + float32(index)*rowHeight, width: bounds.width, height: rowHeight}
		renderer.rect(row, tokenRaised)
		renderer.text(renderer.window.interFont, process.Role, point{x: row.x + renderer.scaleValue(6), y: row.y + renderer.scaleValue(5)}, 9, tokenText)
		renderer.text(renderer.window.monoFont, string(process.State), point{x: row.x + row.width*0.62, y: row.y + renderer.scaleValue(5)}, 8, phase7StateColor(string(process.State)))
		renderer.text(renderer.window.interFont, clipText(process.Detail, 52), point{x: row.x + renderer.scaleValue(6), y: row.y + renderer.scaleValue(19)}, 8, tokenMuted)
		renderer.line(point{x: row.x, y: row.y + row.height - 1}, point{x: row.x + row.width, y: row.y + row.height - 1}, 1, tokenDivider)
	}
}

func (renderer *shellRenderer) drawCheckpointStatus(state appstate.JobsWorkspaceState, bounds rectangle) {
	detail, found := selectedJob(state)
	if !found || len(detail.Checkpoints) == 0 {
		renderer.emptyDock(bounds, "No durable checkpoints")
		return
	}
	rowHeight := renderer.scaleValue(31)
	for index := len(detail.Checkpoints) - 1; index >= 0; index-- {
		visualIndex := len(detail.Checkpoints) - 1 - index
		if float32(visualIndex+1)*rowHeight > bounds.height {
			break
		}
		checkpoint := detail.Checkpoints[index]
		y := bounds.y + float32(visualIndex)*rowHeight
		renderer.text(renderer.window.monoFont, fmt.Sprintf("#%d", checkpoint.Sequence), point{x: bounds.x, y: y + renderer.scaleValue(5)}, 9, tokenPurple)
		renderer.text(renderer.window.monoFont, string(checkpoint.State), point{x: bounds.x + renderer.scaleValue(46), y: y + renderer.scaleValue(5)}, 9, phase7StateColor(string(checkpoint.State)))
		renderer.text(renderer.window.monoFont, formatBytes(checkpoint.Bytes), point{x: bounds.x + bounds.width*0.48, y: y + renderer.scaleValue(5)}, 8, tokenText)
		renderer.text(renderer.window.monoFont, checkpoint.CommittedAt.Format("15:04:05"), point{x: bounds.x + bounds.width*0.76, y: y + renderer.scaleValue(5)}, 8, tokenMuted)
		renderer.line(point{x: bounds.x, y: y + rowHeight - 1}, point{x: bounds.x + bounds.width, y: y + rowHeight - 1}, 1, tokenDivider)
	}
}

func (renderer *shellRenderer) drawJobLogs(state appstate.JobsWorkspaceState, bounds rectangle) {
	detail, found := selectedJob(state)
	if !found || len(detail.Logs) == 0 {
		renderer.emptyDock(bounds, "No structured logs")
		return
	}
	rowHeight := renderer.scaleValue(26)
	visible := max(0, int(bounds.height/rowHeight))
	start := max(0, len(detail.Logs)-visible)
	for index := start; index < len(detail.Logs); index++ {
		log := detail.Logs[index]
		y := bounds.y + float32(index-start)*rowHeight
		renderer.text(renderer.window.monoFont, log.Timestamp.Format("15:04:05"), point{x: bounds.x, y: y + renderer.scaleValue(5)}, 8, tokenMuted)
		renderer.text(renderer.window.monoFont, strings.ToUpper(string(log.Level)), point{x: bounds.x + renderer.scaleValue(55), y: y + renderer.scaleValue(5)}, 8, jobLogColor(log.Level))
		renderer.text(renderer.window.interFont, clipText(log.Message, 74), point{x: bounds.x + renderer.scaleValue(98), y: y + renderer.scaleValue(5)}, 8, tokenText)
	}
}

func (renderer *shellRenderer) drawRunBrowser(state appstate.ResultsWorkspaceState, bounds rectangle) {
	filterWidth := minFloat32(bounds.width, renderer.scaleValue(190))
	filterBounds := rectangle{x: bounds.x, y: bounds.y, width: filterWidth, height: renderer.scaleValue(22)}
	if renderer.phase7Button(filterBounds, "Filter: "+emptyLabel(state.Filter, "all immutable runs"), true) {
		filters := []string{"", "phase8-a", "phase8-b", "phase8-c"}
		next := filters[0]
		for index, filter := range filters {
			if filter == state.Filter {
				next = filters[(index+1)%len(filters)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetResultsFilterAction{Filter: next})
	}
	y := bounds.y + renderer.scaleValue(28)
	rowHeight := renderer.scaleValue(45)
	visible := max(0, int((bounds.height-renderer.scaleValue(28))/rowHeight))
	for index, detail := range state.Snapshot.Runs {
		if index >= visible {
			break
		}
		row := rectangle{x: bounds.x, y: y + float32(index)*rowHeight, width: bounds.width, height: rowHeight}
		selected := containsRunID(state.SelectedRunIDs, detail.Summary.ID)
		if selected {
			renderer.rect(row, tokenRaised)
			renderer.rect(rectangle{x: row.x, y: row.y, width: renderer.scaleValue(2), height: row.height}, tokenPurple)
		}
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) {
			renderer.actions = append(renderer.actions, appstate.SelectResultRunAction{RunID: detail.Summary.ID, Toggle: renderer.input.shiftDown})
		}
		renderer.text(renderer.window.monoFont, string(detail.Summary.ID), point{x: row.x + renderer.scaleValue(7), y: row.y + renderer.scaleValue(6)}, 9, tokenText)
		renderer.text(renderer.window.interFont, "immutable  "+detail.Summary.CompletedAt.Format("2006-01-02 15:04"), point{x: row.x + renderer.scaleValue(7), y: row.y + renderer.scaleValue(23)}, 8, tokenPositive)
		renderer.line(point{x: row.x, y: row.y + row.height - 1}, point{x: row.x + row.width, y: row.y + row.height - 1}, 1, tokenDivider)
	}
}

func (renderer *shellRenderer) drawMetricComparison(state appstate.ResultsWorkspaceState, bounds rectangle) {
	runs := selectedResultRuns(state)
	if len(runs) == 0 {
		renderer.emptyDock(bounds, "Select one or more runs")
		return
	}
	columnWidth := bounds.width / float32(len(runs)+1)
	renderer.text(renderer.window.interFont, "Metric", point{x: bounds.x + renderer.scaleValue(5), y: bounds.y + renderer.scaleValue(5)}, 9, tokenMuted)
	for index, run := range runs {
		renderer.text(renderer.window.monoFont, clipText(string(run.Summary.ID), 18), point{x: bounds.x + float32(index+1)*columnWidth + renderer.scaleValue(4), y: bounds.y + renderer.scaleValue(5)}, 8, tokenPurple)
	}
	metricKeys := []struct {
		name      string
		partition appstate.MetricPartition
	}{{"mse", appstate.MetricValidation}, {"mae", appstate.MetricValidation}, {"pearson_ic", appstate.MetricValidation}, {"pearson_ic", appstate.MetricTest}, {"sharpe", appstate.MetricTest}, {"max_drawdown", appstate.MetricTest}}
	rowHeight := renderer.scaleValue(27)
	for rowIndex, key := range metricKeys {
		y := bounds.y + renderer.scaleValue(24) + float32(rowIndex)*rowHeight
		label := string(key.partition) + " " + key.name
		labelColor := tokenCyan
		if key.partition == appstate.MetricTest {
			label = "TEST  " + key.name
			labelColor = tokenWarning
		}
		renderer.text(renderer.window.interFont, clipText(label, 24), point{x: bounds.x + renderer.scaleValue(5), y: y + renderer.scaleValue(6)}, 8, labelColor)
		for runIndex, run := range runs {
			metric, found := resultMetric(run.Summary.Metrics, key.name, key.partition)
			value := "--"
			if found && !metric.Missing {
				value = fmt.Sprintf("%.4f", metric.Value)
			}
			renderer.text(renderer.window.monoFont, value, point{x: bounds.x + float32(runIndex+1)*columnWidth + renderer.scaleValue(4), y: y + renderer.scaleValue(6)}, 8, labelColor)
		}
		renderer.line(point{x: bounds.x, y: y + rowHeight - 1}, point{x: bounds.x + bounds.width, y: y + rowHeight - 1}, 1, tokenDivider)
	}
	renderer.text(renderer.window.interFont, "Validation selects; TEST remains visually isolated from tuning.", point{x: bounds.x + renderer.scaleValue(5), y: bounds.y + bounds.height - renderer.scaleValue(16)}, 8, tokenWarning)
}

func (renderer *shellRenderer) drawResultEquity(state appstate.ResultsWorkspaceState, bounds rectangle) {
	runs := selectedResultRuns(state)
	if len(runs) == 0 {
		renderer.emptyDock(bounds, "Select a run")
		return
	}
	equityBounds := rectangle{x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height * 0.68}
	drawdownBounds := rectangle{x: bounds.x, y: bounds.y + bounds.height*0.72, width: bounds.width, height: bounds.height * 0.28}
	renderer.drawResultSeries(equityBounds, runs, func(run appstate.RunResultDetail) []appstate.ResultSeriesPoint { return run.Equity }, []colorValue{tokenCyan, tokenPurple, tokenPositive, tokenWarning})
	renderer.drawResultSeries(drawdownBounds, runs, func(run appstate.RunResultDetail) []appstate.ResultSeriesPoint { return run.Drawdown }, []colorValue{tokenNegative, tokenPurple, tokenWarning, tokenCyan})
	renderer.text(renderer.window.interFont, "Equity", point{x: equityBounds.x + renderer.scaleValue(5), y: equityBounds.y + renderer.scaleValue(4)}, 8, tokenMuted)
	renderer.text(renderer.window.interFont, "Drawdown", point{x: drawdownBounds.x + renderer.scaleValue(5), y: drawdownBounds.y + renderer.scaleValue(4)}, 8, tokenMuted)
}

func (renderer *shellRenderer) drawResultSeries(bounds rectangle, runs []appstate.RunResultDetail, values func(appstate.RunResultDetail) []appstate.ResultSeriesPoint, colors []colorValue) {
	minimum, maximum, initialized := 0.0, 0.0, false
	for _, run := range runs {
		for _, item := range values(run) {
			if item.Missing {
				continue
			}
			if !initialized {
				minimum, maximum, initialized = item.Value, item.Value, true
			}
			minimum, maximum = math.Min(minimum, item.Value), math.Max(maximum, item.Value)
		}
	}
	if !initialized {
		return
	}
	span := maximum - minimum
	if span == 0 {
		span = 1
	}
	plot := inset(bounds, renderer.scaleValue(5))
	renderer.outline(bounds, tokenDivider)
	for runIndex, run := range runs {
		series := values(run)
		for index := 0; index+1 < len(series); index++ {
			if series[index].Missing || series[index+1].Missing {
				continue
			}
			x1 := plot.x + plot.width*float32(index)/float32(len(series)-1)
			x2 := plot.x + plot.width*float32(index+1)/float32(len(series)-1)
			y1 := plot.y + plot.height*(1-float32((series[index].Value-minimum)/span))
			y2 := plot.y + plot.height*(1-float32((series[index+1].Value-minimum)/span))
			renderer.line(point{x: x1, y: y1}, point{x: x2, y: y2}, 1.4, colors[runIndex%len(colors)])
		}
	}
}

func (renderer *shellRenderer) drawFoldTimeline(state appstate.ResultsWorkspaceState, bounds rectangle) {
	run, found := primaryResultRun(state)
	if !found || len(run.Folds) == 0 {
		renderer.emptyDock(bounds, "Select a run with folds")
		return
	}
	rowHeight := bounds.height / float32(len(run.Folds))
	for index, fold := range run.Folds {
		y := bounds.y + float32(index)*rowHeight
		renderer.text(renderer.window.monoFont, fmt.Sprintf("F%d", fold.Index), point{x: bounds.x, y: y + renderer.scaleValue(5)}, 8, tokenText)
		track := rectangle{x: bounds.x + renderer.scaleValue(30), y: y + renderer.scaleValue(5), width: bounds.width - renderer.scaleValue(35), height: maxFloat32(renderer.scaleValue(12), rowHeight-renderer.scaleValue(10))}
		renderer.rect(rectangle{x: track.x, y: track.y, width: track.width * 0.62, height: track.height}, withAlpha(tokenCyan, 90))
		renderer.rect(rectangle{x: track.x + track.width*0.62, y: track.y, width: track.width * 0.18, height: track.height}, withAlpha(tokenPurple, 130))
		renderer.rect(rectangle{x: track.x + track.width*0.80, y: track.y, width: track.width * 0.20, height: track.height}, withAlpha(tokenWarning, 150))
	}
	legend := "train  validation  TEST   purge/embargo applied"
	renderer.text(renderer.window.interFont, legend, point{x: bounds.x + renderer.scaleValue(34), y: bounds.y + bounds.height - renderer.scaleValue(14)}, 8, tokenMuted)
}

func (renderer *shellRenderer) drawPredictionOverlay(state appstate.ResultsWorkspaceState, bounds rectangle) {
	run, found := primaryResultRun(state)
	if !found || len(run.Overlay) < 2 {
		renderer.emptyDock(bounds, "No prediction overlay")
		return
	}
	minimum, maximum := run.Overlay[0].Prediction, run.Overlay[0].Prediction
	for _, item := range run.Overlay {
		minimum = math.Min(minimum, math.Min(item.Prediction, item.Market))
		maximum = math.Max(maximum, math.Max(item.Prediction, item.Market))
	}
	span := maximum - minimum
	if span == 0 {
		span = 1
	}
	plot := inset(bounds, renderer.scaleValue(5))
	renderer.outline(bounds, tokenDivider)
	for index := 0; index+1 < len(run.Overlay); index++ {
		first, second := run.Overlay[index], run.Overlay[index+1]
		x1 := plot.x + plot.width*float32(index)/float32(len(run.Overlay)-1)
		x2 := plot.x + plot.width*float32(index+1)/float32(len(run.Overlay)-1)
		predictionY1 := plot.y + plot.height*(1-float32((first.Prediction-minimum)/span))
		predictionY2 := plot.y + plot.height*(1-float32((second.Prediction-minimum)/span))
		marketY1 := plot.y + plot.height*(1-float32((first.Market-minimum)/span))
		marketY2 := plot.y + plot.height*(1-float32((second.Market-minimum)/span))
		renderer.line(point{x: x1, y: predictionY1}, point{x: x2, y: predictionY2}, 1.4, tokenCyan)
		renderer.line(point{x: x1, y: marketY1}, point{x: x2, y: marketY2}, 1.1, tokenPurple)
	}
	renderer.text(renderer.window.interFont, "prediction", point{x: bounds.x + renderer.scaleValue(6), y: bounds.y + renderer.scaleValue(5)}, 8, tokenCyan)
	renderer.text(renderer.window.interFont, "next-open realized", point{x: bounds.x + renderer.scaleValue(80), y: bounds.y + renderer.scaleValue(5)}, 8, tokenPurple)
}

func (renderer *shellRenderer) drawResultDistributions(state appstate.ResultsWorkspaceState, bounds rectangle) {
	run, found := primaryResultRun(state)
	if !found {
		renderer.emptyDock(bounds, "Select a run")
		return
	}
	left := rectangle{x: bounds.x, y: bounds.y, width: bounds.width * 0.49, height: bounds.height}
	right := rectangle{x: bounds.x + bounds.width*0.51, y: bounds.y, width: bounds.width * 0.49, height: bounds.height}
	renderer.drawHistogram(left, run.InformationCoefficient, tokenCyan, "IC distribution")
	renderer.drawHistogram(right, run.Predictions, tokenPurple, "Prediction distribution")
}

func (renderer *shellRenderer) drawHistogram(bounds rectangle, bins []appstate.HistogramBin, color colorValue, label string) {
	if len(bins) == 0 {
		return
	}
	maximum := uint64(1)
	for _, bin := range bins {
		if bin.Count > maximum {
			maximum = bin.Count
		}
	}
	plot := rectangle{x: bounds.x + renderer.scaleValue(4), y: bounds.y + renderer.scaleValue(18), width: bounds.width - renderer.scaleValue(8), height: bounds.height - renderer.scaleValue(22)}
	barWidth := plot.width / float32(len(bins))
	for index, bin := range bins {
		height := plot.height * float32(bin.Count) / float32(maximum)
		renderer.rect(rectangle{x: plot.x + float32(index)*barWidth + 1, y: plot.y + plot.height - height, width: maxFloat32(1, barWidth-2), height: height}, withAlpha(color, 185))
	}
	renderer.outline(bounds, tokenDivider)
	renderer.text(renderer.window.interFont, label, point{x: bounds.x + renderer.scaleValue(5), y: bounds.y + renderer.scaleValue(4)}, 8, color)
}

func (renderer *shellRenderer) drawConfigurationDiff(state appstate.ResultsWorkspaceState, bounds rectangle) {
	runs := selectedResultRuns(state)
	if len(runs) == 0 {
		renderer.emptyDock(bounds, "Select runs to diff")
		return
	}
	paths := configurationPaths(runs)
	columnWidth := bounds.width / float32(len(runs)+1)
	renderer.text(renderer.window.interFont, "Configuration", point{x: bounds.x + renderer.scaleValue(4), y: bounds.y + renderer.scaleValue(4)}, 8, tokenMuted)
	for index, run := range runs {
		renderer.text(renderer.window.monoFont, clipText(string(run.Summary.ID), 15), point{x: bounds.x + float32(index+1)*columnWidth + renderer.scaleValue(3), y: bounds.y + renderer.scaleValue(4)}, 8, tokenPurple)
	}
	rowHeight := renderer.scaleValue(25)
	for rowIndex, path := range paths {
		y := bounds.y + renderer.scaleValue(22) + float32(rowIndex)*rowHeight
		if y+rowHeight > bounds.y+bounds.height {
			break
		}
		renderer.text(renderer.window.monoFont, clipText(path, 25), point{x: bounds.x + renderer.scaleValue(4), y: y + renderer.scaleValue(5)}, 8, tokenMuted)
		values := make([]string, len(runs))
		for index, run := range runs {
			values[index] = configurationValue(run.Configuration, path)
		}
		different := !allEqual(values)
		for index, value := range values {
			color := tokenText
			if different {
				color = tokenWarning
			}
			renderer.text(renderer.window.monoFont, clipText(value, 18), point{x: bounds.x + float32(index+1)*columnWidth + renderer.scaleValue(3), y: y + renderer.scaleValue(5)}, 8, color)
		}
		renderer.line(point{x: bounds.x, y: y + rowHeight - 1}, point{x: bounds.x + bounds.width, y: y + rowHeight - 1}, 1, tokenDivider)
	}
}

func selectedJob(state appstate.JobsWorkspaceState) (appstate.JobDetail, bool) {
	for _, detail := range state.Snapshot.Jobs {
		if detail.Summary.ID == state.SelectedJobID {
			return detail.Clone(), true
		}
	}
	return appstate.JobDetail{}, false
}

func selectedResultRuns(state appstate.ResultsWorkspaceState) []appstate.RunResultDetail {
	output := make([]appstate.RunResultDetail, 0, len(state.SelectedRunIDs))
	for _, selected := range state.SelectedRunIDs {
		for _, detail := range state.Snapshot.Runs {
			if detail.Summary.ID == selected {
				output = append(output, detail.Clone())
				break
			}
		}
	}
	return output
}

func primaryResultRun(state appstate.ResultsWorkspaceState) (appstate.RunResultDetail, bool) {
	for _, detail := range state.Snapshot.Runs {
		if detail.Summary.ID == state.PrimaryRunID {
			return detail.Clone(), true
		}
	}
	runs := selectedResultRuns(state)
	if len(runs) > 0 {
		return runs[0], true
	}
	return appstate.RunResultDetail{}, false
}

func containsRunID(ids []appstate.RunID, target appstate.RunID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func resultMetric(metrics []appstate.MetricSummary, name string, partition appstate.MetricPartition) (appstate.MetricSummary, bool) {
	for _, metric := range metrics {
		if metric.Name == name && metric.Partition == partition {
			return metric, true
		}
	}
	return appstate.MetricSummary{}, false
}

func configurationPaths(runs []appstate.RunResultDetail) []string {
	seen := make(map[string]struct{})
	var output []string
	for _, run := range runs {
		for _, value := range run.Configuration {
			path := value.Section + "." + value.Path
			if _, found := seen[path]; found {
				continue
			}
			seen[path] = struct{}{}
			output = append(output, path)
		}
	}
	return output
}

func configurationValue(configuration []appstate.ConfigurationValue, path string) string {
	for _, value := range configuration {
		if value.Section+"."+value.Path == path {
			return value.Value
		}
	}
	return "--"
}

func allEqual(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index] != values[0] {
			return false
		}
	}
	return true
}

func emptyLabel(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func jobLogColor(level appstate.JobLogLevel) colorValue {
	switch level {
	case appstate.JobLogError:
		return tokenNegative
	case appstate.JobLogWarn:
		return tokenWarning
	case appstate.JobLogDebug:
		return tokenMuted
	default:
		return tokenCyan
	}
}

func maxFloat(first float64, second float64) float64 {
	if first > second {
		return first
	}
	return second
}
