package appstate

import (
	"fmt"
	"strings"
)

func beginModelsQuery(state *AppState, query ModelsWorkspaceQuery) (UIEffect, error) {
	query = query.Clone()
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if query.Generation <= state.ModelsWorkspace.Generation {
		return nil, fmt.Errorf("%w: Models generation %d is not newer than %d", ErrInvariant, query.Generation, state.ModelsWorkspace.Generation)
	}
	state.ModelsWorkspace.Generation = query.Generation
	state.ModelsWorkspace.Query = query
	state.ModelsWorkspace.Scenario = query.Scenario
	state.ModelsWorkspace.State = WorkspaceLoading
	state.ModelsWorkspace.Stale = state.ModelsWorkspace.Snapshot.Query.Generation != 0
	state.ModelsWorkspace.Error = ErrorSnapshot{}
	return QueryModelsWorkspaceEffect{ID: EffectID(fmt.Sprintf("models-%020d", query.Generation)), Query: query}, nil
}

func refreshModelsWorkspace(state *AppState) (UIEffect, error) {
	scenario := state.ModelsWorkspace.Scenario
	if !scenario.Valid() {
		scenario = ModelsScenarioNormal
	}
	generation := state.ModelsWorkspace.Generation + 1
	return beginModelsQuery(state, ModelsWorkspaceQuery{
		CorrelationID: CorrelationID(fmt.Sprintf("models-%020d", generation)),
		Generation:    generation,
		Scenario:      scenario,
		Filter:        state.ModelsWorkspace.Query.Filter,
		Page:          state.ModelsWorkspace.Query.Page,
		PageSize:      state.ModelsWorkspace.Query.PageSize,
	})
}

func applyModelsWorkspaceResponse(state *AppState, message ModelsWorkspaceMessage) bool {
	snapshot := message.Snapshot.Clone()
	if snapshot.Query.Generation != state.ModelsWorkspace.Generation ||
		snapshot.Query.CorrelationID != state.ModelsWorkspace.Query.CorrelationID {
		return false
	}
	if err := snapshot.Validate(); err != nil {
		state.ModelsWorkspace.State = WorkspaceFailed
		state.ModelsWorkspace.Error = ErrorSnapshot{Code: ErrorModelsFailed, Message: err.Error(), Retryable: false, CorrelationID: snapshot.Query.CorrelationID}
		return false
	}
	state.ModelsWorkspace.Query = snapshot.Query
	state.ModelsWorkspace.Snapshot = snapshot
	state.ModelsWorkspace.Stale = false
	state.ModelsWorkspace.Error = ErrorSnapshot{}
	if !containsModelArtifact(snapshot.Registry, state.ModelsWorkspace.SelectedModelID) {
		state.ModelsWorkspace.SelectedModelID = ""
		state.ModelsWorkspace.SelectedTree = 0
		if len(snapshot.Registry) != 0 {
			state.ModelsWorkspace.SelectedModelID = snapshot.Registry[0].Summary.ID
		}
	}
	if artifact, found := findModelArtifact(snapshot.Registry, state.ModelsWorkspace.SelectedModelID); found {
		if state.ModelsWorkspace.SelectedTree < 0 || state.ModelsWorkspace.SelectedTree >= len(artifact.Trees) {
			state.ModelsWorkspace.SelectedTree = 0
		}
		state.LinkContext.ModelID = artifact.Summary.ID
	}
	state.Models = modelSummaries(snapshot.Registry)
	switch {
	case snapshot.Degraded:
		state.ModelsWorkspace.State = WorkspaceDegraded
	case snapshot.Query.Scenario == ModelsScenarioRecovered:
		state.ModelsWorkspace.State = WorkspaceRecovered
	case len(snapshot.Registry) == 0:
		state.ModelsWorkspace.State = WorkspaceEmpty
	default:
		state.ModelsWorkspace.State = WorkspaceReady
	}
	return true
}

func beginInferenceQuery(state *AppState, query InferenceWorkspaceQuery) (UIEffect, error) {
	query = query.Clone()
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if query.Generation <= state.InferenceWorkspace.Generation {
		return nil, fmt.Errorf("%w: Inference generation %d is not newer than %d", ErrInvariant, query.Generation, state.InferenceWorkspace.Generation)
	}
	state.InferenceWorkspace.Generation = query.Generation
	state.InferenceWorkspace.Query = query
	state.InferenceWorkspace.Scenario = query.Scenario
	state.InferenceWorkspace.State = WorkspaceLoading
	state.InferenceWorkspace.Stale = state.InferenceWorkspace.Snapshot.Query.Generation != 0
	state.InferenceWorkspace.Error = ErrorSnapshot{}
	return QueryInferenceWorkspaceEffect{ID: EffectID(fmt.Sprintf("inference-%020d", query.Generation)), Query: query}, nil
}

func refreshInferenceWorkspace(state *AppState) (UIEffect, error) {
	scenario := state.InferenceWorkspace.Scenario
	if !scenario.Valid() {
		scenario = InferenceScenarioNormal
	}
	dataset, found := selectedInferenceDataset(*state)
	if !found {
		return nil, nil
	}
	modelID := state.InferenceWorkspace.SelectedModelID
	if modelID == "" {
		modelID = state.LinkContext.ModelID
	}
	alias := state.InferenceWorkspace.SelectedAlias
	if modelID == "" && alias == "" {
		alias = "champion"
	}
	generation := state.InferenceWorkspace.Generation + 1
	return beginInferenceQuery(state, InferenceWorkspaceQuery{
		CorrelationID:      CorrelationID(fmt.Sprintf("inference-%020d", generation)),
		Generation:         generation,
		Scenario:           scenario,
		ModelID:            modelID,
		Alias:              alias,
		DatasetID:          dataset.ID,
		DatasetRevision:    dataset.Revision,
		DatasetFingerprint: dataset.Fingerprint,
		Symbols:            cloneSymbols(dataset.Symbols),
		TimeRange:          TimeRange{Start: dataset.Start, End: dataset.End},
		Mode:               InferenceLatestSnapshot,
	})
}

func selectedInferenceDataset(state AppState) (DatasetSummary, bool) {
	for _, dataset := range state.Datasets {
		if dataset.ID == state.LinkContext.DatasetID {
			return dataset.Clone(), true
		}
	}
	if len(state.Datasets) == 0 {
		return DatasetSummary{}, false
	}
	return state.Datasets[0].Clone(), true
}

func applyInferenceWorkspaceResponse(state *AppState, message InferenceWorkspaceMessage) bool {
	snapshot := message.Snapshot.Clone()
	if snapshot.Query.Generation != state.InferenceWorkspace.Generation ||
		snapshot.Query.CorrelationID != state.InferenceWorkspace.Query.CorrelationID {
		return false
	}
	if err := snapshot.Validate(); err != nil {
		state.InferenceWorkspace.State = WorkspaceFailed
		state.InferenceWorkspace.Error = ErrorSnapshot{Code: ErrorInferenceFailed, Message: err.Error(), Retryable: false, CorrelationID: snapshot.Query.CorrelationID}
		return false
	}
	state.InferenceWorkspace.Query = snapshot.Query
	state.InferenceWorkspace.Snapshot = snapshot
	state.InferenceWorkspace.Stale = false
	state.InferenceWorkspace.Error = ErrorSnapshot{}
	state.InferenceWorkspace.SelectedModelID = snapshot.Query.ModelID
	state.InferenceWorkspace.SelectedAlias = snapshot.Query.Alias
	if snapshot.HasOutput {
		state.LinkContext.ModelID = snapshot.Output.ModelID
		state.Inferences = appendInferenceSummary(state.Inferences, inferenceSummary(snapshot.Output))
		if !containsPredictionSymbol(snapshot.Output.Predictions, state.InferenceWorkspace.SelectedSymbol) {
			state.InferenceWorkspace.SelectedSymbol = firstPredictionSymbol(snapshot.Output.Predictions)
		}
	}
	switch {
	case snapshot.Degraded:
		state.InferenceWorkspace.State = WorkspaceDegraded
	case snapshot.Query.Scenario == InferenceScenarioRecovered:
		state.InferenceWorkspace.State = WorkspaceRecovered
	case !snapshot.Compatibility.Compatible:
		state.InferenceWorkspace.State = WorkspaceEmpty
	case !snapshot.HasOutput:
		state.InferenceWorkspace.State = WorkspaceEmpty
	default:
		state.InferenceWorkspace.State = WorkspaceReady
	}
	return true
}

func applyAliasAssigned(state *AppState, message AliasAssignedMessage) bool {
	if message.Generation != state.ModelsWorkspace.Generation || message.CommandID == "" ||
		message.CommandID != state.ModelsWorkspace.PendingAlias.CommandID {
		return false
	}
	snapshot := message.Snapshot.Clone()
	if err := snapshot.Validate(); err != nil {
		state.ModelsWorkspace.State = WorkspaceFailed
		state.ModelsWorkspace.Error = ErrorSnapshot{Code: ErrorAliasRejected, Message: err.Error(), CorrelationID: message.Event.CorrelationID}
		return false
	}
	state.ModelsWorkspace.Snapshot = snapshot
	state.ModelsWorkspace.PendingAlias = AliasAssignmentCommand{}
	state.ModelsWorkspace.AwaitingConfirm = false
	state.Models = modelSummaries(snapshot.Registry)
	return true
}

func applyInferenceExport(state *AppState, message InferenceExportMessage) bool {
	if message.Generation != state.InferenceWorkspace.Generation || message.CommandID == "" ||
		message.CommandID != state.InferenceWorkspace.PendingExport.CommandID {
		return false
	}
	export := message.Export.Clone()
	if export.State != ExportReady || export.InferenceID == "" || export.Checksum == "" {
		state.InferenceWorkspace.Error = ErrorSnapshot{Code: ErrorExportFailed, Message: "export did not complete with a checksum", CorrelationID: message.Event.CorrelationID}
		return false
	}
	state.InferenceWorkspace.Snapshot.Output.Export = export
	for index := range state.InferenceWorkspace.Snapshot.History {
		if state.InferenceWorkspace.Snapshot.History[index].ID == export.InferenceID {
			state.InferenceWorkspace.Snapshot.History[index].Export = export
		}
	}
	state.InferenceWorkspace.PendingExport = ExportInferenceCommand{}
	return true
}

func containsModelArtifact(artifacts []ModelArtifact, id ModelID) bool {
	_, found := findModelArtifact(artifacts, id)
	return found
}

func findModelArtifact(artifacts []ModelArtifact, id ModelID) (ModelArtifact, bool) {
	for _, artifact := range artifacts {
		if artifact.Summary.ID == id {
			return artifact.Clone(), true
		}
	}
	return ModelArtifact{}, false
}

func modelSummaries(artifacts []ModelArtifact) []ModelSummary {
	output := make([]ModelSummary, len(artifacts))
	for index, artifact := range artifacts {
		output[index] = artifact.Summary.Clone()
	}
	return output
}

func containsPredictionSymbol(predictions []Prediction, symbol Symbol) bool {
	for _, prediction := range predictions {
		if prediction.Symbol == symbol {
			return true
		}
	}
	return false
}

func firstPredictionSymbol(predictions []Prediction) Symbol {
	if len(predictions) == 0 {
		return ""
	}
	return predictions[0].Symbol
}

func inferenceSummary(output InferenceOutput) InferenceSummary {
	scores := make([]ScoredSymbol, 0)
	if len(output.Rankings) != 0 {
		for _, row := range output.Rankings[len(output.Rankings)-1].Rows {
			scores = append(scores, ScoredSymbol{Symbol: row.Symbol, Rank: row.Rank, Score: row.Score})
		}
	}
	return InferenceSummary{ID: output.ID, ModelID: output.ModelID, DatasetID: output.DatasetID,
		State: InferenceCompleted, TimeRange: output.TimeRange.Normalize(), Scores: scores, GeneratedAt: output.CompletedAt.UTC()}
}

func appendInferenceSummary(input []InferenceSummary, summary InferenceSummary) []InferenceSummary {
	output := cloneInferences(input)
	for index := range output {
		if output[index].ID == summary.ID {
			output[index] = summary.Clone()
			return output
		}
	}
	return append(output, summary.Clone())
}

func normalizedModelAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}
