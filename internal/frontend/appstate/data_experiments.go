package appstate

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	// ErrInvalidDataRequest identifies a malformed Data workspace request.
	ErrInvalidDataRequest = errors.New("invalid data request")
	// ErrInvalidExperiment identifies a malformed experiment draft or request.
	ErrInvalidExperiment = errors.New("invalid experiment")
)

// DataScenario selects a deterministic catalog/import condition.
type DataScenario string

const (
	DataScenarioNormal    DataScenario = "normal"
	DataScenarioLoading   DataScenario = "loading"
	DataScenarioEmpty     DataScenario = "empty"
	DataScenarioDuplicate DataScenario = "duplicate"
	DataScenarioMalformed DataScenario = "malformed"
	DataScenarioFailure   DataScenario = "failure"
	DataScenarioDegraded  DataScenario = "degraded"
	DataScenarioRecovered DataScenario = "recovered"
	DataScenarioCancelled DataScenario = "cancelled"
	DataScenarioSaturated DataScenario = "queue_saturated"
)

// Valid reports whether scenario is supported.
func (scenario DataScenario) Valid() bool {
	switch scenario {
	case DataScenarioNormal, DataScenarioLoading, DataScenarioEmpty,
		DataScenarioDuplicate, DataScenarioMalformed, DataScenarioFailure,
		DataScenarioDegraded, DataScenarioRecovered, DataScenarioCancelled,
		DataScenarioSaturated:
		return true
	default:
		return false
	}
}

// DataScenarios returns stable UI ordering.
func DataScenarios() []DataScenario {
	return []DataScenario{
		DataScenarioNormal, DataScenarioLoading, DataScenarioEmpty,
		DataScenarioDuplicate, DataScenarioMalformed, DataScenarioFailure,
		DataScenarioDegraded, DataScenarioRecovered, DataScenarioCancelled,
		DataScenarioSaturated,
	}
}

// DataWorkspaceQuery identifies one generation-ordered catalog request.
type DataWorkspaceQuery struct {
	CorrelationID CorrelationID
	Generation    uint64
	Scenario      DataScenario
}

// Validate checks the complete query identity.
func (query DataWorkspaceQuery) Validate() error {
	if query.CorrelationID == "" || query.Generation == 0 || !query.Scenario.Valid() {
		return fmt.Errorf("%w: correlation, generation, and scenario are required", ErrInvalidDataRequest)
	}
	return nil
}

// DataImportMode selects atomic append or selected-range replacement.
type DataImportMode string

const (
	DataImportAppend      DataImportMode = "append"
	DataImportReplacement DataImportMode = "range_replacement"
)

// Valid reports whether the import mode is supported.
func (mode DataImportMode) Valid() bool {
	return mode == DataImportAppend || mode == DataImportReplacement
}

// DataSourceKind identifies a supported simulated source format.
type DataSourceKind string

const (
	DataSourceCSV     DataSourceKind = "csv"
	DataSourceParquet DataSourceKind = "parquet"
)

// Valid reports whether the source kind is supported.
func (kind DataSourceKind) Valid() bool {
	return kind == DataSourceCSV || kind == DataSourceParquet
}

// DataImportRequest is an immutable simulated import command.
type DataImportRequest struct {
	CorrelationID CorrelationID
	CommandID     CorrelationID
	Generation    uint64
	DatasetID     DatasetID
	SourceName    string
	SourceKind    DataSourceKind
	Mode          DataImportMode
	Symbols       []Symbol
	Interval      BarInterval
	TimeRange     TimeRange
	Adjustment    string
	Scenario      DataScenario
}

// Clone returns an independent normalized command.
func (request DataImportRequest) Clone() DataImportRequest {
	request.Symbols = cloneSymbols(request.Symbols)
	request.TimeRange = request.TimeRange.Normalize()
	return request
}

// Validate rejects ambiguous or unsafe import combinations.
func (request DataImportRequest) Validate() error {
	if request.CorrelationID == "" || request.CommandID == "" || request.Generation == 0 || request.DatasetID == "" {
		return fmt.Errorf("%w: correlation, command, generation, and dataset are required", ErrInvalidDataRequest)
	}
	if strings.TrimSpace(request.SourceName) == "" || !request.SourceKind.Valid() || !request.Mode.Valid() || !request.Interval.Valid() || !request.Scenario.Valid() {
		return fmt.Errorf("%w: invalid source, mode, interval, or scenario", ErrInvalidDataRequest)
	}
	if request.Adjustment != "split_dividend_adjusted" {
		return fmt.Errorf("%w: adjustment metadata is required", ErrInvalidDataRequest)
	}
	if len(request.Symbols) == 0 || len(request.Symbols) > 64 {
		return fmt.Errorf("%w: one to 64 symbols are required", ErrInvalidDataRequest)
	}
	seen := make(map[Symbol]struct{}, len(request.Symbols))
	for _, symbol := range request.Symbols {
		if strings.TrimSpace(string(symbol)) == "" {
			return fmt.Errorf("%w: symbol is empty", ErrInvalidDataRequest)
		}
		if _, duplicate := seen[symbol]; duplicate {
			return fmt.Errorf("%w: duplicate symbol %q", ErrInvalidDataRequest, symbol)
		}
		seen[symbol] = struct{}{}
	}
	if request.Mode == DataImportAppend {
		if !request.TimeRange.Start.IsZero() || !request.TimeRange.End.IsZero() {
			return fmt.Errorf("%w: append cannot declare a replacement range", ErrInvalidDataRequest)
		}
		return nil
	}
	if request.TimeRange.Start.Location() != time.UTC || request.TimeRange.End.Location() != time.UTC ||
		request.TimeRange.Start.IsZero() || request.TimeRange.End.IsZero() || !request.TimeRange.Start.Before(request.TimeRange.End) {
		return fmt.Errorf("%w: replacement requires an ordered UTC range", ErrInvalidDataRequest)
	}
	return nil
}

// ValidationSeverity classifies import or experiment diagnostics.
type ValidationSeverity string

const (
	ValidationInfo    ValidationSeverity = "info"
	ValidationWarning ValidationSeverity = "warning"
	ValidationError   ValidationSeverity = "error"
)

// ValidationDiagnostic is one stable field-addressed message.
type ValidationDiagnostic struct {
	Code     string
	Field    string
	Message  string
	Severity ValidationSeverity
	Row      uint64
}

// DataImportState is a simulated import lifecycle state.
type DataImportState string

const (
	DataImportIdle       DataImportState = "idle"
	DataImportQueued     DataImportState = "queued"
	DataImportValidating DataImportState = "validating"
	DataImportRunning    DataImportState = "importing"
	DataImportReady      DataImportState = "ready"
	DataImportRejected   DataImportState = "rejected"
	DataImportFailed     DataImportState = "failed"
	DataImportCancelled  DataImportState = "cancelled"
	DataImportBusy       DataImportState = "queue_saturated"
)

// DataCoverage describes one symbol's catalog range.
type DataCoverage struct {
	Symbol Symbol
	Start  time.Time
	End    time.Time
	Rows   uint64
}

// DataImportRecord is one stable import/validation queue row.
type DataImportRecord struct {
	ID                   string
	Request              DataImportRequest
	State                DataImportState
	ProgressPermil       int
	Diagnostics          []ValidationDiagnostic
	StartedAt            time.Time
	CompletedAt          time.Time
	PublishedRevision    uint64
	PublishedFingerprint string
}

// Clone returns an independent record.
func (record DataImportRecord) Clone() DataImportRecord {
	record.Request = record.Request.Clone()
	record.Diagnostics = append([]ValidationDiagnostic(nil), record.Diagnostics...)
	record.StartedAt = record.StartedAt.UTC()
	if !record.CompletedAt.IsZero() {
		record.CompletedAt = record.CompletedAt.UTC()
	}
	return record
}

// DataLogEntry is one deterministic structured import log row.
type DataLogEntry struct {
	ID        string
	ImportID  string
	Timestamp time.Time
	Level     ValidationSeverity
	Message   string
}

// DataWorkspaceSnapshot is one immutable catalog/import response.
type DataWorkspaceSnapshot struct {
	Query      DataWorkspaceQuery
	Catalog    []DatasetSummary
	Coverage   []DataCoverage
	Imports    []DataImportRecord
	Logs       []DataLogEntry
	PreparedAt time.Time
	Degraded   bool
}

// Clone returns an independent snapshot.
func (snapshot DataWorkspaceSnapshot) Clone() DataWorkspaceSnapshot {
	snapshot.Catalog = cloneDatasets(snapshot.Catalog)
	snapshot.Coverage = append([]DataCoverage(nil), snapshot.Coverage...)
	for index := range snapshot.Coverage {
		snapshot.Coverage[index].Start = snapshot.Coverage[index].Start.UTC()
		snapshot.Coverage[index].End = snapshot.Coverage[index].End.UTC()
	}
	if len(snapshot.Imports) > 0 {
		imports := make([]DataImportRecord, len(snapshot.Imports))
		for index, record := range snapshot.Imports {
			imports[index] = record.Clone()
		}
		snapshot.Imports = imports
	}
	snapshot.Logs = append([]DataLogEntry(nil), snapshot.Logs...)
	for index := range snapshot.Logs {
		snapshot.Logs[index].Timestamp = snapshot.Logs[index].Timestamp.UTC()
	}
	snapshot.PreparedAt = snapshot.PreparedAt.UTC()
	return snapshot
}

// DataWorkspaceMessage publishes catalog/import state.
type DataWorkspaceMessage struct {
	Event    EventEnvelope
	Snapshot DataWorkspaceSnapshot
}

func (DataWorkspaceMessage) isClientMessage() {}

// WorkspaceLoadState is shared by Data and Experiments panels.
type WorkspaceLoadState string

const (
	WorkspaceIdle      WorkspaceLoadState = "idle"
	WorkspaceLoading   WorkspaceLoadState = "loading"
	WorkspaceReady     WorkspaceLoadState = "ready"
	WorkspaceEmpty     WorkspaceLoadState = "empty"
	WorkspaceFailed    WorkspaceLoadState = "error"
	WorkspaceDegraded  WorkspaceLoadState = "degraded"
	WorkspaceRecovered WorkspaceLoadState = "recovered"
	WorkspaceCancelled WorkspaceLoadState = "cancelled"
	WorkspaceBusy      WorkspaceLoadState = "queue_saturated"
)

// DataWorkspaceState owns Data selection and asynchronous state.
type DataWorkspaceState struct {
	Generation        uint64
	Scenario          DataScenario
	State             WorkspaceLoadState
	Stale             bool
	Query             DataWorkspaceQuery
	Snapshot          DataWorkspaceSnapshot
	SelectedDatasetID DatasetID
	SelectedImportID  string
	Error             ErrorSnapshot
}

// DefaultDataWorkspaceState returns canonical demo state.
func DefaultDataWorkspaceState() DataWorkspaceState {
	return DataWorkspaceState{Scenario: DataScenarioNormal, State: WorkspaceIdle}
}

// Clone returns an independent state.
func (state DataWorkspaceState) Clone() DataWorkspaceState {
	state.Snapshot = state.Snapshot.Clone()
	return state
}

// ExperimentScenario selects deterministic experiment workflow behavior.
type ExperimentScenario string

const (
	ExperimentScenarioNormal    ExperimentScenario = "normal"
	ExperimentScenarioLoading   ExperimentScenario = "loading"
	ExperimentScenarioEmpty     ExperimentScenario = "empty"
	ExperimentScenarioInvalid   ExperimentScenario = "invalid"
	ExperimentScenarioFailure   ExperimentScenario = "failure"
	ExperimentScenarioDegraded  ExperimentScenario = "degraded"
	ExperimentScenarioRecovered ExperimentScenario = "recovered"
	ExperimentScenarioCancelled ExperimentScenario = "cancelled"
	ExperimentScenarioSaturated ExperimentScenario = "queue_saturated"
)

// Valid reports whether scenario is supported.
func (scenario ExperimentScenario) Valid() bool {
	switch scenario {
	case ExperimentScenarioNormal, ExperimentScenarioLoading,
		ExperimentScenarioEmpty, ExperimentScenarioInvalid,
		ExperimentScenarioFailure, ExperimentScenarioDegraded,
		ExperimentScenarioRecovered, ExperimentScenarioCancelled,
		ExperimentScenarioSaturated:
		return true
	default:
		return false
	}
}

// ExperimentScenarios returns stable UI ordering.
func ExperimentScenarios() []ExperimentScenario {
	return []ExperimentScenario{
		ExperimentScenarioNormal, ExperimentScenarioLoading,
		ExperimentScenarioEmpty, ExperimentScenarioInvalid,
		ExperimentScenarioFailure, ExperimentScenarioDegraded,
		ExperimentScenarioRecovered, ExperimentScenarioCancelled,
		ExperimentScenarioSaturated,
	}
}

// FeatureDescriptorSummary identifies one compiled feature implementation.
type FeatureDescriptorSummary struct {
	Name        FeatureName
	Version     string
	Lookback    int
	Output      string
	Fingerprint string
}

// ExperimentSweep describes an optional deterministic parameter sweep.
type ExperimentSweep struct {
	Enabled       bool `json:"enabled"`
	DepthMinimum  int  `json:"depth_minimum"`
	DepthMaximum  int  `json:"depth_maximum"`
	EstimatorStep int  `json:"estimator_step"`
}

// ExperimentQuery identifies one generation-ordered workspace request.
type ExperimentQuery struct {
	CorrelationID CorrelationID
	Generation    uint64
	Scenario      ExperimentScenario
}

// Validate checks the request identity.
func (query ExperimentQuery) Validate() error {
	if query.CorrelationID == "" || query.Generation == 0 || !query.Scenario.Valid() {
		return fmt.Errorf("%w: correlation, generation, and scenario are required", ErrInvalidExperiment)
	}
	return nil
}

// ExperimentSection is one searchable configuration-tree node.
type ExperimentSection string

const (
	ExperimentSectionDataset   ExperimentSection = "dataset"
	ExperimentSectionFeatures  ExperimentSection = "features"
	ExperimentSectionTarget    ExperimentSection = "target"
	ExperimentSectionSplit     ExperimentSection = "split"
	ExperimentSectionModel     ExperimentSection = "model"
	ExperimentSectionPortfolio ExperimentSection = "portfolio"
	ExperimentSectionSweep     ExperimentSection = "sweep"
)

// ExperimentSections returns the stable editor order.
func ExperimentSections() []ExperimentSection {
	return []ExperimentSection{
		ExperimentSectionDataset, ExperimentSectionFeatures,
		ExperimentSectionTarget, ExperimentSectionSplit,
		ExperimentSectionModel, ExperimentSectionPortfolio,
		ExperimentSectionSweep,
	}
}

// ExperimentValidationIssue is one field-addressed draft error/warning.
type ExperimentValidationIssue struct {
	Code     string
	Section  ExperimentSection
	Field    string
	Message  string
	Severity ValidationSeverity
}

// ExperimentResourceEstimate is deterministic preparation output.
type ExperimentResourceEstimate struct {
	Rows              uint64
	FeatureValues     uint64
	EstimatedBytes    uint64
	EstimatedSeconds  int
	RequestedCPU      int
	SweepCombinations int
}

// ExperimentDefinition is an immutable accepted experiment.
type ExperimentDefinition struct {
	ID          ExperimentID
	CommandID   CorrelationID
	Draft       ExperimentDraft
	SubmittedAt time.Time
	Immutable   bool
}

// Clone returns an independent definition.
func (definition ExperimentDefinition) Clone() ExperimentDefinition {
	definition.Draft = definition.Draft.Clone()
	definition.SubmittedAt = definition.SubmittedAt.UTC()
	return definition
}

// ExperimentWorkspaceSnapshot is the immutable editor/query response.
type ExperimentWorkspaceSnapshot struct {
	Query       ExperimentQuery
	Definitions []ExperimentDefinition
	Features    []FeatureDescriptorSummary
	PreparedAt  time.Time
	Degraded    bool
}

// Clone returns an independent snapshot.
func (snapshot ExperimentWorkspaceSnapshot) Clone() ExperimentWorkspaceSnapshot {
	if len(snapshot.Definitions) > 0 {
		definitions := make([]ExperimentDefinition, len(snapshot.Definitions))
		for index, definition := range snapshot.Definitions {
			definitions[index] = definition.Clone()
		}
		snapshot.Definitions = definitions
	}
	snapshot.Features = append([]FeatureDescriptorSummary(nil), snapshot.Features...)
	snapshot.PreparedAt = snapshot.PreparedAt.UTC()
	return snapshot
}

// ExperimentWorkspaceMessage publishes editor catalog state.
type ExperimentWorkspaceMessage struct {
	Event    EventEnvelope
	Snapshot ExperimentWorkspaceSnapshot
}

func (ExperimentWorkspaceMessage) isClientMessage() {}

// ExperimentEvaluationRequest requests validation and estimation.
type ExperimentEvaluationRequest struct {
	CorrelationID CorrelationID
	Generation    uint64
	Draft         ExperimentDraft
	Scenario      ExperimentScenario
}

// Clone returns an independent request.
func (request ExperimentEvaluationRequest) Clone() ExperimentEvaluationRequest {
	request.Draft = request.Draft.Clone()
	return request
}

// Validate checks request identity and draft revision.
func (request ExperimentEvaluationRequest) Validate() error {
	if request.CorrelationID == "" || request.Generation == 0 || request.Draft.Revision == 0 || !request.Scenario.Valid() {
		return fmt.Errorf("%w: correlation, generation, draft revision, and scenario are required", ErrInvalidExperiment)
	}
	return nil
}

// ExperimentEvaluationMessage publishes generation-safe validation output.
type ExperimentEvaluationMessage struct {
	Event      EventEnvelope
	Generation uint64
	Revision   uint64
	Issues     []ExperimentValidationIssue
	Estimate   ExperimentResourceEstimate
}

func (ExperimentEvaluationMessage) isClientMessage() {}

// ExperimentSubmittedMessage publishes one immutable accepted definition.
type ExperimentSubmittedMessage struct {
	Event      EventEnvelope
	Definition ExperimentDefinition
	Job        JobSummary
}

func (ExperimentSubmittedMessage) isClientMessage() {}

// ExperimentsWorkspaceState owns draft, validation, and submission state.
type ExperimentsWorkspaceState struct {
	Generation           uint64
	EvaluationGeneration uint64
	Scenario             ExperimentScenario
	State                WorkspaceLoadState
	Stale                bool
	Query                ExperimentQuery
	Snapshot             ExperimentWorkspaceSnapshot
	Draft                ExperimentDraft
	SelectedSection      ExperimentSection
	SelectedExperimentID ExperimentID
	Issues               []ExperimentValidationIssue
	Estimate             ExperimentResourceEstimate
	Error                ErrorSnapshot
	DraftPersistence     PersistenceState
}

// DefaultExperimentsWorkspaceState returns canonical editor state.
func DefaultExperimentsWorkspaceState() ExperimentsWorkspaceState {
	return ExperimentsWorkspaceState{
		Scenario: ExperimentScenarioNormal, State: WorkspaceIdle,
		SelectedSection: ExperimentSectionDataset,
	}
}

// Clone returns independent state.
func (state ExperimentsWorkspaceState) Clone() ExperimentsWorkspaceState {
	state.Snapshot = state.Snapshot.Clone()
	state.Draft = state.Draft.Clone()
	state.Issues = append([]ExperimentValidationIssue(nil), state.Issues...)
	return state
}

// ValidateExperimentDraft returns every deterministic field issue.
func ValidateExperimentDraft(draft ExperimentDraft) []ExperimentValidationIssue {
	issues := make([]ExperimentValidationIssue, 0, 8)
	add := func(code string, section ExperimentSection, field string, message string) {
		issues = append(issues, ExperimentValidationIssue{Code: code, Section: section, Field: field, Message: message, Severity: ValidationError})
	}
	if draft.Revision == 0 {
		add("draft.revision", ExperimentSectionDataset, "revision", "Draft revision is required.")
	}
	if strings.TrimSpace(draft.Name) == "" || len(draft.Name) > 80 {
		add("draft.name", ExperimentSectionDataset, "name", "Name must contain 1-80 characters.")
	}
	if draft.DatasetID == "" || draft.DatasetRevision == 0 || draft.DatasetFingerprint == "" {
		add("draft.dataset", ExperimentSectionDataset, "dataset", "Dataset revision and fingerprint are required.")
	}
	if len(draft.Features) == 0 || len(draft.Features) > 64 {
		add("draft.features", ExperimentSectionFeatures, "features", "Select 1-64 compiled features.")
	}
	seen := make(map[FeatureName]struct{}, len(draft.Features))
	for _, feature := range draft.Features {
		if feature == "" {
			add("draft.feature.empty", ExperimentSectionFeatures, "features", "Feature names cannot be empty.")
			continue
		}
		if _, duplicate := seen[feature]; duplicate {
			add("draft.feature.duplicate", ExperimentSectionFeatures, "features", "Feature selection contains a duplicate.")
		}
		seen[feature] = struct{}{}
	}
	if err := draft.Target.Validate(); err != nil {
		add("draft.target", ExperimentSectionTarget, "target", err.Error())
	}
	if draft.Split.Kind != "walk_forward" || draft.Split.TrainBars < 64 || draft.Split.ValidationBars < 16 || draft.Split.TestBars < 16 || draft.Split.PurgeBars < draft.Target.HorizonBars || draft.Split.EmbargoBars < 0 {
		add("draft.split", ExperimentSectionSplit, "split", "Walk-forward windows are invalid or purge is shorter than the target horizon.")
	}
	if !draft.Model.Kind.Valid() || draft.Model.MaxDepth < 1 || draft.Model.MaxDepth > 32 || draft.Model.MinLeafSamples < 2 || draft.Model.EstimatorCount < 1 || draft.Model.HistogramBins < 16 {
		add("draft.model", ExperimentSectionModel, "model", "Model parameters are outside supported bounds.")
	}
	if !finiteExperiment(draft.Portfolio.LongQuantile) || !finiteExperiment(draft.Portfolio.ShortQuantile) || !finiteExperiment(draft.Portfolio.CostBPS) || draft.Portfolio.LongQuantile <= draft.Portfolio.ShortQuantile || draft.Portfolio.CostBPS < 0 {
		add("draft.portfolio", ExperimentSectionPortfolio, "portfolio", "Portfolio quantiles or costs are invalid.")
	}
	if draft.RequestedCPU < 1 || draft.RequestedCPU > 256 {
		add("draft.cpu", ExperimentSectionModel, "requested_cpu", "Requested CPU slots must be between 1 and 256.")
	}
	if draft.Sweep.Enabled && (draft.Sweep.DepthMinimum < 1 || draft.Sweep.DepthMaximum < draft.Sweep.DepthMinimum || draft.Sweep.DepthMaximum > 32 || draft.Sweep.EstimatorStep < 1) {
		add("draft.sweep", ExperimentSectionSweep, "sweep", "Sweep depth range or estimator step is invalid.")
	}
	return issues
}

// Valid reports whether model kind is supported.
func (kind ModelKind) Valid() bool {
	switch kind {
	case ModelHistogramTree, ModelRandomForest, ModelGradientBoost:
		return true
	default:
		return false
	}
}

func finiteExperiment(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
