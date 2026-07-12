package simulator

import (
	"context"
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func BenchmarkDataWorkspaceSnapshot(b *testing.B) {
	client, err := NewDemoCoordinator(Options{Seed: 109, Clock: phase7Clock()})
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()
	query := appstate.DataWorkspaceQuery{CorrelationID: "benchmark-data", Generation: 1, Scenario: appstate.DataScenarioNormal}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := client.DataWorkspace(context.Background(), query); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExperimentEvaluation(b *testing.B) {
	client, err := NewDemoCoordinator(Options{Seed: 113, Clock: phase7Clock()})
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()
	data, err := client.DataWorkspace(context.Background(), appstate.DataWorkspaceQuery{CorrelationID: "data", Generation: 1, Scenario: appstate.DataScenarioNormal})
	if err != nil {
		b.Fatal(err)
	}
	request := appstate.ExperimentEvaluationRequest{CorrelationID: "benchmark-experiment", Generation: 1, Draft: defaultDemoDraft(data.Snapshot.Catalog[0]), Scenario: appstate.ExperimentScenarioNormal}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := client.EvaluateExperiment(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}
