package simulator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestModelsRegistryIsDeterministicAndAliasCommandsAreTransactional(t *testing.T) {
	t.Parallel()
	first := newPhase9Client(t)
	second := newPhase9Client(t)
	t.Cleanup(func() { _ = first.Close() })
	t.Cleanup(func() { _ = second.Close() })
	query := appstate.ModelsWorkspaceQuery{CorrelationID: "models", Generation: 1, Scenario: appstate.ModelsScenarioNormal}
	left, err := first.ModelsWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	right, err := second.ModelsWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left.Snapshot, right.Snapshot) {
		t.Fatal("same seed/query produced different registry snapshot")
	}
	if len(left.Snapshot.Registry) != 2 || left.Snapshot.Registry[0].Summary.ID != "model-demo-candidate" {
		t.Fatalf("registry = %#v", left.Snapshot.Registry)
	}
	command := appstate.AliasAssignmentCommand{CorrelationID: "promote", CommandID: "promote-candidate", Generation: 1, Alias: "champion", ModelID: "model-demo-candidate", Confirmed: true}
	promoted, err := first.AssignModelAlias(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if len(promoted.Snapshot.Registry) != 2 || promoted.Snapshot.Registry[0].Summary.ID != "model-demo-candidate" || promoted.Snapshot.Registry[0].Summary.Alias != "champion" {
		t.Fatalf("promotion registry = %#v", promoted.Snapshot.Registry)
	}
	if promoted.Snapshot.Registry[1].Summary.ID != "model-demo-champion" || promoted.Snapshot.Registry[1].Summary.Alias != "" {
		t.Fatalf("previous target was deleted or mutated incorrectly: %#v", promoted.Snapshot.Registry[1].Summary)
	}
	if last := promoted.Snapshot.AliasHistory[len(promoted.Snapshot.AliasHistory)-1]; last.PreviousModelID != "model-demo-champion" || last.ModelID != "model-demo-candidate" {
		t.Fatalf("history = %#v", last)
	}
	replayed, err := first.AssignModelAlias(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, promoted) {
		t.Fatal("alias command replay changed transaction result")
	}
	command.Confirmed = false
	if _, err := first.AssignModelAlias(context.Background(), command); err == nil {
		t.Fatal("unconfirmed alias assignment succeeded")
	}
}

func TestInferenceCompatibilityRankingAndExportAreDeterministic(t *testing.T) {
	t.Parallel()
	client := newPhase9Client(t)
	t.Cleanup(func() { _ = client.Close() })
	snapshot, err := client.Snapshot(context.Background(), appstate.SnapshotRequest{CorrelationID: "snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	dataset := snapshot.Datasets[0]
	query := appstate.InferenceWorkspaceQuery{CorrelationID: "inference", Generation: 1, Scenario: appstate.InferenceScenarioNormal, ModelID: "model-demo-champion", DatasetID: dataset.ID, DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint, Symbols: dataset.Symbols, TimeRange: appstate.TimeRange{Start: dataset.Start, End: dataset.End}, Mode: appstate.InferenceLatestSnapshot}
	message, err := client.InferenceWorkspace(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if !message.Snapshot.Compatibility.Compatible || !message.Snapshot.HasOutput {
		t.Fatalf("compatibility/output = %#v/%t", message.Snapshot.Compatibility, message.Snapshot.HasOutput)
	}
	rows := message.Snapshot.Output.Rankings[0].Rows
	if len(rows) != len(dataset.Symbols) || rows[0].Symbol != "AAPL" || rows[1].Symbol != "AMD" {
		t.Fatalf("stable score ranking = %#v", rows)
	}
	if message.Snapshot.Output.Export.State != appstate.ExportIdle || message.Snapshot.Output.Checksum == "" {
		t.Fatalf("incomplete output = %#v", message.Snapshot.Output)
	}
	historical := query
	historical.Generation = 2
	historical.CorrelationID = "historical"
	historical.Mode = appstate.InferenceHistorical
	historical.TimeRange = appstate.TimeRange{Start: dataset.Start.AddDate(0, 0, 20), End: dataset.End.AddDate(0, 0, -1)}
	historicalMessage, err := client.InferenceWorkspace(context.Background(), historical)
	if err != nil {
		t.Fatal(err)
	}
	if !historicalMessage.Snapshot.HasOutput || historicalMessage.Snapshot.Output.Mode != appstate.InferenceHistorical || !historicalMessage.Snapshot.Output.Rankings[0].Timestamp.Equal(historical.TimeRange.End) {
		t.Fatalf("historical output = %#v", historicalMessage.Snapshot.Output)
	}
	exportCommand := appstate.ExportInferenceCommand{CorrelationID: "export", CommandID: "export-1", Generation: 1, InferenceID: message.Snapshot.Output.ID}
	exported, err := client.ExportInference(context.Background(), exportCommand)
	if err != nil {
		t.Fatal(err)
	}
	if exported.Export.State != appstate.ExportReady || exported.Export.Checksum == "" {
		t.Fatalf("export = %#v", exported.Export)
	}
	replayed, err := client.ExportInference(context.Background(), exportCommand)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, exported) {
		t.Fatal("export command replay changed result")
	}
	incompatible := query
	incompatible.Generation = 3
	incompatible.DatasetFingerprint = "wrong-fingerprint"
	failed, err := client.InferenceWorkspace(context.Background(), incompatible)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Snapshot.Compatibility.Compatible || failed.Snapshot.HasOutput || len(failed.Snapshot.Compatibility.Diagnostics) == 0 {
		t.Fatalf("incompatible result = %#v", failed.Snapshot)
	}
	for generation := uint64(4); generation <= maxDemoInferenceHistory+4; generation++ {
		bounded := query
		bounded.Generation = generation
		bounded.CorrelationID = "bounded"
		if _, err := client.InferenceWorkspace(context.Background(), bounded); err != nil {
			t.Fatal(err)
		}
	}
	client.mu.RLock()
	historyCount := len(client.inferenceHistory)
	client.mu.RUnlock()
	if historyCount != maxDemoInferenceHistory {
		t.Fatalf("bounded history = %d, want %d", historyCount, maxDemoInferenceHistory)
	}
}

func TestModelsAndInferenceHonorCancellation(t *testing.T) {
	t.Parallel()
	client, err := NewDemoCoordinator(Options{Seed: 42, Clock: appstate.FixedClock{Time: phase9Time()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.ModelsWorkspace(ctx, appstate.ModelsWorkspaceQuery{CorrelationID: "models", Generation: 1, Scenario: appstate.ModelsScenarioNormal}); !errors.Is(err, context.Canceled) {
		t.Fatalf("models cancellation = %v", err)
	}
	snapshot, err := client.Snapshot(context.Background(), appstate.SnapshotRequest{})
	if err != nil {
		t.Fatal(err)
	}
	dataset := snapshot.Datasets[0]
	if _, err := client.InferenceWorkspace(ctx, appstate.InferenceWorkspaceQuery{CorrelationID: "inference", Generation: 1, Scenario: appstate.InferenceScenarioNormal, ModelID: "model-demo-champion", DatasetID: dataset.ID, DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint, Symbols: dataset.Symbols, Mode: appstate.InferenceLatestSnapshot}); !errors.Is(err, context.Canceled) {
		t.Fatalf("inference cancellation = %v", err)
	}
}

func BenchmarkInferenceWorkspace(b *testing.B) {
	client, err := NewDemoCoordinator(Options{Seed: 42, Clock: appstate.FixedClock{Time: phase9Time()}})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = client.Close() })
	snapshot, err := client.Snapshot(context.Background(), appstate.SnapshotRequest{})
	if err != nil {
		b.Fatal(err)
	}
	dataset := snapshot.Datasets[0]
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, err := client.InferenceWorkspace(context.Background(), appstate.InferenceWorkspaceQuery{CorrelationID: "bench", Generation: uint64(index + 1), Scenario: appstate.InferenceScenarioNormal, ModelID: "model-demo-champion", DatasetID: dataset.ID, DatasetRevision: dataset.Revision, DatasetFingerprint: dataset.Fingerprint, Symbols: dataset.Symbols, TimeRange: appstate.TimeRange{Start: dataset.Start, End: dataset.End}, Mode: appstate.InferenceLatestSnapshot})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func newPhase9Client(t *testing.T) *DemoCoordinator {
	t.Helper()
	client, err := NewDemoCoordinator(Options{Seed: 42, Clock: appstate.FixedClock{Time: phase9Time()}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func phase9Time() time.Time {
	return time.Date(2026, 7, 12, 19, 0, 0, 0, time.UTC)
}
