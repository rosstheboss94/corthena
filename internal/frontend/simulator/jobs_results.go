package simulator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

// JobsWorkspace returns deterministic queue, progress, resource, checkpoint,
// process, and log telemetry for one generation.
func (client *DemoCoordinator) JobsWorkspace(
	ctx context.Context,
	query appstate.JobsWorkspaceQuery,
) (appstate.JobsWorkspaceMessage, error) {
	if err := query.Validate(); err != nil {
		return appstate.JobsWorkspaceMessage{}, phase8Failure(err.Error(), query.CorrelationID, appstate.ErrorValidation, false)
	}
	if err := client.wait(ctx, client.delays.Jobs); err != nil {
		return appstate.JobsWorkspaceMessage{}, err
	}
	if client.failures.Jobs {
		return appstate.JobsWorkspaceMessage{}, phase8Failure("deterministic Jobs request failed", query.CorrelationID, appstate.ErrorJobsFailed, true)
	}
	snapshot := buildJobsWorkspace(client.baseTime(), client.seed, query)
	client.mu.Lock()
	client.jobs = snapshot.Clone()
	client.snapshot.Jobs = summariesForJobs(snapshot.Jobs)
	client.mu.Unlock()
	return appstate.JobsWorkspaceMessage{
		Event:    event(appstate.EventID(fmt.Sprintf("evt-jobs-%06d", query.Generation)), "jobs.ready", snapshot.PreparedAt, query.CorrelationID),
		Snapshot: snapshot,
	}, nil
}

// ResultsWorkspace returns deterministic immutable comparison output.
func (client *DemoCoordinator) ResultsWorkspace(
	ctx context.Context,
	query appstate.ResultsWorkspaceQuery,
) (appstate.ResultsWorkspaceMessage, error) {
	query = query.Clone()
	if err := query.Validate(); err != nil {
		return appstate.ResultsWorkspaceMessage{}, phase8Failure(err.Error(), query.CorrelationID, appstate.ErrorValidation, false)
	}
	if err := client.wait(ctx, client.delays.Results); err != nil {
		return appstate.ResultsWorkspaceMessage{}, err
	}
	if client.failures.Results || query.Scenario == appstate.ResultsScenarioFailure {
		return appstate.ResultsWorkspaceMessage{}, phase8Failure("deterministic Results request failed", query.CorrelationID, appstate.ErrorResultsFailed, true)
	}
	if query.Scenario == appstate.ResultsScenarioLoading {
		<-ctx.Done()
		return appstate.ResultsWorkspaceMessage{}, ctx.Err()
	}
	if query.Scenario == appstate.ResultsScenarioCancelled {
		return appstate.ResultsWorkspaceMessage{}, phase8Failure("deterministic Results request cancelled", query.CorrelationID, appstate.ErrorResultsCancelled, true)
	}
	if query.Scenario == appstate.ResultsScenarioSaturated {
		return appstate.ResultsWorkspaceMessage{}, phase8Failure("deterministic Results queue is saturated", query.CorrelationID, appstate.ErrorEffectBusy, true)
	}
	snapshot := buildResultsWorkspace(client.baseTime(), client.seed, query)
	client.mu.Lock()
	client.results = snapshot.Clone()
	client.snapshot.Results = summariesForResults(snapshot.Runs)
	client.mu.Unlock()
	eventType := "results.ready"
	if query.Scenario == appstate.ResultsScenarioEmpty {
		eventType = "results.empty"
	}
	if query.Scenario == appstate.ResultsScenarioRecovered {
		eventType = "results.recovered"
	}
	return appstate.ResultsWorkspaceMessage{
		Event:    event(appstate.EventID(fmt.Sprintf("evt-results-%06d", query.Generation)), eventType, snapshot.PreparedAt, query.CorrelationID),
		Snapshot: snapshot,
	}, nil
}

func (client *DemoCoordinator) controlJob(
	ctx context.Context,
	command appstate.JobControlCommand,
) (appstate.JobUpdateMessage, error) {
	if err := command.Validate(); err != nil {
		return appstate.JobUpdateMessage{}, phase8Failure(err.Error(), command.CorrelationID, appstate.ErrorValidation, false)
	}
	if err := client.wait(ctx, client.delays.Commands); err != nil {
		return appstate.JobUpdateMessage{}, err
	}
	if client.failures.Commands {
		return appstate.JobUpdateMessage{}, phase8Failure("demo job command failed", command.CorrelationID, appstate.ErrorJobsFailed, true)
	}
	now := client.baseTime().Add(time.Duration(command.Generation+20) * time.Second)
	client.mu.Lock()
	defer client.mu.Unlock()
	if previous, found := client.jobCommands[command.CommandID]; found {
		return previous.Clone(), nil
	}
	index := -1
	for candidate := range client.jobs.Jobs {
		if client.jobs.Jobs[candidate].Summary.ID == command.JobID {
			index = candidate
			break
		}
	}
	if index < 0 {
		return appstate.JobUpdateMessage{}, phase8Failure("job does not exist in the current generation", command.CorrelationID, appstate.ErrorJobControlRejected, false)
	}
	detail := client.jobs.Jobs[index].Clone()
	if !detail.Summary.State.AllowsControl(command.Control) {
		return appstate.JobUpdateMessage{}, phase8Failure("job state rejects the requested control", command.CorrelationID, appstate.ErrorJobControlRejected, false)
	}
	switch command.Control {
	case appstate.JobControlPause:
		detail.Summary.State = appstate.JobPaused
		detail.Summary.Stage = "paused after durable checkpoint"
		for stageIndex := range detail.Stages {
			if detail.Stages[stageIndex].State == appstate.JobStageRunning {
				detail.Stages[stageIndex].State = appstate.JobStagePaused
				detail.Stages[stageIndex].UpdatedAt = now
			}
		}
		detail.Checkpoints = append(detail.Checkpoints, checkpointFor(detail.Summary.ID, uint64(len(detail.Checkpoints)+1), now))
		detail.Summary.CheckpointCount = len(detail.Checkpoints)
		detail.Logs = append(detail.Logs, jobLog(uint64(len(detail.Logs)+1), now, appstate.JobLogInfo, "worker", "pause acknowledged after durable node commit"))
	case appstate.JobControlResume:
		detail.Summary.State = appstate.JobRunning
		detail.Summary.Stage = "resumed from checkpoint"
		for stageIndex := range detail.Stages {
			if detail.Stages[stageIndex].State == appstate.JobStagePaused {
				detail.Stages[stageIndex].State = appstate.JobStageRunning
				detail.Stages[stageIndex].UpdatedAt = now
			}
		}
		detail.Logs = append(detail.Logs, jobLog(uint64(len(detail.Logs)+1), now, appstate.JobLogInfo, "coordinator", "resume accepted from latest compatible checkpoint"))
	case appstate.JobControlCancel:
		detail.Summary.State = appstate.JobCancelled
		detail.Summary.Stage = "cancelled cooperatively"
		detail.Summary.CPUSlots = 0
		detail.Worker.State = appstate.ComponentStopping
		detail.Worker.ActiveTasks = 0
		for stageIndex := range detail.Stages {
			if detail.Stages[stageIndex].State == appstate.JobStageRunning || detail.Stages[stageIndex].State == appstate.JobStagePaused {
				detail.Stages[stageIndex].State = appstate.JobStageCancelled
				detail.Stages[stageIndex].UpdatedAt = now
			}
		}
		detail.Logs = append(detail.Logs, jobLog(uint64(len(detail.Logs)+1), now, appstate.JobLogWarn, "worker", "cancellation completed; immutable committed artifacts retained"))
	}
	detail.Summary.UpdatedAt = now
	detail.Worker.HeartbeatAt = now
	client.jobs.Jobs[index] = detail.Clone()
	client.snapshot.Jobs = summariesForJobs(client.jobs.Jobs)
	message := appstate.JobUpdateMessage{
		Event: event(appstate.EventID("evt-command-"+string(command.CommandID)), "job.updated", now, command.CorrelationID),
		Job:   detail.Summary.Clone(), Detail: detail, HasDetail: true,
	}
	client.jobCommands[command.CommandID] = message.Clone()
	return message, nil
}

func buildJobsWorkspace(now time.Time, seed uint64, query appstate.JobsWorkspaceQuery) appstate.JobsWorkspaceSnapshot {
	generator := newGenerator(seed ^ uint64(query.Generation)*0x9E3779B97F4A7C15)
	primaryState := appstate.JobRunning
	primaryStage := "walk-forward fold 3 / tree 82"
	progress := 638
	switch query.Scenario {
	case appstate.JobsScenarioPauseResume:
		primaryState, primaryStage, progress = appstate.JobPaused, "paused at node boundary", 512
	case appstate.JobsScenarioCancellation:
		primaryState, primaryStage, progress = appstate.JobCancelled, "cancelled cooperatively", 447
	case appstate.JobsScenarioInterruption:
		primaryState, primaryStage, progress = appstate.JobInterrupted, "worker heartbeat expired", 583
	case appstate.JobsScenarioFailure:
		primaryState, primaryStage, progress = appstate.JobFailed, "checkpoint compatibility failure", 731
	}
	primary := buildJobDetail(now, &generator, "job-phase8-primary", "experiment-phase8-forest", "run-phase8-primary", primaryState, primaryStage, progress, 0)
	jobs := []appstate.JobDetail{primary}
	jobs = append(jobs,
		buildJobDetail(now.Add(-time.Minute), &generator, "job-phase8-complete", "experiment-phase8-boost", "run-phase8-complete", appstate.JobCompleted, "immutable result published", 1000, 0),
		buildJobDetail(now.Add(-2*time.Minute), &generator, "job-phase8-queued", "experiment-phase8-tree", "run-phase8-queued", appstate.JobQueued, "waiting for 4 CPU slots", 0, 1),
	)
	for index := 0; index < 256; index++ {
		id := appstate.JobID(fmt.Sprintf("job-queued-%04d", index+1))
		jobs = append(jobs, appstate.JobDetail{Summary: appstate.JobSummary{
			ID: id, ExperimentID: appstate.ExperimentID(fmt.Sprintf("experiment-sweep-%04d", index+1)),
			RunID: appstate.RunID(fmt.Sprintf("run-sweep-%04d", index+1)), State: appstate.JobQueued,
			Stage: "queued deterministic trial", QueuePosition: index + 2, RequestedSlots: 2 + index%4,
			CreatedAt: now.Add(-time.Duration(index+3) * time.Second), UpdatedAt: now,
		}})
	}
	processes := []appstate.ProcessSnapshot{
		{ID: "process-coordinator", Component: appstate.ComponentCoordinator, Role: "coordinator", PID: 4200, State: appstate.ComponentHealthy, Detail: "scheduler and durable writer", StartedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
		{ID: "process-worker-primary", Component: appstate.ComponentWorkerPool, Role: "training-worker", PID: 7312, State: primary.Worker.State, Detail: primary.Worker.Degradation, StartedAt: now.Add(-34 * time.Minute), UpdatedAt: now},
		{ID: "process-cache", Component: appstate.ComponentCache, Role: "visualization-cache", PID: 4200, State: appstate.ComponentHealthy, Detail: "byte-bounded", StartedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
	}
	return appstate.JobsWorkspaceSnapshot{
		Query: query, Jobs: jobs, Processes: processes, TotalCPUSlots: 14,
		LeasedCPUSlots: primary.Summary.CPUSlots, PreparedAt: now.Add(3 * time.Second),
	}
}

func buildJobDetail(now time.Time, generator *demoGenerator, id appstate.JobID, experimentID appstate.ExperimentID, runID appstate.RunID, state appstate.JobState, stage string, progress int, queue int) appstate.JobDetail {
	active := state == appstate.JobRunning || state == appstate.JobPaused || state == appstate.JobInterrupted || state == appstate.JobFailed
	slots := 0
	if active {
		slots = 6
	}
	summary := appstate.JobSummary{
		ID: id, ExperimentID: experimentID, RunID: runID, State: state, ProgressPermil: progress,
		Stage: stage, CPUSlots: slots, CheckpointCount: 3, QueuePosition: queue, RequestedSlots: 6,
		CreatedAt: now.Add(-42 * time.Minute), StartedAt: now.Add(-39 * time.Minute), UpdatedAt: now,
	}
	if state == appstate.JobFailed {
		summary.Error = appstate.ErrorSnapshot{Code: appstate.ErrorJobsFailed, Message: "checkpoint schema does not match the pinned engine version", Retryable: false}
	}
	stageStates := []appstate.JobStageState{appstate.JobStageCompleted, appstate.JobStageCompleted, appstate.JobStageRunning, appstate.JobStagePending}
	if state == appstate.JobPaused || state == appstate.JobInterrupted {
		stageStates[2] = appstate.JobStagePaused
	}
	if state == appstate.JobFailed {
		stageStates[2] = appstate.JobStageFailed
	}
	if state == appstate.JobCancelled {
		stageStates[2] = appstate.JobStageCancelled
	}
	if state == appstate.JobCompleted {
		for index := range stageStates {
			stageStates[index] = appstate.JobStageCompleted
		}
	}
	stages := []appstate.JobStageProgress{
		{ID: "materialize", Name: "Materialize features", State: stageStates[0], ProgressPermil: 1000, CompletedUnits: 946000, TotalUnits: 946000, UpdatedAt: now.Add(-34 * time.Minute)},
		{ID: "folds", Name: "Prepare walk-forward folds", State: stageStates[1], ProgressPermil: 1000, CompletedUnits: 6, TotalUnits: 6, UpdatedAt: now.Add(-31 * time.Minute)},
		{ID: "train", Name: "Train deterministic estimators", State: stageStates[2], ProgressPermil: min(progress*10/7, 1000), CompletedUnits: uint64(progress), TotalUnits: 1000, UpdatedAt: now},
		{ID: "evaluate", Name: "Evaluate and publish", State: stageStates[3], ProgressPermil: 0, TotalUnits: 1, UpdatedAt: now},
	}
	metrics := make([]appstate.JobMetricSeries, 3)
	for seriesIndex, name := range []string{"validation_mse", "validation_ic", "rss_mib"} {
		points := make([]appstate.JobMetricPoint, 36)
		for index := range points {
			value := 0.14 - float64(index)*0.0017 + generator.nextFloat(-0.002, 0.002)
			if seriesIndex == 1 {
				value = 0.018 + float64(index)*0.0012 + generator.nextFloat(-0.003, 0.003)
			}
			if seriesIndex == 2 {
				value = 780 + float64(index)*18 + generator.nextFloat(-8, 8)
			}
			points[index] = appstate.JobMetricPoint{Step: uint64(index + 1), Value: value, Timestamp: now.Add(time.Duration(index-35) * time.Minute)}
		}
		metrics[seriesIndex] = appstate.JobMetricSeries{Name: name, Points: points}
	}
	workerState := appstate.ComponentHealthy
	degradation := "healthy heartbeat; immutable inputs"
	if state == appstate.JobInterrupted {
		workerState, degradation = appstate.ComponentFailed, "heartbeat expired; explicit resume required"
	}
	if state == appstate.JobFailed {
		workerState, degradation = appstate.ComponentFailed, "worker stopped after fail-closed validation"
	}
	checkpoints := []appstate.CheckpointSummary{
		checkpointFor(id, 10, now.Add(-18*time.Minute)), checkpointFor(id, 11, now.Add(-9*time.Minute)), checkpointFor(id, 12, now.Add(-2*time.Minute)),
	}
	logs := []appstate.JobLogEntry{
		jobLog(1, now.Add(-39*time.Minute), appstate.JobLogInfo, "coordinator", "leased 6 of 14 compute slots"),
		jobLog(2, now.Add(-34*time.Minute), appstate.JobLogInfo, "worker", "published read-only materialization"),
		jobLog(3, now.Add(-9*time.Minute), appstate.JobLogDebug, "checkpoint", "durably committed checkpoint sequence 11"),
		jobLog(4, now.Add(-2*time.Minute), appstate.JobLogInfo, "worker", stage),
	}
	if state == appstate.JobFailed {
		logs = append(logs, jobLog(5, now, appstate.JobLogError, "checkpoint", summary.Error.Message))
	}
	return appstate.JobDetail{
		Summary: summary, Stages: stages, Metrics: metrics,
		Worker:      appstate.WorkerResourceSnapshot{JobID: id, WorkerID: "worker-" + string(id), PID: 7312, State: workerState, LeasedSlots: slots, GOMAXPROCS: maxInt(slots, 1), Goroutines: 19 + slots, ActiveTasks: min(slots, 4), MemoryBytes: 1480 << 20, HeartbeatAt: now, Degradation: degradation},
		Checkpoints: checkpoints, Logs: logs,
	}
}

func checkpointFor(jobID appstate.JobID, sequence uint64, timestamp time.Time) appstate.CheckpointSummary {
	return appstate.CheckpointSummary{
		ID: fmt.Sprintf("checkpoint-%s-%03d", jobID, sequence), JobID: jobID, State: appstate.CheckpointCommitted,
		Sequence: sequence, NodeCount: sequence * 128, Bytes: sequence * 65536,
		Checksum: fmt.Sprintf("sha256:%016x", sequence*0x9E3779B97F4A7C15), CommittedAt: timestamp, Compatible: true,
	}
}

func jobLog(sequence uint64, timestamp time.Time, level appstate.JobLogLevel, component string, message string) appstate.JobLogEntry {
	return appstate.JobLogEntry{Sequence: sequence, Timestamp: timestamp, Level: level, Component: component, Message: message}
}

func buildResultsWorkspace(now time.Time, seed uint64, query appstate.ResultsWorkspaceQuery) appstate.ResultsWorkspaceSnapshot {
	if query.Scenario == appstate.ResultsScenarioEmpty {
		return appstate.ResultsWorkspaceSnapshot{Query: query.Clone(), PreparedAt: now.Add(4 * time.Second)}
	}
	runs := make([]appstate.RunResultDetail, 3)
	for index := range runs {
		runs[index] = buildRunDetail(now.Add(-time.Duration(index)*24*time.Hour), seed+uint64(index)*31, index)
	}
	filter := strings.ToLower(strings.TrimSpace(query.Filter))
	if filter != "" {
		filtered := make([]appstate.RunResultDetail, 0, len(runs))
		for _, run := range runs {
			haystack := strings.ToLower(string(run.Summary.ID) + " " + string(run.Summary.ExperimentID) + " " + string(run.Summary.ModelID))
			if strings.Contains(haystack, filter) {
				filtered = append(filtered, run)
			}
		}
		runs = filtered
	}
	selected := make(map[appstate.RunID]int, len(query.RunIDs))
	for index, runID := range query.RunIDs {
		selected[runID] = index
	}
	sort.SliceStable(runs, func(left, right int) bool {
		leftIndex, leftSelected := selected[runs[left].Summary.ID]
		rightIndex, rightSelected := selected[runs[right].Summary.ID]
		if leftSelected != rightSelected {
			return leftSelected
		}
		if leftSelected {
			return leftIndex < rightIndex
		}
		return runs[left].Summary.CompletedAt.After(runs[right].Summary.CompletedAt)
	})
	return appstate.ResultsWorkspaceSnapshot{
		Query: query.Clone(), Runs: runs, PreparedAt: now.Add(4 * time.Second),
		Degraded: query.Scenario == appstate.ResultsScenarioDegraded,
	}
}

func buildRunDetail(now time.Time, seed uint64, index int) appstate.RunResultDetail {
	generator := newGenerator(seed)
	id := appstate.RunID(fmt.Sprintf("run-phase8-%c", 'a'+rune(index)))
	validationIC := 0.061 - float64(index)*0.006 + generator.nextFloat(-0.002, 0.002)
	testIC := 0.049 - float64(index)*0.004 + generator.nextFloat(-0.002, 0.002)
	sharpe := 1.54 - float64(index)*0.18 + generator.nextFloat(-0.04, 0.04)
	metrics := []appstate.MetricSummary{
		{Name: "mse", Value: 0.012 + float64(index)*0.001, Partition: appstate.MetricValidation, Stability: 0.91 - float64(index)*0.03},
		{Name: "mae", Value: 0.078 + float64(index)*0.003, Partition: appstate.MetricValidation, Stability: 0.89 - float64(index)*0.02},
		{Name: "pearson_ic", Value: validationIC, Partition: appstate.MetricValidation, Stability: 0.83 - float64(index)*0.04},
		{Name: "pearson_ic", Value: testIC, Partition: appstate.MetricTest, Stability: 0.79 - float64(index)*0.04},
		{Name: "sharpe", Value: sharpe, Partition: appstate.MetricTest, Stability: 0.74 - float64(index)*0.03},
		{Name: "max_drawdown", Value: -0.112 - float64(index)*0.018, Partition: appstate.MetricTest, Stability: 0.86},
	}
	summary := appstate.RunResultSummary{
		ID: id, ExperimentID: appstate.ExperimentID(fmt.Sprintf("experiment-phase8-%c", 'a'+rune(index))),
		JobID: appstate.JobID(fmt.Sprintf("job-phase8-%c", 'a'+rune(index))), DatasetID: "dataset-us-equities",
		ModelID: appstate.ModelID(fmt.Sprintf("model-phase8-%c", 'a'+rune(index))), Metrics: metrics,
		ValidationStart: now.AddDate(-2, 0, 0), ValidationEnd: now.AddDate(-1, -6, 0),
		TestStart: now.AddDate(-1, -6, 1), TestEnd: now.AddDate(0, 0, -1), CompletedAt: now, Immutable: true,
	}
	equity := make([]appstate.ResultSeriesPoint, 160)
	drawdown := make([]appstate.ResultSeriesPoint, len(equity))
	overlay := make([]appstate.PredictionMarketPoint, len(equity))
	value, peak := 1.0, 1.0
	for pointIndex := range equity {
		trend := 0.0012 - float64(index)*0.00015
		ret := trend + 0.006*math.Sin(float64(pointIndex)*0.23+float64(index)) + generator.nextFloat(-0.004, 0.004)
		value *= 1 + ret
		peak = math.Max(peak, value)
		timestamp := now.AddDate(0, 0, pointIndex-len(equity))
		equity[pointIndex] = appstate.ResultSeriesPoint{Timestamp: timestamp, Value: value}
		drawdown[pointIndex] = appstate.ResultSeriesPoint{Timestamp: timestamp, Value: value/peak - 1}
		prediction := 0.02*math.Sin(float64(pointIndex)*0.31) + generator.nextFloat(-0.012, 0.012)
		overlay[pointIndex] = appstate.PredictionMarketPoint{Timestamp: timestamp, Symbol: "AAPL", Prediction: prediction, Market: ret, Eligible: true}
	}
	folds := make([]appstate.FoldResult, 4)
	for foldIndex := range folds {
		end := now.AddDate(0, -foldIndex*3, 0)
		folds[foldIndex] = appstate.FoldResult{
			Index:           foldIndex + 1,
			Train:           appstate.TimeRange{Start: end.AddDate(-3, 0, 0), End: end.AddDate(0, -8, 0)},
			Validation:      appstate.TimeRange{Start: end.AddDate(0, -8, 1), End: end.AddDate(0, -4, 0)},
			Test:            appstate.TimeRange{Start: end.AddDate(0, -4, 1), End: end},
			ValidationStats: []appstate.MetricSummary{{Name: "pearson_ic", Value: validationIC + float64(foldIndex-2)*0.004, Partition: appstate.MetricValidation}},
			TestStats:       []appstate.MetricSummary{{Name: "pearson_ic", Value: testIC + float64(foldIndex-2)*0.005, Partition: appstate.MetricTest}},
		}
	}
	return appstate.RunResultDetail{
		Summary: summary, Folds: folds, Equity: equity, Drawdown: drawdown,
		InformationCoefficient: resultHistogram(-0.12, 0.16, 14, 18+index*3), Predictions: resultHistogram(-0.08, 0.09, 16, 24+index*2), Overlay: overlay,
		Configuration: []appstate.ConfigurationValue{
			{Section: "model", Path: "kind", Value: "random_forest"},
			{Section: "model", Path: "max_depth", Value: fmt.Sprintf("%d", 8+index*2)},
			{Section: "model", Path: "estimators", Value: fmt.Sprintf("%d", 200+index*100)},
			{Section: "split", Path: "purge_bars", Value: "5"},
			{Section: "portfolio", Path: "cost_bps", Value: fmt.Sprintf("%d", 5+index)},
		},
		Disclosures: []string{"Test metrics were not used for model selection.", "Borrow availability, impact, and survivorship correction are not modeled.", "Orders with missing next-open values are ineligible."},
	}
}

func resultHistogram(minimum float64, maximum float64, bins int, peak int) []appstate.HistogramBin {
	output := make([]appstate.HistogramBin, bins)
	width := (maximum - minimum) / float64(bins)
	center := float64(bins-1) / 2
	for index := range output {
		distance := math.Abs(float64(index)-center) / center
		count := maxInt(1, int(float64(peak)*(1-distance*distance)))
		output[index] = appstate.HistogramBin{Minimum: minimum + float64(index)*width, Maximum: minimum + float64(index+1)*width, Count: uint64(count)}
	}
	return output
}

func summariesForJobs(details []appstate.JobDetail) []appstate.JobSummary {
	output := make([]appstate.JobSummary, len(details))
	for index, detail := range details {
		output[index] = detail.Summary.Clone()
	}
	return output
}

func summariesForResults(details []appstate.RunResultDetail) []appstate.RunResultSummary {
	output := make([]appstate.RunResultSummary, len(details))
	for index, detail := range details {
		output[index] = detail.Summary.Clone()
	}
	return output
}

func phase8Failure(message string, correlationID appstate.CorrelationID, code appstate.ErrorCode, retryable bool) error {
	return DemoError{Snapshot: appstate.ErrorSnapshot{Code: code, Message: message, Retryable: retryable, CorrelationID: correlationID}}
}
