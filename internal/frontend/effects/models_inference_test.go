package effects_test

import (
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/simulator"
)

func TestRuntimeExecutesModelsAndInferenceWorkflows(t *testing.T) {
	runtime, cleanup := startRuntime(t, simulatorFailureNone())
	defer cleanup()
	if !runtime.Enqueue(appstate.LoadSnapshotEffect{ID: "snapshot", CorrelationID: "snapshot"}) {
		t.Fatal("snapshot enqueue failed")
	}
	snapshotAction := waitAction(t, runtime.Actions())
	clientAction, ok := snapshotAction.(appstate.ClientMessageAction)
	if !ok {
		t.Fatalf("snapshot action = %T", snapshotAction)
	}
	snapshot, ok := clientAction.Message.(appstate.SnapshotMessage)
	if !ok || len(snapshot.Datasets) == 0 {
		t.Fatalf("snapshot message = %T", clientAction.Message)
	}
	if !runtime.Enqueue(appstate.QueryModelsWorkspaceEffect{ID: "models", Query: appstate.ModelsWorkspaceQuery{CorrelationID: "models", Generation: 1, Scenario: appstate.ModelsScenarioNormal}}) {
		t.Fatal("models enqueue failed")
	}
	modelsAction := waitAction(t, runtime.Actions())
	clientAction, ok = modelsAction.(appstate.ClientMessageAction)
	if !ok {
		t.Fatalf("models action = %T", modelsAction)
	}
	if _, ok := clientAction.Message.(appstate.ModelsWorkspaceMessage); !ok {
		t.Fatalf("models message = %T", clientAction.Message)
	}
	dataset := snapshot.Datasets[0]
	query := appstate.InferenceWorkspaceQuery{CorrelationID: "inference", Generation: 1, Scenario: appstate.InferenceScenarioNormal, ModelID: "model-demo-champion", DatasetID: dataset.ID, DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint, Symbols: dataset.Symbols, TimeRange: appstate.TimeRange{Start: dataset.Start, End: dataset.End}, Mode: appstate.InferenceLatestSnapshot}
	if !runtime.Enqueue(appstate.QueryInferenceWorkspaceEffect{ID: "inference", Query: query}) {
		t.Fatal("inference enqueue failed")
	}
	inferenceAction := waitAction(t, runtime.Actions())
	clientAction, ok = inferenceAction.(appstate.ClientMessageAction)
	if !ok {
		t.Fatalf("inference action = %T", inferenceAction)
	}
	message, ok := clientAction.Message.(appstate.InferenceWorkspaceMessage)
	if !ok || !message.Snapshot.HasOutput || !message.Snapshot.Compatibility.Compatible {
		t.Fatalf("inference message = %#v", clientAction.Message)
	}
}

func simulatorFailureNone() simulator.FailureProfile {
	return simulator.FailureProfile{}
}
