package appstate

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func validDataImportRequest() DataImportRequest {
	return DataImportRequest{
		CorrelationID: "data-import-1", CommandID: "command-import-1", Generation: 1,
		DatasetID: "dataset-us-equities", SourceName: "bars.csv", SourceKind: DataSourceCSV,
		Mode: DataImportAppend, Symbols: []Symbol{"AAPL", "MSFT"}, Interval: IntervalDaily,
		Adjustment: "split_dividend_adjusted", Scenario: DataScenarioNormal,
	}
}

func validPhase7Draft() ExperimentDraft {
	return ExperimentDraft{
		Revision: 1, Name: "Baseline", DatasetID: "dataset-us-equities",
		DatasetRevision: 18, DatasetFingerprint: "data-demo-a",
		Features:  []FeatureName{"ret_5", "volatility_20"},
		Target:    TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
		Split:     SplitSpec{Kind: "walk_forward", TrainBars: 504, ValidationBars: 126, TestBars: 126, PurgeBars: 5, EmbargoBars: 1},
		Model:     ModelSpec{Kind: ModelRandomForest, MaxDepth: 8, MinLeafSamples: 32, EstimatorCount: 200, HistogramBins: 64, Seed: 42},
		Portfolio: PortfolioSpec{LongQuantile: 0.8, ShortQuantile: 0.2, CostBPS: 5}, RequestedCPU: 6,
	}
}

func TestDataImportRequestValidationAndClone(t *testing.T) {
	request := validDataImportRequest()
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	clone := request.Clone()
	clone.Symbols[0] = "NVDA"
	if request.Symbols[0] != "AAPL" {
		t.Fatal("Data import clone shares symbols")
	}
	replacement := request
	replacement.Mode = DataImportReplacement
	replacement.TimeRange = TimeRange{Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
	if err := replacement.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := replacement
	invalid.TimeRange.Start = invalid.TimeRange.Start.In(time.FixedZone("offset", 3600))
	if !errors.Is(invalid.Validate(), ErrInvalidDataRequest) {
		t.Fatalf("non-UTC replacement error = %v", invalid.Validate())
	}
}

func TestExperimentDraftValidationCoversLeakageAndTypedBounds(t *testing.T) {
	draft := validPhase7Draft()
	if issues := ValidateExperimentDraft(draft); len(issues) != 0 {
		t.Fatalf("valid draft issues = %+v", issues)
	}
	draft.Split.PurgeBars = draft.Target.HorizonBars - 1
	draft.Features = append(draft.Features, draft.Features[0])
	draft.RequestedCPU = 0
	issues := ValidateExperimentDraft(draft)
	if len(issues) < 3 {
		t.Fatalf("invalid draft issues = %+v", issues)
	}
}

func TestPhase7ReducersRejectStaleResultsAndLateDraftLoad(t *testing.T) {
	clock := FixedClock{Time: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}
	state, _, err := NewInitialState(clock, NewSequentialIDSource("phase7"))
	if err != nil {
		t.Fatal(err)
	}
	query := DataWorkspaceQuery{CorrelationID: "data-1", Generation: 1, Scenario: DataScenarioNormal}
	state, _, err = Reduce(state, RequestDataWorkspaceAction{Query: query})
	if err != nil {
		t.Fatal(err)
	}
	newer := DataWorkspaceQuery{CorrelationID: "data-2", Generation: 2, Scenario: DataScenarioNormal}
	state, _, err = Reduce(state, RequestDataWorkspaceAction{Query: newer})
	if err != nil {
		t.Fatal(err)
	}
	previousConnection := state.Connection
	stale := DataWorkspaceMessage{Event: EventEnvelope{ID: "stale", Timestamp: clock.Time.Add(time.Hour)}, Snapshot: DataWorkspaceSnapshot{Query: query, Degraded: true}}
	state, _, err = Reduce(state, ClientMessageAction{Message: stale})
	if err != nil {
		t.Fatal(err)
	}
	if state.Data.Generation != 2 || state.Connection != previousConnection {
		t.Fatalf("stale Data response changed state: %+v", state.Data)
	}
	state.Experiments.Draft = validPhase7Draft()
	updated := state.Experiments.Draft.Clone()
	updated.Revision = 2
	state, effects, err := Reduce(state, UpdateExperimentDraftAction{Draft: updated, UpdatedAt: clock.Time})
	if err != nil || len(effects) != 2 {
		t.Fatalf("draft update effects=%d err=%v", len(effects), err)
	}
	state, _, err = Reduce(state, ExperimentDraftLoadedAction{BaseRevision: 0, Draft: validPhase7Draft(), LoadedAt: clock.Time})
	if err != nil || state.Experiments.Draft.Revision != 2 {
		t.Fatalf("late draft load overwrote local revision: %+v err=%v", state.Experiments.Draft, err)
	}
}

func TestPhase7ActionReplayIsDeterministic(t *testing.T) {
	replay := func() AppState {
		clock := FixedClock{Time: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}
		state, _, err := NewInitialState(clock, NewSequentialIDSource("phase7-replay"))
		if err != nil {
			t.Fatal(err)
		}
		state.Datasets = []DatasetSummary{{ID: "dataset-us-equities", Symbols: []Symbol{"AAPL"}, Interval: IntervalDaily, Start: clock.Time.AddDate(-1, 0, 0), End: clock.Time, Revision: 18, Fingerprint: "data-demo-a"}}
		state.Data.Snapshot.Catalog = cloneDatasets(state.Datasets)
		state.Experiments.Draft = validPhase7Draft()
		actions := []UIAction{
			SelectDataDatasetAction{DatasetID: "dataset-us-equities"},
			SetExperimentScenarioAction{Scenario: ExperimentScenarioInvalid},
			SelectExperimentSectionAction{Section: ExperimentSectionModel},
		}
		for _, action := range actions {
			state, _, err = Reduce(state, action)
			if err != nil {
				t.Fatalf("reduce %T: %v", action, err)
			}
		}
		return state
	}
	if !reflect.DeepEqual(replay(), replay()) {
		t.Fatal("identical Phase 7 replay produced different state")
	}
}
