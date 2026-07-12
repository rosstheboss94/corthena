package appstate

import (
	"errors"
	"testing"
	"time"
)

func TestJobStateControlsAreExhaustive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state  JobState
		pause  bool
		resume bool
		cancel bool
	}{
		{JobQueued, true, false, true},
		{JobRunning, true, false, true},
		{JobPauseRequested, false, false, true},
		{JobPaused, false, true, true},
		{JobCompleted, false, false, false},
		{JobFailed, false, false, false},
		{JobCancelled, false, false, false},
		{JobInterrupted, false, true, true},
	}
	for _, test := range tests {
		test := test
		t.Run(string(test.state), func(t *testing.T) {
			t.Parallel()
			if !test.state.Valid() {
				t.Fatalf("state %q is invalid", test.state)
			}
			if got := test.state.AllowsControl(JobControlPause); got != test.pause {
				t.Fatalf("pause = %t, want %t", got, test.pause)
			}
			if got := test.state.AllowsControl(JobControlResume); got != test.resume {
				t.Fatalf("resume = %t, want %t", got, test.resume)
			}
			if got := test.state.AllowsControl(JobControlCancel); got != test.cancel {
				t.Fatalf("cancel = %t, want %t", got, test.cancel)
			}
		})
	}
}

func TestJobsReducerRejectsStaleResponsesAndIllegalControls(t *testing.T) {
	t.Parallel()
	state := AppState{JobsWorkspace: DefaultJobsWorkspaceState()}
	next, effects, err := Reduce(state, RequestJobsWorkspaceAction{Query: JobsWorkspaceQuery{
		CorrelationID: "jobs-1", Generation: 1, Scenario: JobsScenarioSuccess,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 1 || next.JobsWorkspace.State != WorkspaceLoading {
		t.Fatalf("effects/state = %d/%q", len(effects), next.JobsWorkspace.State)
	}
	stale, _, err := Reduce(next, ClientMessageAction{Message: JobsWorkspaceMessage{Snapshot: JobsWorkspaceSnapshot{
		Query: JobsWorkspaceQuery{CorrelationID: "stale", Generation: 0, Scenario: JobsScenarioSuccess},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if stale.JobsWorkspace.State != WorkspaceLoading {
		t.Fatalf("stale response changed state to %q", stale.JobsWorkspace.State)
	}
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	ready, _, err := Reduce(next, ClientMessageAction{Message: JobsWorkspaceMessage{
		Event: EventEnvelope{ID: "evt-jobs", Timestamp: now},
		Snapshot: JobsWorkspaceSnapshot{
			Query: JobsWorkspaceQuery{CorrelationID: "jobs-1", Generation: 1, Scenario: JobsScenarioSuccess},
			Jobs:  []JobDetail{{Summary: JobSummary{ID: "job-running", State: JobRunning, UpdatedAt: now}}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if ready.JobsWorkspace.SelectedJobID != "job-running" || ready.JobsWorkspace.State != WorkspaceReady {
		t.Fatalf("selection/state = %q/%q", ready.JobsWorkspace.SelectedJobID, ready.JobsWorkspace.State)
	}
	command := JobControlCommand{CorrelationID: "control-1", CommandID: "command-1", Generation: 1, JobID: "job-running", Control: JobControlPause}
	controlled, controlEffects, err := Reduce(ready, ControlJobAction{Command: command})
	if err != nil {
		t.Fatal(err)
	}
	if controlled.JobsWorkspace.PendingControl != JobControlPause || len(controlEffects) != 1 {
		t.Fatalf("pending/effects = %q/%d", controlled.JobsWorkspace.PendingControl, len(controlEffects))
	}
	command.Control = JobControlResume
	if _, _, err := Reduce(ready, ControlJobAction{Command: command}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("illegal resume error = %v", err)
	}
}

func TestResultsSelectionIsStableAndBounded(t *testing.T) {
	t.Parallel()
	runs := make([]RunResultDetail, 5)
	for index := range runs {
		runs[index].Summary = RunResultSummary{ID: RunID(string(rune('a' + index))), Immutable: true}
	}
	state := AppState{ResultsWorkspace: DefaultResultsWorkspaceState()}
	state.ResultsWorkspace.Snapshot.Runs = runs
	for index := 0; index < 4; index++ {
		next, _, err := Reduce(state, SelectResultRunAction{RunID: runs[index].Summary.ID, Toggle: true})
		if err != nil {
			t.Fatal(err)
		}
		state = next
	}
	if len(state.ResultsWorkspace.SelectedRunIDs) != 4 {
		t.Fatalf("selection count = %d", len(state.ResultsWorkspace.SelectedRunIDs))
	}
	if _, _, err := Reduce(state, SelectResultRunAction{RunID: runs[4].Summary.ID, Toggle: true}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("fifth selection error = %v", err)
	}
	original := state.Clone()
	state.ResultsWorkspace.Snapshot.Runs[0].Configuration = append(state.ResultsWorkspace.Snapshot.Runs[0].Configuration, ConfigurationValue{Path: "mutated"})
	if len(original.ResultsWorkspace.Snapshot.Runs[0].Configuration) != 0 {
		t.Fatal("clone shares result configuration")
	}
}

func TestResultsWorkspaceQueryValidation(t *testing.T) {
	t.Parallel()
	query := ResultsWorkspaceQuery{CorrelationID: "results", Generation: 1, Scenario: ResultsScenarioNormal, RunIDs: []RunID{"a", "a"}}
	if !errors.Is(query.Validate(), ErrInvalidResults) {
		t.Fatalf("duplicate validation error = %v", query.Validate())
	}
	query.RunIDs = []RunID{"a", "b", "c", "d", "e"}
	if !errors.Is(query.Validate(), ErrInvalidResults) {
		t.Fatalf("comparison bound error = %v", query.Validate())
	}
}
