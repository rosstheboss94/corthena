package appstate

import (
	"testing"
	"time"
)

func FuzzResearchQueryValidate(f *testing.F) {
	f.Add("dataset", "AAPL", "", int64(0), int64(86400), 640, 100)
	f.Add("", "", "bad", int64(10), int64(5), -1, 0)
	f.Fuzz(func(t *testing.T, dataset string, symbol string, cursor string, startSeconds int64, spanSeconds int64, resolution int, pageSize int) {
		if spanSeconds > 10*365*24*60*60 {
			spanSeconds = 10 * 365 * 24 * 60 * 60
		}
		if spanSeconds < -10*365*24*60*60 {
			spanSeconds = -10 * 365 * 24 * 60 * 60
		}
		start := time.Unix(startSeconds, 0).UTC()
		query := ResearchQuery{
			CorrelationID: "fuzz", GroupID: "fuzz", Generation: 1,
			DatasetID: DatasetID(dataset), Symbols: []Symbol{Symbol(symbol)}, Interval: IntervalDaily,
			TimeRange:        TimeRange{Start: start, End: start.Add(time.Duration(spanSeconds) * time.Second)},
			SelectedFeatures: []FeatureName{"ret_5"}, Target: TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
			Resolution: resolution, Cursor: cursor, PageSize: pageSize,
			Sort: ResearchSortTimeAscending, Scenario: ResearchScenarioNormal,
		}
		if err := query.Validate(); err == nil {
			if query.DatasetID == "" || query.Symbols[0] == "" || !query.TimeRange.Start.Before(query.TimeRange.End) || query.Resolution < 64 || query.PageSize < 1 {
				t.Fatalf("validator accepted invalid query: %+v", query)
			}
		}
	})
}
