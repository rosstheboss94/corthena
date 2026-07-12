package simulator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func (client *DemoCoordinator) DataWorkspace(ctx context.Context, query appstate.DataWorkspaceQuery) (appstate.DataWorkspaceMessage, error) {
	if err := query.Validate(); err != nil {
		return appstate.DataWorkspaceMessage{}, dataFailure(err.Error(), query.CorrelationID, appstate.ErrorDataFailed, false)
	}
	if err := client.wait(ctx, client.delays.Data); err != nil {
		return appstate.DataWorkspaceMessage{}, err
	}
	switch query.Scenario {
	case appstate.DataScenarioLoading:
		<-ctx.Done()
		return appstate.DataWorkspaceMessage{}, ctx.Err()
	case appstate.DataScenarioFailure:
		return appstate.DataWorkspaceMessage{}, dataFailure("deterministic Data request failed", query.CorrelationID, appstate.ErrorDataFailed, true)
	case appstate.DataScenarioCancelled:
		return appstate.DataWorkspaceMessage{}, dataFailure("deterministic Data request cancelled", query.CorrelationID, appstate.ErrorDataCancelled, true)
	case appstate.DataScenarioSaturated:
		return appstate.DataWorkspaceMessage{}, dataFailure("deterministic Data queue is saturated", query.CorrelationID, appstate.ErrorEffectBusy, true)
	}
	if client.failures.Data {
		return appstate.DataWorkspaceMessage{}, dataFailure("demo Data client failed", query.CorrelationID, appstate.ErrorDataFailed, true)
	}
	client.mu.RLock()
	snapshot := client.dataSnapshotLocked(query)
	client.mu.RUnlock()
	if query.Scenario == appstate.DataScenarioEmpty {
		snapshot.Catalog = nil
		snapshot.Coverage = nil
		snapshot.Imports = nil
		snapshot.Logs = nil
	}
	snapshot.Degraded = query.Scenario == appstate.DataScenarioDegraded
	eventType := "data.ready"
	if query.Scenario == appstate.DataScenarioRecovered {
		eventType = "data.recovered"
	}
	return appstate.DataWorkspaceMessage{
		Event:    event(appstate.EventID(fmt.Sprintf("evt-data-%06d", query.Generation)), eventType, snapshot.PreparedAt, query.CorrelationID),
		Snapshot: snapshot,
	}, nil
}

func (client *DemoCoordinator) ImportData(ctx context.Context, request appstate.DataImportRequest) (appstate.DataWorkspaceMessage, error) {
	request = request.Clone()
	if err := request.Validate(); err != nil {
		return appstate.DataWorkspaceMessage{}, dataFailure(err.Error(), request.CorrelationID, appstate.ErrorDataFailed, false)
	}
	if err := client.wait(ctx, client.delays.Data); err != nil {
		return appstate.DataWorkspaceMessage{}, err
	}
	switch request.Scenario {
	case appstate.DataScenarioLoading:
		<-ctx.Done()
		return appstate.DataWorkspaceMessage{}, ctx.Err()
	case appstate.DataScenarioFailure:
		return appstate.DataWorkspaceMessage{}, dataFailure("deterministic import failed", request.CorrelationID, appstate.ErrorDataFailed, true)
	case appstate.DataScenarioCancelled:
		return appstate.DataWorkspaceMessage{}, dataFailure("deterministic import cancelled", request.CorrelationID, appstate.ErrorDataCancelled, true)
	case appstate.DataScenarioSaturated:
		return appstate.DataWorkspaceMessage{}, dataFailure("deterministic import queue is saturated", request.CorrelationID, appstate.ErrorEffectBusy, true)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	datasetIndex := -1
	for index, dataset := range client.snapshot.Datasets {
		if dataset.ID == request.DatasetID {
			datasetIndex = index
			break
		}
	}
	if datasetIndex < 0 {
		return appstate.DataWorkspaceMessage{}, dataFailure("import dataset is unknown", request.CorrelationID, appstate.ErrorDataFailed, false)
	}
	dataset := client.snapshot.Datasets[datasetIndex].Clone()
	if dataset.Interval != request.Interval {
		return appstate.DataWorkspaceMessage{}, dataFailure("import interval does not match the catalog", request.CorrelationID, appstate.ErrorDataFailed, false)
	}
	availableSymbols := make(map[appstate.Symbol]struct{}, len(dataset.Symbols))
	for _, symbol := range dataset.Symbols {
		availableSymbols[symbol] = struct{}{}
	}
	for _, symbol := range request.Symbols {
		if _, found := availableSymbols[symbol]; !found {
			return appstate.DataWorkspaceMessage{}, dataFailure("import symbol is outside the selected dataset", request.CorrelationID, appstate.ErrorDataFailed, false)
		}
	}
	if request.Mode == appstate.DataImportReplacement &&
		(request.TimeRange.Start.Before(dataset.Start) || request.TimeRange.End.After(dataset.End)) {
		return appstate.DataWorkspaceMessage{}, dataFailure("replacement range is outside catalog coverage", request.CorrelationID, appstate.ErrorDataFailed, false)
	}
	now := client.snapshot.Event.Timestamp.Add(time.Duration(request.Generation) * time.Second)
	record := appstate.DataImportRecord{
		ID: fmt.Sprintf("import-%s", request.CommandID), Request: request,
		State: appstate.DataImportReady, ProgressPermil: 1000,
		StartedAt: now.Add(-2 * time.Second), CompletedAt: now,
	}
	if request.Scenario == appstate.DataScenarioDuplicate || request.Scenario == appstate.DataScenarioMalformed {
		record.State = appstate.DataImportRejected
		record.ProgressPermil = 350
		diagnostic := appstate.ValidationDiagnostic{Severity: appstate.ValidationError, Row: 17}
		if request.Scenario == appstate.DataScenarioDuplicate {
			diagnostic.Code = "duplicate_key"
			diagnostic.Field = "timestamp"
			diagnostic.Message = "Duplicate (symbol, timestamp) key rejected before catalog publication."
		} else {
			diagnostic.Code = "invalid_ohlc"
			diagnostic.Field = "high"
			diagnostic.Message = "High price is below open or close; import rejected atomically."
		}
		record.Diagnostics = []appstate.ValidationDiagnostic{diagnostic}
	} else {
		dataset.Revision++
		addedRows := uint64(512 * len(request.Symbols))
		if request.Mode == appstate.DataImportReplacement {
			addedRows = uint64(128 * len(request.Symbols))
		}
		dataset.Rows += addedRows
		dataset.Fingerprint = fmt.Sprintf("data-demo-%s-r%03d-%016x", dataset.ID, dataset.Revision, stableTextHash(string(request.CommandID)))
		dataset.Adjustment = request.Adjustment
		dataset.ImportedAt = now
		dataset.Status = appstate.DatasetReady
		client.snapshot.Datasets[datasetIndex] = dataset.Clone()
		record.PublishedRevision = dataset.Revision
		record.PublishedFingerprint = dataset.Fingerprint
	}
	client.imports = append([]appstate.DataImportRecord{record.Clone()}, client.imports...)
	level := appstate.ValidationInfo
	message := fmt.Sprintf("Published dataset revision %d atomically.", record.PublishedRevision)
	if record.State == appstate.DataImportRejected {
		level = appstate.ValidationError
		message = record.Diagnostics[0].Message
	}
	client.logs = append([]appstate.DataLogEntry{{
		ID: fmt.Sprintf("log-%s", request.CommandID), ImportID: record.ID,
		Timestamp: now, Level: level, Message: message,
	}}, client.logs...)
	query := appstate.DataWorkspaceQuery{CorrelationID: request.CorrelationID, Generation: request.Generation, Scenario: request.Scenario}
	snapshot := client.dataSnapshotLocked(query)
	return appstate.DataWorkspaceMessage{
		Event:    event(appstate.EventID(fmt.Sprintf("evt-import-%06d", request.Generation)), "data.imported", now, request.CorrelationID),
		Snapshot: snapshot,
	}, nil
}

func (client *DemoCoordinator) dataSnapshotLocked(query appstate.DataWorkspaceQuery) appstate.DataWorkspaceSnapshot {
	catalog := make([]appstate.DatasetSummary, len(client.snapshot.Datasets))
	for index, dataset := range client.snapshot.Datasets {
		catalog[index] = dataset.Clone()
	}
	imports := make([]appstate.DataImportRecord, len(client.imports))
	for index, record := range client.imports {
		imports[index] = record.Clone()
	}
	logs := append([]appstate.DataLogEntry(nil), client.logs...)
	return appstate.DataWorkspaceSnapshot{
		Query: query, Catalog: catalog, Coverage: dataCoverage(catalog),
		Imports: imports, Logs: logs,
		PreparedAt: client.snapshot.Event.Timestamp.Add(time.Duration(query.Generation) * time.Millisecond),
	}
}

func dataCoverage(catalog []appstate.DatasetSummary) []appstate.DataCoverage {
	var coverage []appstate.DataCoverage
	for _, dataset := range catalog {
		rows := dataset.Rows
		if len(dataset.Symbols) > 0 {
			rows /= uint64(len(dataset.Symbols))
		}
		for _, symbol := range dataset.Symbols {
			coverage = append(coverage, appstate.DataCoverage{Symbol: symbol, Start: dataset.Start, End: dataset.End, Rows: rows})
		}
	}
	sort.Slice(coverage, func(left int, right int) bool {
		if coverage[left].Symbol == coverage[right].Symbol {
			return coverage[left].Start.Before(coverage[right].Start)
		}
		return coverage[left].Symbol < coverage[right].Symbol
	})
	return coverage
}

func (client *DemoCoordinator) Experiments(ctx context.Context, query appstate.ExperimentQuery) (appstate.ExperimentWorkspaceMessage, error) {
	if err := query.Validate(); err != nil {
		return appstate.ExperimentWorkspaceMessage{}, experimentFailure(err.Error(), query.CorrelationID, appstate.ErrorExperimentFailed, false)
	}
	if err := client.wait(ctx, client.delays.Experiments); err != nil {
		return appstate.ExperimentWorkspaceMessage{}, err
	}
	switch query.Scenario {
	case appstate.ExperimentScenarioLoading:
		<-ctx.Done()
		return appstate.ExperimentWorkspaceMessage{}, ctx.Err()
	case appstate.ExperimentScenarioFailure:
		return appstate.ExperimentWorkspaceMessage{}, experimentFailure("deterministic Experiments request failed", query.CorrelationID, appstate.ErrorExperimentFailed, true)
	case appstate.ExperimentScenarioCancelled:
		return appstate.ExperimentWorkspaceMessage{}, experimentFailure("deterministic Experiments request cancelled", query.CorrelationID, appstate.ErrorExperimentCancelled, true)
	case appstate.ExperimentScenarioSaturated:
		return appstate.ExperimentWorkspaceMessage{}, experimentFailure("deterministic Experiments queue is saturated", query.CorrelationID, appstate.ErrorEffectBusy, true)
	}
	if client.failures.Experiments {
		return appstate.ExperimentWorkspaceMessage{}, experimentFailure("demo Experiments client failed", query.CorrelationID, appstate.ErrorExperimentFailed, true)
	}
	client.mu.RLock()
	definitions := make([]appstate.ExperimentDefinition, len(client.definitions))
	for index, definition := range client.definitions {
		definitions[index] = definition.Clone()
	}
	preparedAt := client.snapshot.Event.Timestamp.Add(time.Duration(query.Generation) * time.Millisecond)
	client.mu.RUnlock()
	if query.Scenario == appstate.ExperimentScenarioEmpty {
		definitions = nil
	}
	snapshot := appstate.ExperimentWorkspaceSnapshot{
		Query: query, Definitions: definitions, Features: demoExperimentFeatures(),
		PreparedAt: preparedAt, Degraded: query.Scenario == appstate.ExperimentScenarioDegraded,
	}
	eventType := "experiments.ready"
	if query.Scenario == appstate.ExperimentScenarioRecovered {
		eventType = "experiments.recovered"
	}
	return appstate.ExperimentWorkspaceMessage{
		Event:    event(appstate.EventID(fmt.Sprintf("evt-experiments-%06d", query.Generation)), eventType, preparedAt, query.CorrelationID),
		Snapshot: snapshot,
	}, nil
}

func (client *DemoCoordinator) EvaluateExperiment(ctx context.Context, request appstate.ExperimentEvaluationRequest) (appstate.ExperimentEvaluationMessage, error) {
	request = request.Clone()
	if err := request.Validate(); err != nil {
		return appstate.ExperimentEvaluationMessage{}, experimentFailure(err.Error(), request.CorrelationID, appstate.ErrorExperimentFailed, false)
	}
	if err := client.wait(ctx, client.delays.Experiments); err != nil {
		return appstate.ExperimentEvaluationMessage{}, err
	}
	if request.Scenario == appstate.ExperimentScenarioLoading {
		<-ctx.Done()
		return appstate.ExperimentEvaluationMessage{}, ctx.Err()
	}
	if request.Scenario == appstate.ExperimentScenarioFailure {
		return appstate.ExperimentEvaluationMessage{}, experimentFailure("deterministic experiment validation failed", request.CorrelationID, appstate.ErrorExperimentFailed, true)
	}
	issues := appstate.ValidateExperimentDraft(request.Draft)
	client.mu.RLock()
	dataset, found := datasetByID(client.snapshot.Datasets, request.Draft.DatasetID)
	preparedAt := client.snapshot.Event.Timestamp.Add(time.Duration(request.Generation) * time.Millisecond)
	client.mu.RUnlock()
	if !found || dataset.Revision != request.Draft.DatasetRevision || dataset.Fingerprint != request.Draft.DatasetFingerprint {
		issues = append(issues, appstate.ExperimentValidationIssue{
			Code: "draft.dataset.stale", Section: appstate.ExperimentSectionDataset,
			Field: "dataset", Message: "Draft dataset revision or fingerprint is stale.", Severity: appstate.ValidationError,
		})
	}
	if request.Scenario == appstate.ExperimentScenarioInvalid {
		issues = append(issues, appstate.ExperimentValidationIssue{
			Code: "scenario.invalid", Section: appstate.ExperimentSectionSplit,
			Field: "split", Message: "Deterministic validation scenario rejected the split.", Severity: appstate.ValidationError,
		})
	}
	estimate := estimateExperiment(dataset, request.Draft)
	return appstate.ExperimentEvaluationMessage{
		Event:      event(appstate.EventID(fmt.Sprintf("evt-experiment-evaluate-%06d", request.Generation)), "experiment.validated", preparedAt, request.CorrelationID),
		Generation: request.Generation, Revision: request.Draft.Revision,
		Issues: issues, Estimate: estimate,
	}, nil
}

func (client *DemoCoordinator) SubmitExperiment(ctx context.Context, command appstate.SubmitExperimentCommand) (appstate.ExperimentSubmittedMessage, error) {
	command.Draft = command.Draft.Clone()
	if command.CorrelationID == "" || command.CommandID == "" || command.Generation == 0 || !command.Scenario.Valid() {
		return appstate.ExperimentSubmittedMessage{}, experimentFailure("invalid experiment submission identity", command.CorrelationID, appstate.ErrorExperimentFailed, false)
	}
	if err := client.wait(ctx, client.delays.Commands); err != nil {
		return appstate.ExperimentSubmittedMessage{}, err
	}
	if command.Scenario == appstate.ExperimentScenarioFailure || client.failures.Commands {
		return appstate.ExperimentSubmittedMessage{}, experimentFailure("deterministic experiment submission failed", command.CorrelationID, appstate.ErrorExperimentFailed, true)
	}
	if command.Scenario == appstate.ExperimentScenarioCancelled {
		return appstate.ExperimentSubmittedMessage{}, experimentFailure("deterministic experiment submission cancelled", command.CorrelationID, appstate.ErrorExperimentCancelled, true)
	}
	issues := appstate.ValidateExperimentDraft(command.Draft)
	if len(issues) != 0 {
		return appstate.ExperimentSubmittedMessage{}, experimentFailure("experiment submission failed validation", command.CorrelationID, appstate.ErrorValidation, false)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	dataset, found := datasetByID(client.snapshot.Datasets, command.Draft.DatasetID)
	if !found || dataset.Revision != command.Draft.DatasetRevision || dataset.Fingerprint != command.Draft.DatasetFingerprint {
		return appstate.ExperimentSubmittedMessage{}, experimentFailure("experiment dataset revision is stale", command.CorrelationID, appstate.ErrorValidation, false)
	}
	if existing, found := client.submissions[command.CommandID]; found {
		if !sameExperimentDraft(existing.Draft, command.Draft) {
			return appstate.ExperimentSubmittedMessage{}, experimentFailure("experiment command ID conflicts with an accepted definition", command.CorrelationID, appstate.ErrorValidation, false)
		}
		return client.submissionMessageLocked(existing, command.CorrelationID), nil
	}
	now := client.snapshot.Event.Timestamp.Add(5 * time.Minute).Add(time.Duration(len(client.definitions)) * time.Second)
	definition := appstate.ExperimentDefinition{
		ID:        appstate.ExperimentID("experiment-" + string(command.CommandID)),
		CommandID: command.CommandID, Draft: command.Draft.Clone(), SubmittedAt: now, Immutable: true,
	}
	client.definitions = append([]appstate.ExperimentDefinition{definition.Clone()}, client.definitions...)
	client.submissions[command.CommandID] = definition.Clone()
	message := client.submissionMessageLocked(definition, command.CorrelationID)
	client.snapshot.Jobs = upsertDemoJob(client.snapshot.Jobs, message.Job)
	return message, nil
}

func (client *DemoCoordinator) submissionMessageLocked(definition appstate.ExperimentDefinition, correlationID appstate.CorrelationID) appstate.ExperimentSubmittedMessage {
	job := appstate.JobSummary{
		ID: appstate.JobID("job-" + string(definition.CommandID)), ExperimentID: definition.ID,
		State: appstate.JobQueued, Stage: "queued", CPUSlots: maxInt(1, definition.Draft.RequestedCPU), UpdatedAt: definition.SubmittedAt,
	}
	return appstate.ExperimentSubmittedMessage{
		Event:      event(appstate.EventID("evt-submit-"+string(definition.CommandID)), "experiment.submitted", definition.SubmittedAt, correlationID),
		Definition: definition.Clone(), Job: job,
	}
}

func buildDemoExperimentDefinitions(now time.Time, datasets []appstate.DatasetSummary) []appstate.ExperimentDefinition {
	if len(datasets) == 0 {
		return nil
	}
	draft := defaultDemoDraft(datasets[0])
	return []appstate.ExperimentDefinition{
		{ID: "experiment-demo-complete", CommandID: "demo-complete", Draft: draft, SubmittedAt: now.Add(-48 * time.Hour), Immutable: true},
		{ID: "experiment-demo-forest", CommandID: "demo-forest", Draft: draft, SubmittedAt: now.Add(-6 * time.Hour), Immutable: true},
	}
}

func defaultDemoDraft(dataset appstate.DatasetSummary) appstate.ExperimentDraft {
	return appstate.ExperimentDraft{
		Revision: 1, Name: "Daily equity baseline", DatasetID: dataset.ID,
		DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint,
		Features:  []appstate.FeatureName{"ret_5", "volatility_20", "volume_z_30"},
		Target:    appstate.TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
		Split:     appstate.SplitSpec{Kind: "walk_forward", TrainBars: 504, ValidationBars: 126, TestBars: 126, PurgeBars: 5, EmbargoBars: 1, ExpandingWindows: true},
		Model:     appstate.ModelSpec{Kind: appstate.ModelRandomForest, MaxDepth: 8, MinLeafSamples: 32, EstimatorCount: 200, HistogramBins: 64, Seed: 42},
		Portfolio: appstate.PortfolioSpec{LongQuantile: 0.8, ShortQuantile: 0.2, CostBPS: 5}, RequestedCPU: 6,
	}
}

func demoExperimentFeatures() []appstate.FeatureDescriptorSummary {
	return []appstate.FeatureDescriptorSummary{
		{Name: "ret_5", Version: "1.0.0", Lookback: 5, Output: "float32", Fingerprint: "demo-ret5-v1"},
		{Name: "volatility_20", Version: "1.0.0", Lookback: 20, Output: "float32", Fingerprint: "demo-vol20-v1"},
		{Name: "volume_z_30", Version: "1.0.0", Lookback: 30, Output: "float32", Fingerprint: "demo-volz30-v1"},
		{Name: "cross_rank_1", Version: "1.0.0", Lookback: 1, Output: "float32", Fingerprint: "demo-cross-rank-v1"},
	}
}

func estimateExperiment(dataset appstate.DatasetSummary, draft appstate.ExperimentDraft) appstate.ExperimentResourceEstimate {
	combinations := 1
	if draft.Sweep.Enabled && draft.Sweep.DepthMaximum >= draft.Sweep.DepthMinimum {
		combinations = (draft.Sweep.DepthMaximum-draft.Sweep.DepthMinimum)/maxInt(1, draft.Sweep.EstimatorStep) + 1
	}
	featureValues := dataset.Rows * uint64(len(draft.Features))
	estimatedBytes := featureValues*4 + dataset.Rows*8
	seconds := int(math.Ceil(float64(featureValues*uint64(combinations)) / float64(maxInt(1, draft.RequestedCPU)*2_000_000)))
	return appstate.ExperimentResourceEstimate{
		Rows: dataset.Rows, FeatureValues: featureValues, EstimatedBytes: estimatedBytes,
		EstimatedSeconds: maxInt(1, seconds), RequestedCPU: draft.RequestedCPU, SweepCombinations: combinations,
	}
}

func datasetByID(datasets []appstate.DatasetSummary, id appstate.DatasetID) (appstate.DatasetSummary, bool) {
	for _, dataset := range datasets {
		if dataset.ID == id {
			return dataset.Clone(), true
		}
	}
	return appstate.DatasetSummary{}, false
}

func sameExperimentDraft(left appstate.ExperimentDraft, right appstate.ExperimentDraft) bool {
	if left.Revision != right.Revision || left.Name != right.Name || left.DatasetID != right.DatasetID ||
		left.DatasetRevision != right.DatasetRevision || left.DatasetFingerprint != right.DatasetFingerprint ||
		left.Target != right.Target || left.Model != right.Model || left.Split != right.Split ||
		left.Portfolio != right.Portfolio || left.Sweep != right.Sweep || left.RequestedCPU != right.RequestedCPU ||
		len(left.Features) != len(right.Features) {
		return false
	}
	for index := range left.Features {
		if left.Features[index] != right.Features[index] {
			return false
		}
	}
	return true
}

func upsertDemoJob(input []appstate.JobSummary, job appstate.JobSummary) []appstate.JobSummary {
	output := append([]appstate.JobSummary(nil), input...)
	for index := range output {
		if output[index].ID == job.ID {
			output[index] = job
			return output
		}
	}
	return append(output, job)
}

func dataFailure(message string, correlationID appstate.CorrelationID, code appstate.ErrorCode, retryable bool) error {
	return DemoError{Snapshot: appstate.ErrorSnapshot{Code: code, Message: message, Retryable: retryable, CorrelationID: correlationID}}
}

func experimentFailure(message string, correlationID appstate.CorrelationID, code appstate.ErrorCode, retryable bool) error {
	return DemoError{Snapshot: appstate.ErrorSnapshot{Code: code, Message: message, Retryable: retryable, CorrelationID: correlationID}}
}
