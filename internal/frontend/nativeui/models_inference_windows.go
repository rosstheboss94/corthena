package nativeui

import (
	"fmt"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func isModelsPanel(panelType appstate.PanelType) bool {
	switch panelType {
	case appstate.PanelModelRegistry, appstate.PanelAliasHistory, appstate.PanelArtifactMetadata,
		appstate.PanelFeatureImportance, appstate.PanelTreeInspector:
		return true
	default:
		return false
	}
}

func isInferencePanel(panelType appstate.PanelType) bool {
	switch panelType {
	case appstate.PanelModelSelector, appstate.PanelInferenceDataset, appstate.PanelRankedScores,
		appstate.PanelScoreDistribution, appstate.PanelPredictionHistory, appstate.PanelExportStatus:
		return true
	default:
		return false
	}
}

func (renderer *shellRenderer) drawModelsPanel(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	content := phase8Content(bounds, renderer.scaleValue(24))
	renderer.drawModelsScenarioControl(state.ModelsWorkspace, bounds)
	workspace := state.ModelsWorkspace
	if workspace.State == appstate.WorkspaceLoading && !workspace.Stale {
		renderer.emptyDock(content, "Loading immutable model registry...")
		return
	}
	if (workspace.State == appstate.WorkspaceFailed || workspace.State == appstate.WorkspaceCancelled || workspace.State == appstate.WorkspaceBusy) && !workspace.Stale {
		renderer.drawModelsFailure(workspace, content)
		return
	}
	if workspace.State == appstate.WorkspaceEmpty {
		renderer.emptyDock(content, "No complete final-refit artifacts match this registry view")
		return
	}
	switch panel.Type {
	case appstate.PanelModelRegistry:
		renderer.drawModelRegistry(workspace, content)
	case appstate.PanelAliasHistory:
		renderer.drawAliasHistory(workspace, content)
	case appstate.PanelArtifactMetadata:
		renderer.drawArtifactMetadata(workspace, content)
	case appstate.PanelFeatureImportance:
		renderer.drawFeatureImportance(workspace, content)
	case appstate.PanelTreeInspector:
		renderer.drawTreeInspector(workspace, content)
	}
	if workspace.Stale || workspace.State == appstate.WorkspaceDegraded || workspace.State == appstate.WorkspaceRecovered {
		renderer.drawPhase7Status(string(workspace.State), workspace.Stale, bounds)
	}
}

func (renderer *shellRenderer) drawInferencePanel(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	content := phase8Content(bounds, renderer.scaleValue(24))
	renderer.drawInferenceScenarioControl(state.InferenceWorkspace, bounds)
	workspace := state.InferenceWorkspace
	if workspace.State == appstate.WorkspaceLoading && !workspace.Stale {
		renderer.emptyDock(content, "Validating compatibility and preparing immutable scores...")
		return
	}
	if (workspace.State == appstate.WorkspaceFailed || workspace.State == appstate.WorkspaceCancelled || workspace.State == appstate.WorkspaceBusy) && !workspace.Stale {
		renderer.drawInferenceFailure(workspace, content)
		return
	}
	if !workspace.Snapshot.Compatibility.Compatible {
		renderer.drawCompatibilityFailure(workspace, content)
		return
	}
	if workspace.State == appstate.WorkspaceEmpty || !workspace.Snapshot.HasOutput {
		renderer.emptyDock(content, "No complete compatible inference output")
		return
	}
	switch panel.Type {
	case appstate.PanelModelSelector:
		renderer.drawInferenceSelector(state, content)
	case appstate.PanelInferenceDataset:
		renderer.drawInferenceDataset(state, content)
	case appstate.PanelRankedScores:
		renderer.drawRankedScores(workspace, content)
	case appstate.PanelScoreDistribution:
		renderer.drawInferenceDistribution(workspace, content)
	case appstate.PanelPredictionHistory:
		renderer.drawPredictionHistory(workspace, content)
	case appstate.PanelExportStatus:
		renderer.drawExportStatus(workspace, content)
	}
	if workspace.Stale || workspace.State == appstate.WorkspaceDegraded || workspace.State == appstate.WorkspaceRecovered {
		renderer.drawPhase7Status(string(workspace.State), workspace.Stale, bounds)
	}
}

func (renderer *shellRenderer) drawModelsScenarioControl(state appstate.ModelsWorkspaceState, bounds rectangle) {
	button := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(158), y: bounds.y, width: renderer.scaleValue(154), height: renderer.scaleValue(20)}
	if renderer.phase7Button(button, "Scenario: "+string(state.Scenario), true) {
		scenarios := appstate.ModelsScenarios()
		next := scenarios[0]
		for index, scenario := range scenarios {
			if scenario == state.Scenario {
				next = scenarios[(index+1)%len(scenarios)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetModelsScenarioAction{Scenario: next})
	}
}

func (renderer *shellRenderer) drawInferenceScenarioControl(state appstate.InferenceWorkspaceState, bounds rectangle) {
	button := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(166), y: bounds.y, width: renderer.scaleValue(162), height: renderer.scaleValue(20)}
	if renderer.phase7Button(button, "Scenario: "+string(state.Scenario), true) {
		scenarios := appstate.InferenceScenarios()
		next := scenarios[0]
		for index, scenario := range scenarios {
			if scenario == state.Scenario {
				next = scenarios[(index+1)%len(scenarios)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetInferenceScenarioAction{Scenario: next})
	}
}

func (renderer *shellRenderer) drawModelsFailure(state appstate.ModelsWorkspaceState, bounds rectangle) {
	message := state.Error.Message
	if message == "" {
		message = "Model registry request failed"
	}
	renderer.textBlock(bounds, []string{string(state.State), message, "Retry keeps the last immutable registry visible."})
	retry := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(72), width: renderer.scaleValue(84), height: renderer.scaleValue(24)}
	if renderer.phase7Button(retry, "Retry", true) {
		generation := state.Generation + 1
		renderer.actions = append(renderer.actions, appstate.RequestModelsWorkspaceAction{Query: appstate.ModelsWorkspaceQuery{
			CorrelationID: appstate.CorrelationID(fmt.Sprintf("models-%020d", generation)), Generation: generation, Scenario: appstate.ModelsScenarioNormal,
			Filter: state.Query.Filter, Page: state.Query.Page, PageSize: state.Query.PageSize,
		}})
	}
}

func (renderer *shellRenderer) drawInferenceFailure(state appstate.InferenceWorkspaceState, bounds rectangle) {
	message := state.Error.Message
	if message == "" {
		message = "Inference request failed"
	}
	renderer.textBlock(bounds, []string{string(state.State), message, "Retry starts a newer cancellation-safe generation."})
	retry := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(72), width: renderer.scaleValue(84), height: renderer.scaleValue(24)}
	if renderer.phase7Button(retry, "Retry", true) {
		if query, found := inferenceRetryQuery(state); found {
			renderer.actions = append(renderer.actions, appstate.RequestInferenceWorkspaceAction{Query: query})
		}
	}
}

func (renderer *shellRenderer) drawModelRegistry(state appstate.ModelsWorkspaceState, bounds rectangle) {
	if len(state.Snapshot.Registry) == 0 {
		renderer.emptyDock(bounds, "No immutable inference artifacts")
		return
	}
	rowHeight := renderer.scaleValue(43)
	for index, artifact := range state.Snapshot.Registry {
		if bounds.y+float32(index+1)*rowHeight > bounds.y+bounds.height {
			break
		}
		row := rectangle{x: bounds.x, y: bounds.y + float32(index)*rowHeight, width: bounds.width, height: rowHeight}
		selected := artifact.Summary.ID == state.SelectedModelID
		if selected {
			renderer.rect(row, tokenRaised)
			renderer.rect(rectangle{x: row.x, y: row.y, width: renderer.scaleValue(2), height: row.height}, tokenPurple)
		}
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) {
			renderer.actions = append(renderer.actions, appstate.SelectModelArtifactAction{ModelID: artifact.Summary.ID, Tree: 0})
		}
		alias := artifact.Summary.Alias
		if alias == "" {
			alias = "unaliased"
		}
		renderer.text(renderer.window.monoFont, clipText(string(artifact.Summary.ID), 34), point{x: row.x + renderer.scaleValue(6), y: row.y + renderer.scaleValue(5)}, 9, tokenText)
		renderer.text(renderer.window.interFont, string(artifact.Summary.Kind)+" · "+alias+" · final refit", point{x: row.x + renderer.scaleValue(6), y: row.y + renderer.scaleValue(22)}, 8, tokenCyan)
		renderer.line(point{x: row.x, y: row.y + row.height - 1}, point{x: row.x + row.width, y: row.y + row.height - 1}, 1, tokenDivider)
	}
}

func (renderer *shellRenderer) drawAliasHistory(state appstate.ModelsWorkspaceState, bounds rectangle) {
	artifact, found := selectedModelArtifact(state)
	if !found {
		renderer.emptyDock(bounds, "Select a completed artifact")
		return
	}
	stage := rectangle{x: bounds.x, y: bounds.y, width: renderer.scaleValue(126), height: renderer.scaleValue(22)}
	if renderer.phase7Button(stage, "Stage champion", !state.AwaitingConfirm) {
		generation := state.Generation
		renderer.actions = append(renderer.actions, appstate.BeginAliasAssignmentAction{Command: appstate.AliasAssignmentCommand{
			CorrelationID: appstate.CorrelationID("alias-" + string(artifact.Summary.ID)), CommandID: appstate.CorrelationID(fmt.Sprintf("alias-champion-%s-%020d-%04d", artifact.Summary.ID, generation, len(state.Snapshot.AliasHistory))),
			Generation: generation, Alias: "champion", ModelID: artifact.Summary.ID,
		}})
	}
	if state.AwaitingConfirm {
		confirm := rectangle{x: bounds.x + renderer.scaleValue(132), y: bounds.y, width: renderer.scaleValue(80), height: renderer.scaleValue(22)}
		if renderer.phase7Button(confirm, "Confirm", true) {
			renderer.actions = append(renderer.actions, appstate.ConfirmAliasAssignmentAction{CommandID: state.PendingAlias.CommandID})
		}
	}
	y := bounds.y + renderer.scaleValue(29)
	for index := len(state.Snapshot.AliasHistory) - 1; index >= 0; index-- {
		entry := state.Snapshot.AliasHistory[index]
		if y+renderer.scaleValue(31) > bounds.y+bounds.height {
			break
		}
		previous := string(entry.PreviousModelID)
		if previous == "" {
			previous = "none"
		}
		renderer.text(renderer.window.monoFont, entry.Alias+" → "+clipText(string(entry.ModelID), 24), point{x: bounds.x, y: y + renderer.scaleValue(4)}, 8, tokenPurple)
		renderer.text(renderer.window.interFont, "previous "+clipText(previous, 18)+" · "+entry.ChangedAt.Format("2006-01-02 15:04"), point{x: bounds.x, y: y + renderer.scaleValue(17)}, 8, tokenMuted)
		y += renderer.scaleValue(32)
	}
}

func (renderer *shellRenderer) drawArtifactMetadata(state appstate.ModelsWorkspaceState, bounds rectangle) {
	artifact, found := selectedModelArtifact(state)
	if !found {
		renderer.emptyDock(bounds, "Select a completed artifact")
		return
	}
	metadata := artifact.Metadata
	lines := []string{
		"schema     " + metadata.SchemaVersion,
		"engine     " + metadata.EngineVersion,
		"features   " + metadata.FeatureSchema,
		"target     " + metadata.Target.Kind + fmt.Sprintf(" / %d", metadata.Target.HorizonBars),
		"training   " + clipText(metadata.TrainingFingerprint, 30),
		"cutoff     " + metadata.TrainingCutoff.Format("2006-01-02 15:04"),
		"generator  " + metadata.GeneratorVersion + fmt.Sprintf(" / seed %d", metadata.Seed),
		"build      " + metadata.BuildRevision,
		"lookback   " + fmt.Sprintf("%d bars", metadata.RequiredLookback),
	}
	for _, feature := range artifact.Summary.FeatureNames {
		lines = append(lines, "feature    "+string(feature))
	}
	for _, value := range metadata.Configuration {
		lines = append(lines, "config     "+value.Name+"="+value.Value)
	}
	for _, fingerprint := range metadata.FeatureFingerprints {
		lines = append(lines, "feature fp "+fingerprint)
	}
	for _, checksum := range metadata.Checksums {
		lines = append(lines, "checksum  "+checksum.Path+" "+clipText(checksum.SHA256, 22))
	}
	renderer.textBlock(bounds, lines)
}

func (renderer *shellRenderer) drawFeatureImportance(state appstate.ModelsWorkspaceState, bounds rectangle) {
	artifact, found := selectedModelArtifact(state)
	if !found {
		renderer.emptyDock(bounds, "Select a completed artifact")
		return
	}
	maximum := 0.0
	for _, importance := range artifact.Importance {
		if importance.Gain > maximum {
			maximum = importance.Gain
		}
	}
	if maximum == 0 {
		maximum = 1
	}
	rowHeight := renderer.scaleValue(30)
	for index, importance := range artifact.Importance {
		y := bounds.y + float32(index)*rowHeight
		if y+rowHeight > bounds.y+bounds.height {
			break
		}
		renderer.text(renderer.window.monoFont, clipText(string(importance.Feature), 24), point{x: bounds.x, y: y + renderer.scaleValue(5)}, 8, tokenText)
		bar := rectangle{x: bounds.x + bounds.width*0.42, y: y + renderer.scaleValue(5), width: bounds.width * 0.45 * float32(importance.Gain/maximum), height: renderer.scaleValue(10)}
		renderer.rect(bar, tokenCyan)
		renderer.text(renderer.window.monoFont, fmt.Sprintf("%.3f", importance.Gain), point{x: bounds.x + bounds.width*0.89, y: y + renderer.scaleValue(5)}, 8, tokenMuted)
	}
}

func (renderer *shellRenderer) drawTreeInspector(state appstate.ModelsWorkspaceState, bounds rectangle) {
	artifact, found := selectedModelArtifact(state)
	if !found || len(artifact.Trees) == 0 {
		renderer.emptyDock(bounds, "Select an artifact with validated tree buffers")
		return
	}
	treeIndex := state.SelectedTree
	if treeIndex < 0 || treeIndex >= len(artifact.Trees) {
		treeIndex = 0
	}
	tree := artifact.Trees[treeIndex]
	for index := range tree.Leaves {
		y := bounds.y + float32(index)*renderer.scaleValue(29)
		if y+renderer.scaleValue(27) > bounds.y+bounds.height {
			break
		}
		label := fmt.Sprintf("#%d", index)
		if tree.Leaves[index] {
			label += fmt.Sprintf(" leaf  %.4f", tree.LeafValues[index])
		} else {
			feature := artifact.Summary.FeatureNames[tree.FeatureIndices[index]]
			label += fmt.Sprintf(" %s < %.4f  L:%d R:%d", feature, tree.Thresholds[index], tree.LeftChildren[index], tree.RightChildren[index])
		}
		renderer.text(renderer.window.monoFont, clipText(label, 58), point{x: bounds.x, y: y + renderer.scaleValue(5)}, 8, tokenText)
	}
}

func (renderer *shellRenderer) drawInferenceSelector(state appstate.AppState, bounds rectangle) {
	workspace := state.InferenceWorkspace
	model := workspace.SelectedModelID
	if model == "" {
		model = state.LinkContext.ModelID
	}
	if model == "" {
		model = "champion alias"
	}
	renderer.textBlock(bounds, []string{"Model / alias", string(model), "Mode: " + string(workspace.Query.Mode), "Select a registry artifact in Models, then run scoring."})
	button := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(82), width: renderer.scaleValue(116), height: renderer.scaleValue(24)}
	if renderer.phase7Button(button, "Run latest", true) {
		if query, found := inferenceQueryForRenderer(state, appstate.InferenceLatestSnapshot); found {
			renderer.actions = append(renderer.actions, appstate.RequestInferenceWorkspaceAction{Query: query})
		}
	}
	historical := rectangle{x: bounds.x + renderer.scaleValue(122), y: bounds.y + renderer.scaleValue(82), width: renderer.scaleValue(128), height: renderer.scaleValue(24)}
	if renderer.phase7Button(historical, "Run historical", true) {
		if query, found := inferenceQueryForRenderer(state, appstate.InferenceHistorical); found {
			renderer.actions = append(renderer.actions, appstate.RequestInferenceWorkspaceAction{Query: query})
		}
	}
}

func (renderer *shellRenderer) drawInferenceDataset(state appstate.AppState, bounds rectangle) {
	workspace := state.InferenceWorkspace
	diagnostics := workspace.Snapshot.Compatibility.Diagnostics
	lines := []string{
		"Dataset    " + string(workspace.Query.DatasetID),
		"Revision   " + fmt.Sprintf("%d", workspace.Query.DatasetRevision),
		"Range      " + workspace.Query.TimeRange.Start.Format("2006-01-02") + " to " + workspace.Query.TimeRange.End.Format("2006-01-02"),
		"Compatibility: compatible",
	}
	if len(diagnostics) > 0 {
		lines = append(lines, "Diagnostics: "+diagnostics[0].Field)
	}
	renderer.textBlock(bounds, lines)
}

func (renderer *shellRenderer) drawRankedScores(state appstate.InferenceWorkspaceState, bounds rectangle) {
	output := state.Snapshot.Output
	if len(output.Rankings) == 0 {
		renderer.emptyDock(bounds, "No rankable predictions")
		return
	}
	renderer.tableHeader(bounds, []string{"Rank", "Symbol", "Score", "Status"})
	y := bounds.y + renderer.scaleValue(28)
	rows := output.Rankings[len(output.Rankings)-1].Rows
	for _, row := range rows {
		line := rectangle{x: bounds.x, y: y - renderer.scaleValue(4), width: bounds.width, height: renderer.scaleValue(23)}
		if row.Symbol == state.SelectedSymbol {
			renderer.rect(line, tokenRaised)
		}
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, line) {
			renderer.actions = append(renderer.actions, appstate.SelectInferenceSymbolAction{Symbol: row.Symbol})
		}
		renderer.tableText(bounds.x+renderer.scaleValue(8), y, fmt.Sprintf("%d", row.Rank), tokenText)
		renderer.tableText(bounds.x+bounds.width*0.20, y, string(row.Symbol), tokenCyan)
		renderer.tableText(bounds.x+bounds.width*0.48, y, fmt.Sprintf("%.4f", row.Score), tokenText)
		renderer.tableText(bounds.x+bounds.width*0.73, y, "eligible", tokenPositive)
		y += renderer.scaleValue(24)
		if y > bounds.y+bounds.height {
			break
		}
	}
}

func (renderer *shellRenderer) drawInferenceDistribution(state appstate.InferenceWorkspaceState, bounds rectangle) {
	bins := state.Snapshot.Output.Distribution
	if len(bins) == 0 {
		renderer.emptyDock(bounds, "No score distribution")
		return
	}
	maximum := uint64(1)
	for _, bin := range bins {
		if bin.Count > maximum {
			maximum = bin.Count
		}
	}
	plot := rectangle{x: bounds.x + renderer.scaleValue(4), y: bounds.y + renderer.scaleValue(20), width: bounds.width - renderer.scaleValue(8), height: bounds.height - renderer.scaleValue(24)}
	barWidth := plot.width / float32(len(bins))
	for index, bin := range bins {
		height := plot.height * float32(bin.Count) / float32(maximum)
		renderer.rect(rectangle{x: plot.x + float32(index)*barWidth + 1, y: plot.y + plot.height - height, width: maxFloat32(1, barWidth-2), height: height}, withAlpha(tokenPurple, 185))
		renderer.text(renderer.window.monoFont, fmt.Sprintf("%d", bin.Count), point{x: plot.x + float32(index)*barWidth + renderer.scaleValue(4), y: plot.y + plot.height - height - renderer.scaleValue(12)}, 8, tokenText)
	}
	renderer.text(renderer.window.interFont, "score distribution", point{x: bounds.x + renderer.scaleValue(5), y: bounds.y + renderer.scaleValue(4)}, 8, tokenPurple)
}

func (renderer *shellRenderer) drawPredictionHistory(state appstate.InferenceWorkspaceState, bounds rectangle) {
	if len(state.Snapshot.History) == 0 {
		renderer.emptyDock(bounds, "No complete prediction snapshots")
		return
	}
	y := bounds.y
	for index := len(state.Snapshot.History) - 1; index >= 0; index-- {
		output := state.Snapshot.History[index]
		if y+renderer.scaleValue(32) > bounds.y+bounds.height {
			break
		}
		renderer.text(renderer.window.monoFont, clipText(string(output.ID), 38), point{x: bounds.x, y: y + renderer.scaleValue(4)}, 8, tokenText)
		renderer.text(renderer.window.interFont, output.CompletedAt.Format("2006-01-02 15:04")+" · "+output.Checksum, point{x: bounds.x, y: y + renderer.scaleValue(17)}, 8, tokenMuted)
		y += renderer.scaleValue(33)
	}
}

func (renderer *shellRenderer) drawExportStatus(state appstate.InferenceWorkspaceState, bounds rectangle) {
	export := state.Snapshot.Output.Export
	lines := []string{"State: " + string(export.State)}
	if export.Checksum != "" {
		lines = append(lines, "Checksum: "+clipText(export.Checksum, 34), fmt.Sprintf("Bytes: %d", export.Bytes))
	}
	renderer.textBlock(bounds, lines)
	if export.State != appstate.ExportReady && state.PendingExport.CommandID == "" {
		button := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(70), width: renderer.scaleValue(108), height: renderer.scaleValue(24)}
		if renderer.phase7Button(button, "Prepare export", true) {
			output := state.Snapshot.Output
			renderer.actions = append(renderer.actions, appstate.RequestInferenceExportAction{Command: appstate.ExportInferenceCommand{
				CorrelationID: appstate.CorrelationID("export-" + string(output.ID)), CommandID: appstate.CorrelationID(fmt.Sprintf("export-%s-%020d", output.ID, state.Generation)),
				Generation: state.Generation, InferenceID: output.ID,
			}})
		}
	}
}

func (renderer *shellRenderer) drawCompatibilityFailure(state appstate.InferenceWorkspaceState, bounds rectangle) {
	lines := []string{"Scoring is disabled: compatibility checks failed."}
	for _, diagnostic := range state.Snapshot.Compatibility.Diagnostics {
		lines = append(lines, diagnostic.Field+": "+clipText(diagnostic.Message, 50))
	}
	renderer.textBlock(bounds, lines)
}

func selectedModelArtifact(state appstate.ModelsWorkspaceState) (appstate.ModelArtifact, bool) {
	for _, artifact := range state.Snapshot.Registry {
		if artifact.Summary.ID == state.SelectedModelID {
			return artifact.Clone(), true
		}
	}
	return appstate.ModelArtifact{}, false
}

func inferenceQueryForRenderer(state appstate.AppState, mode appstate.InferenceMode) (appstate.InferenceWorkspaceQuery, bool) {
	dataset, found := rendererInferenceDataset(state)
	if !found {
		return appstate.InferenceWorkspaceQuery{}, false
	}
	modelID := state.ModelsWorkspace.SelectedModelID
	if modelID == "" {
		modelID = state.LinkContext.ModelID
	}
	alias := ""
	if modelID == "" {
		alias = "champion"
	}
	rangeValue := appstate.TimeRange{Start: dataset.Start, End: dataset.End}
	if mode == appstate.InferenceHistorical {
		rangeValue.Start = rangeValue.Start.AddDate(0, 0, 20)
	}
	generation := state.InferenceWorkspace.Generation + 1
	return appstate.InferenceWorkspaceQuery{
		CorrelationID: appstate.CorrelationID(fmt.Sprintf("inference-%020d", generation)), Generation: generation,
		Scenario: state.InferenceWorkspace.Scenario, ModelID: modelID, Alias: alias, DatasetID: dataset.ID,
		DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint, Symbols: append([]appstate.Symbol(nil), dataset.Symbols...),
		TimeRange: rangeValue, Mode: mode,
	}, true
}

func inferenceRetryQuery(state appstate.InferenceWorkspaceState) (appstate.InferenceWorkspaceQuery, bool) {
	query := state.Query.Clone()
	if query.DatasetID == "" {
		return appstate.InferenceWorkspaceQuery{}, false
	}
	query.Generation = state.Generation + 1
	query.CorrelationID = appstate.CorrelationID(fmt.Sprintf("inference-%020d", query.Generation))
	query.Scenario = appstate.InferenceScenarioNormal
	return query, true
}

func rendererInferenceDataset(state appstate.AppState) (appstate.DatasetSummary, bool) {
	for _, dataset := range state.Datasets {
		if dataset.ID == state.LinkContext.DatasetID {
			return dataset.Clone(), true
		}
	}
	if len(state.Datasets) == 0 {
		return appstate.DatasetSummary{}, false
	}
	return state.Datasets[0].Clone(), true
}
