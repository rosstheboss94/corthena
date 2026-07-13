package nativeui

import (
	"fmt"
	"math"

	"github.com/rosstheboss94/corthena/internal/frontend/chart"
)

// DrawPhase5GoldenFrame renders the domain-neutral visualization acceptance
// fixture through the same locked-thread primitives used by the workstation.
func (window *Window) DrawPhase5GoldenFrame(scalePercent int) error {
	if err := window.requireOpen("draw Phase 5 golden frame"); err != nil {
		return err
	}
	if scalePercent != 100 && scalePercent != 150 && scalePercent != 200 {
		return fmt.Errorf("draw Phase 5 golden frame: unsupported scale %d", scalePercent)
	}
	width, height := float32(window.backend.screenWidth()), float32(window.backend.screenHeight())
	scale := float32(scalePercent) / 100
	inset, header, gap := 16*scale, 44*scale, 12*scale
	tableHeight := min(float32(190)*scale, height*0.32)
	chartBounds := chart.Rect{MinX: float64(inset), MinY: float64(header + inset), MaxX: float64(width - inset), MaxY: float64(height - inset - tableHeight - gap)}
	tableBounds := rectangle{x: inset, y: float32(chartBounds.MaxY) + gap, width: width - inset*2, height: height - inset - (float32(chartBounds.MaxY) + gap)}
	frame, err := phase5GoldenChart(chartBounds)
	if err != nil {
		return err
	}
	renderer := &shellRenderer{window: window, scale: scale}
	window.backend.beginDrawing()
	window.backend.clearBackground(tokenBackground)
	window.backend.drawRectangle(rectangle{x: 0, y: 0, width: width, height: 44 * scale}, tokenPanel)
	window.backend.drawText(window.interFont, "Corthena  /  Generic Visualization", point{x: 16 * scale, y: 11 * scale}, 14*scale, 0, tokenText)
	clip := rectangle{x: float32(chartBounds.MinX), y: float32(chartBounds.MinY), width: float32(chartBounds.Width()), height: float32(chartBounds.Height())}
	window.backend.drawRectangle(clip, tokenBackground)
	window.backend.beginScissor(clip)
	renderer.drawChartFrame(frame)
	window.backend.endScissor()
	window.backend.drawRectangleLines(clip, 1, tokenDivider)
	phase5GoldenTable(window, tableBounds, scale)
	window.backend.endDrawing()
	return renderer.err
}

func phase5GoldenChart(bounds chart.Rect) (chart.Frame, error) {
	samples := make([]chart.Sample, 101)
	predictions := make([]chart.Sample, 101)
	for index := range 101 {
		x := float64(index)
		samples[index] = chart.Sample{SourceIndex: uint64(index), X: x, Y: 48 + 18*math.Sin(x/8) + x*0.16}
		predictions[index] = chart.Sample{SourceIndex: uint64(index), X: x, Y: 50 + 15*math.Sin((x+3)/8) + x*0.14}
	}
	candles := make([]chart.Candle, 18)
	for index := range 18 {
		x := float64(index)
		candles[index] = chart.Candle{SourceIndex: uint64(index), X: x*5 + 2, Open: 42 + x*2, High: 50 + x*2, Low: 38 + x*2, Close: 47 + x*2, Volume: 100 + x*20}
	}
	heat := make([]chart.HeatCell, 0, 16)
	for row := range 4 {
		for column := range 4 {
			heat = append(heat, chart.HeatCell{Bounds: chart.Rect{MinX: float64(82 + column*4), MinY: float64(5 + row*5), MaxX: float64(86 + column*4), MaxY: float64(10 + row*5)}, Value: float64(row+column) / 7})
		}
	}
	strided := func(source []chart.Sample, step int) []chart.Sample {
		result := make([]chart.Sample, 0, (len(source)+step-1)/step)
		for index := 0; index < len(source); index += step {
			result = append(result, source[index])
		}
		return result
	}
	layers := []chart.Layer{
		chart.RegionLayer{ID: "partitions", Regions: []chart.Region{{MinimumX: 0, MaximumX: 60, Kind: chart.RegionTrain}, {MinimumX: 60, MaximumX: 80, Kind: chart.RegionValidation}, {MinimumX: 80, MaximumX: 100, Kind: chart.RegionTest}}},
		chart.OHLCVLayer{ID: "ohlcv", Candles: candles},
		chart.LineLayer{ID: "line", Samples: samples, Style: chart.StylePrimary},
		chart.AreaLayer{ID: "area", Samples: strided(samples, 4), Baseline: 35, Style: chart.StylePrimary},
		chart.HistogramLayer{ID: "histogram", Samples: strided(samples, 5), Baseline: 35, Style: chart.StyleWarning},
		chart.ScatterLayer{ID: "scatter", Samples: strided(samples, 8), Style: chart.StyleSecondary},
		chart.EquityLayer{ID: "equity", Samples: samples},
		chart.DrawdownLayer{ID: "drawdown", Samples: strided(samples, 3)},
		chart.HeatmapLayer{ID: "heatmap", Cells: heat, Minimum: 0, Maximum: 1},
		chart.FeatureImportanceLayer{ID: "importance", Values: []chart.ImportanceValue{{Name: "momentum", Value: 18, Row: 82}, {Name: "volatility", Value: 14, Row: 87}, {Name: "volume", Value: 10, Row: 92}}, Style: chart.StyleSecondary},
		chart.PredictionLayer{ID: "predictions", Samples: predictions},
		chart.TradeLayer{ID: "trades", Trades: []chart.Trade{{X: 25, Y: 58, Side: chart.TradeBuy, SourceIndex: 0}, {X: 55, Y: 67, Side: chart.TradeSell, SourceIndex: 1}, {X: 75, Y: 72, Side: chart.TradeBuy, SourceIndex: 2}}},
	}
	return chart.Prepare(chart.Rect{MinX: 0, MinY: 0, MaxX: 100, MaxY: 100}, bounds, max(1, int(bounds.Width()+0.5)), layers)
}

func phase5GoldenTable(window *Window, bounds rectangle, scale float32) {
	headers := []string{"Symbol", "Last", "Prediction", "Signal", "Partition"}
	rows := [][]string{{"AAPL", "228.14", "231.02", "BUY", "test"}, {"MSFT", "512.44", "508.91", "HOLD", "validation"}, {"NVDA", "189.27", "193.85", "BUY", "test"}, {"AMD", "171.33", "168.22", "SELL", "train"}}
	rowHeight := 24 * scale
	columnWidth := bounds.width / float32(len(headers))
	window.backend.beginScissor(bounds)
	for column, title := range headers {
		x := bounds.x + float32(column)*columnWidth
		cell := rectangle{x: x, y: bounds.y, width: columnWidth, height: rowHeight}
		window.backend.drawRectangle(cell, tokenRaised)
		window.backend.drawRectangleLines(cell, 1, tokenDivider)
		window.backend.drawText(window.interFont, title, point{x: x + 6*scale, y: bounds.y + 5*scale}, 12*scale, 0, tokenText)
	}
	for rowIndex, row := range rows {
		y := bounds.y + float32(rowIndex+1)*rowHeight
		for column, value := range row {
			x := bounds.x + float32(column)*columnWidth
			cell := rectangle{x: x, y: y, width: columnWidth, height: rowHeight}
			if rowIndex == 0 {
				window.backend.drawRectangle(cell, tokenRaised)
			}
			window.backend.drawRectangleLines(cell, 1, withAlpha(tokenDivider, 160))
			window.backend.drawText(window.monoFont, value, point{x: x + 6*scale, y: y + 5*scale}, 12*scale, 0, tokenText)
		}
	}
	window.backend.endScissor()
}
