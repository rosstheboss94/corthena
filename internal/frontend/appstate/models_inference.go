package appstate

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ErrInvalidModels identifies an invalid Models workspace value.
var ErrInvalidModels = errors.New("invalid Models request")

// ErrInvalidInference identifies an invalid Inference workspace value.
var ErrInvalidInference = errors.New("invalid Inference request")

// ModelsScenario selects a deterministic registry condition.
type ModelsScenario string

const (
	ModelsScenarioNormal    ModelsScenario = "normal"
	ModelsScenarioLoading   ModelsScenario = "loading"
	ModelsScenarioEmpty     ModelsScenario = "empty"
	ModelsScenarioFailure   ModelsScenario = "failure"
	ModelsScenarioDegraded  ModelsScenario = "degraded"
	ModelsScenarioRecovered ModelsScenario = "recovered"
	ModelsScenarioCancelled ModelsScenario = "cancelled"
	ModelsScenarioBusy      ModelsScenario = "busy"
)

// Valid reports whether the scenario is supported.
func (scenario ModelsScenario) Valid() bool {
	switch scenario {
	case ModelsScenarioNormal, ModelsScenarioLoading, ModelsScenarioEmpty,
		ModelsScenarioFailure, ModelsScenarioDegraded, ModelsScenarioRecovered,
		ModelsScenarioCancelled, ModelsScenarioBusy:
		return true
	default:
		return false
	}
}

// ModelsScenarios returns the stable scenario-control order.
func ModelsScenarios() []ModelsScenario {
	return []ModelsScenario{ModelsScenarioNormal, ModelsScenarioLoading, ModelsScenarioEmpty,
		ModelsScenarioFailure, ModelsScenarioDegraded, ModelsScenarioRecovered,
		ModelsScenarioCancelled, ModelsScenarioBusy}
}

// InferenceScenario selects a deterministic scoring condition.
type InferenceScenario string

const (
	InferenceScenarioNormal       InferenceScenario = "normal"
	InferenceScenarioLoading      InferenceScenario = "loading"
	InferenceScenarioEmpty        InferenceScenario = "empty"
	InferenceScenarioIncompatible InferenceScenario = "incompatible"
	InferenceScenarioFailure      InferenceScenario = "failure"
	InferenceScenarioDegraded     InferenceScenario = "degraded"
	InferenceScenarioRecovered    InferenceScenario = "recovered"
	InferenceScenarioCancelled    InferenceScenario = "cancelled"
	InferenceScenarioBusy         InferenceScenario = "busy"
)

// Valid reports whether the scenario is supported.
func (scenario InferenceScenario) Valid() bool {
	switch scenario {
	case InferenceScenarioNormal, InferenceScenarioLoading, InferenceScenarioEmpty,
		InferenceScenarioIncompatible, InferenceScenarioFailure, InferenceScenarioDegraded,
		InferenceScenarioRecovered, InferenceScenarioCancelled, InferenceScenarioBusy:
		return true
	default:
		return false
	}
}

// InferenceScenarios returns the stable scenario-control order.
func InferenceScenarios() []InferenceScenario {
	return []InferenceScenario{InferenceScenarioNormal, InferenceScenarioLoading,
		InferenceScenarioEmpty, InferenceScenarioIncompatible, InferenceScenarioFailure,
		InferenceScenarioDegraded, InferenceScenarioRecovered, InferenceScenarioCancelled,
		InferenceScenarioBusy}
}

// InferenceMode selects historical-range or latest-snapshot scoring.
type InferenceMode string

const (
	InferenceHistorical     InferenceMode = "historical"
	InferenceLatestSnapshot InferenceMode = "latest_snapshot"
)

// Valid reports whether the mode is supported.
func (mode InferenceMode) Valid() bool {
	return mode == InferenceHistorical || mode == InferenceLatestSnapshot
}

// ModelsWorkspaceQuery requests one immutable registry generation.
type ModelsWorkspaceQuery struct {
	CorrelationID CorrelationID
	Generation    uint64
	Scenario      ModelsScenario
	Filter        string
	Page          int
	PageSize      int
}

// Clone returns a normalized independent query.
func (query ModelsWorkspaceQuery) Clone() ModelsWorkspaceQuery {
	query.Filter = strings.ToLower(strings.TrimSpace(query.Filter))
	return query
}

// Validate checks query identity, scenario, and pagination bounds.
func (query ModelsWorkspaceQuery) Validate() error {
	if query.CorrelationID == "" || query.Generation == 0 || !query.Scenario.Valid() {
		return fmt.Errorf("%w: correlation, generation, and scenario are required", ErrInvalidModels)
	}
	if query.Page < 0 || query.PageSize < 0 || query.PageSize > 200 {
		return fmt.Errorf("%w: page must be non-negative and page size must be at most 200", ErrInvalidModels)
	}
	return nil
}

// ArtifactMetadata contains immutable, validated artifact provenance.
type ArtifactMetadata struct {
	SchemaVersion       string
	EngineVersion       string
	FeatureSchema       string
	Target              TargetSpec
	TrainingFingerprint string
	TrainingCutoff      time.Time
	Seed                uint64
	GeneratorVersion    string
	BuildRevision       string
	Configuration       []ModelConfigurationValue
	FeatureFingerprints []string
	Checksums           []ArtifactChecksum
	RequiredLookback    int
}

// Clone returns independent metadata.
func (metadata ArtifactMetadata) Clone() ArtifactMetadata {
	metadata.TrainingCutoff = metadata.TrainingCutoff.UTC()
	metadata.Configuration = append([]ModelConfigurationValue(nil), metadata.Configuration...)
	metadata.FeatureFingerprints = append([]string(nil), metadata.FeatureFingerprints...)
	metadata.Checksums = append([]ArtifactChecksum(nil), metadata.Checksums...)
	return metadata
}

// Validate checks the provenance fields required for a completed artifact.
func (metadata ArtifactMetadata) Validate() error {
	if metadata.SchemaVersion == "" || metadata.EngineVersion == "" || metadata.FeatureSchema == "" ||
		metadata.TrainingFingerprint == "" || metadata.TrainingCutoff.IsZero() ||
		metadata.GeneratorVersion == "" || metadata.BuildRevision == "" || metadata.RequiredLookback < 0 {
		return fmt.Errorf("%w: incomplete artifact metadata", ErrInvalidModels)
	}
	if len(metadata.Configuration) == 0 || len(metadata.FeatureFingerprints) == 0 || len(metadata.Checksums) == 0 {
		return fmt.Errorf("%w: configuration, feature fingerprints, and checksums are required", ErrInvalidModels)
	}
	for _, checksum := range metadata.Checksums {
		if err := checksum.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ModelConfigurationValue is one stable model configuration setting.
type ModelConfigurationValue struct {
	Name  string
	Value string
}

// ArtifactChecksum is one complete immutable artifact file checksum.
type ArtifactChecksum struct {
	Path   string
	SHA256 string
	Bytes  uint64
}

// Validate checks a checksum record.
func (checksum ArtifactChecksum) Validate() error {
	if checksum.Path == "" || checksum.SHA256 == "" {
		return fmt.Errorf("%w: checksum path and digest are required", ErrInvalidModels)
	}
	return nil
}

// FeatureImportance is one stable feature contribution row.
type FeatureImportance struct {
	Feature FeatureName
	Gain    float64
}

// TreeBuffer stores one tree in same-length, array-based typed form.
type TreeBuffer struct {
	FeatureIndices []int
	LeftChildren   []int
	RightChildren  []int
	Thresholds     []float64
	LeafValues     []float64
	Leaves         []bool
	MissingGoLeft  []bool
}

// Clone returns independent tree arrays.
func (tree TreeBuffer) Clone() TreeBuffer {
	tree.FeatureIndices = append([]int(nil), tree.FeatureIndices...)
	tree.LeftChildren = append([]int(nil), tree.LeftChildren...)
	tree.RightChildren = append([]int(nil), tree.RightChildren...)
	tree.Thresholds = append([]float64(nil), tree.Thresholds...)
	tree.LeafValues = append([]float64(nil), tree.LeafValues...)
	tree.Leaves = append([]bool(nil), tree.Leaves...)
	tree.MissingGoLeft = append([]bool(nil), tree.MissingGoLeft...)
	return tree
}

// Validate checks lengths, node bounds, leaf/split exclusivity, and cycles.
func (tree TreeBuffer) Validate(featureCount int) error {
	nodes := len(tree.FeatureIndices)
	if nodes == 0 || featureCount <= 0 || len(tree.LeftChildren) != nodes || len(tree.RightChildren) != nodes ||
		len(tree.Thresholds) != nodes || len(tree.LeafValues) != nodes || len(tree.Leaves) != nodes ||
		len(tree.MissingGoLeft) != nodes {
		return fmt.Errorf("%w: tree arrays must be non-empty and same length", ErrInvalidModels)
	}
	for index := 0; index < nodes; index++ {
		if math.IsNaN(tree.Thresholds[index]) || math.IsInf(tree.Thresholds[index], 0) ||
			math.IsNaN(tree.LeafValues[index]) || math.IsInf(tree.LeafValues[index], 0) {
			return fmt.Errorf("%w: tree node %d contains a non-finite value", ErrInvalidModels, index)
		}
		if tree.Leaves[index] {
			if tree.LeftChildren[index] != -1 || tree.RightChildren[index] != -1 || tree.FeatureIndices[index] != -1 {
				return fmt.Errorf("%w: leaf node %d has split data", ErrInvalidModels, index)
			}
			continue
		}
		if tree.FeatureIndices[index] < 0 || tree.FeatureIndices[index] >= featureCount ||
			tree.LeftChildren[index] < 0 || tree.LeftChildren[index] >= nodes ||
			tree.RightChildren[index] < 0 || tree.RightChildren[index] >= nodes ||
			tree.LeftChildren[index] == index || tree.RightChildren[index] == index {
			return fmt.Errorf("%w: split node %d has invalid feature or child", ErrInvalidModels, index)
		}
	}
	marks := make([]uint8, nodes)
	var visit func(int) error
	visit = func(index int) error {
		switch marks[index] {
		case 1:
			return fmt.Errorf("%w: tree contains a cycle at node %d", ErrInvalidModels, index)
		case 2:
			return nil
		}
		marks[index] = 1
		if !tree.Leaves[index] {
			if err := visit(tree.LeftChildren[index]); err != nil {
				return err
			}
			if err := visit(tree.RightChildren[index]); err != nil {
				return err
			}
		}
		marks[index] = 2
		return nil
	}
	if err := visit(0); err != nil {
		return err
	}
	for index, mark := range marks {
		if mark != 2 {
			return fmt.Errorf("%w: tree node %d is unreachable", ErrInvalidModels, index)
		}
	}
	return nil
}

// ModelArtifact is one final-refit immutable inference artifact.
type ModelArtifact struct {
	Summary          ModelSummary
	FinalRefit       bool
	Metadata         ArtifactMetadata
	Importance       []FeatureImportance
	Trees            []TreeBuffer
	ArtifactComplete bool
}

// Clone returns an independent immutable artifact.
func (artifact ModelArtifact) Clone() ModelArtifact {
	artifact.Summary = artifact.Summary.Clone()
	artifact.Metadata = artifact.Metadata.Clone()
	artifact.Importance = append([]FeatureImportance(nil), artifact.Importance...)
	if len(artifact.Trees) != 0 {
		trees := make([]TreeBuffer, len(artifact.Trees))
		for index, tree := range artifact.Trees {
			trees[index] = tree.Clone()
		}
		artifact.Trees = trees
	}
	return artifact
}

// Validate checks whether an artifact is safe to publish to the workspace.
func (artifact ModelArtifact) Validate() error {
	if artifact.Summary.ID == "" || artifact.Summary.RunID == "" || !artifact.Summary.Immutable || !artifact.FinalRefit ||
		!artifact.ArtifactComplete || len(artifact.Summary.FeatureNames) == 0 {
		return fmt.Errorf("%w: only complete immutable final-refit artifacts are publishable", ErrInvalidModels)
	}
	if err := artifact.Metadata.Validate(); err != nil {
		return err
	}
	if len(artifact.Importance) == 0 || len(artifact.Trees) == 0 {
		return fmt.Errorf("%w: importance and trees are required", ErrInvalidModels)
	}
	for _, importance := range artifact.Importance {
		if importance.Feature == "" || math.IsNaN(importance.Gain) || math.IsInf(importance.Gain, 0) || importance.Gain < 0 {
			return fmt.Errorf("%w: invalid feature importance", ErrInvalidModels)
		}
	}
	for _, tree := range artifact.Trees {
		if err := tree.Validate(len(artifact.Summary.FeatureNames)); err != nil {
			return err
		}
	}
	return nil
}

// AliasHistoryEntry is an immutable transactional alias-promotion record.
type AliasHistoryEntry struct {
	Alias           string
	ModelID         ModelID
	PreviousModelID ModelID
	CommandID       CorrelationID
	ChangedAt       time.Time
}

// Clone returns normalized history.
func (entry AliasHistoryEntry) Clone() AliasHistoryEntry {
	entry.ChangedAt = entry.ChangedAt.UTC()
	return entry
}

// AliasAssignmentCommand requests an explicitly confirmed alias transaction.
type AliasAssignmentCommand struct {
	CorrelationID CorrelationID
	CommandID     CorrelationID
	Generation    uint64
	Alias         string
	ModelID       ModelID
	Confirmed     bool
}

// Validate checks command identity and explicit confirmation.
func (command AliasAssignmentCommand) Validate() error {
	if command.CorrelationID == "" || command.CommandID == "" || command.Generation == 0 ||
		strings.TrimSpace(command.Alias) == "" || command.ModelID == "" || !command.Confirmed {
		return fmt.Errorf("%w: confirmed alias command identity is required", ErrInvalidModels)
	}
	return nil
}

// ModelsWorkspaceSnapshot is one complete immutable registry publication.
type ModelsWorkspaceSnapshot struct {
	Query        ModelsWorkspaceQuery
	Registry     []ModelArtifact
	AliasHistory []AliasHistoryEntry
	PreparedAt   time.Time
	Degraded     bool
}

// Clone returns independent registry data.
func (snapshot ModelsWorkspaceSnapshot) Clone() ModelsWorkspaceSnapshot {
	snapshot.Query = snapshot.Query.Clone()
	if len(snapshot.Registry) != 0 {
		registry := make([]ModelArtifact, len(snapshot.Registry))
		for index, artifact := range snapshot.Registry {
			registry[index] = artifact.Clone()
		}
		snapshot.Registry = registry
	}
	if len(snapshot.AliasHistory) != 0 {
		history := make([]AliasHistoryEntry, len(snapshot.AliasHistory))
		for index, entry := range snapshot.AliasHistory {
			history[index] = entry.Clone()
		}
		snapshot.AliasHistory = history
	}
	snapshot.PreparedAt = snapshot.PreparedAt.UTC()
	return snapshot
}

// Validate checks stable ordering and every published artifact.
func (snapshot ModelsWorkspaceSnapshot) Validate() error {
	if err := snapshot.Query.Validate(); err != nil {
		return err
	}
	for index, artifact := range snapshot.Registry {
		if err := artifact.Validate(); err != nil {
			return err
		}
		if index > 0 && snapshot.Registry[index-1].Summary.ID >= artifact.Summary.ID {
			return fmt.Errorf("%w: registry is not strictly ordered by model ID", ErrInvalidModels)
		}
	}
	return nil
}

// ModelsWorkspaceMessage publishes immutable registry data.
type ModelsWorkspaceMessage struct {
	Event    EventEnvelope
	Snapshot ModelsWorkspaceSnapshot
}

func (ModelsWorkspaceMessage) isClientMessage() {}

// AliasAssignedMessage publishes the result of an idempotent alias command.
type AliasAssignedMessage struct {
	Event      EventEnvelope
	Generation uint64
	CommandID  CorrelationID
	Snapshot   ModelsWorkspaceSnapshot
}

func (AliasAssignedMessage) isClientMessage() {}

// ModelsWorkspaceState owns registry selection and pending alias confirmation.
type ModelsWorkspaceState struct {
	Generation      uint64
	Scenario        ModelsScenario
	State           WorkspaceLoadState
	Stale           bool
	Query           ModelsWorkspaceQuery
	Snapshot        ModelsWorkspaceSnapshot
	SelectedModelID ModelID
	SelectedTree    int
	PendingAlias    AliasAssignmentCommand
	AwaitingConfirm bool
	Error           ErrorSnapshot
}

// DefaultModelsWorkspaceState returns the canonical Models workspace state.
func DefaultModelsWorkspaceState() ModelsWorkspaceState {
	return ModelsWorkspaceState{Scenario: ModelsScenarioNormal, State: WorkspaceIdle}
}

// Clone returns independent state.
func (state ModelsWorkspaceState) Clone() ModelsWorkspaceState {
	state.Query = state.Query.Clone()
	state.Snapshot = state.Snapshot.Clone()
	return state
}

// InferenceWorkspaceQuery requests compatibility and complete scoring output.
type InferenceWorkspaceQuery struct {
	CorrelationID      CorrelationID
	Generation         uint64
	Scenario           InferenceScenario
	ModelID            ModelID
	Alias              string
	DatasetID          DatasetID
	DatasetRevision    uint64
	DatasetFingerprint string
	Symbols            []Symbol
	TimeRange          TimeRange
	Mode               InferenceMode
}

// Clone returns normalized independent query data.
func (query InferenceWorkspaceQuery) Clone() InferenceWorkspaceQuery {
	query.Alias = strings.ToLower(strings.TrimSpace(query.Alias))
	query.Symbols = cloneSymbols(query.Symbols)
	query.TimeRange = query.TimeRange.Normalize()
	return query
}

// Validate checks that scoring has explicit compatible boundary identities.
func (query InferenceWorkspaceQuery) Validate() error {
	if query.CorrelationID == "" || query.Generation == 0 || !query.Scenario.Valid() || !query.Mode.Valid() ||
		(query.ModelID == "" && strings.TrimSpace(query.Alias) == "") || query.DatasetID == "" ||
		query.DatasetRevision == 0 || query.DatasetFingerprint == "" || len(query.Symbols) == 0 {
		return fmt.Errorf("%w: identity, model or alias, dataset revision/fingerprint, symbols, mode, and scenario are required", ErrInvalidInference)
	}
	if query.Mode == InferenceHistorical && (query.TimeRange.Start.IsZero() || query.TimeRange.End.IsZero() || query.TimeRange.End.Before(query.TimeRange.Start)) {
		return fmt.Errorf("%w: historical scoring requires a non-empty ordered range", ErrInvalidInference)
	}
	return nil
}

// CompatibilityDiagnostic records one field-addressed compatibility failure.
type CompatibilityDiagnostic struct {
	Field    string
	Expected string
	Actual   string
	Message  string
}

// CompatibilitySummary records the checks required before scoring.
type CompatibilitySummary struct {
	Compatible  bool
	Diagnostics []CompatibilityDiagnostic
}

// Clone returns independent diagnostics.
func (summary CompatibilitySummary) Clone() CompatibilitySummary {
	summary.Diagnostics = append([]CompatibilityDiagnostic(nil), summary.Diagnostics...)
	return summary
}

// Prediction is one immutable score or explicitly unavailable prediction.
type Prediction struct {
	ID                  string
	Symbol              Symbol
	Timestamp           time.Time
	ModelID             ModelID
	RunID               RunID
	DatasetFingerprint  string
	FeatureFingerprints []string
	Score               float64
	Missing             bool
	Ineligible          bool
	Reason              string
}

// Clone returns an independent prediction.
func (prediction Prediction) Clone() Prediction {
	prediction.Timestamp = prediction.Timestamp.UTC()
	prediction.FeatureFingerprints = append([]string(nil), prediction.FeatureFingerprints...)
	return prediction
}

// RankingRow is one rank for a timestamp, excluding missing or ineligible rows.
type RankingRow struct {
	PredictionID string
	Symbol       Symbol
	Rank         int
	Score        float64
}

// TimestampRanking preserves ordering for each scored timestamp.
type TimestampRanking struct {
	Timestamp time.Time
	Rows      []RankingRow
}

// Clone returns independent ranking data.
func (ranking TimestampRanking) Clone() TimestampRanking {
	ranking.Timestamp = ranking.Timestamp.UTC()
	ranking.Rows = append([]RankingRow(nil), ranking.Rows...)
	return ranking
}

// ExportState describes off-thread simulated export preparation.
type ExportState string

const (
	ExportIdle      ExportState = "idle"
	ExportPreparing ExportState = "preparing"
	ExportReady     ExportState = "ready"
	ExportFailed    ExportState = "failed"
)

// ExportSnapshot represents only a fully checksummed exported output.
type ExportSnapshot struct {
	State       ExportState
	InferenceID InferenceID
	Checksum    string
	Bytes       uint64
	CompletedAt time.Time
	Error       ErrorSnapshot
}

// Clone returns normalized export data.
func (snapshot ExportSnapshot) Clone() ExportSnapshot {
	snapshot.CompletedAt = snapshot.CompletedAt.UTC()
	return snapshot
}

// InferenceOutput is a complete immutable scoring publication.
type InferenceOutput struct {
	ID           InferenceID
	ModelID      ModelID
	RunID        RunID
	DatasetID    DatasetID
	Fingerprint  string
	Mode         InferenceMode
	TimeRange    TimeRange
	Predictions  []Prediction
	Rankings     []TimestampRanking
	Distribution []HistogramBin
	CompletedAt  time.Time
	Checksum     string
	Export       ExportSnapshot
}

// Clone returns independent output data.
func (output InferenceOutput) Clone() InferenceOutput {
	output.TimeRange = output.TimeRange.Normalize()
	output.CompletedAt = output.CompletedAt.UTC()
	if len(output.Predictions) != 0 {
		predictions := make([]Prediction, len(output.Predictions))
		for index, prediction := range output.Predictions {
			predictions[index] = prediction.Clone()
		}
		output.Predictions = predictions
	}
	if len(output.Rankings) != 0 {
		rankings := make([]TimestampRanking, len(output.Rankings))
		for index, ranking := range output.Rankings {
			rankings[index] = ranking.Clone()
		}
		output.Rankings = rankings
	}
	output.Distribution = append([]HistogramBin(nil), output.Distribution...)
	output.Export = output.Export.Clone()
	return output
}

// Validate checks complete output invariants before publishing history/export.
func (output InferenceOutput) Validate() error {
	if output.ID == "" || output.ModelID == "" || output.RunID == "" || output.DatasetID == "" ||
		output.Fingerprint == "" || !output.Mode.Valid() || output.CompletedAt.IsZero() || output.Checksum == "" ||
		len(output.Predictions) == 0 {
		return fmt.Errorf("%w: incomplete immutable inference output", ErrInvalidInference)
	}
	for _, prediction := range output.Predictions {
		if prediction.ID == "" || prediction.Symbol == "" || prediction.Timestamp.IsZero() || prediction.ModelID != output.ModelID ||
			prediction.RunID != output.RunID || prediction.DatasetFingerprint != output.Fingerprint || len(prediction.FeatureFingerprints) == 0 {
			return fmt.Errorf("%w: incomplete prediction identity", ErrInvalidInference)
		}
		if !prediction.Missing && !prediction.Ineligible && (math.IsNaN(prediction.Score) || math.IsInf(prediction.Score, 0)) {
			return fmt.Errorf("%w: non-finite prediction score", ErrInvalidInference)
		}
	}
	return nil
}

// InferenceWorkspaceSnapshot is one complete compatibility/scoring publication.
type InferenceWorkspaceSnapshot struct {
	Query         InferenceWorkspaceQuery
	Compatibility CompatibilitySummary
	Output        InferenceOutput
	HasOutput     bool
	History       []InferenceOutput
	PreparedAt    time.Time
	Degraded      bool
}

// Clone returns independent inference data.
func (snapshot InferenceWorkspaceSnapshot) Clone() InferenceWorkspaceSnapshot {
	snapshot.Query = snapshot.Query.Clone()
	snapshot.Compatibility = snapshot.Compatibility.Clone()
	snapshot.Output = snapshot.Output.Clone()
	if len(snapshot.History) != 0 {
		history := make([]InferenceOutput, len(snapshot.History))
		for index, output := range snapshot.History {
			history[index] = output.Clone()
		}
		snapshot.History = history
	}
	snapshot.PreparedAt = snapshot.PreparedAt.UTC()
	return snapshot
}

// Validate checks that incompatible requests never carry scoring output.
func (snapshot InferenceWorkspaceSnapshot) Validate() error {
	if err := snapshot.Query.Validate(); err != nil {
		return err
	}
	if !snapshot.Compatibility.Compatible && snapshot.HasOutput {
		return fmt.Errorf("%w: incompatible scoring request has output", ErrInvalidInference)
	}
	if snapshot.HasOutput {
		if err := snapshot.Output.Validate(); err != nil {
			return err
		}
	}
	for _, output := range snapshot.History {
		if err := output.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// InferenceWorkspaceMessage publishes compatibility and immutable scoring data.
type InferenceWorkspaceMessage struct {
	Event    EventEnvelope
	Snapshot InferenceWorkspaceSnapshot
}

func (InferenceWorkspaceMessage) isClientMessage() {}

// ExportInferenceCommand requests idempotent preparation of a completed output.
type ExportInferenceCommand struct {
	CorrelationID CorrelationID
	CommandID     CorrelationID
	Generation    uint64
	InferenceID   InferenceID
}

// Validate checks command identity.
func (command ExportInferenceCommand) Validate() error {
	if command.CorrelationID == "" || command.CommandID == "" || command.Generation == 0 || command.InferenceID == "" {
		return fmt.Errorf("%w: export command identity is required", ErrInvalidInference)
	}
	return nil
}

// InferenceExportMessage publishes an idempotent export result.
type InferenceExportMessage struct {
	Event      EventEnvelope
	Generation uint64
	CommandID  CorrelationID
	Export     ExportSnapshot
}

func (InferenceExportMessage) isClientMessage() {}

// InferenceWorkspaceState owns model/dataset scoring selection and history.
type InferenceWorkspaceState struct {
	Generation      uint64
	Scenario        InferenceScenario
	State           WorkspaceLoadState
	Stale           bool
	Query           InferenceWorkspaceQuery
	Snapshot        InferenceWorkspaceSnapshot
	SelectedModelID ModelID
	SelectedAlias   string
	SelectedSymbol  Symbol
	PendingExport   ExportInferenceCommand
	Error           ErrorSnapshot
}

// DefaultInferenceWorkspaceState returns canonical Inference state.
func DefaultInferenceWorkspaceState() InferenceWorkspaceState {
	return InferenceWorkspaceState{Scenario: InferenceScenarioNormal, State: WorkspaceIdle, SelectedAlias: "champion"}
}

// Clone returns independent state.
func (state InferenceWorkspaceState) Clone() InferenceWorkspaceState {
	state.Query = state.Query.Clone()
	state.Snapshot = state.Snapshot.Clone()
	return state
}

func sortedRankingRows(predictions []Prediction) []RankingRow {
	rows := make([]RankingRow, 0, len(predictions))
	for _, prediction := range predictions {
		if prediction.Missing || prediction.Ineligible {
			continue
		}
		rows = append(rows, RankingRow{PredictionID: prediction.ID, Symbol: prediction.Symbol, Score: prediction.Score})
	}
	sort.SliceStable(rows, func(left int, right int) bool {
		if rows[left].Score != rows[right].Score {
			return rows[left].Score > rows[right].Score
		}
		return rows[left].Symbol < rows[right].Symbol
	})
	for index := range rows {
		rows[index].Rank = index + 1
	}
	return rows
}

// RankPredictions ranks eligible scores descending with stable symbol-ID tie-breaking.
func RankPredictions(predictions []Prediction) []RankingRow {
	return sortedRankingRows(predictions)
}
