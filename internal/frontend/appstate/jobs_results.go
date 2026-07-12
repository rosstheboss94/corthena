package appstate

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidJobs identifies an invalid Jobs workspace request or payload.
var ErrInvalidJobs = errors.New("invalid Jobs request")

// ErrInvalidResults identifies an invalid Results workspace request or payload.
var ErrInvalidResults = errors.New("invalid Results request")

// JobsScenario selects one deterministic job lifecycle demonstration.
type JobsScenario string

const (
	JobsScenarioSuccess      JobsScenario = "success"
	JobsScenarioPauseResume  JobsScenario = "pause_resume"
	JobsScenarioCancellation JobsScenario = "cancellation"
	JobsScenarioInterruption JobsScenario = "interruption"
	JobsScenarioFailure      JobsScenario = "failure"
)

// Valid reports whether the scenario is supported.
func (scenario JobsScenario) Valid() bool {
	switch scenario {
	case JobsScenarioSuccess, JobsScenarioPauseResume, JobsScenarioCancellation,
		JobsScenarioInterruption, JobsScenarioFailure:
		return true
	default:
		return false
	}
}

// JobsScenarios returns the stable scenario-control order.
func JobsScenarios() []JobsScenario {
	return []JobsScenario{
		JobsScenarioSuccess,
		JobsScenarioPauseResume,
		JobsScenarioCancellation,
		JobsScenarioInterruption,
		JobsScenarioFailure,
	}
}

// JobsWorkspaceQuery requests one generation of job telemetry.
type JobsWorkspaceQuery struct {
	CorrelationID CorrelationID
	Generation    uint64
	Scenario      JobsScenario
}

// Validate checks the request identity and closed scenario value.
func (query JobsWorkspaceQuery) Validate() error {
	if query.CorrelationID == "" || query.Generation == 0 || !query.Scenario.Valid() {
		return fmt.Errorf("%w: correlation, generation, and scenario are required", ErrInvalidJobs)
	}
	return nil
}

// JobStageState describes one ordered unit of job progress.
type JobStageState string

const (
	JobStagePending   JobStageState = "pending"
	JobStageRunning   JobStageState = "running"
	JobStageCompleted JobStageState = "completed"
	JobStagePaused    JobStageState = "paused"
	JobStageFailed    JobStageState = "failed"
	JobStageCancelled JobStageState = "cancelled"
)

// JobStageProgress is one deterministic stage in a selected job.
type JobStageProgress struct {
	ID             string
	Name           string
	State          JobStageState
	ProgressPermil int
	CompletedUnits uint64
	TotalUnits     uint64
	UpdatedAt      time.Time
}

// JobMetricPoint is one immutable live-metric sample.
type JobMetricPoint struct {
	Step      uint64
	Value     float64
	Missing   bool
	Timestamp time.Time
}

// JobMetricSeries is one named metric in stable sample order.
type JobMetricSeries struct {
	Name   string
	Unit   string
	Points []JobMetricPoint
}

// Clone returns an independent metric series.
func (series JobMetricSeries) Clone() JobMetricSeries {
	if len(series.Points) == 0 {
		series.Points = nil
		return series
	}
	series.Points = append([]JobMetricPoint(nil), series.Points...)
	for index := range series.Points {
		series.Points[index].Timestamp = series.Points[index].Timestamp.UTC()
	}
	return series
}

// WorkerResourceSnapshot describes one worker's bounded compute lease.
type WorkerResourceSnapshot struct {
	JobID       JobID
	WorkerID    string
	PID         int
	State       ComponentState
	LeasedSlots int
	GOMAXPROCS  int
	Goroutines  int
	ActiveTasks int
	MemoryBytes uint64
	HeartbeatAt time.Time
	Degradation string
}

// ProcessSnapshot describes one process or runtime component.
type ProcessSnapshot struct {
	ID        string
	Component Component
	Role      string
	PID       int
	State     ComponentState
	Detail    string
	StartedAt time.Time
	UpdatedAt time.Time
}

// CheckpointState identifies durable checkpoint publication state.
type CheckpointState string

const (
	CheckpointWriting   CheckpointState = "writing"
	CheckpointCommitted CheckpointState = "committed"
	CheckpointRejected  CheckpointState = "rejected"
)

// CheckpointSummary is one immutable checkpoint record visible to the UI.
type CheckpointSummary struct {
	ID          string
	JobID       JobID
	State       CheckpointState
	Sequence    uint64
	NodeCount   uint64
	Bytes       uint64
	Checksum    string
	CommittedAt time.Time
	Compatible  bool
}

// JobLogLevel identifies structured job log severity.
type JobLogLevel string

const (
	JobLogDebug JobLogLevel = "debug"
	JobLogInfo  JobLogLevel = "info"
	JobLogWarn  JobLogLevel = "warn"
	JobLogError JobLogLevel = "error"
)

// JobLogEntry is one ordered structured worker or coordinator log record.
type JobLogEntry struct {
	Sequence  uint64
	Timestamp time.Time
	Level     JobLogLevel
	Component string
	Message   string
}

// JobDetail contains render-ready telemetry for one job.
type JobDetail struct {
	Summary     JobSummary
	Stages      []JobStageProgress
	Metrics     []JobMetricSeries
	Worker      WorkerResourceSnapshot
	Checkpoints []CheckpointSummary
	Logs        []JobLogEntry
}

// Clone returns an independent job detail.
func (detail JobDetail) Clone() JobDetail {
	detail.Summary = detail.Summary.Clone()
	detail.Stages = append([]JobStageProgress(nil), detail.Stages...)
	for index := range detail.Stages {
		detail.Stages[index].UpdatedAt = detail.Stages[index].UpdatedAt.UTC()
	}
	if len(detail.Metrics) > 0 {
		metrics := make([]JobMetricSeries, len(detail.Metrics))
		for index, series := range detail.Metrics {
			metrics[index] = series.Clone()
		}
		detail.Metrics = metrics
	}
	detail.Worker.HeartbeatAt = detail.Worker.HeartbeatAt.UTC()
	detail.Checkpoints = append([]CheckpointSummary(nil), detail.Checkpoints...)
	for index := range detail.Checkpoints {
		detail.Checkpoints[index].CommittedAt = detail.Checkpoints[index].CommittedAt.UTC()
	}
	detail.Logs = append([]JobLogEntry(nil), detail.Logs...)
	for index := range detail.Logs {
		detail.Logs[index].Timestamp = detail.Logs[index].Timestamp.UTC()
	}
	return detail
}

// JobsWorkspaceSnapshot is one immutable job queue and telemetry generation.
type JobsWorkspaceSnapshot struct {
	Query          JobsWorkspaceQuery
	Jobs           []JobDetail
	Processes      []ProcessSnapshot
	TotalCPUSlots  int
	LeasedCPUSlots int
	PreparedAt     time.Time
}

// Clone returns an independent workspace snapshot.
func (snapshot JobsWorkspaceSnapshot) Clone() JobsWorkspaceSnapshot {
	if len(snapshot.Jobs) > 0 {
		jobs := make([]JobDetail, len(snapshot.Jobs))
		for index, job := range snapshot.Jobs {
			jobs[index] = job.Clone()
		}
		snapshot.Jobs = jobs
	}
	snapshot.Processes = append([]ProcessSnapshot(nil), snapshot.Processes...)
	for index := range snapshot.Processes {
		snapshot.Processes[index].StartedAt = snapshot.Processes[index].StartedAt.UTC()
		snapshot.Processes[index].UpdatedAt = snapshot.Processes[index].UpdatedAt.UTC()
	}
	snapshot.PreparedAt = snapshot.PreparedAt.UTC()
	return snapshot
}

// JobsWorkspaceMessage publishes a generation-ordered job snapshot.
type JobsWorkspaceMessage struct {
	Event    EventEnvelope
	Snapshot JobsWorkspaceSnapshot
}

func (JobsWorkspaceMessage) isClientMessage() {}

// JobsWorkspaceState owns queue selection, telemetry, and control state.
type JobsWorkspaceState struct {
	Generation     uint64
	Scenario       JobsScenario
	State          WorkspaceLoadState
	Stale          bool
	Query          JobsWorkspaceQuery
	Snapshot       JobsWorkspaceSnapshot
	SelectedJobID  JobID
	PendingControl JobControl
	Error          ErrorSnapshot
}

// DefaultJobsWorkspaceState returns canonical job state.
func DefaultJobsWorkspaceState() JobsWorkspaceState {
	return JobsWorkspaceState{Scenario: JobsScenarioSuccess, State: WorkspaceIdle}
}

// Clone returns independent state.
func (state JobsWorkspaceState) Clone() JobsWorkspaceState {
	state.Snapshot = state.Snapshot.Clone()
	return state
}

// ResultsScenario selects one deterministic Results workflow condition.
type ResultsScenario string

const (
	ResultsScenarioNormal    ResultsScenario = "normal"
	ResultsScenarioLoading   ResultsScenario = "loading"
	ResultsScenarioEmpty     ResultsScenario = "empty"
	ResultsScenarioFailure   ResultsScenario = "failure"
	ResultsScenarioDegraded  ResultsScenario = "degraded"
	ResultsScenarioRecovered ResultsScenario = "recovered"
	ResultsScenarioCancelled ResultsScenario = "cancelled"
	ResultsScenarioSaturated ResultsScenario = "saturated"
)

// Valid reports whether the scenario is supported.
func (scenario ResultsScenario) Valid() bool {
	switch scenario {
	case ResultsScenarioNormal, ResultsScenarioLoading, ResultsScenarioEmpty,
		ResultsScenarioFailure, ResultsScenarioDegraded, ResultsScenarioRecovered,
		ResultsScenarioCancelled, ResultsScenarioSaturated:
		return true
	default:
		return false
	}
}

// ResultsScenarios returns the stable scenario-control order.
func ResultsScenarios() []ResultsScenario {
	return []ResultsScenario{
		ResultsScenarioNormal,
		ResultsScenarioLoading,
		ResultsScenarioEmpty,
		ResultsScenarioFailure,
		ResultsScenarioDegraded,
		ResultsScenarioRecovered,
		ResultsScenarioCancelled,
		ResultsScenarioSaturated,
	}
}

// ResultsWorkspaceQuery requests filtered immutable run details.
type ResultsWorkspaceQuery struct {
	CorrelationID CorrelationID
	Generation    uint64
	Scenario      ResultsScenario
	Filter        string
	RunIDs        []RunID
}

// Clone returns an independent request.
func (query ResultsWorkspaceQuery) Clone() ResultsWorkspaceQuery {
	query.RunIDs = append([]RunID(nil), query.RunIDs...)
	return query
}

// Validate checks request identity and comparison bounds.
func (query ResultsWorkspaceQuery) Validate() error {
	if query.CorrelationID == "" || query.Generation == 0 || !query.Scenario.Valid() {
		return fmt.Errorf("%w: correlation, generation, and scenario are required", ErrInvalidResults)
	}
	if len(query.RunIDs) > 4 {
		return fmt.Errorf("%w: at most four runs may be compared", ErrInvalidResults)
	}
	seen := make(map[RunID]struct{}, len(query.RunIDs))
	for _, runID := range query.RunIDs {
		if runID == "" {
			return fmt.Errorf("%w: run ID is empty", ErrInvalidResults)
		}
		if _, duplicate := seen[runID]; duplicate {
			return fmt.Errorf("%w: duplicate run ID %q", ErrInvalidResults, runID)
		}
		seen[runID] = struct{}{}
	}
	return nil
}

// MetricPartition distinguishes selection metrics from untouched test metrics.
type MetricPartition string

const (
	MetricValidation MetricPartition = "validation"
	MetricTest       MetricPartition = "test"
)

// FoldResult describes one chronologically ordered walk-forward fold.
type FoldResult struct {
	Index           int
	Train           TimeRange
	Validation      TimeRange
	Test            TimeRange
	ValidationStats []MetricSummary
	TestStats       []MetricSummary
}

// Clone returns an independent fold.
func (fold FoldResult) Clone() FoldResult {
	fold.Train = fold.Train.Normalize()
	fold.Validation = fold.Validation.Normalize()
	fold.Test = fold.Test.Normalize()
	fold.ValidationStats = cloneMetrics(fold.ValidationStats)
	fold.TestStats = cloneMetrics(fold.TestStats)
	return fold
}

// ResultSeriesPoint is one timestamped chart value.
type ResultSeriesPoint struct {
	Timestamp time.Time
	Value     float64
	Missing   bool
}

// HistogramBin is one deterministic half-open distribution bucket, except the last.
type HistogramBin struct {
	Minimum float64
	Maximum float64
	Count   uint64
}

// PredictionMarketPoint overlays a prediction with the realized forward market value.
type PredictionMarketPoint struct {
	Timestamp  time.Time
	Symbol     Symbol
	Prediction float64
	Market     float64
	Eligible   bool
}

// ConfigurationValue is one stable typed display value used for run diffs.
type ConfigurationValue struct {
	Section string
	Path    string
	Value   string
}

// RunResultDetail contains immutable render-ready evaluation output.
type RunResultDetail struct {
	Summary                RunResultSummary
	Folds                  []FoldResult
	Equity                 []ResultSeriesPoint
	Drawdown               []ResultSeriesPoint
	InformationCoefficient []HistogramBin
	Predictions            []HistogramBin
	Overlay                []PredictionMarketPoint
	Configuration          []ConfigurationValue
	Disclosures            []string
}

// Clone returns an independent result detail.
func (detail RunResultDetail) Clone() RunResultDetail {
	detail.Summary = detail.Summary.Clone()
	if len(detail.Folds) > 0 {
		folds := make([]FoldResult, len(detail.Folds))
		for index, fold := range detail.Folds {
			folds[index] = fold.Clone()
		}
		detail.Folds = folds
	}
	detail.Equity = cloneResultSeries(detail.Equity)
	detail.Drawdown = cloneResultSeries(detail.Drawdown)
	detail.InformationCoefficient = append([]HistogramBin(nil), detail.InformationCoefficient...)
	detail.Predictions = append([]HistogramBin(nil), detail.Predictions...)
	detail.Overlay = append([]PredictionMarketPoint(nil), detail.Overlay...)
	for index := range detail.Overlay {
		detail.Overlay[index].Timestamp = detail.Overlay[index].Timestamp.UTC()
	}
	detail.Configuration = append([]ConfigurationValue(nil), detail.Configuration...)
	detail.Disclosures = append([]string(nil), detail.Disclosures...)
	return detail
}

// ResultsWorkspaceSnapshot is one immutable filtered comparison generation.
type ResultsWorkspaceSnapshot struct {
	Query      ResultsWorkspaceQuery
	Runs       []RunResultDetail
	PreparedAt time.Time
	Degraded   bool
}

// Clone returns an independent snapshot.
func (snapshot ResultsWorkspaceSnapshot) Clone() ResultsWorkspaceSnapshot {
	snapshot.Query = snapshot.Query.Clone()
	if len(snapshot.Runs) > 0 {
		runs := make([]RunResultDetail, len(snapshot.Runs))
		for index, run := range snapshot.Runs {
			runs[index] = run.Clone()
		}
		snapshot.Runs = runs
	}
	snapshot.PreparedAt = snapshot.PreparedAt.UTC()
	return snapshot
}

// ResultsWorkspaceMessage publishes immutable run details.
type ResultsWorkspaceMessage struct {
	Event    EventEnvelope
	Snapshot ResultsWorkspaceSnapshot
}

func (ResultsWorkspaceMessage) isClientMessage() {}

// ResultsWorkspaceState owns run filtering, stable comparison, and request state.
type ResultsWorkspaceState struct {
	Generation     uint64
	Scenario       ResultsScenario
	State          WorkspaceLoadState
	Stale          bool
	Query          ResultsWorkspaceQuery
	Snapshot       ResultsWorkspaceSnapshot
	SelectedRunIDs []RunID
	PrimaryRunID   RunID
	Filter         string
	Error          ErrorSnapshot
}

// DefaultResultsWorkspaceState returns canonical Results state.
func DefaultResultsWorkspaceState() ResultsWorkspaceState {
	return ResultsWorkspaceState{Scenario: ResultsScenarioNormal, State: WorkspaceIdle}
}

// Clone returns independent state.
func (state ResultsWorkspaceState) Clone() ResultsWorkspaceState {
	state.Query = state.Query.Clone()
	state.Snapshot = state.Snapshot.Clone()
	state.SelectedRunIDs = append([]RunID(nil), state.SelectedRunIDs...)
	return state
}

func cloneResultSeries(input []ResultSeriesPoint) []ResultSeriesPoint {
	output := append([]ResultSeriesPoint(nil), input...)
	for index := range output {
		output[index].Timestamp = output[index].Timestamp.UTC()
	}
	return output
}
