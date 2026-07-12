package appstate

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestTreeBufferRejectsInvalidTopology(t *testing.T) {
	t.Parallel()
	valid := phase9TestTree()
	if err := valid.Validate(1); err != nil {
		t.Fatalf("valid tree: %v", err)
	}
	cycle := valid.Clone()
	cycle.LeftChildren[0] = 0
	if !errors.Is(cycle.Validate(1), ErrInvalidModels) {
		t.Fatalf("cycle validation = %v", cycle.Validate(1))
	}
	leafSplit := valid.Clone()
	leafSplit.LeftChildren[1] = 0
	if !errors.Is(leafSplit.Validate(1), ErrInvalidModels) {
		t.Fatalf("leaf/split validation = %v", leafSplit.Validate(1))
	}
	mismatch := valid.Clone()
	mismatch.Thresholds = mismatch.Thresholds[:2]
	if !errors.Is(mismatch.Validate(1), ErrInvalidModels) {
		t.Fatalf("length validation = %v", mismatch.Validate(1))
	}
}

func TestRankPredictionsUsesScoreThenStableSymbolAndSkipsIneligible(t *testing.T) {
	t.Parallel()
	rows := RankPredictions([]Prediction{
		{ID: "p-nvda", Symbol: "NVDA", Score: 0.8},
		{ID: "p-msft", Symbol: "MSFT", Score: 0.9},
		{ID: "p-aapl", Symbol: "AAPL", Score: 0.9},
		{ID: "p-missing", Symbol: "META", Missing: true},
		{ID: "p-ineligible", Symbol: "TSLA", Ineligible: true},
	})
	want := []RankingRow{{PredictionID: "p-aapl", Symbol: "AAPL", Rank: 1, Score: 0.9}, {PredictionID: "p-msft", Symbol: "MSFT", Rank: 2, Score: 0.9}, {PredictionID: "p-nvda", Symbol: "NVDA", Rank: 3, Score: 0.8}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rankings = %#v, want %#v", rows, want)
	}
}

func TestModelsReducerPreservesSelectionAndRequiresAliasConfirmation(t *testing.T) {
	t.Parallel()
	now := phase9TestTime()
	state := AppState{ModelsWorkspace: DefaultModelsWorkspaceState()}
	requested, _, err := Reduce(state, RequestModelsWorkspaceAction{Query: ModelsWorkspaceQuery{CorrelationID: "models-1", Generation: 1, Scenario: ModelsScenarioNormal}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := phase9ModelsSnapshot(ModelsWorkspaceQuery{CorrelationID: "models-1", Generation: 1, Scenario: ModelsScenarioNormal}, now)
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("fixture validation: %v", err)
	}
	ready, _, err := Reduce(requested, ClientMessageAction{Message: ModelsWorkspaceMessage{Event: EventEnvelope{ID: "models", Timestamp: now}, Snapshot: snapshot}})
	if err != nil {
		t.Fatal(err)
	}
	if ready.ModelsWorkspace.SelectedModelID != "model-a" {
		t.Fatalf("selected model = %q state=%q registry=%d generation=%d query=%+v", ready.ModelsWorkspace.SelectedModelID, ready.ModelsWorkspace.State, len(ready.ModelsWorkspace.Snapshot.Registry), ready.ModelsWorkspace.Generation, ready.ModelsWorkspace.Query)
	}
	staged, effects, err := Reduce(ready, BeginAliasAssignmentAction{Command: AliasAssignmentCommand{CorrelationID: "alias", CommandID: "alias-command", Generation: 1, Alias: "champion", ModelID: "model-b"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 0 || !staged.ModelsWorkspace.AwaitingConfirm {
		t.Fatalf("alias stage effects/confirm = %d/%t", len(effects), staged.ModelsWorkspace.AwaitingConfirm)
	}
	confirmed, effects, err := Reduce(staged, ConfirmAliasAssignmentAction{CommandID: "alias-command"})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.ModelsWorkspace.AwaitingConfirm || len(effects) != 1 {
		t.Fatalf("alias confirm state/effects = %t/%d", confirmed.ModelsWorkspace.AwaitingConfirm, len(effects))
	}
	if _, ok := effects[0].(AssignModelAliasEffect); !ok {
		t.Fatalf("effect = %T", effects[0])
	}
	stale := snapshot.Clone()
	stale.Query.Generation = 0
	stale.Query.CorrelationID = "stale"
	next, _, err := Reduce(ready, ClientMessageAction{Message: ModelsWorkspaceMessage{Snapshot: stale}})
	if err != nil {
		t.Fatal(err)
	}
	if next.ModelsWorkspace.SelectedModelID != "model-a" || next.ModelsWorkspace.State != WorkspaceReady {
		t.Fatalf("stale response overwrote registry state: %+v", next.ModelsWorkspace)
	}
}

func TestInferenceReducerRejectsIncompatibleOutputAndClonesPublishedData(t *testing.T) {
	t.Parallel()
	now := phase9TestTime()
	dataset := DatasetSummary{ID: "dataset", Revision: 3, Fingerprint: "dataset-fingerprint", Symbols: []Symbol{"AAPL", "MSFT"}, Start: now.AddDate(0, 0, -30), End: now}
	query := InferenceWorkspaceQuery{CorrelationID: "inference-1", Generation: 1, Scenario: InferenceScenarioNormal, ModelID: "model-a", DatasetID: dataset.ID, DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint, Symbols: dataset.Symbols, TimeRange: TimeRange{Start: dataset.Start, End: dataset.End}, Mode: InferenceLatestSnapshot}
	state := AppState{Datasets: []DatasetSummary{dataset}, InferenceWorkspace: DefaultInferenceWorkspaceState()}
	requested, _, err := Reduce(state, RequestInferenceWorkspaceAction{Query: query})
	if err != nil {
		t.Fatal(err)
	}
	incompatible := InferenceWorkspaceSnapshot{Query: query, Compatibility: CompatibilitySummary{Diagnostics: []CompatibilityDiagnostic{{Field: "dataset_fingerprint", Message: "changed"}}}, PreparedAt: now}
	next, _, err := Reduce(requested, ClientMessageAction{Message: InferenceWorkspaceMessage{Snapshot: incompatible}})
	if err != nil {
		t.Fatal(err)
	}
	if next.InferenceWorkspace.State != WorkspaceEmpty || next.InferenceWorkspace.Snapshot.HasOutput {
		t.Fatalf("incompatible state/output = %q/%t", next.InferenceWorkspace.State, next.InferenceWorkspace.Snapshot.HasOutput)
	}
	output := phase9InferenceOutput(query, now)
	readySnapshot := InferenceWorkspaceSnapshot{Query: query, Compatibility: CompatibilitySummary{Compatible: true}, Output: output, HasOutput: true, History: []InferenceOutput{output}, PreparedAt: now}
	ready, _, err := Reduce(requested, ClientMessageAction{Message: InferenceWorkspaceMessage{Snapshot: readySnapshot}})
	if err != nil {
		t.Fatal(err)
	}
	if ready.InferenceWorkspace.State != WorkspaceReady || ready.InferenceWorkspace.SelectedSymbol != "AAPL" {
		t.Fatalf("ready state/symbol = %q/%q", ready.InferenceWorkspace.State, ready.InferenceWorkspace.SelectedSymbol)
	}
	clone := ready.Clone()
	ready.InferenceWorkspace.Snapshot.Output.Predictions[0].FeatureFingerprints[0] = "mutated"
	if clone.InferenceWorkspace.Snapshot.Output.Predictions[0].FeatureFingerprints[0] == "mutated" {
		t.Fatal("published prediction shares feature fingerprints")
	}
}

func phase9TestTime() time.Time {
	return time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC)
}

func phase9TestTree() TreeBuffer {
	return TreeBuffer{FeatureIndices: []int{0, -1, -1}, LeftChildren: []int{1, -1, -1}, RightChildren: []int{2, -1, -1}, Thresholds: []float64{0.1, 0, 0}, LeafValues: []float64{0, -0.1, 0.1}, Leaves: []bool{false, true, true}, MissingGoLeft: []bool{true, false, false}}
}

func phase9Artifact(id ModelID, run RunID, now time.Time) ModelArtifact {
	return ModelArtifact{Summary: ModelSummary{ID: id, RunID: run, Kind: ModelRandomForest, FeatureNames: []FeatureName{"ret_5"}, TrainingCutoff: now.Add(-time.Hour), CreatedAt: now, ArtifactFingerprint: string(id) + "-fingerprint", Immutable: true}, FinalRefit: true, ArtifactComplete: true,
		Metadata:   ArtifactMetadata{SchemaVersion: "artifact/v1", EngineVersion: "corthena-tree/v1", FeatureSchema: "features/v1", Target: TargetSpec{Kind: "forward_open_return", HorizonBars: 5}, TrainingFingerprint: "training-" + string(id), TrainingCutoff: now.Add(-time.Hour), GeneratorVersion: "counter-v1", BuildRevision: "test", Configuration: []ModelConfigurationValue{{Name: "max_depth", Value: "4"}}, FeatureFingerprints: []string{"ret_5@v1"}, Checksums: []ArtifactChecksum{{Path: "manifest", SHA256: "checksum"}}, RequiredLookback: 2},
		Importance: []FeatureImportance{{Feature: "ret_5", Gain: 1}}, Trees: []TreeBuffer{phase9TestTree()},
	}
}

func phase9ModelsSnapshot(query ModelsWorkspaceQuery, now time.Time) ModelsWorkspaceSnapshot {
	return ModelsWorkspaceSnapshot{Query: query, Registry: []ModelArtifact{phase9Artifact("model-a", "run-a", now), phase9Artifact("model-b", "run-b", now)}, PreparedAt: now}
}

func phase9InferenceOutput(query InferenceWorkspaceQuery, now time.Time) InferenceOutput {
	prediction := Prediction{ID: "prediction-aapl", Symbol: "AAPL", Timestamp: now, ModelID: query.ModelID, RunID: "run-a", DatasetFingerprint: query.DatasetFingerprint, FeatureFingerprints: []string{"ret_5@v1"}, Score: 0.8}
	return InferenceOutput{ID: "inference-a", ModelID: query.ModelID, RunID: "run-a", DatasetID: query.DatasetID, Fingerprint: query.DatasetFingerprint, Mode: query.Mode, TimeRange: query.TimeRange, Predictions: []Prediction{prediction}, Rankings: []TimestampRanking{{Timestamp: now, Rows: []RankingRow{{PredictionID: prediction.ID, Symbol: prediction.Symbol, Rank: 1, Score: prediction.Score}}}}, Distribution: []HistogramBin{{Minimum: 0.5, Maximum: 1, Count: 1}}, CompletedAt: now, Checksum: "checksum", Export: ExportSnapshot{State: ExportIdle}}
}
