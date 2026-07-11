package chart

import "testing"

func TestPrepareSupportsEverySpecifiedLayer(t *testing.T) {
	t.Parallel()
	samples := []Sample{{X: 1, Y: 2, SourceIndex: 1}, {X: 2, Y: 6, SourceIndex: 2}, {X: 3, Y: 4, SourceIndex: 3}}
	candles := []Candle{
		{X: 1, Open: 2, High: 5, Low: 1, Close: 4, Volume: 10, SourceIndex: 1},
		{X: 2, Open: 4, High: 7, Low: 3, Close: 5, Volume: 20, SourceIndex: 2},
	}
	tests := []struct {
		name  string
		kind  LayerKind
		layer Layer
		data  Rect
	}{
		{name: "ohlcv and volume", kind: LayerOHLCV, layer: OHLCVLayer{ID: "ohlcv", Candles: candles}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "line", kind: LayerLine, layer: LineLayer{ID: "line", Samples: samples, Style: StylePrimary}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "area", kind: LayerArea, layer: AreaLayer{ID: "area", Samples: samples, Baseline: 0, Style: StylePrimary}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "histogram", kind: LayerHistogram, layer: HistogramLayer{ID: "hist", Samples: samples, Baseline: 0}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "scatter", kind: LayerScatter, layer: ScatterLayer{ID: "scatter", Samples: samples}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "equity", kind: LayerEquity, layer: EquityLayer{ID: "equity", Samples: samples}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "drawdown", kind: LayerDrawdown, layer: DrawdownLayer{ID: "drawdown", Samples: []Sample{{X: 1, Y: -1, SourceIndex: 1}, {X: 2, Y: -3, SourceIndex: 2}}}, data: Rect{MinX: 0, MinY: -4, MaxX: 4, MaxY: 1}},
		{name: "heatmap", kind: LayerHeatmap, layer: HeatmapLayer{ID: "heat", Minimum: 0, Maximum: 10, Cells: []HeatCell{{Bounds: Rect{MinX: 1, MinY: 1, MaxX: 2, MaxY: 2}, Value: 5}}}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 4}},
		{name: "importance", kind: LayerFeatureImportance, layer: FeatureImportanceLayer{ID: "importance", Values: []ImportanceValue{{Name: "f1", Value: 8, Row: 0}, {Name: "f2", Value: 4, Row: 1}}}, data: Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 2}},
		{name: "predictions", kind: LayerPredictions, layer: PredictionLayer{ID: "predictions", Samples: samples}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "trades", kind: LayerTrades, layer: TradeLayer{ID: "trades", Trades: []Trade{{X: 1, Y: 2, Side: TradeBuy, SourceIndex: 1}, {X: 2, Y: 3, Side: TradeSell, SourceIndex: 2}}}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
		{name: "train validation test regions", kind: LayerRegions, layer: RegionLayer{ID: "regions", Regions: []Region{{MinimumX: 0, MaximumX: 1, Kind: RegionTrain}, {MinimumX: 1, MaximumX: 2, Kind: RegionValidation}, {MinimumX: 2, MaximumX: 3, Kind: RegionTest}}}, data: Rect{MinX: 0, MinY: 0, MaxX: 4, MaxY: 8}},
	}
	viewport := Rect{MinX: 10, MinY: 20, MaxX: 410, MaxY: 220}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame, err := Prepare(test.data, viewport, 400, []Layer{test.layer})
			if err != nil {
				t.Fatal(err)
			}
			if len(frame.Layers) != 1 || frame.Layers[0].Kind != test.kind || frame.WorkCount == 0 {
				t.Fatalf("frame = %+v", frame)
			}
			if frame.ByteSize() == 0 {
				t.Fatal("frame byte size is zero")
			}
		})
	}
}

func TestFrameCloneDoesNotShareOwnedBuffers(t *testing.T) {
	t.Parallel()
	frame, err := Prepare(
		Rect{MinX: 0, MinY: 0, MaxX: 2, MaxY: 2},
		Rect{MinX: 0, MinY: 0, MaxX: 200, MaxY: 100},
		200,
		[]Layer{LineLayer{ID: "line", Samples: []Sample{{X: 0, Y: 0, SourceIndex: 1}, {X: 2, Y: 2, SourceIndex: 2}}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	clone := frame.Clone()
	clone.Layers[0].Segments[0].Start.X = 99
	if frame.Layers[0].Segments[0].Start.X == 99 {
		t.Fatal("clone mutated source frame")
	}
}
