package simulator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func phase7Clock() appstate.FixedClock {
	return appstate.FixedClock{Time: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}
}

func phase7Import(dataset appstate.DatasetSummary, generation uint64, scenario appstate.DataScenario) appstate.DataImportRequest {
	return appstate.DataImportRequest{
		CorrelationID: appstate.CorrelationID("import-correlation"), CommandID: appstate.CorrelationID(fmt.Sprintf("command-%d", generation)), Generation: generation,
		DatasetID: dataset.ID, SourceName: "bars.csv", SourceKind: appstate.DataSourceCSV,
		Mode: appstate.DataImportAppend, Symbols: append([]appstate.Symbol(nil), dataset.Symbols...), Interval: dataset.Interval,
		Adjustment: "split_dividend_adjusted", Scenario: scenario,
	}
}

func TestDataWorkspaceIsDeterministicAndOwnsPublishedValues(t *testing.T) {
	left, err := NewDemoCoordinator(Options{Seed: 71, Clock: phase7Clock()})
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	right, err := NewDemoCoordinator(Options{Seed: 71, Clock: phase7Clock()})
	if err != nil {
		t.Fatal(err)
	}
	defer right.Close()
	query := appstate.DataWorkspaceQuery{CorrelationID: "data", Generation: 1, Scenario: appstate.DataScenarioNormal}
	first, err := left.DataWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	second, err := right.DataWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed, clock, and Data query produced different responses")
	}
	wantName := first.Snapshot.Catalog[0].Name
	first.Snapshot.Catalog[0].Name = "mutated"
	again, err := left.DataWorkspace(context.Background(), query)
	if err != nil || again.Snapshot.Catalog[0].Name != wantName {
		t.Fatalf("caller mutation escaped Data boundary: name=%q err=%v", again.Snapshot.Catalog[0].Name, err)
	}
}

func TestDataImportsPublishAtomicallyAndRejectInvalidRows(t *testing.T) {
	client, err := NewDemoCoordinator(Options{Seed: 73, Clock: phase7Clock()})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	initial, err := client.DataWorkspace(context.Background(), appstate.DataWorkspaceQuery{CorrelationID: "initial", Generation: 1, Scenario: appstate.DataScenarioNormal})
	if err != nil {
		t.Fatal(err)
	}
	dataset := initial.Snapshot.Catalog[0]
	rejected, err := client.ImportData(context.Background(), phase7Import(dataset, 2, appstate.DataScenarioDuplicate))
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Snapshot.Catalog[0].Revision != dataset.Revision || rejected.Snapshot.Imports[0].State != appstate.DataImportRejected {
		t.Fatalf("duplicate import mutated catalog: dataset=%+v import=%+v", rejected.Snapshot.Catalog[0], rejected.Snapshot.Imports[0])
	}
	acceptedRequest := phase7Import(dataset, 3, appstate.DataScenarioNormal)
	acceptedRequest.CommandID = "accepted"
	accepted, err := client.ImportData(context.Background(), acceptedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Snapshot.Catalog[0].Revision != dataset.Revision+1 || accepted.Snapshot.Imports[0].PublishedFingerprint == "" {
		t.Fatalf("accepted import did not publish exactly one revision: %+v", accepted.Snapshot.Imports[0])
	}
}

func TestExperimentEvaluationAndSubmissionAreDeterministicAndImmutable(t *testing.T) {
	client, err := NewDemoCoordinator(Options{Seed: 79, Clock: phase7Clock()})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	data, err := client.DataWorkspace(context.Background(), appstate.DataWorkspaceQuery{CorrelationID: "data", Generation: 1, Scenario: appstate.DataScenarioNormal})
	if err != nil {
		t.Fatal(err)
	}
	draft := defaultDemoDraft(data.Snapshot.Catalog[0])
	evaluationRequest := appstate.ExperimentEvaluationRequest{CorrelationID: "evaluate", Generation: 1, Draft: draft, Scenario: appstate.ExperimentScenarioNormal}
	evaluation, err := client.EvaluateExperiment(context.Background(), evaluationRequest)
	if err != nil || len(evaluation.Issues) != 0 || evaluation.Estimate.Rows == 0 {
		t.Fatalf("evaluation=%+v err=%v", evaluation, err)
	}
	command := appstate.SubmitExperimentCommand{CorrelationID: "submit", CommandID: "immutable", Generation: 1, Draft: draft, Scenario: appstate.ExperimentScenarioNormal}
	first, err := client.SubmitExperiment(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.SubmitExperiment(context.Background(), command)
	if err != nil || !reflect.DeepEqual(first.Definition, second.Definition) {
		t.Fatalf("idempotent retry changed definition: first=%+v second=%+v err=%v", first.Definition, second.Definition, err)
	}
	command.Draft.Name = "mutated retry"
	if _, err := client.SubmitExperiment(context.Background(), command); err == nil {
		t.Fatal("conflicting command retry was accepted")
	}
	query := appstate.ExperimentQuery{CorrelationID: "experiments", Generation: 1, Scenario: appstate.ExperimentScenarioNormal}
	workspace, err := client.Experiments(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if !workspace.Snapshot.Definitions[0].Immutable || workspace.Snapshot.Definitions[0].Draft.Name != draft.Name {
		t.Fatalf("accepted definition was mutated: %+v", workspace.Snapshot.Definitions[0])
	}
	importRequest := phase7Import(data.Snapshot.Catalog[0], 2, appstate.DataScenarioNormal)
	importRequest.CommandID = "catalog-change"
	if _, err := client.ImportData(context.Background(), importRequest); err != nil {
		t.Fatal(err)
	}
	afterChange, err := client.Experiments(context.Background(), appstate.ExperimentQuery{CorrelationID: "after-change", Generation: 2, Scenario: appstate.ExperimentScenarioNormal})
	if err != nil {
		t.Fatal(err)
	}
	if afterChange.Snapshot.Definitions[0].Draft.DatasetFingerprint != draft.DatasetFingerprint {
		t.Fatal("catalog change rewrote an immutable submitted definition")
	}
	stale, err := client.EvaluateExperiment(context.Background(), appstate.ExperimentEvaluationRequest{CorrelationID: "stale", Generation: 2, Draft: draft, Scenario: appstate.ExperimentScenarioNormal})
	if err != nil || len(stale.Issues) == 0 {
		t.Fatalf("stale catalog draft was not rejected: issues=%+v err=%v", stale.Issues, err)
	}
}

func TestPhase7LoadingAndTypedFailureScenarios(t *testing.T) {
	client, err := NewDemoCoordinator(Options{Seed: 83, Clock: phase7Clock()})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, loadErr := client.DataWorkspace(ctx, appstate.DataWorkspaceQuery{CorrelationID: "loading", Generation: 1, Scenario: appstate.DataScenarioLoading})
		done <- loadErr
	}()
	select {
	case err := <-done:
		t.Fatalf("loading completed early: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("loading cancellation = %v", err)
	}
	for _, scenario := range []appstate.ExperimentScenario{appstate.ExperimentScenarioFailure, appstate.ExperimentScenarioCancelled, appstate.ExperimentScenarioSaturated} {
		_, err := client.Experiments(context.Background(), appstate.ExperimentQuery{CorrelationID: "failure", Generation: 1, Scenario: scenario})
		var typed DemoError
		if !errors.As(err, &typed) || typed.Snapshot.CorrelationID != "failure" {
			t.Fatalf("scenario %s error=%#v", scenario, err)
		}
	}
}
