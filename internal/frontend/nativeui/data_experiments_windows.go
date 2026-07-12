package nativeui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func isDataPanel(panelType appstate.PanelType) bool {
	switch panelType {
	case appstate.PanelCatalogTable, appstate.PanelCoverageTimeline, appstate.PanelImportQueue,
		appstate.PanelDatasetInspector, appstate.PanelImportLogs:
		return true
	default:
		return false
	}
}

func isExperimentPanel(panelType appstate.PanelType) bool {
	switch panelType {
	case appstate.PanelExperimentList, appstate.PanelConfigurationTree, appstate.PanelPropertyEditor,
		appstate.PanelExperimentInspector, appstate.PanelValidationSummary, appstate.PanelResourceEstimate:
		return true
	default:
		return false
	}
}

func (renderer *shellRenderer) drawDataPanel(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	content := bounds
	content.y += renderer.scaleValue(24)
	content.height -= renderer.scaleValue(24)
	renderer.drawDataScenarioControl(state.Data, bounds)
	if state.Data.State == appstate.WorkspaceLoading && !state.Data.Stale {
		renderer.emptyDock(content, "Loading deterministic catalog and import state...")
		return
	}
	if (state.Data.State == appstate.WorkspaceFailed || state.Data.State == appstate.WorkspaceCancelled || state.Data.State == appstate.WorkspaceBusy) && !state.Data.Stale {
		renderer.drawDataFailure(state.Data, content)
		return
	}
	if state.Data.State == appstate.WorkspaceEmpty {
		renderer.emptyDock(content, "The deterministic Data catalog is empty")
		return
	}
	switch panel.Type {
	case appstate.PanelCatalogTable:
		renderer.drawDataCatalog(state.Data, content)
	case appstate.PanelCoverageTimeline:
		renderer.drawDataCoverage(state.Data, content)
	case appstate.PanelImportQueue:
		renderer.drawDataImports(state, content)
	case appstate.PanelDatasetInspector:
		renderer.drawDatasetInspector(state.Data, content)
	case appstate.PanelImportLogs:
		renderer.drawDataLogs(state.Data, content)
	}
	if state.Data.Stale || state.Data.State == appstate.WorkspaceDegraded || state.Data.State == appstate.WorkspaceRecovered {
		renderer.drawPhase7Status(string(state.Data.State), state.Data.Stale, bounds)
	}
}

func (renderer *shellRenderer) drawDataScenarioControl(state appstate.DataWorkspaceState, bounds rectangle) {
	label := "Scenario: " + string(state.Scenario)
	button := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(138), y: bounds.y, width: renderer.scaleValue(134), height: renderer.scaleValue(20)}
	if renderer.phase7Button(button, label, true) {
		scenarios := appstate.DataScenarios()
		next := scenarios[0]
		for index, scenario := range scenarios {
			if scenario == state.Scenario {
				next = scenarios[(index+1)%len(scenarios)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetDataScenarioAction{Scenario: next})
	}
}

func (renderer *shellRenderer) drawDataFailure(state appstate.DataWorkspaceState, bounds rectangle) {
	message := state.Error.Message
	if message == "" {
		message = "Data request failed"
	}
	renderer.textBlock(bounds, []string{string(state.State), message, "Retry requests a fresh immutable generation."})
	retry := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(72), width: renderer.scaleValue(84), height: renderer.scaleValue(24)}
	if renderer.phase7Button(retry, "Retry", true) {
		generation := state.Generation + 1
		renderer.actions = append(renderer.actions, appstate.RequestDataWorkspaceAction{Query: appstate.DataWorkspaceQuery{
			CorrelationID: appstate.CorrelationID(fmt.Sprintf("data-%020d", generation)), Generation: generation, Scenario: appstate.DataScenarioNormal,
		}})
	}
}

func (renderer *shellRenderer) drawDataCatalog(state appstate.DataWorkspaceState, bounds rectangle) {
	columns := []struct {
		label string
		x     float32
	}{{"Dataset", 0}, {"Status", 0.42}, {"Rows", 0.62}, {"Revision", 0.79}}
	header := rectangle{x: bounds.x, y: bounds.y, width: bounds.width, height: renderer.scaleValue(24)}
	renderer.rect(header, tokenRaised)
	for _, column := range columns {
		renderer.text(renderer.window.interFont, column.label, point{x: bounds.x + bounds.width*column.x + renderer.scaleValue(6), y: bounds.y + renderer.scaleValue(6)}, 10, tokenMuted)
	}
	rowHeight := renderer.scaleValue(30)
	visible := max(0, int((bounds.height-header.height)/rowHeight))
	for index, dataset := range state.Snapshot.Catalog {
		if index >= visible {
			break
		}
		row := rectangle{x: bounds.x, y: bounds.y + header.height + float32(index)*rowHeight, width: bounds.width, height: rowHeight}
		selected := dataset.ID == state.SelectedDatasetID
		if selected {
			renderer.rect(row, tokenRaised)
			renderer.rect(rectangle{x: row.x, y: row.y, width: renderer.scaleValue(2), height: row.height}, tokenCyan)
		}
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) {
			renderer.actions = append(renderer.actions, appstate.SelectDataDatasetAction{DatasetID: dataset.ID})
		}
		renderer.text(renderer.window.interFont, clipText(dataset.Name, 28), point{x: row.x + renderer.scaleValue(6), y: row.y + renderer.scaleValue(8)}, 10, tokenText)
		renderer.text(renderer.window.monoFont, string(dataset.Status), point{x: row.x + row.width*0.42 + renderer.scaleValue(6), y: row.y + renderer.scaleValue(8)}, 9, phase7StateColor(string(dataset.Status)))
		renderer.text(renderer.window.monoFont, strconv.FormatUint(dataset.Rows, 10), point{x: row.x + row.width*0.62 + renderer.scaleValue(6), y: row.y + renderer.scaleValue(8)}, 9, tokenText)
		renderer.text(renderer.window.monoFont, strconv.FormatUint(dataset.Revision, 10), point{x: row.x + row.width*0.79 + renderer.scaleValue(6), y: row.y + renderer.scaleValue(8)}, 9, tokenPurple)
		renderer.line(point{x: row.x, y: row.y + row.height - 1}, point{x: row.x + row.width, y: row.y + row.height - 1}, 1, tokenDivider)
	}
}

func (renderer *shellRenderer) drawDataCoverage(state appstate.DataWorkspaceState, bounds rectangle) {
	if len(state.Snapshot.Coverage) == 0 {
		renderer.emptyDock(bounds, "No symbol coverage")
		return
	}
	minimum := state.Snapshot.Coverage[0].Start
	maximum := state.Snapshot.Coverage[0].End
	for _, coverage := range state.Snapshot.Coverage {
		if coverage.Start.Before(minimum) {
			minimum = coverage.Start
		}
		if coverage.End.After(maximum) {
			maximum = coverage.End
		}
	}
	span := maximum.Sub(minimum)
	rowHeight := renderer.scaleValue(26)
	visible := max(0, int(bounds.height/rowHeight))
	for index, coverage := range state.Snapshot.Coverage {
		if index >= visible {
			break
		}
		y := bounds.y + float32(index)*rowHeight
		renderer.text(renderer.window.monoFont, string(coverage.Symbol), point{x: bounds.x, y: y + renderer.scaleValue(6)}, 9, tokenText)
		track := rectangle{x: bounds.x + renderer.scaleValue(54), y: y + renderer.scaleValue(7), width: bounds.width - renderer.scaleValue(62), height: renderer.scaleValue(10)}
		renderer.rect(track, tokenBackground)
		start, width := float32(0), track.width
		if span > 0 {
			start = float32(float64(coverage.Start.Sub(minimum))/float64(span)) * track.width
			width = float32(float64(coverage.End.Sub(coverage.Start))/float64(span)) * track.width
		}
		renderer.rect(rectangle{x: track.x + start, y: track.y, width: maxFloat32(renderer.scaleValue(2), width), height: track.height}, withAlpha(tokenCyan, 180))
	}
}

func (renderer *shellRenderer) drawDataImports(state appstate.AppState, bounds rectangle) {
	buttonWidth := renderer.scaleValue(112)
	button := rectangle{x: bounds.x + bounds.width - buttonWidth, y: bounds.y, width: buttonWidth, height: renderer.scaleValue(24)}
	if renderer.phase7Button(button, "Import demo", true) && state.Data.SelectedDatasetID != "" {
		var dataset appstate.DatasetSummary
		for _, candidate := range state.Data.Snapshot.Catalog {
			if candidate.ID == state.Data.SelectedDatasetID {
				dataset = candidate
				break
			}
		}
		generation := state.Data.Generation + 1
		mode := appstate.DataImportAppend
		replacement := appstate.TimeRange{}
		if state.Data.Scenario == appstate.DataScenarioRecovered {
			mode = appstate.DataImportReplacement
			replacement = appstate.TimeRange{Start: dataset.End.AddDate(0, -1, 0), End: dataset.End}.Normalize()
		}
		request := appstate.DataImportRequest{
			CorrelationID: appstate.CorrelationID(fmt.Sprintf("data-import-%020d", generation)),
			CommandID:     appstate.CorrelationID(fmt.Sprintf("import-%020d", generation)), Generation: generation,
			DatasetID: dataset.ID, SourceName: "deterministic-bars." + string(appstate.DataSourceCSV), SourceKind: appstate.DataSourceCSV,
			Mode: mode, Symbols: append([]appstate.Symbol(nil), dataset.Symbols...), Interval: dataset.Interval,
			TimeRange: replacement, Adjustment: "split_dividend_adjusted", Scenario: state.Data.Scenario,
		}
		renderer.actions = append(renderer.actions, appstate.SubmitDataImportAction{Request: request})
	}
	renderer.text(renderer.window.interFont, fmt.Sprintf("%d immutable import records", len(state.Data.Snapshot.Imports)), point{x: bounds.x, y: bounds.y + renderer.scaleValue(7)}, 10, tokenMuted)
	y := bounds.y + renderer.scaleValue(34)
	rowHeight := renderer.scaleValue(30)
	visible := max(0, int((bounds.height-renderer.scaleValue(34))/rowHeight))
	for index, record := range state.Data.Snapshot.Imports {
		if index >= visible {
			break
		}
		row := rectangle{x: bounds.x, y: y + float32(index)*rowHeight, width: bounds.width, height: rowHeight}
		if record.ID == state.Data.SelectedImportID {
			renderer.rect(row, tokenRaised)
		}
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) {
			renderer.actions = append(renderer.actions, appstate.SelectDataImportAction{ImportID: record.ID})
		}
		renderer.text(renderer.window.monoFont, clipText(record.ID, 26), point{x: row.x + renderer.scaleValue(5), y: row.y + renderer.scaleValue(8)}, 9, tokenText)
		renderer.text(renderer.window.monoFont, string(record.State), point{x: row.x + row.width*0.5, y: row.y + renderer.scaleValue(8)}, 9, phase7StateColor(string(record.State)))
		renderer.text(renderer.window.monoFont, string(record.Request.Mode), point{x: row.x + row.width*0.72, y: row.y + renderer.scaleValue(8)}, 9, tokenMuted)
	}
}

func (renderer *shellRenderer) drawDatasetInspector(state appstate.DataWorkspaceState, bounds rectangle) {
	for _, dataset := range state.Snapshot.Catalog {
		if dataset.ID != state.SelectedDatasetID {
			continue
		}
		lines := []string{
			"Dataset  " + dataset.Name,
			"ID       " + string(dataset.ID),
			"Revision " + strconv.FormatUint(dataset.Revision, 10),
			"Rows     " + strconv.FormatUint(dataset.Rows, 10),
			"Interval " + string(dataset.Interval),
			"Symbols  " + symbolsText(dataset.Symbols),
			"Range    " + dataset.Start.Format("2006-01-02") + " to " + dataset.End.Format("2006-01-02"),
			"Adjusted " + dataset.Adjustment,
			"Hash     " + clipText(dataset.Fingerprint, 38),
		}
		renderer.textBlock(bounds, lines)
		return
	}
	renderer.emptyDock(bounds, "Select a catalog dataset")
}

func (renderer *shellRenderer) drawDataLogs(state appstate.DataWorkspaceState, bounds rectangle) {
	rowHeight := renderer.scaleValue(25)
	visible := max(0, int(bounds.height/rowHeight))
	for index, entry := range state.Snapshot.Logs {
		if index >= visible {
			break
		}
		y := bounds.y + float32(index)*rowHeight
		renderer.text(renderer.window.monoFont, entry.Timestamp.Format("15:04:05"), point{x: bounds.x, y: y + renderer.scaleValue(5)}, 9, tokenMuted)
		renderer.text(renderer.window.interFont, clipText(entry.Message, 70), point{x: bounds.x + renderer.scaleValue(68), y: y + renderer.scaleValue(5)}, 9, phase7SeverityColor(entry.Level))
	}
	if len(state.Snapshot.Logs) == 0 {
		renderer.emptyDock(bounds, "No import log entries yet")
	}
}

func (renderer *shellRenderer) drawExperimentPanel(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	content := bounds
	content.y += renderer.scaleValue(24)
	content.height -= renderer.scaleValue(24)
	renderer.drawExperimentScenarioControl(state.Experiments, bounds)
	if state.Experiments.State == appstate.WorkspaceLoading && !state.Experiments.Stale {
		renderer.emptyDock(content, "Loading deterministic experiment definitions...")
		return
	}
	if (state.Experiments.State == appstate.WorkspaceFailed || state.Experiments.State == appstate.WorkspaceCancelled || state.Experiments.State == appstate.WorkspaceBusy) && !state.Experiments.Stale {
		renderer.drawExperimentFailure(state.Experiments, content)
		return
	}
	switch panel.Type {
	case appstate.PanelExperimentList:
		renderer.drawExperimentList(state.Experiments, content)
	case appstate.PanelConfigurationTree:
		renderer.drawExperimentTree(state.Experiments, content)
	case appstate.PanelPropertyEditor:
		renderer.drawExperimentProperties(state, content)
	case appstate.PanelExperimentInspector:
		renderer.drawExperimentInspector(state.Experiments, content)
	case appstate.PanelValidationSummary:
		renderer.drawExperimentValidation(state.Experiments, content)
	case appstate.PanelResourceEstimate:
		renderer.drawExperimentResources(state.Experiments, content)
	}
	if state.Experiments.Stale || state.Experiments.State == appstate.WorkspaceDegraded || state.Experiments.State == appstate.WorkspaceRecovered {
		renderer.drawPhase7Status(string(state.Experiments.State), state.Experiments.Stale, bounds)
	}
}

func (renderer *shellRenderer) drawExperimentScenarioControl(state appstate.ExperimentsWorkspaceState, bounds rectangle) {
	button := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(146), y: bounds.y, width: renderer.scaleValue(142), height: renderer.scaleValue(20)}
	if renderer.phase7Button(button, "Scenario: "+string(state.Scenario), true) {
		scenarios := appstate.ExperimentScenarios()
		next := scenarios[0]
		for index, scenario := range scenarios {
			if scenario == state.Scenario {
				next = scenarios[(index+1)%len(scenarios)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetExperimentScenarioAction{Scenario: next})
	}
}

func (renderer *shellRenderer) drawExperimentFailure(state appstate.ExperimentsWorkspaceState, bounds rectangle) {
	message := state.Error.Message
	if message == "" {
		message = "Experiment request failed"
	}
	renderer.textBlock(bounds, []string{string(state.State), message, "Retry keeps the local draft and requests a fresh generation."})
	retry := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(72), width: renderer.scaleValue(84), height: renderer.scaleValue(24)}
	if renderer.phase7Button(retry, "Retry", true) {
		generation := state.Generation + 1
		renderer.actions = append(renderer.actions, appstate.RequestExperimentsAction{Query: appstate.ExperimentQuery{
			CorrelationID: appstate.CorrelationID(fmt.Sprintf("experiments-%020d", generation)), Generation: generation, Scenario: appstate.ExperimentScenarioNormal,
		}})
	}
}

func (renderer *shellRenderer) drawExperimentList(state appstate.ExperimentsWorkspaceState, bounds rectangle) {
	rowHeight := renderer.scaleValue(38)
	visible := max(0, int(bounds.height/rowHeight))
	for index, definition := range state.Snapshot.Definitions {
		if index >= visible {
			break
		}
		row := rectangle{x: bounds.x, y: bounds.y + float32(index)*rowHeight, width: bounds.width, height: rowHeight}
		if definition.ID == state.SelectedExperimentID {
			renderer.rect(row, tokenRaised)
			renderer.rect(rectangle{x: row.x, y: row.y, width: renderer.scaleValue(2), height: row.height}, tokenPurple)
		}
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) {
			renderer.actions = append(renderer.actions, appstate.SelectExperimentDefinitionAction{ExperimentID: definition.ID})
		}
		renderer.text(renderer.window.interFont, clipText(definition.Draft.Name, 28), point{x: row.x + renderer.scaleValue(6), y: row.y + renderer.scaleValue(6)}, 10, tokenText)
		renderer.text(renderer.window.monoFont, clipText(string(definition.ID), 34), point{x: row.x + renderer.scaleValue(6), y: row.y + renderer.scaleValue(22)}, 8, tokenMuted)
	}
	if len(state.Snapshot.Definitions) == 0 {
		renderer.emptyDock(bounds, "No immutable experiment definitions")
	}
}

func (renderer *shellRenderer) drawExperimentTree(state appstate.ExperimentsWorkspaceState, bounds rectangle) {
	rowHeight := renderer.scaleValue(30)
	for index, section := range appstate.ExperimentSections() {
		row := rectangle{x: bounds.x, y: bounds.y + float32(index)*rowHeight, width: bounds.width, height: rowHeight}
		if section == state.SelectedSection {
			renderer.rect(row, tokenRaised)
		}
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) {
			renderer.actions = append(renderer.actions, appstate.SelectExperimentSectionAction{Section: section})
		}
		renderer.text(renderer.window.interFont, titleSection(section), point{x: row.x + renderer.scaleValue(8), y: row.y + renderer.scaleValue(8)}, 10, activeColor(section == state.SelectedSection))
	}
}

func (renderer *shellRenderer) drawExperimentProperties(state appstate.AppState, bounds rectangle) {
	draft := state.Experiments.Draft.Clone()
	if draft.Revision == 0 {
		renderer.emptyDock(bounds, "Waiting for a dataset-backed draft")
		return
	}
	section := state.Experiments.SelectedSection
	renderer.text(renderer.window.interFont, "Editing "+titleSection(section), point{x: bounds.x, y: bounds.y}, 12, tokenCyan)
	y := bounds.y + renderer.scaleValue(28)
	line := func(label string, value string) {
		renderer.text(renderer.window.interFont, label, point{x: bounds.x, y: y}, 10, tokenMuted)
		renderer.text(renderer.window.monoFont, clipText(value, 54), point{x: bounds.x + renderer.scaleValue(112), y: y}, 10, tokenText)
		y += renderer.scaleValue(26)
	}
	switch section {
	case appstate.ExperimentSectionDataset:
		line("Name", draft.Name)
		line("Dataset", string(draft.DatasetID))
		line("Revision", strconv.FormatUint(draft.DatasetRevision, 10))
		line("Fingerprint", draft.DatasetFingerprint)
	case appstate.ExperimentSectionFeatures:
		line("Selected", featureText(draft.Features))
		line("Registry", "compiled Go descriptors only")
		if renderer.phase7Button(rectangle{x: bounds.x, y: y, width: renderer.scaleValue(126), height: renderer.scaleValue(24)}, "Toggle cross-rank", true) {
			draft.Features = toggleFeature(draft.Features, "cross_rank_1")
			renderer.emitDraftUpdate(state, draft)
		}
	case appstate.ExperimentSectionTarget:
		line("Kind", draft.Target.Kind)
		line("Horizon", fmt.Sprintf("%d bars", draft.Target.HorizonBars))
		if renderer.phase7Button(rectangle{x: bounds.x, y: y, width: renderer.scaleValue(116), height: renderer.scaleValue(24)}, "Cycle horizon", true) {
			if draft.Target.HorizonBars >= 20 {
				draft.Target.HorizonBars = 1
			} else {
				draft.Target.HorizonBars += 5
			}
			draft.Split.PurgeBars = max(draft.Split.PurgeBars, draft.Target.HorizonBars)
			renderer.emitDraftUpdate(state, draft)
		}
	case appstate.ExperimentSectionSplit:
		line("Kind", draft.Split.Kind)
		line("Train / Val / Test", fmt.Sprintf("%d / %d / %d", draft.Split.TrainBars, draft.Split.ValidationBars, draft.Split.TestBars))
		line("Purge / Embargo", fmt.Sprintf("%d / %d", draft.Split.PurgeBars, draft.Split.EmbargoBars))
	case appstate.ExperimentSectionModel:
		line("Kind", string(draft.Model.Kind))
		line("Depth", strconv.Itoa(draft.Model.MaxDepth))
		line("Estimators", strconv.Itoa(draft.Model.EstimatorCount))
		line("CPU slots", strconv.Itoa(draft.RequestedCPU))
		if renderer.phase7Button(rectangle{x: bounds.x, y: y, width: renderer.scaleValue(104), height: renderer.scaleValue(24)}, "Cycle model", true) {
			switch draft.Model.Kind {
			case appstate.ModelHistogramTree:
				draft.Model.Kind = appstate.ModelRandomForest
			case appstate.ModelRandomForest:
				draft.Model.Kind = appstate.ModelGradientBoost
			default:
				draft.Model.Kind = appstate.ModelHistogramTree
			}
			renderer.emitDraftUpdate(state, draft)
		}
	case appstate.ExperimentSectionPortfolio:
		line("Long / Short", fmt.Sprintf("%.2f / %.2f", draft.Portfolio.LongQuantile, draft.Portfolio.ShortQuantile))
		line("Cost", fmt.Sprintf("%.1f bps", draft.Portfolio.CostBPS))
	case appstate.ExperimentSectionSweep:
		line("Enabled", strconv.FormatBool(draft.Sweep.Enabled))
		line("Depth range", fmt.Sprintf("%d to %d", draft.Sweep.DepthMinimum, draft.Sweep.DepthMaximum))
		if renderer.phase7Button(rectangle{x: bounds.x, y: y, width: renderer.scaleValue(106), height: renderer.scaleValue(24)}, "Toggle sweep", true) {
			draft.Sweep.Enabled = !draft.Sweep.Enabled
			if draft.Sweep.Enabled {
				draft.Sweep.DepthMinimum, draft.Sweep.DepthMaximum, draft.Sweep.EstimatorStep = 4, 10, 2
			}
			renderer.emitDraftUpdate(state, draft)
		}
	}
	submit := rectangle{x: bounds.x, y: bounds.y + bounds.height - renderer.scaleValue(28), width: renderer.scaleValue(132), height: renderer.scaleValue(26)}
	valid := len(state.Experiments.Issues) == 0 && draft.Revision != 0
	if renderer.phase7Button(submit, "Submit immutable", valid) && valid {
		generation := max(uint64(1), state.Experiments.EvaluationGeneration)
		commandID := appstate.CorrelationID(fmt.Sprintf("experiment-%020d", draft.Revision))
		renderer.actions = append(renderer.actions, appstate.SubmitExperimentAction{Command: appstate.SubmitExperimentCommand{
			CorrelationID: appstate.CorrelationID("submit-" + string(commandID)), CommandID: commandID,
			Generation: generation, Draft: draft.Clone(), Scenario: state.Experiments.Scenario,
		}})
	}
}

func (renderer *shellRenderer) emitDraftUpdate(state appstate.AppState, draft appstate.ExperimentDraft) {
	draft.Revision = state.Experiments.Draft.Revision + 1
	renderer.actions = append(renderer.actions, appstate.UpdateExperimentDraftAction{Draft: draft, UpdatedAt: state.Connection.UpdatedAt})
}

func (renderer *shellRenderer) drawExperimentInspector(state appstate.ExperimentsWorkspaceState, bounds rectangle) {
	for _, definition := range state.Snapshot.Definitions {
		if definition.ID != state.SelectedExperimentID {
			continue
		}
		renderer.textBlock(bounds, []string{
			"Immutable definition",
			"ID       " + string(definition.ID),
			"Command  " + string(definition.CommandID),
			"Dataset  " + string(definition.Draft.DatasetID),
			"Revision " + strconv.FormatUint(definition.Draft.DatasetRevision, 10),
			"Features " + featureText(definition.Draft.Features),
			"Model    " + string(definition.Draft.Model.Kind),
			"Submitted " + definition.SubmittedAt.Format(time.RFC3339),
		})
		return
	}
	renderer.textBlock(bounds, []string{
		"Current autosaved draft",
		"Revision " + strconv.FormatUint(state.Draft.Revision, 10),
		"Dataset  " + string(state.Draft.DatasetID),
		"Features " + featureText(state.Draft.Features),
		"Autosave " + autosaveText(state.DraftPersistence),
	})
}

func (renderer *shellRenderer) drawExperimentValidation(state appstate.ExperimentsWorkspaceState, bounds rectangle) {
	if len(state.Issues) == 0 {
		renderer.textBlock(bounds, []string{"Valid", "No blocking configuration issues.", "Submission will freeze this exact draft revision."})
		return
	}
	rowHeight := renderer.scaleValue(42)
	visible := max(0, int(bounds.height/rowHeight))
	for index, issue := range state.Issues {
		if index >= visible {
			break
		}
		y := bounds.y + float32(index)*rowHeight
		renderer.text(renderer.window.monoFont, issue.Code, point{x: bounds.x, y: y}, 9, tokenNegative)
		renderer.text(renderer.window.interFont, clipText(issue.Message, 64), point{x: bounds.x, y: y + renderer.scaleValue(18)}, 9, tokenText)
	}
}

func (renderer *shellRenderer) drawExperimentResources(state appstate.ExperimentsWorkspaceState, bounds rectangle) {
	estimate := state.Estimate
	renderer.textBlock(bounds, []string{
		"Deterministic estimate",
		"Rows       " + strconv.FormatUint(estimate.Rows, 10),
		"Values     " + strconv.FormatUint(estimate.FeatureValues, 10),
		"Memory     " + formatBytes(estimate.EstimatedBytes),
		"CPU slots  " + strconv.Itoa(estimate.RequestedCPU),
		"Sweep runs " + strconv.Itoa(estimate.SweepCombinations),
		"Estimate   " + strconv.Itoa(estimate.EstimatedSeconds) + " seconds",
	})
}

func (renderer *shellRenderer) phase7Button(bounds rectangle, label string, enabled bool) bool {
	color := tokenPanel
	textColor := tokenMuted
	if enabled {
		color = tokenRaised
		textColor = tokenText
	}
	renderer.rect(bounds, color)
	renderer.outline(bounds, tokenDivider)
	renderer.text(renderer.window.interFont, clipText(label, 22), point{x: bounds.x + renderer.scaleValue(6), y: bounds.y + renderer.scaleValue(5)}, 9, textColor)
	return enabled && renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, bounds)
}

func (renderer *shellRenderer) drawPhase7Status(state string, stale bool, bounds rectangle) {
	message := state
	color := tokenCyan
	if stale {
		message = "refreshing - showing immutable stale generation"
		color = tokenWarning
	}
	status := rectangle{x: bounds.x + renderer.scaleValue(4), y: bounds.y + renderer.scaleValue(4), width: renderer.scaleValue(float32(len(message))*5 + 14), height: renderer.scaleValue(19)}
	renderer.rect(status, withAlpha(tokenPanel, 235))
	renderer.outline(status, color)
	renderer.text(renderer.window.interFont, message, point{x: status.x + renderer.scaleValue(6), y: status.y + renderer.scaleValue(4)}, 8, color)
}

func phase7StateColor(state string) colorValue {
	switch state {
	case "ready", "recovered":
		return tokenPositive
	case "failed", "rejected", "error", "cancelled":
		return tokenNegative
	case "validation", "validating", "degraded", "queue_saturated":
		return tokenWarning
	default:
		return tokenCyan
	}
}

func phase7SeverityColor(severity appstate.ValidationSeverity) colorValue {
	switch severity {
	case appstate.ValidationError:
		return tokenNegative
	case appstate.ValidationWarning:
		return tokenWarning
	default:
		return tokenText
	}
}

func symbolsText(symbols []appstate.Symbol) string {
	parts := make([]string, len(symbols))
	for index, symbol := range symbols {
		parts[index] = string(symbol)
	}
	return strings.Join(parts, ", ")
}

func featureText(features []appstate.FeatureName) string {
	parts := make([]string, len(features))
	for index, feature := range features {
		parts[index] = string(feature)
	}
	return strings.Join(parts, ", ")
}

func toggleFeature(features []appstate.FeatureName, target appstate.FeatureName) []appstate.FeatureName {
	result := make([]appstate.FeatureName, 0, len(features)+1)
	found := false
	for _, feature := range features {
		if feature == target {
			found = true
			continue
		}
		result = append(result, feature)
	}
	if !found {
		result = append(result, target)
	}
	return result
}

func autosaveText(persistence appstate.PersistenceState) string {
	if persistence.PendingRevision != 0 {
		return "saving revision " + strconv.FormatUint(persistence.PendingRevision, 10)
	}
	if persistence.LastError.Code != "" {
		return "failed - retryable"
	}
	if persistence.LastSavedRevision != 0 {
		return "saved revision " + strconv.FormatUint(persistence.LastSavedRevision, 10)
	}
	return "not saved"
}

func formatBytes(value uint64) string {
	const mebibyte = 1024 * 1024
	if value >= mebibyte {
		return fmt.Sprintf("%.1f MiB", float64(value)/mebibyte)
	}
	return strconv.FormatUint(value, 10) + " B"
}

func titleSection(section appstate.ExperimentSection) string {
	value := string(section)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
