package simulator

import (
	"context"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func BenchmarkResearchHourlyDenseViewport(b *testing.B) {
	clock := appstate.FixedClock{Time: time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)}
	client, err := NewDemoCoordinator(Options{Seed: 41, Clock: clock})
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()
	query := appstate.ResearchQuery{
		CorrelationID: "benchmark", GroupID: "benchmark", Generation: 1,
		DatasetID: "dataset-index-watchlist", Symbols: []appstate.Symbol{"SPY", "QQQ"}, Interval: appstate.IntervalHourly,
		TimeRange:        appstate.TimeRange{Start: clock.Now().AddDate(-2, 0, 0), End: clock.Now().Add(-time.Hour)},
		SelectedFeatures: []appstate.FeatureName{"volatility_20"}, Target: appstate.TargetSpec{Kind: "forward_open_return", HorizonBars: 12},
		Resolution: 1920, PageSize: 200, Sort: appstate.ResearchSortTimeAscending, Scenario: appstate.ResearchScenarioNormal,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		query.Generation = uint64(index + 1)
		if _, err := client.Research(context.Background(), query); err != nil {
			b.Fatal(err)
		}
	}
}
