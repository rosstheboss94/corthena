// Package simulator provides the seeded demo coordinator used before the real
// coordinator and Go client exist.
package simulator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/chart"
)

// DelayProfile configures deterministic simulated latency.
type DelayProfile struct {
	Snapshot    time.Duration
	Events      time.Duration
	Commands    time.Duration
	Research    time.Duration
	Data        time.Duration
	Experiments time.Duration
	Jobs        time.Duration
	Results     time.Duration
}

// FailureProfile configures deterministic simulated failures.
type FailureProfile struct {
	Snapshot          bool
	OpenEvents        bool
	InterruptEvents   bool
	EventsBeforeError int
	Commands          bool
	Research          bool
	Data              bool
	Experiments       bool
	Jobs              bool
	Results           bool
}

// Options configures a DemoCoordinator.
type Options struct {
	Seed     uint64
	Clock    appstate.Clock
	Delays   DelayProfile
	Failures FailureProfile
}

// DemoCoordinator implements appstate.FrontendClient with deterministic dummy
// data and events.
type DemoCoordinator struct {
	mu            sync.RWMutex
	snapshot      appstate.SnapshotMessage
	events        []appstate.ClientMessage
	delays        DelayProfile
	failures      FailureProfile
	closed        chan struct{}
	seed          uint64
	researchCache *chart.FrameCache
	once          sync.Once
	imports       []appstate.DataImportRecord
	logs          []appstate.DataLogEntry
	definitions   []appstate.ExperimentDefinition
	submissions   map[appstate.CorrelationID]appstate.ExperimentDefinition
	jobs          appstate.JobsWorkspaceSnapshot
	results       appstate.ResultsWorkspaceSnapshot
	jobCommands   map[appstate.CorrelationID]appstate.JobUpdateMessage
}

// NewDemoCoordinator creates a seeded demo client.
func NewDemoCoordinator(options Options) (*DemoCoordinator, error) {
	clock := options.Clock
	if clock == nil {
		clock = appstate.RealClock{}
	}
	seed := options.Seed
	if seed == 0 {
		seed = 1
	}
	now := clock.Now().UTC()
	generator := newGenerator(seed)
	snapshot := buildSnapshot(now, &generator)
	events := buildEvents(now, snapshot, &generator)
	researchCache, err := chart.NewFrameCache(32 << 20)
	if err != nil {
		return nil, fmt.Errorf("new demo coordinator: %w", err)
	}
	return &DemoCoordinator{
		snapshot:      snapshot,
		events:        events,
		delays:        options.Delays,
		failures:      options.Failures,
		closed:        make(chan struct{}),
		seed:          seed,
		researchCache: researchCache,
		definitions:   buildDemoExperimentDefinitions(now, snapshot.Datasets),
		submissions:   make(map[appstate.CorrelationID]appstate.ExperimentDefinition),
		jobCommands:   make(map[appstate.CorrelationID]appstate.JobUpdateMessage),
	}, nil
}

// Snapshot returns a reconciled immutable demo snapshot.
func (client *DemoCoordinator) Snapshot(
	ctx context.Context,
	request appstate.SnapshotRequest,
) (appstate.SnapshotMessage, error) {
	if err := client.wait(ctx, client.delays.Snapshot); err != nil {
		return appstate.SnapshotMessage{}, err
	}
	if client.failures.Snapshot {
		return appstate.SnapshotMessage{}, failure("demo snapshot failed", request.CorrelationID, true)
	}
	client.mu.RLock()
	snapshot := client.snapshot.Clone()
	client.mu.RUnlock()
	snapshot.Event.CorrelationID = request.CorrelationID
	return snapshot, nil
}

// ControlJob returns a deterministic job state transition.
func (client *DemoCoordinator) ControlJob(
	ctx context.Context,
	command appstate.JobControlCommand,
) (appstate.JobUpdateMessage, error) {
	return client.controlJob(ctx, command)
}

// RunInference returns a deterministic queued inference update.
func (client *DemoCoordinator) RunInference(
	ctx context.Context,
	command appstate.InferenceCommand,
) (appstate.InferenceUpdateMessage, error) {
	if err := client.wait(ctx, client.delays.Commands); err != nil {
		return appstate.InferenceUpdateMessage{}, err
	}
	if client.failures.Commands {
		return appstate.InferenceUpdateMessage{}, failure("demo command failed", command.CorrelationID, true)
	}
	now := client.baseTime().Add(7 * time.Minute)
	return appstate.InferenceUpdateMessage{
		Event: event("evt-command-inference", "inference.updated", now, command.CorrelationID),
		Inference: appstate.InferenceSummary{
			ID:          appstate.InferenceID("inference-" + string(command.CommandID)),
			ModelID:     command.ModelID,
			DatasetID:   command.DatasetID,
			State:       appstate.InferenceQueued,
			TimeRange:   command.TimeRange.Normalize(),
			GeneratedAt: now,
		},
	}, nil
}

// Research returns deterministic, leakage-safe, render-ready Research data.
func (client *DemoCoordinator) Research(
	ctx context.Context,
	query appstate.ResearchQuery,
) (appstate.ResearchResponseMessage, error) {
	if err := query.Validate(); err != nil {
		return appstate.ResearchResponseMessage{}, failure(err.Error(), query.CorrelationID, false)
	}
	if err := client.wait(ctx, client.delays.Research); err != nil {
		return appstate.ResearchResponseMessage{}, err
	}
	if client.failures.Research || query.Scenario == appstate.ResearchScenarioFailure {
		return appstate.ResearchResponseMessage{}, researchFailure("deterministic Research request failed", query, appstate.ErrorResearchFailed)
	}
	if query.Scenario == appstate.ResearchScenarioLoading {
		<-ctx.Done()
		return appstate.ResearchResponseMessage{}, ctx.Err()
	}
	if query.Scenario == appstate.ResearchScenarioReconnecting {
		<-ctx.Done()
		return appstate.ResearchResponseMessage{}, ctx.Err()
	}
	if query.Scenario == appstate.ResearchScenarioCancelled {
		return appstate.ResearchResponseMessage{}, researchFailure("deterministic Research request cancelled", query, appstate.ErrorResearchCancelled)
	}
	if query.Scenario == appstate.ResearchScenarioQueueFull {
		return appstate.ResearchResponseMessage{}, researchFailure("deterministic Research queue is saturated", query, appstate.ErrorEffectBusy)
	}
	snapshot, err := client.buildResearchSnapshot(ctx, query)
	if err != nil {
		return appstate.ResearchResponseMessage{}, researchFailure(err.Error(), query, appstate.ErrorResearchFailed)
	}
	snapshot.Degraded = query.Scenario == appstate.ResearchScenarioDegraded
	eventType := "research.ready"
	if query.Scenario == appstate.ResearchScenarioEmpty {
		eventType = "research.empty"
	}
	if query.Scenario == appstate.ResearchScenarioRecovered {
		eventType = "research.recovered"
	}
	return appstate.ResearchResponseMessage{
		Event: event(
			appstate.EventID(fmt.Sprintf("evt-research-%s-%06d", query.GroupID, query.Generation)),
			eventType, snapshot.PreparedAt, query.CorrelationID,
		),
		Snapshot: snapshot,
	}, nil
}

// Subscribe opens a deterministic event stream. The stream goroutine owns
// sending and closing the returned buffered channel.
func (client *DemoCoordinator) Subscribe(
	ctx context.Context,
	subscription appstate.EventSubscription,
) (<-chan appstate.ClientMessage, error) {
	if client.failures.OpenEvents {
		return nil, failure("demo event stream failed", "", true)
	}
	output := make(chan appstate.ClientMessage, 8)
	go func() {
		defer close(output)
		sent := 0
		for _, message := range client.eventsAfter(subscription.Since) {
			if err := client.wait(ctx, client.delays.Events); err != nil {
				return
			}
			if client.failures.InterruptEvents &&
				client.failures.EventsBeforeError == sent {
				_ = sendMessage(ctx, output, appstate.ClientFailureMessage{
					Event: event(
						"evt-demo-failure",
						"client.failure",
						client.baseTime().Add(30*time.Second),
						"",
					),
					Error: appstate.ErrorSnapshot{
						Code:      appstate.ErrorClientUnavailable,
						Message:   "demo event stream interrupted",
						Retryable: true,
					},
				})
				return
			}
			if !sendMessage(ctx, output, message) {
				return
			}
			sent++
		}
	}()
	return output, nil
}

// Close stops future delayed work. It is idempotent.
func (client *DemoCoordinator) Close() error {
	client.once.Do(func() {
		close(client.closed)
	})
	return nil
}

func (client *DemoCoordinator) eventsAfter(since appstate.EventID) []appstate.ClientMessage {
	if since == "" {
		return cloneMessages(client.events)
	}
	for index, message := range client.events {
		if messageEventID(message) == since {
			if index+1 >= len(client.events) {
				return nil
			}
			return cloneMessages(client.events[index+1:])
		}
	}
	return cloneMessages(client.events)
}

func (client *DemoCoordinator) baseTime() time.Time {
	client.mu.RLock()
	defer client.mu.RUnlock()
	return client.snapshot.Event.Timestamp.UTC()
}

func (client *DemoCoordinator) wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("demo coordinator: %w", ctx.Err())
		case <-client.closed:
			return errors.New("demo coordinator: closed")
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("demo coordinator: %w", ctx.Err())
	case <-client.closed:
		return errors.New("demo coordinator: closed")
	case <-timer.C:
		return nil
	}
}

func sendMessage(
	ctx context.Context,
	output chan<- appstate.ClientMessage,
	message appstate.ClientMessage,
) bool {
	select {
	case <-ctx.Done():
		return false
	case output <- message:
		return true
	}
}

func failure(message string, correlationID appstate.CorrelationID, retryable bool) error {
	return DemoError{
		Snapshot: appstate.ErrorSnapshot{
			Code:          appstate.ErrorClientUnavailable,
			Message:       message,
			Retryable:     retryable,
			CorrelationID: correlationID,
		},
	}
}

func researchFailure(message string, query appstate.ResearchQuery, code appstate.ErrorCode) error {
	return DemoError{Snapshot: appstate.ErrorSnapshot{
		Code: code, Message: message, Retryable: true, CorrelationID: query.CorrelationID,
	}}
}

// DemoError wraps a typed frontend error snapshot.
type DemoError struct {
	Snapshot appstate.ErrorSnapshot
}

// Error returns the human message.
func (err DemoError) Error() string {
	return err.Snapshot.Message
}

// FrontendError returns the typed frontend error payload.
func (err DemoError) FrontendError() appstate.ErrorSnapshot {
	return err.Snapshot
}

func buildSnapshot(now time.Time, generator *demoGenerator) appstate.SnapshotMessage {
	datasets := buildDatasets(now, generator)
	jobs := buildJobs(now, datasets, generator)
	results := buildResults(now, datasets, jobs, generator)
	models := buildModels(now, results, generator)
	inferences := buildInferences(now, datasets, models, generator)
	return appstate.SnapshotMessage{
		Event: event("evt-snapshot-0001", "snapshot", now, ""),
		Connection: appstate.ConnectionSnapshot{
			State:     appstate.ConnectionConnected,
			UpdatedAt: now,
			Detail:    "demo coordinator",
		},
		Components: []appstate.ComponentSnapshot{
			{
				Component: appstate.ComponentCoordinator,
				State:     appstate.ComponentHealthy,
				Detail:    "demo",
				UpdatedAt: now,
			},
			{
				Component: appstate.ComponentCatalog,
				State:     appstate.ComponentHealthy,
				Detail:    "2 demo datasets",
				UpdatedAt: now,
			},
			{
				Component: appstate.ComponentScheduler,
				State:     appstate.ComponentDegraded,
				Detail:    "simulated queue",
				UpdatedAt: now,
			},
		},
		Cache: appstate.CacheSnapshot{
			Generation: generator.nextRange(10, 99),
			BytesUsed:  96 * 1024 * 1024,
			UpdatedAt:  now,
		},
		Datasets:   datasets,
		Jobs:       jobs,
		Results:    results,
		Models:     models,
		Inferences: inferences,
	}
}

func buildDatasets(now time.Time, generator *demoGenerator) []appstate.DatasetSummary {
	symbols := []appstate.Symbol{"AAPL", "MSFT", "NVDA", "AMD", "SPY", "QQQ"}
	dailyEnd := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC)
	hourlyEnd := now.UTC().Truncate(time.Hour).Add(-time.Hour)
	return []appstate.DatasetSummary{
		{
			ID:          "dataset-us-equities",
			Name:        "US equities daily",
			Status:      appstate.DatasetReady,
			Symbols:     symbols[:4],
			Interval:    appstate.IntervalDaily,
			Rows:        950000 + generator.nextRange(0, 25000),
			Start:       dailyEnd.AddDate(-6, 0, 0),
			End:         dailyEnd,
			Revision:    18,
			Fingerprint: "data-demo-a",
			Adjustment:  "split_dividend_adjusted",
			ImportedAt:  now.Add(-24 * time.Hour),
		},
		{
			ID:          "dataset-index-watchlist",
			Name:        "Index watchlist hourly",
			Status:      appstate.DatasetValidation,
			Symbols:     symbols[4:],
			Interval:    appstate.IntervalHourly,
			Rows:        210000 + generator.nextRange(0, 10000),
			Start:       hourlyEnd.AddDate(-2, 0, 0),
			End:         hourlyEnd,
			Revision:    7,
			Fingerprint: "data-demo-b",
			Adjustment:  "split_dividend_adjusted",
			ImportedAt:  now.Add(-2 * time.Hour),
		},
	}
}

func buildJobs(
	now time.Time,
	datasets []appstate.DatasetSummary,
	generator *demoGenerator,
) []appstate.JobSummary {
	return []appstate.JobSummary{
		{
			ID:              "job-demo-forest",
			ExperimentID:    "experiment-demo-forest",
			RunID:           "run-demo-forest",
			State:           appstate.JobRunning,
			ProgressPermil:  420 + int(generator.nextRange(0, 30)),
			Stage:           "fold 2 / tree 38",
			CPUSlots:        6,
			CheckpointCount: 12,
			UpdatedAt:       now,
		},
		{
			ID:             "job-demo-boost",
			ExperimentID:   "experiment-demo-boost",
			RunID:          "run-demo-boost",
			State:          appstate.JobQueued,
			ProgressPermil: 0,
			Stage:          "queued",
			CPUSlots:       4,
			UpdatedAt:      now.Add(-2 * time.Minute),
		},
		{
			ID:             "job-demo-complete",
			ExperimentID:   "experiment-demo-complete",
			RunID:          "run-demo-complete",
			State:          appstate.JobCompleted,
			ProgressPermil: 1000,
			Stage:          "completed on " + string(datasets[0].ID),
			CPUSlots:       6,
			UpdatedAt:      now.Add(-30 * time.Minute),
		},
	}
}

func buildResults(
	now time.Time,
	datasets []appstate.DatasetSummary,
	jobs []appstate.JobSummary,
	generator *demoGenerator,
) []appstate.RunResultSummary {
	return []appstate.RunResultSummary{
		{
			ID:           "run-demo-complete",
			ExperimentID: "experiment-demo-complete",
			JobID:        jobs[2].ID,
			DatasetID:    datasets[0].ID,
			ModelID:      "model-demo-champion",
			Metrics: []appstate.MetricSummary{
				{Name: "validation_ic", Value: round(generator.nextFloat(0.052, 0.083), 10000)},
				{Name: "test_ic", Value: round(generator.nextFloat(0.034, 0.061), 10000)},
				{Name: "test_sharpe", Value: round(generator.nextFloat(1.1, 1.8), 100)},
			},
			ValidationStart: now.AddDate(-1, -6, 0),
			ValidationEnd:   now.AddDate(-1, 0, 0),
			TestStart:       now.AddDate(-1, 0, 1),
			TestEnd:         now.AddDate(0, 0, -1),
			CompletedAt:     now.Add(-30 * time.Minute),
			Immutable:       true,
		},
	}
}

func buildModels(
	now time.Time,
	results []appstate.RunResultSummary,
	generator *demoGenerator,
) []appstate.ModelSummary {
	return []appstate.ModelSummary{
		{
			ID:                  "model-demo-champion",
			RunID:               results[0].ID,
			Kind:                appstate.ModelRandomForest,
			Alias:               "champion",
			FeatureNames:        []appstate.FeatureName{"ret_5", "volatility_20", "cross_rank_1"},
			TrainingCutoff:      now.AddDate(0, 0, -1),
			CreatedAt:           now.Add(-25 * time.Minute),
			ArtifactFingerprint: fmt.Sprintf("model-demo-%04d", generator.nextRange(0, 9999)),
			Immutable:           true,
		},
	}
}

func buildInferences(
	now time.Time,
	datasets []appstate.DatasetSummary,
	models []appstate.ModelSummary,
	generator *demoGenerator,
) []appstate.InferenceSummary {
	scores := []appstate.ScoredSymbol{
		{Symbol: "NVDA", Rank: 1, Score: round(generator.nextFloat(0.71, 0.92), 1000)},
		{Symbol: "MSFT", Rank: 2, Score: round(generator.nextFloat(0.54, 0.70), 1000)},
		{Symbol: "AAPL", Rank: 3, Score: round(generator.nextFloat(0.48, 0.58), 1000)},
	}
	return []appstate.InferenceSummary{
		{
			ID:        "inference-demo-latest",
			ModelID:   models[0].ID,
			DatasetID: datasets[0].ID,
			State:     appstate.InferenceCompleted,
			TimeRange: appstate.TimeRange{
				Start: now.AddDate(0, 0, -1),
				End:   now.AddDate(0, 0, -1),
			},
			Scores:      scores,
			GeneratedAt: now.Add(-15 * time.Minute),
		},
	}
}

func buildEvents(
	now time.Time,
	snapshot appstate.SnapshotMessage,
	generator *demoGenerator,
) []appstate.ClientMessage {
	job := snapshot.Jobs[0]
	job.ProgressPermil = 610 + int(generator.nextRange(0, 20))
	job.Stage = "fold 3 / tree 12"
	job.UpdatedAt = now.Add(10 * time.Second)
	inference := snapshot.Inferences[0].Clone()
	inference.GeneratedAt = now.Add(20 * time.Second)
	inference.Scores[0].Score = round(inference.Scores[0].Score+0.012, 1000)
	return []appstate.ClientMessage{
		appstate.JobUpdateMessage{
			Event: event("evt-demo-job-0002", "job.updated", now.Add(10*time.Second), ""),
			Job:   job,
		},
		appstate.ComponentStatusMessage{
			Event: event("evt-demo-component-0003", "component.updated", now.Add(15*time.Second), ""),
			Component: appstate.ComponentSnapshot{
				Component: appstate.ComponentWorkerPool,
				State:     appstate.ComponentHealthy,
				Detail:    "6 leased slots",
				UpdatedAt: now.Add(15 * time.Second),
			},
		},
		appstate.InferenceUpdateMessage{
			Event:     event("evt-demo-inference-0004", "inference.updated", now.Add(20*time.Second), ""),
			Inference: inference,
		},
	}
}

func event(
	id appstate.EventID,
	eventType string,
	timestamp time.Time,
	correlationID appstate.CorrelationID,
) appstate.EventEnvelope {
	return appstate.EventEnvelope{
		ID:            id,
		Type:          eventType,
		SchemaVersion: 1,
		Timestamp:     timestamp.UTC(),
		CorrelationID: correlationID,
	}
}

func cloneMessages(input []appstate.ClientMessage) []appstate.ClientMessage {
	if len(input) == 0 {
		return nil
	}
	output := make([]appstate.ClientMessage, len(input))
	copy(output, input)
	return output
}

func messageEventID(message appstate.ClientMessage) appstate.EventID {
	switch message := message.(type) {
	case appstate.SnapshotMessage:
		return message.Event.ID
	case appstate.DatasetCatalogMessage:
		return message.Event.ID
	case appstate.JobUpdateMessage:
		return message.Event.ID
	case appstate.RunResultsMessage:
		return message.Event.ID
	case appstate.ModelRegistryMessage:
		return message.Event.ID
	case appstate.InferenceUpdateMessage:
		return message.Event.ID
	case appstate.ComponentStatusMessage:
		return message.Event.ID
	case appstate.ClientFailureMessage:
		return message.Event.ID
	case appstate.ResearchResponseMessage:
		return message.Event.ID
	case appstate.DataWorkspaceMessage:
		return message.Event.ID
	case appstate.ExperimentWorkspaceMessage:
		return message.Event.ID
	case appstate.ExperimentEvaluationMessage:
		return message.Event.ID
	case appstate.ExperimentSubmittedMessage:
		return message.Event.ID
	case appstate.JobsWorkspaceMessage:
		return message.Event.ID
	case appstate.ResultsWorkspaceMessage:
		return message.Event.ID
	default:
		return ""
	}
}

type demoGenerator struct {
	state uint64
}

func newGenerator(seed uint64) demoGenerator {
	return demoGenerator{state: seed ^ 0x9E3779B97F4A7C15}
}

func (generator *demoGenerator) next() uint64 {
	generator.state = generator.state*6364136223846793005 + 1442695040888963407
	return generator.state
}

func (generator *demoGenerator) nextRange(minimum uint64, maximum uint64) uint64 {
	if maximum <= minimum {
		return minimum
	}
	return minimum + generator.next()%(maximum-minimum+1)
}

func (generator *demoGenerator) nextFloat(minimum float64, maximum float64) float64 {
	value := float64(generator.next()>>11) / float64(uint64(1)<<53)
	return minimum + (maximum-minimum)*value
}

func round(value float64, scale float64) float64 {
	if scale == 0 {
		return value
	}
	if value >= 0 {
		return float64(int64(value*scale+0.5)) / scale
	}
	return float64(int64(value*scale-0.5)) / scale
}

func maxInt(first int, second int) int {
	if first > second {
		return first
	}
	return second
}
