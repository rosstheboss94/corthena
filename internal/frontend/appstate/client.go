package appstate

import (
	"context"
	"time"
)

// FrontendClient is the narrow asynchronous boundary used by the UI effects
// runtime. Implementations may be the demo coordinator or a real Go client.
type FrontendClient interface {
	Snapshot(context.Context, SnapshotRequest) (SnapshotMessage, error)
	SubmitExperiment(context.Context, SubmitExperimentCommand) (JobUpdateMessage, error)
	ControlJob(context.Context, JobControlCommand) (JobUpdateMessage, error)
	RunInference(context.Context, InferenceCommand) (InferenceUpdateMessage, error)
	Research(context.Context, ResearchQuery) (ResearchResponseMessage, error)
	Subscribe(context.Context, EventSubscription) (<-chan ClientMessage, error)
	Close() error
}

// SnapshotRequest requests a reconciled frontend snapshot.
type SnapshotRequest struct {
	CorrelationID CorrelationID
}

// EventSubscription requests client events after a known event ID. The returned
// message channel is closed by the client implementation.
type EventSubscription struct {
	Since EventID
}

// ExperimentDraft is a typed frontend experiment submission draft.
type ExperimentDraft struct {
	Name         string
	DatasetID    DatasetID
	Features     []FeatureName
	Target       TargetSpec
	Model        ModelSpec
	Split        SplitSpec
	Portfolio    PortfolioSpec
	RequestedCPU int
}

// Clone returns an independent copy.
func (draft ExperimentDraft) Clone() ExperimentDraft {
	draft.Features = cloneFeatureNames(draft.Features)
	return draft
}

// SubmitExperimentCommand requests immutable experiment submission.
type SubmitExperimentCommand struct {
	CorrelationID CorrelationID
	CommandID     CorrelationID
	Draft         ExperimentDraft
}

// JobControl is a typed job control command.
type JobControl string

const (
	JobControlPause  JobControl = "pause"
	JobControlResume JobControl = "resume"
	JobControlCancel JobControl = "cancel"
)

// JobControlCommand requests a legal job state transition.
type JobControlCommand struct {
	CorrelationID CorrelationID
	CommandID     CorrelationID
	JobID         JobID
	Control       JobControl
}

// InferenceCommand requests deterministic batch scoring.
type InferenceCommand struct {
	CorrelationID CorrelationID
	CommandID     CorrelationID
	ModelID       ModelID
	DatasetID     DatasetID
	Symbols       []Symbol
	TimeRange     TimeRange
	LatestOnly    bool
}

// Clone returns an independent copy.
func (command InferenceCommand) Clone() InferenceCommand {
	command.Symbols = cloneSymbols(command.Symbols)
	command.TimeRange = command.TimeRange.Normalize()
	return command
}

// ClientMessage is a closed set of immutable messages from the client.
type ClientMessage interface {
	isClientMessage()
}

// EventEnvelope carries reconciliation metadata for client messages.
type EventEnvelope struct {
	ID            EventID
	Type          string
	SchemaVersion int
	Timestamp     time.Time
	CorrelationID CorrelationID
}

// DatasetStatus is a catalog dataset state.
type DatasetStatus string

const (
	DatasetReady      DatasetStatus = "ready"
	DatasetImporting  DatasetStatus = "importing"
	DatasetValidation DatasetStatus = "validation"
	DatasetFailed     DatasetStatus = "failed"
)

// DatasetSummary is a typed immutable catalog row.
type DatasetSummary struct {
	ID          DatasetID
	Name        string
	Status      DatasetStatus
	Symbols     []Symbol
	Interval    BarInterval
	Rows        uint64
	Start       time.Time
	End         time.Time
	Revision    uint64
	Fingerprint string
}

// Clone returns an independent copy.
func (dataset DatasetSummary) Clone() DatasetSummary {
	dataset.Symbols = cloneSymbols(dataset.Symbols)
	dataset.Start = dataset.Start.UTC()
	dataset.End = dataset.End.UTC()
	return dataset
}

// TargetSpec describes a frontend target configuration.
type TargetSpec struct {
	Kind        string
	HorizonBars int
	LogReturn   bool
}

// SplitSpec describes a frontend walk-forward split configuration.
type SplitSpec struct {
	Kind             string
	TrainBars        int
	ValidationBars   int
	TestBars         int
	PurgeBars        int
	EmbargoBars      int
	ExpandingWindows bool
}

// PortfolioSpec describes a reference backtest configuration.
type PortfolioSpec struct {
	LongQuantile  float64
	ShortQuantile float64
	CostBPS       float64
}

// ModelKind is a first-party estimator kind.
type ModelKind string

const (
	ModelHistogramTree ModelKind = "histogram_tree"
	ModelRandomForest  ModelKind = "random_forest"
	ModelGradientBoost ModelKind = "gradient_boosted_trees"
)

// ModelSpec describes a frontend model configuration.
type ModelSpec struct {
	Kind            ModelKind
	MaxDepth        int
	MinLeafSamples  int
	EstimatorCount  int
	HistogramBins   int
	LearningRateBPS int
	Seed            uint64
}

// JobState is a coordinator-owned job lifecycle state.
type JobState string

const (
	JobQueued         JobState = "queued"
	JobRunning        JobState = "running"
	JobPauseRequested JobState = "pause_requested"
	JobPaused         JobState = "paused"
	JobCompleted      JobState = "completed"
	JobFailed         JobState = "failed"
	JobCancelled      JobState = "cancelled"
	JobInterrupted    JobState = "interrupted"
)

// AllowsControl reports whether a job control can be enabled from typed state.
func (state JobState) AllowsControl(control JobControl) bool {
	switch control {
	case JobControlPause:
		return state == JobQueued || state == JobRunning
	case JobControlResume:
		return state == JobPaused || state == JobInterrupted
	case JobControlCancel:
		return state == JobQueued ||
			state == JobRunning ||
			state == JobPauseRequested ||
			state == JobPaused ||
			state == JobInterrupted
	default:
		return false
	}
}

// JobSummary is a typed immutable job row.
type JobSummary struct {
	ID              JobID
	ExperimentID    ExperimentID
	RunID           RunID
	State           JobState
	ProgressPermil  int
	Stage           string
	CPUSlots        int
	CheckpointCount int
	UpdatedAt       time.Time
	Error           ErrorSnapshot
}

// MetricSummary is a stable numeric metric value.
type MetricSummary struct {
	Name    string
	Value   float64
	Missing bool
}

// RunResultSummary describes immutable evaluation output.
type RunResultSummary struct {
	ID              RunID
	ExperimentID    ExperimentID
	JobID           JobID
	DatasetID       DatasetID
	ModelID         ModelID
	Metrics         []MetricSummary
	ValidationStart time.Time
	ValidationEnd   time.Time
	TestStart       time.Time
	TestEnd         time.Time
	CompletedAt     time.Time
	Immutable       bool
}

// Clone returns an independent copy.
func (result RunResultSummary) Clone() RunResultSummary {
	result.Metrics = cloneMetrics(result.Metrics)
	result.ValidationStart = result.ValidationStart.UTC()
	result.ValidationEnd = result.ValidationEnd.UTC()
	result.TestStart = result.TestStart.UTC()
	result.TestEnd = result.TestEnd.UTC()
	result.CompletedAt = result.CompletedAt.UTC()
	return result
}

// ModelSummary describes an immutable model artifact.
type ModelSummary struct {
	ID                  ModelID
	RunID               RunID
	Kind                ModelKind
	Alias               string
	FeatureNames        []FeatureName
	TrainingCutoff      time.Time
	CreatedAt           time.Time
	ArtifactFingerprint string
	Immutable           bool
}

// Clone returns an independent copy.
func (model ModelSummary) Clone() ModelSummary {
	model.FeatureNames = cloneFeatureNames(model.FeatureNames)
	model.TrainingCutoff = model.TrainingCutoff.UTC()
	model.CreatedAt = model.CreatedAt.UTC()
	return model
}

// InferenceState is a batch scoring lifecycle state.
type InferenceState string

const (
	InferenceQueued    InferenceState = "queued"
	InferenceRunning   InferenceState = "running"
	InferenceCompleted InferenceState = "completed"
	InferenceFailed    InferenceState = "failed"
)

// ScoredSymbol is one ranked inference score.
type ScoredSymbol struct {
	Symbol Symbol
	Rank   int
	Score  float64
}

// InferenceSummary describes historical or latest-snapshot scoring output.
type InferenceSummary struct {
	ID          InferenceID
	ModelID     ModelID
	DatasetID   DatasetID
	State       InferenceState
	TimeRange   TimeRange
	Scores      []ScoredSymbol
	GeneratedAt time.Time
	Error       ErrorSnapshot
}

// Clone returns an independent copy.
func (inference InferenceSummary) Clone() InferenceSummary {
	inference.TimeRange = inference.TimeRange.Normalize()
	inference.Scores = cloneScores(inference.Scores)
	inference.GeneratedAt = inference.GeneratedAt.UTC()
	return inference
}

// SnapshotMessage replaces frontend state from a reconciled client snapshot.
type SnapshotMessage struct {
	Event      EventEnvelope
	Connection ConnectionSnapshot
	Components []ComponentSnapshot
	Cache      CacheSnapshot
	Datasets   []DatasetSummary
	Jobs       []JobSummary
	Results    []RunResultSummary
	Models     []ModelSummary
	Inferences []InferenceSummary
}

func (SnapshotMessage) isClientMessage() {}

// Clone returns an independent copy.
func (message SnapshotMessage) Clone() SnapshotMessage {
	message.Event.Timestamp = message.Event.Timestamp.UTC()
	message.Connection.UpdatedAt = message.Connection.UpdatedAt.UTC()
	message.Components = cloneComponents(message.Components)
	message.Datasets = cloneDatasets(message.Datasets)
	message.Jobs = cloneJobs(message.Jobs)
	message.Results = cloneResults(message.Results)
	message.Models = cloneModels(message.Models)
	message.Inferences = cloneInferences(message.Inferences)
	return message
}

// DatasetCatalogMessage replaces the dataset catalog.
type DatasetCatalogMessage struct {
	Event    EventEnvelope
	Datasets []DatasetSummary
}

func (DatasetCatalogMessage) isClientMessage() {}

// JobUpdateMessage upserts one job.
type JobUpdateMessage struct {
	Event EventEnvelope
	Job   JobSummary
}

func (JobUpdateMessage) isClientMessage() {}

// RunResultsMessage replaces known run results.
type RunResultsMessage struct {
	Event   EventEnvelope
	Results []RunResultSummary
}

func (RunResultsMessage) isClientMessage() {}

// ModelRegistryMessage replaces known model artifacts.
type ModelRegistryMessage struct {
	Event  EventEnvelope
	Models []ModelSummary
}

func (ModelRegistryMessage) isClientMessage() {}

// InferenceUpdateMessage upserts one inference output.
type InferenceUpdateMessage struct {
	Event     EventEnvelope
	Inference InferenceSummary
}

func (InferenceUpdateMessage) isClientMessage() {}

// ComponentStatusMessage upserts one component status.
type ComponentStatusMessage struct {
	Event     EventEnvelope
	Component ComponentSnapshot
}

func (ComponentStatusMessage) isClientMessage() {}

// ClientFailureMessage records a typed client-side failure.
type ClientFailureMessage struct {
	Event EventEnvelope
	Error ErrorSnapshot
}

func (ClientFailureMessage) isClientMessage() {}

func cloneComponents(input []ComponentSnapshot) []ComponentSnapshot {
	if len(input) == 0 {
		return nil
	}
	output := make([]ComponentSnapshot, len(input))
	for index, component := range input {
		component.UpdatedAt = component.UpdatedAt.UTC()
		output[index] = component
	}
	return output
}

func cloneDatasets(input []DatasetSummary) []DatasetSummary {
	if len(input) == 0 {
		return nil
	}
	output := make([]DatasetSummary, len(input))
	for index, dataset := range input {
		output[index] = dataset.Clone()
	}
	return output
}

func cloneJobs(input []JobSummary) []JobSummary {
	if len(input) == 0 {
		return nil
	}
	output := make([]JobSummary, len(input))
	for index, job := range input {
		job.UpdatedAt = job.UpdatedAt.UTC()
		output[index] = job
	}
	return output
}

func cloneMetrics(input []MetricSummary) []MetricSummary {
	if len(input) == 0 {
		return nil
	}
	output := make([]MetricSummary, len(input))
	copy(output, input)
	return output
}

func cloneResults(input []RunResultSummary) []RunResultSummary {
	if len(input) == 0 {
		return nil
	}
	output := make([]RunResultSummary, len(input))
	for index, result := range input {
		output[index] = result.Clone()
	}
	return output
}

func cloneModels(input []ModelSummary) []ModelSummary {
	if len(input) == 0 {
		return nil
	}
	output := make([]ModelSummary, len(input))
	for index, model := range input {
		output[index] = model.Clone()
	}
	return output
}

func cloneInferences(input []InferenceSummary) []InferenceSummary {
	if len(input) == 0 {
		return nil
	}
	output := make([]InferenceSummary, len(input))
	for index, inference := range input {
		output[index] = inference.Clone()
	}
	return output
}

func cloneScores(input []ScoredSymbol) []ScoredSymbol {
	if len(input) == 0 {
		return nil
	}
	output := make([]ScoredSymbol, len(input))
	copy(output, input)
	return output
}

func cloneSymbols(input []Symbol) []Symbol {
	if len(input) == 0 {
		return nil
	}
	output := make([]Symbol, len(input))
	copy(output, input)
	return output
}

func cloneFeatureNames(input []FeatureName) []FeatureName {
	if len(input) == 0 {
		return nil
	}
	output := make([]FeatureName, len(input))
	copy(output, input)
	return output
}
