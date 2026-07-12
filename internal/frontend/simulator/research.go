package simulator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/chart"
	virtualtable "github.com/rosstheboss94/corthena/internal/frontend/table"
)

const maximumResearchBars = 100_000

type demoResearchRow struct {
	id        string
	timestamp time.Time
	symbol    appstate.Symbol
	open      float64
	high      float64
	low       float64
	close     float64
	volume    float64
	features  [3]appstate.ResearchValue
	target    appstate.ResearchValue
	index     uint64
}

var demoFeatureDescriptors = []appstate.ResearchFeatureDescriptor{
	{Name: "ret_5", Version: "1.0.0", Lookback: 5, Description: "five-bar close return", Fingerprint: "demo-ret5-v1"},
	{Name: "volatility_20", Version: "1.0.0", Lookback: 20, Description: "rolling return volatility", Fingerprint: "demo-vol20-v1"},
	{Name: "volume_z_30", Version: "1.0.0", Lookback: 30, Description: "rolling volume z-score", Fingerprint: "demo-volz30-v1"},
}

func (client *DemoCoordinator) buildResearchSnapshot(ctx context.Context, query appstate.ResearchQuery) (appstate.ResearchSnapshot, error) {
	preparedAt := client.baseTime().Add(time.Duration(query.Generation) * time.Millisecond)
	for _, feature := range query.SelectedFeatures {
		if featureDescriptorIndex(feature) < 0 {
			return appstate.ResearchSnapshot{}, fmt.Errorf("Research feature %q is unknown", feature)
		}
	}
	dataset, err := client.researchDataset(query)
	if err != nil {
		return appstate.ResearchSnapshot{}, err
	}
	if query.Scenario == appstate.ResearchScenarioEmpty {
		empty, emptyErr := emptyResearchPage()
		if emptyErr != nil {
			return appstate.ResearchSnapshot{}, emptyErr
		}
		return appstate.ResearchSnapshot{
			Query: query.Clone(), Rows: empty, PreparedAt: preparedAt,
		}, nil
	}
	step := 24 * time.Hour
	if query.Interval == appstate.IntervalHourly {
		step = time.Hour
	}
	visibleStart := query.TimeRange.Start.UTC()
	visibleEnd := query.TimeRange.End.UTC()
	if visibleStart.Before(dataset.Start) {
		visibleStart = dataset.Start.UTC()
	}
	if visibleEnd.After(dataset.End) {
		visibleEnd = dataset.End.UTC()
	}
	startOffset := visibleStart.Sub(dataset.Start.UTC())
	if remainder := startOffset % step; remainder != 0 {
		visibleStart = visibleStart.Add(step - remainder)
	}
	endOffset := visibleEnd.Sub(dataset.Start.UTC())
	visibleEnd = visibleEnd.Add(-(endOffset % step))
	if !visibleStart.Before(visibleEnd) {
		empty, emptyErr := emptyResearchPage()
		if emptyErr != nil {
			return appstate.ResearchSnapshot{}, emptyErr
		}
		return appstate.ResearchSnapshot{
			Query: query.Clone(), Rows: empty, PreparedAt: preparedAt,
		}, nil
	}
	start := visibleStart.Add(-30 * step)
	if start.Before(dataset.Start) {
		start = dataset.Start.UTC()
	}
	end := visibleEnd.Add(time.Duration(query.Target.HorizonBars+1) * step)
	if end.After(dataset.End) {
		end = dataset.End.UTC()
	}
	count := int(end.Sub(start)/step) + 1
	if count < 2 {
		count = 2
	}
	if count > maximumResearchBars {
		count = maximumResearchBars
		return appstate.ResearchSnapshot{}, fmt.Errorf("Research query expands to %d bars, maximum is %d", count, maximumResearchBars)
	}
	symbols := append([]appstate.Symbol(nil), query.Symbols...)
	sort.Slice(symbols, func(left int, right int) bool { return symbols[left] < symbols[right] })
	rowsBySymbol := make([][]demoResearchRow, len(symbols))
	allRows := make([]demoResearchRow, 0, count*len(symbols))
	for symbolIndex, symbol := range symbols {
		if err := ctx.Err(); err != nil {
			return appstate.ResearchSnapshot{}, err
		}
		series := buildDemoSymbolRows(client.seed, query, symbol, start, step, count, uint64(symbolIndex), uint64(len(symbols)), dataset.Start.UTC())
		visible := make([]demoResearchRow, 0, len(series))
		for _, row := range series {
			if !row.timestamp.Before(visibleStart) && !row.timestamp.After(visibleEnd) {
				visible = append(visible, row)
			}
		}
		rowsBySymbol[symbolIndex] = visible
	}
	for rowIndex := range len(rowsBySymbol[0]) {
		for symbolIndex := range symbols {
			row := rowsBySymbol[symbolIndex][rowIndex]
			allRows = append(allRows, row)
		}
	}
	primary := rowsBySymbol[0]
	frame, bars, err := client.prepareResearchFrame(query, primary, dataset)
	if err != nil {
		return appstate.ResearchSnapshot{}, err
	}
	series := researchSeries(primary)
	target := researchTarget(primary, query.Target)
	distributions := researchDistributions(series, target)
	page, err := researchPage(query, allRows)
	if err != nil {
		return appstate.ResearchSnapshot{}, err
	}
	return appstate.ResearchSnapshot{
		Query: query.Clone(), Frame: frame, Bars: bars, Features: series, Target: target,
		Distributions: distributions, Rows: page, PreparedAt: preparedAt,
	}, nil
}

func (client *DemoCoordinator) researchDataset(query appstate.ResearchQuery) (appstate.DatasetSummary, error) {
	client.mu.RLock()
	datasets := make([]appstate.DatasetSummary, len(client.snapshot.Datasets))
	for index, dataset := range client.snapshot.Datasets {
		datasets[index] = dataset.Clone()
	}
	client.mu.RUnlock()
	for _, dataset := range datasets {
		if dataset.ID != query.DatasetID {
			continue
		}
		if dataset.Interval != query.Interval {
			return appstate.DatasetSummary{}, fmt.Errorf("Research interval %q does not match dataset interval %q", query.Interval, dataset.Interval)
		}
		available := make(map[appstate.Symbol]struct{}, len(dataset.Symbols))
		for _, symbol := range dataset.Symbols {
			available[symbol] = struct{}{}
		}
		for _, symbol := range query.Symbols {
			if _, found := available[symbol]; !found {
				return appstate.DatasetSummary{}, fmt.Errorf("Research symbol %q is not in dataset %q", symbol, dataset.ID)
			}
		}
		return dataset.Clone(), nil
	}
	return appstate.DatasetSummary{}, fmt.Errorf("Research dataset %q is unknown", query.DatasetID)
}

func buildDemoSymbolRows(seed uint64, query appstate.ResearchQuery, symbol appstate.Symbol, start time.Time, step time.Duration, count int, symbolIndex uint64, symbolCount uint64, datasetStart time.Time) []demoResearchRow {
	rows := make([]demoResearchRow, count)
	base := 70 + float64(stableTextHash(string(query.DatasetID)+"|"+string(symbol))%180)
	stepSeconds := int64(step / time.Second)
	for index := range count {
		timestamp := start.Add(time.Duration(index) * step)
		absolute := timestamp.Unix() / stepSeconds
		ordinal := timestamp.Sub(datasetStart) / step
		phase := float64(ordinal) + float64(symbolIndex)*17
		trend := float64(ordinal) * 0.002
		noise := stableUnit(seed, uint64(absolute), symbolIndex+1) - 0.5
		closeValue := base + trend + 5*math.Sin(phase*0.043) + 2.2*math.Sin(phase*0.013) + noise*1.4
		openValue := base + trend + 5*math.Sin((phase-0.65)*0.043) + 2.2*math.Sin((phase-0.65)*0.013) + (stableUnit(seed, uint64(absolute), symbolIndex+11)-0.5)*1.2
		rangeValue := 0.4 + stableUnit(seed, uint64(absolute), symbolIndex+21)*1.8
		high := math.Max(openValue, closeValue) + rangeValue
		low := math.Min(openValue, closeValue) - rangeValue*(0.7+stableUnit(seed, uint64(absolute), symbolIndex+31)*0.4)
		volume := 650_000 + stableUnit(seed, uint64(absolute), symbolIndex+41)*4_200_000 + math.Abs(math.Sin(phase*0.09))*900_000
		rows[index] = demoResearchRow{
			id:        fmt.Sprintf("%s|%s|%d", query.DatasetID, symbol, timestamp.UnixNano()),
			timestamp: timestamp.UTC(), symbol: symbol, open: openValue, high: high, low: low, close: closeValue, volume: volume,
			index: uint64(timestamp.Sub(datasetStart)/step)*symbolCount + symbolIndex + 1,
		}
	}
	for index := range rows {
		for featureIndex, descriptor := range demoFeatureDescriptors {
			rows[index].features[featureIndex] = computeDemoFeature(rows, index, featureIndex, descriptor)
		}
		horizon := query.Target.HorizonBars
		rows[index].target = appstate.ResearchValue{Timestamp: rows[index].timestamp, Missing: index+1+horizon >= len(rows)}
		if !rows[index].target.Missing {
			startOpen := rows[index+1].open
			endOpen := rows[index+1+horizon].open
			rows[index].target.TargetTimestamp = rows[index+1+horizon].timestamp
			value := endOpen/startOpen - 1
			if query.Target.LogReturn {
				value = math.Log(endOpen / startOpen)
			}
			rows[index].target.Value = value
		}
	}
	return rows
}

func computeDemoFeature(rows []demoResearchRow, index int, featureIndex int, descriptor appstate.ResearchFeatureDescriptor) appstate.ResearchValue {
	value := appstate.ResearchValue{Timestamp: rows[index].timestamp, Missing: index < descriptor.Lookback}
	if value.Missing {
		return value
	}
	switch featureIndex {
	case 0:
		value.Value = rows[index].close/rows[index-descriptor.Lookback].close - 1
	case 1:
		returns := make([]float64, 0, descriptor.Lookback)
		for cursor := index - descriptor.Lookback + 1; cursor <= index; cursor++ {
			returns = append(returns, rows[cursor].close/rows[cursor-1].close-1)
		}
		value.Value = standardDeviation(returns)
	case 2:
		window := make([]float64, 0, descriptor.Lookback)
		for cursor := index - descriptor.Lookback; cursor < index; cursor++ {
			window = append(window, rows[cursor].volume)
		}
		mean, deviation := meanDeviation(window)
		if deviation == 0 {
			value.Value = 0
		} else {
			value.Value = (rows[index].volume - mean) / deviation
		}
	}
	return value
}

func (client *DemoCoordinator) prepareResearchFrame(query appstate.ResearchQuery, rows []demoResearchRow, dataset appstate.DatasetSummary) (chart.Frame, []appstate.ResearchBar, error) {
	if len(rows) < 2 {
		return chart.Frame{}, nil, nil
	}
	key := chart.Query{
		SeriesKey: strings.Join([]string{string(query.DatasetID), string(rows[0].symbol), string(query.Interval), string(query.SelectedFeatures[0]), strconv.Itoa(query.Target.HorizonBars), strconv.FormatBool(query.Target.LogReturn)}, "|"),
		MinimumX:  float64(rows[0].timestamp.Unix()), MaximumX: float64(rows[len(rows)-1].timestamp.Unix()), Resolution: query.Resolution,
	}
	if frame, found := client.researchCache.Get(key); found {
		return frame, researchBars(rows), nil
	}
	candles := make([]chart.Candle, len(rows))
	featureSamples := make([]chart.Sample, 0, len(rows))
	targetSamples := make([]chart.Sample, 0, len(rows))
	minimumY := math.Inf(1)
	maximumY := math.Inf(-1)
	selectedIndex := featureDescriptorIndex(query.SelectedFeatures[0])
	for index, row := range rows {
		x := float64(row.timestamp.Unix())
		candles[index] = chart.Candle{X: x, Open: row.open, High: row.high, Low: row.low, Close: row.close, Volume: row.volume, SourceIndex: uint64(index + 1)}
		minimumY = math.Min(minimumY, row.low)
		maximumY = math.Max(maximumY, row.high)
		if selectedIndex >= 0 && !row.features[selectedIndex].Missing {
			projected := row.close * (1 + math.Max(-4, math.Min(4, row.features[selectedIndex].Value))*0.006)
			featureSamples = append(featureSamples, chart.Sample{X: x, Y: projected, SourceIndex: uint64(index + 1)})
		}
		if !row.target.Missing {
			targetSamples = append(targetSamples, chart.Sample{X: x, Y: row.close * (1 + row.target.Value), SourceIndex: uint64(index + 1)})
		}
	}
	padding := math.Max(1, (maximumY-minimumY)*0.06)
	data := chart.Rect{MinX: key.MinimumX, MinY: minimumY - padding, MaxX: key.MaximumX, MaxY: maximumY + padding}
	viewport := chart.Rect{MinX: 0, MinY: 0, MaxX: float64(query.Resolution), MaxY: 600}
	layers := []chart.Layer{chart.OHLCVLayer{ID: "ohlcv", Candles: candles}}
	if len(featureSamples) > 1 {
		layers = append(layers, chart.LineLayer{ID: "feature", Samples: featureSamples, Style: chart.StyleWarning})
	}
	if len(targetSamples) > 0 {
		layers = append(layers, chart.ScatterLayer{ID: "target", Samples: targetSamples, Style: chart.StyleSecondary})
	}
	datasetMinimum := float64(dataset.Start.Unix())
	datasetMaximum := float64(dataset.End.Unix())
	span := datasetMaximum - datasetMinimum
	layers = append(layers, chart.RegionLayer{ID: "splits", Regions: []chart.Region{
		{MinimumX: datasetMinimum, MaximumX: datasetMinimum + span*0.60, Kind: chart.RegionTrain},
		{MinimumX: datasetMinimum + span*0.60, MaximumX: datasetMinimum + span*0.80, Kind: chart.RegionValidation},
		{MinimumX: datasetMinimum + span*0.80, MaximumX: datasetMaximum, Kind: chart.RegionTest},
	}})
	frame, err := chart.Prepare(data, viewport, query.Resolution, layers)
	if err != nil {
		return chart.Frame{}, nil, err
	}
	if err := client.researchCache.Put(key, frame); err != nil {
		return chart.Frame{}, nil, err
	}
	return frame, researchBars(rows), nil
}

func researchBars(rows []demoResearchRow) []appstate.ResearchBar {
	result := make([]appstate.ResearchBar, len(rows))
	for index, row := range rows {
		result[index] = appstate.ResearchBar{RowID: row.id, Timestamp: row.timestamp, Symbol: row.symbol, Open: row.open, High: row.high, Low: row.low, Close: row.close, Volume: row.volume}
	}
	return result
}

func researchSeries(rows []demoResearchRow) []appstate.ResearchSeries {
	result := make([]appstate.ResearchSeries, len(demoFeatureDescriptors))
	for featureIndex, descriptor := range demoFeatureDescriptors {
		series := appstate.ResearchSeries{Descriptor: descriptor, Values: make([]appstate.ResearchValue, len(rows)), Minimum: math.Inf(1), Maximum: math.Inf(-1)}
		for rowIndex, row := range rows {
			value := row.features[featureIndex]
			series.Values[rowIndex] = value
			if value.Missing {
				series.Missing++
				continue
			}
			series.Minimum = math.Min(series.Minimum, value.Value)
			series.Maximum = math.Max(series.Maximum, value.Value)
		}
		if math.IsInf(series.Minimum, 0) {
			series.Minimum, series.Maximum = 0, 0
		}
		result[featureIndex] = series
	}
	return result
}

func researchTarget(rows []demoResearchRow, spec appstate.TargetSpec) appstate.ResearchTargetPreview {
	target := appstate.ResearchTargetPreview{Spec: spec, Values: make([]appstate.ResearchValue, len(rows))}
	for index, row := range rows {
		target.Values[index] = row.target
		if row.target.Missing {
			target.ExcludedRows++
		} else {
			target.ValidRows++
		}
	}
	return target
}

func researchDistributions(series []appstate.ResearchSeries, target appstate.ResearchTargetPreview) []appstate.ResearchDistribution {
	result := make([]appstate.ResearchDistribution, 0, len(series)+1)
	for _, item := range series {
		result = append(result, appstate.ResearchDistribution{Name: item.Descriptor.Name, Bins: histogram(item.Values, 16)})
	}
	result = append(result, appstate.ResearchDistribution{Name: "target", Bins: histogram(target.Values, 16)})
	return result
}

func histogram(values []appstate.ResearchValue, count int) []appstate.ResearchBin {
	minimum, maximum := math.Inf(1), math.Inf(-1)
	for _, value := range values {
		if !value.Missing {
			minimum, maximum = math.Min(minimum, value.Value), math.Max(maximum, value.Value)
		}
	}
	if math.IsInf(minimum, 0) {
		return nil
	}
	if maximum <= minimum {
		maximum = minimum + 1
	}
	bins := make([]appstate.ResearchBin, count)
	width := (maximum - minimum) / float64(count)
	for index := range bins {
		bins[index] = appstate.ResearchBin{Minimum: minimum + float64(index)*width, Maximum: minimum + float64(index+1)*width}
	}
	for _, value := range values {
		if value.Missing {
			continue
		}
		index := int((value.Value - minimum) / (maximum - minimum) * float64(count))
		index = min(max(index, 0), count-1)
		bins[index].Count++
	}
	return bins
}

func researchPage(query appstate.ResearchQuery, rows []demoResearchRow) (appstate.ResearchPage, error) {
	filtered := make([]demoResearchRow, 0, len(rows))
	for _, row := range rows {
		if row.target.Missing || (query.Filter != "" && !strings.Contains(string(row.symbol), query.Filter)) {
			continue
		}
		filtered = append(filtered, row)
	}
	switch query.Sort {
	case appstate.ResearchSortTimeDescending:
		sort.SliceStable(filtered, func(left int, right int) bool {
			if filtered[left].timestamp.Equal(filtered[right].timestamp) {
				return filtered[left].index < filtered[right].index
			}
			return filtered[left].timestamp.After(filtered[right].timestamp)
		})
	case appstate.ResearchSortTargetDescending:
		sort.SliceStable(filtered, func(left int, right int) bool {
			if filtered[left].target.Value == filtered[right].target.Value {
				return filtered[left].index < filtered[right].index
			}
			return filtered[left].target.Value > filtered[right].target.Value
		})
	}
	offset := 0
	if query.Cursor != "" {
		parsed, _ := strconv.Atoi(query.Cursor)
		offset = min(parsed, len(filtered))
	}
	end := min(offset+query.PageSize, len(filtered))
	page := researchTable(filtered[offset:end])
	model, err := virtualtable.NewModel(page)
	if err != nil {
		return appstate.ResearchPage{}, err
	}
	next := ""
	if end < len(filtered) {
		next = strconv.Itoa(end)
	}
	return appstate.ResearchPage{Dataset: page, Model: model, NextCursor: next, TotalRows: uint64(len(filtered))}, nil
}

func researchTable(rows []demoResearchRow) virtualtable.Dataset {
	columns := []virtualtable.Column{
		{ID: "row_id", Title: "Row ID", Kind: virtualtable.CellString, Width: 210, MinWidth: 100, MaxWidth: 520, Pinned: true, Sortable: true},
		{ID: "timestamp", Title: "Timestamp (UTC)", Kind: virtualtable.CellTime, Width: 170, MinWidth: 120, MaxWidth: 300, Sortable: true},
		{ID: "symbol", Title: "Symbol", Kind: virtualtable.CellString, Width: 80, MinWidth: 60, MaxWidth: 160, Sortable: true},
		{ID: "open", Title: "Open", Kind: virtualtable.CellFloat, Width: 88, MinWidth: 64, MaxWidth: 180, Sortable: true},
		{ID: "high", Title: "High", Kind: virtualtable.CellFloat, Width: 88, MinWidth: 64, MaxWidth: 180, Sortable: true},
		{ID: "low", Title: "Low", Kind: virtualtable.CellFloat, Width: 88, MinWidth: 64, MaxWidth: 180, Sortable: true},
		{ID: "close", Title: "Close", Kind: virtualtable.CellFloat, Width: 88, MinWidth: 64, MaxWidth: 180, Sortable: true},
		{ID: "volume", Title: "Volume", Kind: virtualtable.CellFloat, Width: 110, MinWidth: 80, MaxWidth: 200, Sortable: true},
	}
	for _, descriptor := range demoFeatureDescriptors {
		columns = append(columns, virtualtable.Column{ID: virtualtable.ColumnID(descriptor.Name), Title: string(descriptor.Name), Kind: virtualtable.CellFloat, Width: 112, MinWidth: 80, MaxWidth: 220, Sortable: true})
	}
	columns = append(columns, virtualtable.Column{ID: "target", Title: "Forward target", Kind: virtualtable.CellFloat, Width: 124, MinWidth: 90, MaxWidth: 240, Sortable: true})
	result := virtualtable.Dataset{Columns: columns, Rows: make([]virtualtable.Row, 0, len(rows))}
	for _, row := range rows {
		if row.target.Missing {
			continue
		}
		cells := []virtualtable.Cell{
			{Kind: virtualtable.CellString, String: row.id}, {Kind: virtualtable.CellTime, Time: row.timestamp},
			{Kind: virtualtable.CellString, String: string(row.symbol)}, {Kind: virtualtable.CellFloat, Float: row.open},
			{Kind: virtualtable.CellFloat, Float: row.high}, {Kind: virtualtable.CellFloat, Float: row.low},
			{Kind: virtualtable.CellFloat, Float: row.close}, {Kind: virtualtable.CellFloat, Float: row.volume},
		}
		for _, value := range row.features {
			cells = append(cells, virtualtable.Cell{Kind: virtualtable.CellFloat, Float: value.Value, Null: value.Missing})
		}
		cells = append(cells, virtualtable.Cell{Kind: virtualtable.CellFloat, Float: row.target.Value})
		result.Rows = append(result.Rows, virtualtable.Row{ID: virtualtable.RowID(row.id), Cells: cells, SourceIndex: row.index})
	}
	return result
}

func emptyResearchTable() virtualtable.Dataset { return researchTable(nil) }

func emptyResearchPage() (appstate.ResearchPage, error) {
	dataset := emptyResearchTable()
	model, err := virtualtable.NewModel(dataset)
	if err != nil {
		return appstate.ResearchPage{}, err
	}
	return appstate.ResearchPage{Dataset: dataset, Model: model}, nil
}

func featureDescriptorIndex(name appstate.FeatureName) int {
	for index, descriptor := range demoFeatureDescriptors {
		if descriptor.Name == name {
			return index
		}
	}
	return -1
}

func meanDeviation(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	return mean, math.Sqrt(variance / float64(len(values)))
}

func standardDeviation(values []float64) float64 {
	_, deviation := meanDeviation(values)
	return deviation
}

func stableTextHash(value string) uint64 {
	const offset = uint64(1469598103934665603)
	const prime = uint64(1099511628211)
	hash := offset
	for index := range len(value) {
		hash ^= uint64(value[index])
		hash *= prime
	}
	return hash
}

func stableUnit(seed uint64, index uint64, stream uint64) float64 {
	value := seed ^ (index * 0x9E3779B97F4A7C15) ^ (stream * 0xBF58476D1CE4E5B9)
	value ^= value >> 30
	value *= 0xBF58476D1CE4E5B9
	value ^= value >> 27
	value *= 0x94D049BB133111EB
	value ^= value >> 31
	return float64(value>>11) / float64(uint64(1)<<53)
}
