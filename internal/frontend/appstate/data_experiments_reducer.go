package appstate

import (
	"fmt"
	"time"
)

func beginDataQuery(state *AppState, query DataWorkspaceQuery) (UIEffect, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if query.Generation <= state.Data.Generation {
		return nil, fmt.Errorf("%w: Data generation %d is not newer than %d", ErrInvariant, query.Generation, state.Data.Generation)
	}
	state.Data.Generation = query.Generation
	state.Data.Query = query
	state.Data.Scenario = query.Scenario
	state.Data.Stale = state.Data.Snapshot.Query.Generation != 0
	state.Data.State = WorkspaceLoading
	state.Data.Error = ErrorSnapshot{}
	return QueryDataWorkspaceEffect{ID: EffectID(fmt.Sprintf("data-%020d", query.Generation)), Query: query}, nil
}

func refreshDataWorkspace(state *AppState) (UIEffect, error) {
	generation := state.Data.Generation + 1
	scenario := state.Data.Scenario
	if !scenario.Valid() {
		scenario = DataScenarioNormal
	}
	return beginDataQuery(state, DataWorkspaceQuery{
		CorrelationID: CorrelationID(fmt.Sprintf("data-%020d", generation)),
		Generation:    generation, Scenario: scenario,
	})
}

func beginDataImport(state *AppState, request DataImportRequest) (UIEffect, error) {
	request = request.Clone()
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if request.Generation <= state.Data.Generation {
		return nil, fmt.Errorf("%w: import generation %d is not newer than %d", ErrInvariant, request.Generation, state.Data.Generation)
	}
	state.Data.Generation = request.Generation
	state.Data.Query = DataWorkspaceQuery{CorrelationID: request.CorrelationID, Generation: request.Generation, Scenario: request.Scenario}
	state.Data.Scenario = request.Scenario
	state.Data.Stale = state.Data.Snapshot.Query.Generation != 0
	state.Data.State = WorkspaceLoading
	state.Data.Error = ErrorSnapshot{}
	return ImportDataEffect{ID: EffectID(fmt.Sprintf("data-import-%020d", request.Generation)), Request: request}, nil
}

func applyDataWorkspaceResponse(state *AppState, message DataWorkspaceMessage) bool {
	snapshot := message.Snapshot.Clone()
	if snapshot.Query.Generation != state.Data.Generation ||
		snapshot.Query.CorrelationID != state.Data.Query.CorrelationID {
		return false
	}
	state.Data.Query = snapshot.Query
	state.Data.Snapshot = snapshot
	state.Data.Stale = false
	state.Data.Error = ErrorSnapshot{}
	state.Datasets = cloneDatasets(snapshot.Catalog)
	if state.Data.SelectedDatasetID == "" || !containsDataset(snapshot.Catalog, state.Data.SelectedDatasetID) {
		if len(snapshot.Catalog) > 0 {
			state.Data.SelectedDatasetID = snapshot.Catalog[0].ID
		} else {
			state.Data.SelectedDatasetID = ""
		}
	}
	if state.Data.SelectedImportID != "" && !containsImport(snapshot.Imports, state.Data.SelectedImportID) {
		state.Data.SelectedImportID = ""
	}
	switch {
	case snapshot.Degraded:
		state.Data.State = WorkspaceDegraded
	case snapshot.Query.Scenario == DataScenarioRecovered:
		state.Data.State = WorkspaceRecovered
	case len(snapshot.Catalog) == 0 && len(snapshot.Imports) == 0:
		state.Data.State = WorkspaceEmpty
	default:
		state.Data.State = WorkspaceReady
	}
	return true
}

func beginExperimentsQuery(state *AppState, query ExperimentQuery) (UIEffect, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if query.Generation <= state.Experiments.Generation {
		return nil, fmt.Errorf("%w: Experiments generation %d is not newer than %d", ErrInvariant, query.Generation, state.Experiments.Generation)
	}
	state.Experiments.Generation = query.Generation
	state.Experiments.Query = query
	state.Experiments.Scenario = query.Scenario
	state.Experiments.Stale = state.Experiments.Snapshot.Query.Generation != 0
	state.Experiments.State = WorkspaceLoading
	state.Experiments.Error = ErrorSnapshot{}
	return QueryExperimentsEffect{ID: EffectID(fmt.Sprintf("experiments-%020d", query.Generation)), Query: query}, nil
}

func refreshExperimentsWorkspace(state *AppState) (UIEffect, error) {
	generation := state.Experiments.Generation + 1
	scenario := state.Experiments.Scenario
	if !scenario.Valid() {
		scenario = ExperimentScenarioNormal
	}
	return beginExperimentsQuery(state, ExperimentQuery{
		CorrelationID: CorrelationID(fmt.Sprintf("experiments-%020d", generation)),
		Generation:    generation, Scenario: scenario,
	})
}

func ensureDefaultExperimentDraft(state *AppState) []UIEffect {
	if state.Experiments.Draft.Revision != 0 || len(state.Datasets) == 0 {
		return nil
	}
	dataset := state.Datasets[0]
	draft := ExperimentDraft{
		Revision: 1, Name: "Daily equity baseline", DatasetID: dataset.ID,
		DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint,
		Features:     []FeatureName{"ret_5", "volatility_20", "volume_z_30"},
		Target:       TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
		Split:        SplitSpec{Kind: "walk_forward", TrainBars: 504, ValidationBars: 126, TestBars: 126, PurgeBars: 5, EmbargoBars: 1, ExpandingWindows: true},
		Model:        ModelSpec{Kind: ModelRandomForest, MaxDepth: 8, MinLeafSamples: 32, EstimatorCount: 200, HistogramBins: 64, Seed: 42},
		Portfolio:    PortfolioSpec{LongQuantile: 0.8, ShortQuantile: 0.2, CostBPS: 5},
		RequestedCPU: 6,
	}
	state.Experiments.Draft = draft
	state.Experiments.EvaluationGeneration++
	request := ExperimentEvaluationRequest{
		CorrelationID: CorrelationID(fmt.Sprintf("experiment-evaluate-%020d", state.Experiments.EvaluationGeneration)),
		Generation:    state.Experiments.EvaluationGeneration, Draft: draft.Clone(), Scenario: state.Experiments.Scenario,
	}
	return []UIEffect{
		EvaluateExperimentEffect{ID: EffectID(request.CorrelationID), Request: request},
		PersistExperimentDraftEffect{ID: EffectID(fmt.Sprintf("experiment-draft-%020d", draft.Revision)), Revision: draft.Revision, Draft: draft.Clone()},
	}
}

func updateExperimentDraft(state *AppState, draft ExperimentDraft, updatedAt time.Time) ([]UIEffect, error) {
	draft = draft.Clone()
	if draft.Revision != state.Experiments.Draft.Revision+1 {
		return nil, fmt.Errorf("%w: draft revision %d does not follow %d", ErrInvariant, draft.Revision, state.Experiments.Draft.Revision)
	}
	state.Experiments.Draft = draft
	state.Experiments.EvaluationGeneration++
	state.Experiments.Issues = nil
	state.Experiments.Error = ErrorSnapshot{}
	state.Experiments.DraftPersistence.PendingRevision = draft.Revision
	request := ExperimentEvaluationRequest{
		CorrelationID: CorrelationID(fmt.Sprintf("experiment-evaluate-%020d", state.Experiments.EvaluationGeneration)),
		Generation:    state.Experiments.EvaluationGeneration,
		Draft:         draft.Clone(), Scenario: state.Experiments.Scenario,
	}
	return []UIEffect{
		EvaluateExperimentEffect{ID: EffectID(request.CorrelationID), Request: request},
		PersistExperimentDraftEffect{
			ID:       EffectID(fmt.Sprintf("experiment-draft-%020d", draft.Revision)),
			Revision: draft.Revision, Draft: draft.Clone(), Requested: updatedAt.UTC(),
		},
	}, nil
}

func applyExperimentWorkspaceResponse(state *AppState, message ExperimentWorkspaceMessage) bool {
	snapshot := message.Snapshot.Clone()
	if snapshot.Query.Generation != state.Experiments.Generation ||
		snapshot.Query.CorrelationID != state.Experiments.Query.CorrelationID {
		return false
	}
	state.Experiments.Query = snapshot.Query
	state.Experiments.Snapshot = snapshot
	state.Experiments.Stale = false
	state.Experiments.Error = ErrorSnapshot{}
	if state.Experiments.SelectedExperimentID != "" && !containsExperiment(snapshot.Definitions, state.Experiments.SelectedExperimentID) {
		state.Experiments.SelectedExperimentID = ""
	}
	switch {
	case snapshot.Degraded:
		state.Experiments.State = WorkspaceDegraded
	case snapshot.Query.Scenario == ExperimentScenarioRecovered:
		state.Experiments.State = WorkspaceRecovered
	case len(snapshot.Definitions) == 0 && snapshot.Query.Scenario == ExperimentScenarioEmpty:
		state.Experiments.State = WorkspaceEmpty
	default:
		state.Experiments.State = WorkspaceReady
	}
	return true
}

func applyExperimentEvaluation(state *AppState, message ExperimentEvaluationMessage) bool {
	if message.Generation != state.Experiments.EvaluationGeneration || message.Revision != state.Experiments.Draft.Revision {
		return false
	}
	state.Experiments.Issues = append([]ExperimentValidationIssue(nil), message.Issues...)
	state.Experiments.Estimate = message.Estimate
	return true
}

func containsDataset(datasets []DatasetSummary, id DatasetID) bool {
	for _, dataset := range datasets {
		if dataset.ID == id {
			return true
		}
	}
	return false
}

func containsImport(imports []DataImportRecord, id string) bool {
	for _, record := range imports {
		if record.ID == id {
			return true
		}
	}
	return false
}

func containsExperiment(definitions []ExperimentDefinition, id ExperimentID) bool {
	for _, definition := range definitions {
		if definition.ID == id {
			return true
		}
	}
	return false
}

func upsertExperimentDefinition(input []ExperimentDefinition, definition ExperimentDefinition) []ExperimentDefinition {
	output := make([]ExperimentDefinition, len(input))
	for index, current := range input {
		output[index] = current.Clone()
		if current.ID == definition.ID {
			output[index] = definition.Clone()
			return output
		}
	}
	return append(output, definition.Clone())
}

func workspaceFailureState(code ErrorCode) WorkspaceLoadState {
	switch code {
	case ErrorDataCancelled, ErrorExperimentCancelled:
		return WorkspaceCancelled
	case ErrorEffectBusy:
		return WorkspaceBusy
	default:
		return WorkspaceFailed
	}
}

func contextForDatasetState(current LinkContext, dataset DatasetSummary) LinkContext {
	current.DatasetID = dataset.ID
	current.Symbols = cloneSymbols(dataset.Symbols)
	current.Interval = dataset.Interval
	current.TimeRange = TimeRange{Start: dataset.Start, End: dataset.End}.Normalize()
	return current
}
