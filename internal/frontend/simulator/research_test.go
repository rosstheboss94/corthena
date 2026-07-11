package simulator

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func simulatorResearchQuery() appstate.ResearchQuery {
	return appstate.ResearchQuery{
		CorrelationID: "research-test", GroupID: "link-default-research", Generation: 1,
		DatasetID: "dataset-us-equities", Symbols: []appstate.Symbol{"AAPL", "MSFT"}, Interval: appstate.IntervalDaily,
		TimeRange:        appstate.TimeRange{Start: time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		SelectedFeatures: []appstate.FeatureName{"ret_5"}, Target: appstate.TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
		Resolution: 640, PageSize: 80, Sort: appstate.ResearchSortTimeAscending, Scenario: appstate.ResearchScenarioNormal,
	}
}

func TestResearchIsDeterministicAndLeakageSafe(t *testing.T) {
	clock := appstate.FixedClock{Time: time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)}
	first, err := NewDemoCoordinator(Options{Seed: 42, Clock: clock})
	if err != nil {
		t.Fatalf("first client: %v", err)
	}
	defer first.Close()
	second, err := NewDemoCoordinator(Options{Seed: 42, Clock: clock})
	if err != nil {
		t.Fatalf("second client: %v", err)
	}
	defer second.Close()
	query := simulatorResearchQuery()
	left, err := first.Research(context.Background(), query)
	if err != nil {
		t.Fatalf("first research: %v", err)
	}
	right, err := second.Research(context.Background(), query)
	if err != nil {
		t.Fatalf("second research: %v", err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatal("same seed, clock, and query produced different Research responses")
	}
	if len(left.Snapshot.Features) != 3 {
		t.Fatalf("feature count = %d", len(left.Snapshot.Features))
	}
	for _, series := range left.Snapshot.Features {
		for index := 0; index < series.Descriptor.Lookback; index++ {
			if !series.Values[index].Missing {
				t.Fatalf("feature %s exposed value before lookback at %d", series.Descriptor.Name, index)
			}
		}
		if series.Values[series.Descriptor.Lookback].Missing {
			t.Fatalf("feature %s remained missing after lookback", series.Descriptor.Name)
		}
	}
	if left.Snapshot.Target.ExcludedRows != uint64(query.Target.HorizonBars+1) {
		t.Fatalf("excluded target rows = %d", left.Snapshot.Target.ExcludedRows)
	}
	for _, row := range left.Snapshot.Rows.Dataset.Rows {
		if row.Cells[len(row.Cells)-1].Null {
			t.Fatalf("page contains row without valid future target: %q", row.ID)
		}
	}
	for _, distribution := range left.Snapshot.Distributions {
		nonzero := 0
		for _, bin := range distribution.Bins {
			if bin.Count > 0 {
				nonzero++
			}
		}
		if nonzero < 4 {
			t.Fatalf("distribution %s has only %d populated bins: %+v", distribution.Name, nonzero, distribution.Bins)
		}
	}
}

func TestResearchScenariosExposeTypedStates(t *testing.T) {
	client, err := NewDemoCoordinator(Options{Seed: 7, Clock: appstate.FixedClock{Time: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()
	query := simulatorResearchQuery()
	query.Scenario = appstate.ResearchScenarioLoading
	loadingContext, cancelLoading := context.WithCancel(context.Background())
	loadingDone := make(chan error, 1)
	go func() {
		_, researchErr := client.Research(loadingContext, query)
		loadingDone <- researchErr
	}()
	select {
	case err := <-loadingDone:
		t.Fatalf("loading scenario completed before cancellation: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancelLoading()
	if err := <-loadingDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("loading cancellation error = %v", err)
	}
	query.Scenario = appstate.ResearchScenarioEmpty
	empty, err := client.Research(context.Background(), query)
	if err != nil || len(empty.Snapshot.Bars) != 0 || empty.Snapshot.Rows.TotalRows != 0 {
		t.Fatalf("empty scenario = bars %d rows %d err %v", len(empty.Snapshot.Bars), empty.Snapshot.Rows.TotalRows, err)
	}
	query.Scenario = appstate.ResearchScenarioDegraded
	degraded, err := client.Research(context.Background(), query)
	if err != nil || !degraded.Snapshot.Degraded || len(degraded.Snapshot.Bars) == 0 {
		t.Fatalf("degraded scenario = degraded %t bars %d err %v", degraded.Snapshot.Degraded, len(degraded.Snapshot.Bars), err)
	}
	for _, scenario := range []appstate.ResearchScenario{appstate.ResearchScenarioFailure, appstate.ResearchScenarioCancelled, appstate.ResearchScenarioQueueFull} {
		query.Scenario = scenario
		_, err := client.Research(context.Background(), query)
		var typed DemoError
		if !errors.As(err, &typed) || typed.Snapshot.CorrelationID != query.CorrelationID {
			t.Fatalf("scenario %s error = %#v", scenario, err)
		}
	}
}

func TestResearchRejectsUnknownSelectedFeature(t *testing.T) {
	client, err := NewDemoCoordinator(Options{Seed: 9, Clock: appstate.FixedClock{Time: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	query := simulatorResearchQuery()
	query.SelectedFeatures = []appstate.FeatureName{"future_price"}
	if _, err := client.Research(context.Background(), query); err == nil {
		t.Fatal("Research accepted an unknown selected feature")
	}
}

func TestResearchViewportDoesNotRecomputeFeatureOrTargetMembership(t *testing.T) {
	clock := appstate.FixedClock{Time: time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)}
	client, err := NewDemoCoordinator(Options{Seed: 23, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	fullQuery := simulatorResearchQuery()
	full, err := client.Research(context.Background(), fullQuery)
	if err != nil {
		t.Fatal(err)
	}
	narrowQuery := fullQuery.Clone()
	narrowQuery.Generation = 2
	narrowQuery.CorrelationID = "narrow"
	narrowQuery.TimeRange = appstate.TimeRange{
		Start: time.Date(2022, 1, 1, 15, 4, 5, 0, time.UTC),
		End:   time.Date(2022, 6, 1, 15, 4, 5, 0, time.UTC),
	}
	narrow, err := client.Research(context.Background(), narrowQuery)
	if err != nil {
		t.Fatal(err)
	}
	if narrow.Snapshot.Target.ExcludedRows != 0 {
		t.Fatalf("viewport edge excluded %d valid dataset targets", narrow.Snapshot.Target.ExcludedRows)
	}
	for _, series := range narrow.Snapshot.Features {
		if series.Missing != 0 {
			t.Fatalf("viewport edge introduced %d missing values for %s", series.Missing, series.Descriptor.Name)
		}
	}
	fullValues := make(map[time.Time]float64)
	for _, value := range full.Snapshot.Features[0].Values {
		if !value.Missing {
			fullValues[value.Timestamp] = value.Value
		}
	}
	for _, value := range narrow.Snapshot.Features[0].Values {
		if expected, found := fullValues[value.Timestamp]; !found || expected != value.Value || value.Missing {
			t.Fatalf("viewport changed feature at %s: %+v expected %.12f found %t", value.Timestamp, value, expected, found)
		}
	}
}

func TestResearchResponsesOwnPublishedBuffers(t *testing.T) {
	client, err := NewDemoCoordinator(Options{Seed: 31, Clock: appstate.FixedClock{Time: time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	query := simulatorResearchQuery()
	first, err := client.Research(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	wantOpen := first.Snapshot.Bars[0].Open
	wantCell := first.Snapshot.Rows.Dataset.Rows[0].Cells[0].String
	first.Snapshot.Bars[0].Open = -1
	first.Snapshot.Rows.Dataset.Rows[0].Cells[0].String = "mutated"
	if len(first.Snapshot.Frame.Layers) > 0 && len(first.Snapshot.Frame.Layers[0].Rects) > 0 {
		first.Snapshot.Frame.Layers[0].Rects[0].Width = -1
	}
	second, err := client.Research(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if second.Snapshot.Bars[0].Open != wantOpen || second.Snapshot.Rows.Dataset.Rows[0].Cells[0].String != wantCell {
		t.Fatal("caller mutation escaped the Research ownership boundary")
	}
}

func TestResearchFeatureAndTargetValuesMatchHandCalculation(t *testing.T) {
	rows := make([]demoResearchRow, 8)
	for index := range rows {
		rows[index].timestamp = time.Unix(int64(index), 0).UTC()
		rows[index].close = 100 + float64(index)
		rows[index].volume = 100 + float64(index)
	}
	ret := computeDemoFeature(rows, 2, 0, appstate.ResearchFeatureDescriptor{Lookback: 2})
	if want := 102.0/100.0 - 1; ret.Missing || math.Abs(ret.Value-want) > 1e-15 {
		t.Fatalf("two-bar return = %+v, want %.15f", ret, want)
	}
	vol := computeDemoFeature(rows, 2, 1, appstate.ResearchFeatureDescriptor{Lookback: 2})
	wantVol := standardDeviation([]float64{101.0/100.0 - 1, 102.0/101.0 - 1})
	if vol.Missing || math.Abs(vol.Value-wantVol) > 1e-15 {
		t.Fatalf("rolling volatility = %+v, want %.15f", vol, wantVol)
	}
	volumeZ := computeDemoFeature(rows, 2, 2, appstate.ResearchFeatureDescriptor{Lookback: 2})
	if volumeZ.Missing || volumeZ.Value != 3 {
		t.Fatalf("volume z-score = %+v, want 3", volumeZ)
	}

	client, err := NewDemoCoordinator(Options{Seed: 43, Clock: appstate.FixedClock{Time: time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	query := simulatorResearchQuery()
	response, err := client.Research(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	index := 40
	horizon := query.Target.HorizonBars
	wantTarget := response.Snapshot.Bars[index+1+horizon].Open/response.Snapshot.Bars[index+1].Open - 1
	got := response.Snapshot.Target.Values[index]
	wantTargetTimestamp := response.Snapshot.Bars[index+1+horizon].Timestamp
	if got.Missing || math.Abs(got.Value-wantTarget) > 1e-15 || !got.Timestamp.Equal(response.Snapshot.Bars[index].Timestamp) || !got.TargetTimestamp.Equal(wantTargetTimestamp) {
		t.Fatalf("forward target = %+v, want %.15f at %s", got, wantTarget, response.Snapshot.Bars[index].Timestamp)
	}
}
