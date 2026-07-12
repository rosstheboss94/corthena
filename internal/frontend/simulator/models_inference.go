package simulator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

const maxDemoInferenceHistory = 32

// ModelsWorkspace returns deterministic immutable model artifacts and history.
func (client *DemoCoordinator) ModelsWorkspace(
	ctx context.Context,
	query appstate.ModelsWorkspaceQuery,
) (appstate.ModelsWorkspaceMessage, error) {
	query = query.Clone()
	if err := query.Validate(); err != nil {
		return appstate.ModelsWorkspaceMessage{}, phase9Failure(err.Error(), query.CorrelationID, appstate.ErrorValidation, false)
	}
	if err := client.wait(ctx, client.delays.Models); err != nil {
		return appstate.ModelsWorkspaceMessage{}, err
	}
	switch query.Scenario {
	case appstate.ModelsScenarioLoading:
		<-ctx.Done()
		return appstate.ModelsWorkspaceMessage{}, ctx.Err()
	case appstate.ModelsScenarioFailure:
		return appstate.ModelsWorkspaceMessage{}, phase9Failure("deterministic Models request failed", query.CorrelationID, appstate.ErrorModelsFailed, true)
	case appstate.ModelsScenarioCancelled:
		return appstate.ModelsWorkspaceMessage{}, phase9Failure("deterministic Models request cancelled", query.CorrelationID, appstate.ErrorModelsCancelled, true)
	case appstate.ModelsScenarioBusy:
		return appstate.ModelsWorkspaceMessage{}, phase9Failure("deterministic Models queue is saturated", query.CorrelationID, appstate.ErrorEffectBusy, true)
	}
	if client.failures.Models {
		return appstate.ModelsWorkspaceMessage{}, phase9Failure("demo Models request failed", query.CorrelationID, appstate.ErrorModelsFailed, true)
	}
	client.mu.RLock()
	registry := cloneRegistry(client.modelRegistry)
	history := cloneAliasHistory(client.aliasHistory)
	client.mu.RUnlock()
	if query.Scenario == appstate.ModelsScenarioEmpty {
		registry = nil
	}
	if query.Filter != "" {
		filtered := make([]appstate.ModelArtifact, 0, len(registry))
		for _, artifact := range registry {
			if strings.Contains(strings.ToLower(string(artifact.Summary.ID)), query.Filter) ||
				strings.Contains(strings.ToLower(artifact.Summary.Alias), query.Filter) ||
				strings.Contains(strings.ToLower(string(artifact.Summary.Kind)), query.Filter) {
				filtered = append(filtered, artifact)
			}
		}
		registry = filtered
	}
	if query.PageSize > 0 {
		start := query.Page * query.PageSize
		if start >= len(registry) {
			registry = nil
		} else {
			end := start + query.PageSize
			if end > len(registry) {
				end = len(registry)
			}
			registry = registry[start:end]
		}
	}
	now := client.baseTime().Add(time.Duration(query.Generation) * time.Second)
	snapshot := appstate.ModelsWorkspaceSnapshot{
		Query: query, Registry: registry, AliasHistory: history, PreparedAt: now,
		Degraded: query.Scenario == appstate.ModelsScenarioDegraded,
	}
	if err := snapshot.Validate(); err != nil {
		return appstate.ModelsWorkspaceMessage{}, phase9Failure(err.Error(), query.CorrelationID, appstate.ErrorModelsFailed, false)
	}
	eventType := "models.ready"
	if len(snapshot.Registry) == 0 {
		eventType = "models.empty"
	} else if query.Scenario == appstate.ModelsScenarioRecovered {
		eventType = "models.recovered"
	}
	return appstate.ModelsWorkspaceMessage{
		Event:    event(appstate.EventID(fmt.Sprintf("evt-models-%06d", query.Generation)), eventType, now, query.CorrelationID),
		Snapshot: snapshot,
	}, nil
}

// AssignModelAlias applies one explicitly confirmed, idempotent alias transaction.
func (client *DemoCoordinator) AssignModelAlias(
	ctx context.Context,
	command appstate.AliasAssignmentCommand,
) (appstate.AliasAssignedMessage, error) {
	command.Alias = strings.ToLower(strings.TrimSpace(command.Alias))
	if err := command.Validate(); err != nil {
		return appstate.AliasAssignedMessage{}, phase9Failure(err.Error(), command.CorrelationID, appstate.ErrorAliasRejected, false)
	}
	if err := client.wait(ctx, client.delays.Commands); err != nil {
		return appstate.AliasAssignedMessage{}, err
	}
	if client.failures.Commands {
		return appstate.AliasAssignedMessage{}, phase9Failure("demo alias command failed", command.CorrelationID, appstate.ErrorAliasRejected, true)
	}
	now := client.baseTime().Add(time.Duration(command.Generation) * time.Second)
	client.mu.Lock()
	defer client.mu.Unlock()
	if prior, found := client.aliasCommands[command.CommandID]; found {
		return cloneAliasMessage(prior), nil
	}
	if _, found := findArtifact(client.modelRegistry, command.ModelID); !found {
		return appstate.AliasAssignedMessage{}, phase9Failure("alias target is not a complete final-refit artifact", command.CorrelationID, appstate.ErrorAliasRejected, false)
	}
	registry := cloneRegistry(client.modelRegistry)
	previous := appstate.ModelID("")
	for index := range registry {
		if registry[index].Summary.Alias == command.Alias {
			previous = registry[index].Summary.ID
			registry[index].Summary.Alias = ""
		}
		if registry[index].Summary.ID == command.ModelID {
			registry[index].Summary.Alias = command.Alias
		}
	}
	history := append(cloneAliasHistory(client.aliasHistory), appstate.AliasHistoryEntry{
		Alias: command.Alias, ModelID: command.ModelID, PreviousModelID: previous, CommandID: command.CommandID, ChangedAt: now,
	})
	snapshot := appstate.ModelsWorkspaceSnapshot{
		Query:    appstate.ModelsWorkspaceQuery{CorrelationID: command.CorrelationID, Generation: command.Generation, Scenario: appstate.ModelsScenarioNormal},
		Registry: registry, AliasHistory: history, PreparedAt: now,
	}
	if err := snapshot.Validate(); err != nil {
		return appstate.AliasAssignedMessage{}, phase9Failure(err.Error(), command.CorrelationID, appstate.ErrorAliasRejected, false)
	}
	message := appstate.AliasAssignedMessage{
		Event:      event(appstate.EventID("evt-alias-"+string(command.CommandID)), "models.alias_assigned", now, command.CorrelationID),
		Generation: command.Generation, CommandID: command.CommandID, Snapshot: snapshot,
	}
	client.modelRegistry = registry
	client.aliasHistory = history
	client.aliasCommands[command.CommandID] = message
	return cloneAliasMessage(message), nil
}

// InferenceWorkspace validates compatibility and publishes complete score snapshots.
func (client *DemoCoordinator) InferenceWorkspace(
	ctx context.Context,
	query appstate.InferenceWorkspaceQuery,
) (appstate.InferenceWorkspaceMessage, error) {
	query = query.Clone()
	if err := query.Validate(); err != nil {
		return appstate.InferenceWorkspaceMessage{}, phase9Failure(err.Error(), query.CorrelationID, appstate.ErrorValidation, false)
	}
	if err := client.wait(ctx, client.delays.Inference); err != nil {
		return appstate.InferenceWorkspaceMessage{}, err
	}
	switch query.Scenario {
	case appstate.InferenceScenarioLoading:
		<-ctx.Done()
		return appstate.InferenceWorkspaceMessage{}, ctx.Err()
	case appstate.InferenceScenarioFailure:
		return appstate.InferenceWorkspaceMessage{}, phase9Failure("deterministic Inference request failed", query.CorrelationID, appstate.ErrorInferenceFailed, true)
	case appstate.InferenceScenarioCancelled:
		return appstate.InferenceWorkspaceMessage{}, phase9Failure("deterministic Inference request cancelled", query.CorrelationID, appstate.ErrorInferenceCancelled, true)
	case appstate.InferenceScenarioBusy:
		return appstate.InferenceWorkspaceMessage{}, phase9Failure("deterministic Inference queue is saturated", query.CorrelationID, appstate.ErrorEffectBusy, true)
	}
	if client.failures.Inference {
		return appstate.InferenceWorkspaceMessage{}, phase9Failure("demo Inference request failed", query.CorrelationID, appstate.ErrorInferenceFailed, true)
	}
	client.mu.RLock()
	registry := cloneRegistry(client.modelRegistry)
	datasets := cloneDatasetsForPhase9(client.snapshot.Datasets)
	history := cloneInferenceOutputs(client.inferenceHistory)
	client.mu.RUnlock()
	artifact, resolved := resolveArtifact(registry, query.ModelID, query.Alias)
	dataset, datasetFound := findDataset(datasets, query.DatasetID)
	compatibility := compatibilityFor(query, artifact, resolved, dataset, datasetFound)
	if query.Scenario == appstate.InferenceScenarioIncompatible {
		compatibility = appstate.CompatibilitySummary{Diagnostics: []appstate.CompatibilityDiagnostic{{
			Field: "feature_fingerprints", Expected: "registered immutable fingerprints", Actual: "scenario mismatch", Message: "feature implementation fingerprint is incompatible",
		}}}
	}
	now := client.baseTime().Add(time.Duration(query.Generation) * time.Second)
	snapshot := appstate.InferenceWorkspaceSnapshot{
		Query: query, Compatibility: compatibility, History: history, PreparedAt: now,
		Degraded: query.Scenario == appstate.InferenceScenarioDegraded,
	}
	if compatibility.Compatible && query.Scenario != appstate.InferenceScenarioEmpty {
		output := buildInferenceOutput(now, client.seed, query, artifact)
		snapshot.Output = output
		snapshot.HasOutput = true
		snapshot.History = appendBoundedInferenceHistory(snapshot.History, output)
		client.mu.Lock()
		client.inferenceHistory = appendBoundedInferenceHistory(client.inferenceHistory, output)
		client.mu.Unlock()
	}
	if err := snapshot.Validate(); err != nil {
		return appstate.InferenceWorkspaceMessage{}, phase9Failure(err.Error(), query.CorrelationID, appstate.ErrorInferenceFailed, false)
	}
	eventType := "inference.ready"
	if !compatibility.Compatible {
		eventType = "inference.incompatible"
	} else if !snapshot.HasOutput {
		eventType = "inference.empty"
	} else if query.Scenario == appstate.InferenceScenarioRecovered {
		eventType = "inference.recovered"
	}
	return appstate.InferenceWorkspaceMessage{
		Event:    event(appstate.EventID(fmt.Sprintf("evt-inference-%06d", query.Generation)), eventType, now, query.CorrelationID),
		Snapshot: snapshot,
	}, nil
}

// ExportInference prepares a checksummed in-memory representation off the UI thread.
func (client *DemoCoordinator) ExportInference(
	ctx context.Context,
	command appstate.ExportInferenceCommand,
) (appstate.InferenceExportMessage, error) {
	if err := command.Validate(); err != nil {
		return appstate.InferenceExportMessage{}, phase9Failure(err.Error(), command.CorrelationID, appstate.ErrorExportFailed, false)
	}
	if err := client.wait(ctx, client.delays.Export); err != nil {
		return appstate.InferenceExportMessage{}, err
	}
	if client.failures.Export {
		return appstate.InferenceExportMessage{}, phase9Failure("demo export failed", command.CorrelationID, appstate.ErrorExportFailed, true)
	}
	now := client.baseTime().Add(time.Duration(command.Generation) * time.Second)
	client.mu.Lock()
	defer client.mu.Unlock()
	if prior, found := client.exportCommands[command.CommandID]; found {
		return cloneExportMessage(prior), nil
	}
	var output appstate.InferenceOutput
	found := false
	for _, candidate := range client.inferenceHistory {
		if candidate.ID == command.InferenceID {
			output, found = candidate.Clone(), true
			break
		}
	}
	if !found {
		return appstate.InferenceExportMessage{}, phase9Failure("inference output is not complete or known", command.CorrelationID, appstate.ErrorExportFailed, false)
	}
	export := appstate.ExportSnapshot{State: appstate.ExportReady, InferenceID: output.ID,
		Checksum: fmt.Sprintf("export-%s-%06d", output.Checksum, command.Generation),
		Bytes:    uint64(len(output.Predictions)*192 + len(output.Rankings)*96), CompletedAt: now}
	message := appstate.InferenceExportMessage{
		Event:      event(appstate.EventID("evt-export-"+string(command.CommandID)), "inference.export_ready", now, command.CorrelationID),
		Generation: command.Generation, CommandID: command.CommandID, Export: export,
	}
	client.exportCommands[command.CommandID] = message
	return cloneExportMessage(message), nil
}

func buildDemoModelRegistry(now time.Time, summaries []appstate.ModelSummary, seed uint64) []appstate.ModelArtifact {
	registry := make([]appstate.ModelArtifact, 0, len(summaries))
	for index, summary := range summaries {
		metadata := appstate.ArtifactMetadata{
			SchemaVersion: "artifact/v1", EngineVersion: "corthena-tree/v1", FeatureSchema: "features/v1",
			Target:              appstate.TargetSpec{Kind: "forward_open_return", HorizonBars: 5, LogReturn: true},
			TrainingFingerprint: fmt.Sprintf("train-%s", summary.ArtifactFingerprint), TrainingCutoff: summary.TrainingCutoff,
			Seed: seed + uint64(index), GeneratorVersion: "counter-v1", BuildRevision: "demo-phase9",
			Configuration:       []appstate.ModelConfigurationValue{{Name: "max_depth", Value: "6"}, {Name: "min_leaf_samples", Value: "32"}, {Name: "estimators", Value: "128"}},
			FeatureFingerprints: []string{"ret_5@v1", "volatility_20@v1", "cross_rank_1@v1"},
			Checksums:           []appstate.ArtifactChecksum{{Path: "manifest.json", SHA256: fmt.Sprintf("manifest-%s", summary.ArtifactFingerprint), Bytes: 1536}, {Path: "trees.arrow", SHA256: fmt.Sprintf("trees-%s", summary.ArtifactFingerprint), Bytes: 4096}},
			RequiredLookback:    20,
		}
		registry = append(registry, appstate.ModelArtifact{
			Summary: summary.Clone(), FinalRefit: true, Metadata: metadata, ArtifactComplete: true,
			Importance: []appstate.FeatureImportance{{Feature: "ret_5", Gain: 0.47}, {Feature: "volatility_20", Gain: 0.32}, {Feature: "cross_rank_1", Gain: 0.21}},
			Trees: []appstate.TreeBuffer{{
				FeatureIndices: []int{0, -1, -1}, LeftChildren: []int{1, -1, -1}, RightChildren: []int{2, -1, -1},
				Thresholds: []float64{0.014, 0, 0}, LeafValues: []float64{0, -0.018, 0.023}, Leaves: []bool{false, true, true}, MissingGoLeft: []bool{true, false, false},
			}},
		})
	}
	sort.Slice(registry, func(left int, right int) bool { return registry[left].Summary.ID < registry[right].Summary.ID })
	return registry
}

func compatibilityFor(query appstate.InferenceWorkspaceQuery, artifact appstate.ModelArtifact, found bool, dataset appstate.DatasetSummary, datasetFound bool) appstate.CompatibilitySummary {
	diagnostics := make([]appstate.CompatibilityDiagnostic, 0, 8)
	if !found {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "model", Expected: "registered final-refit model", Actual: string(query.ModelID), Message: "model or alias does not resolve"})
		return appstate.CompatibilitySummary{Diagnostics: diagnostics}
	}
	if artifact.Metadata.SchemaVersion != "artifact/v1" {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "schema_version", Expected: "artifact/v1", Actual: artifact.Metadata.SchemaVersion, Message: "artifact schema is unsupported"})
	}
	if artifact.Metadata.EngineVersion != "corthena-tree/v1" {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "engine_version", Expected: "corthena-tree/v1", Actual: artifact.Metadata.EngineVersion, Message: "model engine is unsupported"})
	}
	if !datasetFound {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "dataset", Expected: "known dataset", Actual: string(query.DatasetID), Message: "dataset is unavailable"})
		return appstate.CompatibilitySummary{Diagnostics: diagnostics}
	}
	if query.DatasetRevision != dataset.Revision {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "dataset_revision", Expected: fmt.Sprintf("%d", dataset.Revision), Actual: fmt.Sprintf("%d", query.DatasetRevision), Message: "dataset revision changed"})
	}
	if query.DatasetFingerprint != dataset.Fingerprint {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "dataset_fingerprint", Expected: dataset.Fingerprint, Actual: query.DatasetFingerprint, Message: "dataset fingerprint changed"})
	}
	if artifact.Metadata.FeatureSchema != "features/v1" || len(artifact.Metadata.FeatureFingerprints) != len(artifact.Summary.FeatureNames) {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "feature_schema", Expected: "features/v1 with registered fingerprints", Actual: artifact.Metadata.FeatureSchema, Message: "feature registry is incompatible"})
	}
	if artifact.Metadata.Target.Kind != "forward_open_return" || artifact.Metadata.Target.HorizonBars != 5 {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "target", Expected: "forward_open_return/5", Actual: artifact.Metadata.Target.Kind, Message: "target definition is incompatible"})
	}
	if query.Mode == appstate.InferenceHistorical && query.TimeRange.Start.Before(dataset.Start.AddDate(0, 0, artifact.Metadata.RequiredLookback)) {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "range.start", Expected: dataset.Start.AddDate(0, 0, artifact.Metadata.RequiredLookback).Format(time.RFC3339), Actual: query.TimeRange.Start.Format(time.RFC3339), Message: "requested range lacks required lookback"})
	}
	if query.Mode == appstate.InferenceHistorical && query.TimeRange.End.After(dataset.End) {
		diagnostics = append(diagnostics, appstate.CompatibilityDiagnostic{Field: "range.end", Expected: dataset.End.Format(time.RFC3339), Actual: query.TimeRange.End.Format(time.RFC3339), Message: "requested range exceeds imported data"})
	}
	return appstate.CompatibilitySummary{Compatible: len(diagnostics) == 0, Diagnostics: diagnostics}
}

func buildInferenceOutput(now time.Time, seed uint64, query appstate.InferenceWorkspaceQuery, artifact appstate.ModelArtifact) appstate.InferenceOutput {
	symbols := append([]appstate.Symbol(nil), query.Symbols...)
	sort.Slice(symbols, func(left int, right int) bool { return symbols[left] < symbols[right] })
	timestamp := now
	if query.Mode == appstate.InferenceHistorical {
		timestamp = query.TimeRange.End.UTC()
	}
	predictions := make([]appstate.Prediction, 0, len(symbols))
	for index, symbol := range symbols {
		score := 0.68 + float64(seed%11)/1000 - float64(index/2)*0.15
		predictions = append(predictions, appstate.Prediction{
			ID: fmt.Sprintf("prediction-%s-%s", symbol, timestamp.Format("20060102T150405")), Symbol: symbol, Timestamp: timestamp,
			ModelID: artifact.Summary.ID, RunID: artifact.Summary.RunID, DatasetFingerprint: query.DatasetFingerprint,
			FeatureFingerprints: append([]string(nil), artifact.Metadata.FeatureFingerprints...), Score: score,
		})
	}
	rows := appstate.RankPredictions(predictions)
	bins := distributionFor(predictions)
	output := appstate.InferenceOutput{
		ID: appstate.InferenceID(fmt.Sprintf("inference-%s-%06d", artifact.Summary.ID, query.Generation)), ModelID: artifact.Summary.ID,
		RunID: artifact.Summary.RunID, DatasetID: query.DatasetID, Fingerprint: query.DatasetFingerprint, Mode: query.Mode,
		TimeRange: query.TimeRange.Normalize(), Predictions: predictions,
		Rankings: []appstate.TimestampRanking{{Timestamp: timestamp, Rows: rows}}, Distribution: bins,
		CompletedAt: now, Checksum: fmt.Sprintf("prediction-%s-%06d", artifact.Summary.ArtifactFingerprint, query.Generation),
		Export: appstate.ExportSnapshot{State: appstate.ExportIdle},
	}
	return output
}

func distributionFor(predictions []appstate.Prediction) []appstate.HistogramBin {
	bins := []appstate.HistogramBin{{Minimum: -1, Maximum: 0, Count: 0}, {Minimum: 0, Maximum: 0.5, Count: 0}, {Minimum: 0.5, Maximum: 1, Count: 0}}
	for _, prediction := range predictions {
		if prediction.Missing || prediction.Ineligible {
			continue
		}
		if prediction.Score < 0 {
			bins[0].Count++
		} else if prediction.Score < 0.5 {
			bins[1].Count++
		} else {
			bins[2].Count++
		}
	}
	return bins
}

func findArtifact(artifacts []appstate.ModelArtifact, id appstate.ModelID) (appstate.ModelArtifact, bool) {
	for _, artifact := range artifacts {
		if artifact.Summary.ID == id {
			return artifact.Clone(), true
		}
	}
	return appstate.ModelArtifact{}, false
}

func resolveArtifact(artifacts []appstate.ModelArtifact, id appstate.ModelID, alias string) (appstate.ModelArtifact, bool) {
	if id != "" {
		return findArtifact(artifacts, id)
	}
	for _, artifact := range artifacts {
		if artifact.Summary.Alias == alias {
			return artifact.Clone(), true
		}
	}
	return appstate.ModelArtifact{}, false
}

func findDataset(datasets []appstate.DatasetSummary, id appstate.DatasetID) (appstate.DatasetSummary, bool) {
	for _, dataset := range datasets {
		if dataset.ID == id {
			return dataset.Clone(), true
		}
	}
	return appstate.DatasetSummary{}, false
}

func cloneRegistry(input []appstate.ModelArtifact) []appstate.ModelArtifact {
	output := make([]appstate.ModelArtifact, len(input))
	for index, artifact := range input {
		output[index] = artifact.Clone()
	}
	return output
}

func cloneAliasHistory(input []appstate.AliasHistoryEntry) []appstate.AliasHistoryEntry {
	output := make([]appstate.AliasHistoryEntry, len(input))
	for index, entry := range input {
		output[index] = entry.Clone()
	}
	return output
}

func cloneInferenceOutputs(input []appstate.InferenceOutput) []appstate.InferenceOutput {
	output := make([]appstate.InferenceOutput, len(input))
	for index, item := range input {
		output[index] = item.Clone()
	}
	return output
}

func appendBoundedInferenceHistory(input []appstate.InferenceOutput, output appstate.InferenceOutput) []appstate.InferenceOutput {
	result := cloneInferenceOutputs(input)
	for index := range result {
		if result[index].ID == output.ID {
			result[index] = output.Clone()
			return result
		}
	}
	result = append(result, output.Clone())
	if len(result) <= maxDemoInferenceHistory {
		return result
	}
	trimmed := make([]appstate.InferenceOutput, maxDemoInferenceHistory)
	copy(trimmed, result[len(result)-maxDemoInferenceHistory:])
	return trimmed
}

func cloneDatasetsForPhase9(input []appstate.DatasetSummary) []appstate.DatasetSummary {
	output := make([]appstate.DatasetSummary, len(input))
	for index, dataset := range input {
		output[index] = dataset.Clone()
	}
	return output
}

func cloneAliasMessage(message appstate.AliasAssignedMessage) appstate.AliasAssignedMessage {
	message.Event.Timestamp = message.Event.Timestamp.UTC()
	message.Snapshot = message.Snapshot.Clone()
	return message
}

func cloneExportMessage(message appstate.InferenceExportMessage) appstate.InferenceExportMessage {
	message.Event.Timestamp = message.Event.Timestamp.UTC()
	message.Export = message.Export.Clone()
	return message
}

func phase9Failure(message string, correlationID appstate.CorrelationID, code appstate.ErrorCode, retryable bool) error {
	return DemoError{Snapshot: appstate.ErrorSnapshot{Code: code, Message: message, Retryable: retryable, CorrelationID: correlationID}}
}
