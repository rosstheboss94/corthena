package appstate

import (
	"testing"
	"time"
)

func FuzzDataImportRequestValidate(f *testing.F) {
	f.Add("dataset", "AAPL", "bars.csv", uint64(1), int64(0), int64(0), false)
	f.Fuzz(func(t *testing.T, dataset string, symbol string, source string, generation uint64, startUnix int64, endUnix int64, replacement bool) {
		mode := DataImportAppend
		rangeValue := TimeRange{}
		if replacement {
			mode = DataImportReplacement
			rangeValue = TimeRange{Start: time.Unix(startUnix, 0).UTC(), End: time.Unix(endUnix, 0).UTC()}
		}
		request := DataImportRequest{
			CorrelationID: "fuzz", CommandID: "fuzz-command", Generation: generation,
			DatasetID: DatasetID(dataset), SourceName: source, SourceKind: DataSourceCSV,
			Mode: mode, Symbols: []Symbol{Symbol(symbol)}, Interval: IntervalDaily,
			TimeRange: rangeValue, Adjustment: "split_dividend_adjusted", Scenario: DataScenarioNormal,
		}
		_ = request.Validate()
	})
}

func FuzzExperimentDraftValidation(f *testing.F) {
	f.Add("draft", 5, 504, 5, 8, 6)
	f.Fuzz(func(t *testing.T, name string, horizon int, train int, purge int, depth int, cpus int) {
		draft := validPhase7Draft()
		draft.Name = name
		draft.Target.HorizonBars = horizon
		draft.Split.TrainBars = train
		draft.Split.PurgeBars = purge
		draft.Model.MaxDepth = depth
		draft.RequestedCPU = cpus
		_ = ValidateExperimentDraft(draft)
	})
}
