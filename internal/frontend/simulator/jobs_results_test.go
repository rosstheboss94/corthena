package simulator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestJobsWorkspaceCoversRequiredLifecycles(t *testing.T) {
	t.Parallel()
	client := newPhase8Client(t)
	t.Cleanup(func() { _ = client.Close() })
	tests := []struct {
		scenario appstate.JobsScenario
		state    appstate.JobState
	}{
		{appstate.JobsScenarioSuccess, appstate.JobRunning},
		{appstate.JobsScenarioPauseResume, appstate.JobPaused},
		{appstate.JobsScenarioCancellation, appstate.JobCancelled},
		{appstate.JobsScenarioInterruption, appstate.JobInterrupted},
		{appstate.JobsScenarioFailure, appstate.JobFailed},
	}
	for index, test := range tests {
		query := appstate.JobsWorkspaceQuery{CorrelationID: appstate.CorrelationID(test.scenario), Generation: uint64(index + 1), Scenario: test.scenario}
		message, err := client.JobsWorkspace(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		if len(message.Snapshot.Jobs) != 259 {
			t.Fatalf("%s job count = %d", test.scenario, len(message.Snapshot.Jobs))
		}
		if got := message.Snapshot.Jobs[0].Summary.State; got != test.state {
			t.Fatalf("%s primary state = %q, want %q", test.scenario, got, test.state)
		}
	}
}

func TestJobControlsMutateOnlyCurrentTypedJob(t *testing.T) {
	t.Parallel()
	client := newPhase8Client(t)
	t.Cleanup(func() { _ = client.Close() })
	query := appstate.JobsWorkspaceQuery{CorrelationID: "jobs", Generation: 1, Scenario: appstate.JobsScenarioPauseResume}
	message, err := client.JobsWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	job := message.Snapshot.Jobs[0]
	command := appstate.JobControlCommand{CorrelationID: "resume", CommandID: "resume-command", Generation: 1, JobID: job.Summary.ID, Control: appstate.JobControlResume}
	resumed, err := client.ControlJob(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Job.State != appstate.JobRunning || !resumed.HasDetail {
		t.Fatalf("resume state/detail = %q/%t", resumed.Job.State, resumed.HasDetail)
	}
	command = appstate.JobControlCommand{CorrelationID: "pause", CommandID: "pause-command", Generation: 1, JobID: job.Summary.ID, Control: appstate.JobControlPause}
	paused, err := client.ControlJob(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Job.State != appstate.JobPaused || paused.Job.CheckpointCount <= job.Summary.CheckpointCount {
		t.Fatalf("pause state/checkpoints = %q/%d", paused.Job.State, paused.Job.CheckpointCount)
	}
	replayed, err := client.ControlJob(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, paused) {
		t.Fatal("idempotent job command replay changed the accepted update")
	}
	command.Control = appstate.JobControlResume
	command.JobID = "missing"
	command.CorrelationID = "missing"
	command.CommandID = "missing-command"
	if _, err := client.ControlJob(context.Background(), command); err == nil {
		t.Fatal("missing job control succeeded")
	}
}

func TestResultsAreDeterministicImmutableAndFiltered(t *testing.T) {
	t.Parallel()
	first := newPhase8Client(t)
	second := newPhase8Client(t)
	t.Cleanup(func() { _ = first.Close() })
	t.Cleanup(func() { _ = second.Close() })
	query := appstate.ResultsWorkspaceQuery{CorrelationID: "results", Generation: 1, Scenario: appstate.ResultsScenarioNormal}
	left, err := first.ResultsWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	right, err := second.ResultsWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left.Snapshot, right.Snapshot) {
		t.Fatal("same seed and query produced different Results snapshots")
	}
	if len(left.Snapshot.Runs) != 3 {
		t.Fatalf("run count = %d", len(left.Snapshot.Runs))
	}
	for _, run := range left.Snapshot.Runs {
		if !run.Summary.Immutable || len(run.Folds) != 4 || len(run.Equity) == 0 || len(run.Predictions) == 0 {
			t.Fatalf("incomplete immutable run %q", run.Summary.ID)
		}
		validation, test := false, false
		for _, metric := range run.Summary.Metrics {
			validation = validation || metric.Partition == appstate.MetricValidation
			test = test || metric.Partition == appstate.MetricTest
		}
		if !validation || !test {
			t.Fatalf("run %q lacks partitioned metrics", run.Summary.ID)
		}
	}
	filteredQuery := query
	filteredQuery.Generation = 2
	filteredQuery.Filter = "phase8-b"
	filtered, err := first.ResultsWorkspace(context.Background(), filteredQuery)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Snapshot.Runs) != 1 || filtered.Snapshot.Runs[0].Summary.ID != "run-phase8-b" {
		t.Fatalf("filtered runs = %+v", summariesForResults(filtered.Snapshot.Runs))
	}
}

func TestResultsLoadingHonorsCancellation(t *testing.T) {
	t.Parallel()
	client := newPhase8Client(t)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.ResultsWorkspace(ctx, appstate.ResultsWorkspaceQuery{CorrelationID: "loading", Generation: 1, Scenario: appstate.ResultsScenarioLoading})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("loading cancellation error = %v", err)
	}
}

func BenchmarkJobsWorkspaceQueue(b *testing.B) {
	client, err := NewDemoCoordinator(Options{Seed: 42, Clock: appstate.FixedClock{Time: phase8Time()}})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = client.Close() })
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, err := client.JobsWorkspace(context.Background(), appstate.JobsWorkspaceQuery{CorrelationID: "bench", Generation: uint64(index + 1), Scenario: appstate.JobsScenarioSuccess})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func newPhase8Client(t *testing.T) *DemoCoordinator {
	t.Helper()
	client, err := NewDemoCoordinator(Options{Seed: 42, Clock: appstate.FixedClock{Time: phase8Time()}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func phase8Time() time.Time {
	return time.Date(2026, 7, 12, 16, 0, 0, 0, time.UTC)
}
