package chart

import (
	"fmt"
)

// LayerKind identifies one closed chart layer variant.
type LayerKind string

const (
	LayerOHLCV             LayerKind = "ohlcv"
	LayerLine              LayerKind = "line"
	LayerArea              LayerKind = "area"
	LayerHistogram         LayerKind = "histogram"
	LayerScatter           LayerKind = "scatter"
	LayerEquity            LayerKind = "equity"
	LayerDrawdown          LayerKind = "drawdown"
	LayerHeatmap           LayerKind = "heatmap"
	LayerFeatureImportance LayerKind = "feature_importance"
	LayerPredictions       LayerKind = "predictions"
	LayerTrades            LayerKind = "trades"
	LayerRegions           LayerKind = "regions"
)

// StyleRole is a semantic palette role resolved by the native renderer.
type StyleRole uint8

const (
	StylePrimary StyleRole = iota + 1
	StyleSecondary
	StylePositive
	StyleNegative
	StyleWarning
	StyleMuted
	StyleTrain
	StyleValidation
	StyleTest
)

// Layer is a closed immutable input variant.
type Layer interface {
	isLayer()
}

// OHLCVLayer renders candles and their aggregated volume.
type OHLCVLayer struct {
	ID      string
	Candles []Candle
}

func (OHLCVLayer) isLayer() {}

// LineLayer renders a continuous series.
type LineLayer struct {
	ID      string
	Samples []Sample
	Style   StyleRole
}

func (LineLayer) isLayer() {}

// AreaLayer renders a continuous series filled to Baseline.
type AreaLayer struct {
	ID       string
	Samples  []Sample
	Baseline float64
	Style    StyleRole
}

func (AreaLayer) isLayer() {}

// HistogramLayer renders vertical bins from Baseline.
type HistogramLayer struct {
	ID       string
	Samples  []Sample
	Baseline float64
	Style    StyleRole
}

func (HistogramLayer) isLayer() {}

// ScatterLayer renders point markers.
type ScatterLayer struct {
	ID      string
	Samples []Sample
	Style   StyleRole
}

func (ScatterLayer) isLayer() {}

// EquityLayer renders an equity curve.
type EquityLayer struct {
	ID      string
	Samples []Sample
}

func (EquityLayer) isLayer() {}

// DrawdownLayer renders a drawdown curve and zero baseline fill.
type DrawdownLayer struct {
	ID      string
	Samples []Sample
}

func (DrawdownLayer) isLayer() {}

// HeatCell is one rectangular heatmap value in data coordinates.
type HeatCell struct {
	Bounds Rect
	Value  float64
}

// HeatmapLayer renders rectangular heat cells. Minimum and Maximum define the
// fixed color normalization domain.
type HeatmapLayer struct {
	ID      string
	Cells   []HeatCell
	Minimum float64
	Maximum float64
}

func (HeatmapLayer) isLayer() {}

// ImportanceValue is one named feature score. Row is a stable vertical index.
type ImportanceValue struct {
	Name  string
	Value float64
	Row   int
}

// FeatureImportanceLayer renders horizontal feature bars.
type FeatureImportanceLayer struct {
	ID     string
	Values []ImportanceValue
	Style  StyleRole
}

func (FeatureImportanceLayer) isLayer() {}

// PredictionLayer renders model predictions as a continuous series.
type PredictionLayer struct {
	ID      string
	Samples []Sample
}

func (PredictionLayer) isLayer() {}

// TradeSide identifies a portfolio trade marker.
type TradeSide uint8

const (
	TradeBuy TradeSide = iota + 1
	TradeSell
)

// Trade is one stable portfolio execution marker.
type Trade struct {
	X           float64
	Y           float64
	Side        TradeSide
	SourceIndex uint64
}

// TradeLayer renders portfolio trade markers.
type TradeLayer struct {
	ID     string
	Trades []Trade
}

func (TradeLayer) isLayer() {}

// RegionKind identifies one leakage-safe dataset partition overlay.
type RegionKind uint8

const (
	RegionTrain RegionKind = iota + 1
	RegionValidation
	RegionTest
)

// Region is one train, validation, or test horizontal interval.
type Region struct {
	MinimumX float64
	MaximumX float64
	Kind     RegionKind
}

// RegionLayer renders train/validation/test backgrounds.
type RegionLayer struct {
	ID      string
	Regions []Region
}

func (RegionLayer) isLayer() {}

// Rect32 is a positive-area render rectangle.
type Rect32 struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
	Style  StyleRole
	Value  float32
}

// Segment32 is a clipped render line.
type Segment32 struct {
	Start Point32
	End   Point32
	Style StyleRole
}

// MarkerShape is a native-adapter-independent marker shape.
type MarkerShape uint8

const (
	MarkerCircle MarkerShape = iota + 1
	MarkerTriangleUp
	MarkerTriangleDown
)

// Marker32 is one render-ready marker.
type Marker32 struct {
	Center Point32
	Size   float32
	Shape  MarkerShape
	Style  StyleRole
}

// Polygon32 is one filled render polygon.
type Polygon32 struct {
	Points []Point32
	Style  StyleRole
}

// Label32 is one render label anchored in screen coordinates.
type Label32 struct {
	Position Point32
	Text     string
	Style    StyleRole
}

// PreparedLayer is an immutable render buffer for one source layer.
type PreparedLayer struct {
	ID       string
	Kind     LayerKind
	Rects    []Rect32
	Segments []Segment32
	Markers  []Marker32
	Polygons []Polygon32
	Labels   []Label32
}

// Clone returns a deep immutable copy.
func (layer PreparedLayer) Clone() PreparedLayer {
	layer.Rects = append([]Rect32(nil), layer.Rects...)
	layer.Segments = append([]Segment32(nil), layer.Segments...)
	layer.Markers = append([]Marker32(nil), layer.Markers...)
	layer.Labels = append([]Label32(nil), layer.Labels...)
	if len(layer.Polygons) > 0 {
		layer.Polygons = make([]Polygon32, len(layer.Polygons))
		for index, polygon := range layer.Polygons {
			layer.Polygons[index] = Polygon32{Points: append([]Point32(nil), polygon.Points...), Style: polygon.Style}
		}
	}
	return layer
}

// Frame is a complete immutable render-ready chart snapshot.
type Frame struct {
	Generation uint64
	Viewport   Rect
	Data       Rect
	Layers     []PreparedLayer
	WorkCount  int
}

// Clone returns a deep immutable copy safe for publication.
func (frame Frame) Clone() Frame {
	source := frame.Layers
	frame.Layers = make([]PreparedLayer, len(source))
	for index, layer := range source {
		frame.Layers[index] = layer.Clone()
	}
	return frame
}

// ByteSize returns deterministic owned-buffer accounting for a frame. It
// includes every owned slice element and label byte, excluding Go allocator
// metadata.
func (frame Frame) ByteSize() uint64 {
	const (
		rectBytes    = uint64(24)
		segmentBytes = uint64(20)
		markerBytes  = uint64(20)
		pointBytes   = uint64(8)
		labelBytes   = uint64(16)
	)
	bytes := uint64(64)
	for _, layer := range frame.Layers {
		bytes += uint64(len(layer.ID)) + 64
		bytes += uint64(len(layer.Rects))*rectBytes + uint64(len(layer.Segments))*segmentBytes + uint64(len(layer.Markers))*markerBytes
		for _, polygon := range layer.Polygons {
			bytes += 16 + uint64(len(polygon.Points))*pointBytes
		}
		for _, label := range layer.Labels {
			bytes += labelBytes + uint64(len(label.Text))
		}
	}
	return bytes
}

// Prepare builds render buffers. Dense continuous and candle inputs are first
// reduced to at most four values or one candle per horizontal pixel.
func Prepare(data Rect, viewport Rect, pixelWidth int, layers []Layer) (Frame, error) {
	transform, err := NewTransform(data, viewport)
	if err != nil {
		return Frame{}, err
	}
	if pixelWidth <= 0 {
		return Frame{}, fmt.Errorf("%w: pixel width must be positive", ErrInvalidGeometry)
	}
	frame := Frame{Viewport: viewport, Data: data, Layers: make([]PreparedLayer, 0, len(layers))}
	for index, source := range layers {
		var prepared PreparedLayer
		var work int
		switch layer := source.(type) {
		case OHLCVLayer:
			prepared, work, err = prepareOHLCV(transform, pixelWidth, layer)
		case LineLayer:
			prepared, work, err = prepareContinuous(transform, pixelWidth, layer.ID, LayerLine, layer.Samples, layer.Style, 0, false)
		case AreaLayer:
			prepared, work, err = prepareContinuous(transform, pixelWidth, layer.ID, LayerArea, layer.Samples, layer.Style, layer.Baseline, true)
		case HistogramLayer:
			prepared, work, err = prepareHistogram(transform, pixelWidth, layer)
		case ScatterLayer:
			prepared, work, err = prepareScatter(transform, pixelWidth, layer)
		case EquityLayer:
			prepared, work, err = prepareContinuous(transform, pixelWidth, layer.ID, LayerEquity, layer.Samples, StylePositive, 0, false)
		case DrawdownLayer:
			prepared, work, err = prepareContinuous(transform, pixelWidth, layer.ID, LayerDrawdown, layer.Samples, StyleNegative, 0, true)
		case HeatmapLayer:
			prepared, work, err = prepareHeatmap(transform, layer)
		case FeatureImportanceLayer:
			prepared, work, err = prepareImportance(transform, layer)
		case PredictionLayer:
			prepared, work, err = prepareContinuous(transform, pixelWidth, layer.ID, LayerPredictions, layer.Samples, StyleSecondary, 0, false)
		case TradeLayer:
			prepared, work, err = prepareTrades(transform, layer)
		case RegionLayer:
			prepared, work, err = prepareRegions(transform, layer)
		default:
			return Frame{}, fmt.Errorf("%w: unsupported layer variant at index %d", ErrInvalidData, index)
		}
		if err != nil {
			return Frame{}, fmt.Errorf("prepare chart layer %d: %w", index, err)
		}
		frame.Layers = append(frame.Layers, prepared)
		frame.WorkCount += work
	}
	return frame, nil
}

func prepareOHLCV(transform Transform, width int, layer OHLCVLayer) (PreparedLayer, int, error) {
	buckets, stats, err := AggregateCandles(layer.Candles, transform.data.MinX, transform.data.MaxX, width)
	prepared := PreparedLayer{ID: layer.ID, Kind: LayerOHLCV}
	if err != nil {
		return prepared, 0, err
	}
	maxVolume := 0.0
	for _, bucket := range buckets {
		maxVolume = max(maxVolume, bucket.Volume)
	}
	candleWidth := max(1.0, transform.screen.Width()/float64(width)*0.72)
	for _, bucket := range buckets {
		high, err := transform.Forward(Point{X: bucket.X, Y: bucket.High})
		if err != nil {
			return prepared, 0, err
		}
		low, err := transform.Forward(Point{X: bucket.X, Y: bucket.Low})
		if err != nil {
			return prepared, 0, err
		}
		start, end, visible, err := ClipSegment(transform.screen, high, low)
		if err != nil {
			return prepared, 0, err
		}
		style := StylePositive
		if bucket.Close < bucket.Open {
			style = StyleNegative
		}
		if visible {
			prepared.Segments = append(prepared.Segments, mustSegment(start, end, style))
		}
		openPoint, _ := transform.Forward(Point{X: bucket.X, Y: bucket.Open})
		closePoint, _ := transform.Forward(Point{X: bucket.X, Y: bucket.Close})
		bodyTop := min(openPoint.Y, closePoint.Y)
		bodyBottom := max(openPoint.Y, closePoint.Y)
		if bodyBottom-bodyTop < 1 {
			bodyBottom = bodyTop + 1
		}
		appendClippedRect(&prepared, transform.screen, Rect{MinX: openPoint.X - candleWidth/2, MinY: bodyTop, MaxX: openPoint.X + candleWidth/2, MaxY: bodyBottom}, style, 0)
		if maxVolume > 0 {
			volumeHeight := transform.screen.Height() * 0.18 * bucket.Volume / maxVolume
			appendClippedRect(&prepared, transform.screen, Rect{
				MinX: openPoint.X - candleWidth/2, MinY: transform.screen.MaxY - volumeHeight,
				MaxX: openPoint.X + candleWidth/2, MaxY: transform.screen.MaxY,
			}, StyleMuted, 0)
		}
	}
	return prepared, stats.OutputValues * 3, nil
}

func prepareContinuous(transform Transform, width int, id string, kind LayerKind, samples []Sample, style StyleRole, baseline float64, area bool) (PreparedLayer, int, error) {
	values, stats, err := AggregateContinuous(samples, transform.data.MinX, transform.data.MaxX, width)
	prepared := PreparedLayer{ID: id, Kind: kind}
	if err != nil {
		return prepared, 0, err
	}
	if style == 0 {
		style = StylePrimary
	}
	points := make([]Point, 0, len(values))
	for _, sample := range values {
		point, forwardErr := transform.Forward(Point{X: sample.X, Y: sample.Y})
		if forwardErr != nil {
			return prepared, 0, forwardErr
		}
		points = append(points, point)
	}
	for index := 0; index+1 < len(points); index++ {
		start, end, visible, clipErr := ClipSegment(transform.screen, points[index], points[index+1])
		if clipErr != nil {
			return prepared, 0, clipErr
		}
		if visible {
			prepared.Segments = append(prepared.Segments, mustSegment(start, end, style))
		}
	}
	if area && len(points) >= 2 && finite(baseline) {
		baselinePoint, forwardErr := transform.Forward(Point{X: transform.data.MinX, Y: baseline})
		if forwardErr != nil {
			return prepared, 0, forwardErr
		}
		clampedY := min(max(baselinePoint.Y, transform.screen.MinY), transform.screen.MaxY)
		polygon := Polygon32{Style: style, Points: make([]Point32, 0, len(points)+2)}
		firstX := min(max(points[0].X, transform.screen.MinX), transform.screen.MaxX)
		polygon.Points = append(polygon.Points, mustPoint32(Point{X: firstX, Y: clampedY}))
		for _, point := range points {
			if point.X >= transform.screen.MinX && point.X <= transform.screen.MaxX {
				polygon.Points = append(polygon.Points, mustPoint32(Point{X: point.X, Y: min(max(point.Y, transform.screen.MinY), transform.screen.MaxY)}))
			}
		}
		lastX := min(max(points[len(points)-1].X, transform.screen.MinX), transform.screen.MaxX)
		polygon.Points = append(polygon.Points, mustPoint32(Point{X: lastX, Y: clampedY}))
		if len(polygon.Points) >= 4 {
			prepared.Polygons = append(prepared.Polygons, polygon)
		}
	}
	return prepared, stats.OutputValues + len(prepared.Segments), nil
}

func prepareHistogram(transform Transform, width int, layer HistogramLayer) (PreparedLayer, int, error) {
	values, stats, err := AggregateContinuous(layer.Samples, transform.data.MinX, transform.data.MaxX, width)
	prepared := PreparedLayer{ID: layer.ID, Kind: LayerHistogram}
	if err != nil {
		return prepared, 0, err
	}
	style := layer.Style
	if style == 0 {
		style = StylePrimary
	}
	barWidth := max(1.0, transform.screen.Width()/float64(width)*0.8)
	for _, sample := range values {
		valuePoint, _ := transform.Forward(Point{X: sample.X, Y: sample.Y})
		basePoint, _ := transform.Forward(Point{X: sample.X, Y: layer.Baseline})
		appendClippedRect(&prepared, transform.screen, Rect{
			MinX: valuePoint.X - barWidth/2, MinY: min(valuePoint.Y, basePoint.Y),
			MaxX: valuePoint.X + barWidth/2, MaxY: max(valuePoint.Y, basePoint.Y),
		}, style, 0)
	}
	return prepared, stats.OutputValues, nil
}

func prepareScatter(transform Transform, width int, layer ScatterLayer) (PreparedLayer, int, error) {
	values, stats, err := AggregateContinuous(layer.Samples, transform.data.MinX, transform.data.MaxX, width)
	prepared := PreparedLayer{ID: layer.ID, Kind: LayerScatter}
	if err != nil {
		return prepared, 0, err
	}
	style := layer.Style
	if style == 0 {
		style = StylePrimary
	}
	for _, sample := range values {
		point, _ := transform.Forward(Point{X: sample.X, Y: sample.Y})
		if transform.screen.Contains(point) {
			prepared.Markers = append(prepared.Markers, Marker32{Center: mustPoint32(point), Size: 3, Shape: MarkerCircle, Style: style})
		}
	}
	return prepared, stats.OutputValues, nil
}

func prepareHeatmap(transform Transform, layer HeatmapLayer) (PreparedLayer, int, error) {
	prepared := PreparedLayer{ID: layer.ID, Kind: LayerHeatmap}
	if !finite(layer.Minimum) || !finite(layer.Maximum) || layer.Maximum <= layer.Minimum {
		return prepared, 0, fmt.Errorf("%w: invalid heatmap color range", ErrInvalidData)
	}
	for index, cell := range layer.Cells {
		if !cell.Bounds.Valid() || !finite(cell.Value) {
			return prepared, 0, fmt.Errorf("%w: invalid heatmap cell %d", ErrInvalidData, index)
		}
		topLeft, _ := transform.Forward(Point{X: cell.Bounds.MinX, Y: cell.Bounds.MaxY})
		bottomRight, _ := transform.Forward(Point{X: cell.Bounds.MaxX, Y: cell.Bounds.MinY})
		normalized := (cell.Value - layer.Minimum) / (layer.Maximum - layer.Minimum)
		appendClippedRect(&prepared, transform.screen, Rect{MinX: topLeft.X, MinY: topLeft.Y, MaxX: bottomRight.X, MaxY: bottomRight.Y}, StylePrimary, min(max(normalized, 0), 1))
	}
	return prepared, len(prepared.Rects), nil
}

func prepareImportance(transform Transform, layer FeatureImportanceLayer) (PreparedLayer, int, error) {
	prepared := PreparedLayer{ID: layer.ID, Kind: LayerFeatureImportance}
	style := layer.Style
	if style == 0 {
		style = StylePrimary
	}
	for index, value := range layer.Values {
		if value.Name == "" || !finite(value.Value) || value.Row < 0 {
			return prepared, 0, fmt.Errorf("%w: invalid importance value %d", ErrInvalidData, index)
		}
		start, _ := transform.Forward(Point{X: 0, Y: float64(value.Row)})
		end, _ := transform.Forward(Point{X: value.Value, Y: float64(value.Row + 1)})
		appendClippedRect(&prepared, transform.screen, Rect{MinX: min(start.X, end.X), MinY: min(start.Y, end.Y), MaxX: max(start.X, end.X), MaxY: max(start.Y, end.Y)}, style, 0)
		if transform.screen.Contains(Point{X: transform.screen.MinX + 2, Y: (start.Y + end.Y) / 2}) {
			prepared.Labels = append(prepared.Labels, Label32{Position: mustPoint32(Point{X: transform.screen.MinX + 2, Y: (start.Y + end.Y) / 2}), Text: value.Name, Style: StyleMuted})
		}
	}
	return prepared, len(prepared.Rects) + len(prepared.Labels), nil
}

func prepareTrades(transform Transform, layer TradeLayer) (PreparedLayer, int, error) {
	prepared := PreparedLayer{ID: layer.ID, Kind: LayerTrades}
	var previous uint64
	for index, trade := range layer.Trades {
		if !finite(trade.X) || !finite(trade.Y) || (trade.Side != TradeBuy && trade.Side != TradeSell) || (index > 0 && trade.SourceIndex <= previous) {
			return prepared, 0, fmt.Errorf("%w: invalid trade %d", ErrInvalidData, index)
		}
		previous = trade.SourceIndex
		point, _ := transform.Forward(Point{X: trade.X, Y: trade.Y})
		if !transform.screen.Contains(point) {
			continue
		}
		marker := Marker32{Center: mustPoint32(point), Size: 5, Shape: MarkerTriangleUp, Style: StylePositive}
		if trade.Side == TradeSell {
			marker.Shape = MarkerTriangleDown
			marker.Style = StyleNegative
		}
		prepared.Markers = append(prepared.Markers, marker)
	}
	return prepared, len(prepared.Markers), nil
}

func prepareRegions(transform Transform, layer RegionLayer) (PreparedLayer, int, error) {
	prepared := PreparedLayer{ID: layer.ID, Kind: LayerRegions}
	for index, region := range layer.Regions {
		if !finite(region.MinimumX) || !finite(region.MaximumX) || region.MaximumX <= region.MinimumX {
			return prepared, 0, fmt.Errorf("%w: invalid region %d", ErrInvalidData, index)
		}
		style := StyleTrain
		switch region.Kind {
		case RegionTrain:
			style = StyleTrain
		case RegionValidation:
			style = StyleValidation
		case RegionTest:
			style = StyleTest
		default:
			return prepared, 0, fmt.Errorf("%w: unknown region kind %d", ErrInvalidData, region.Kind)
		}
		left, _ := transform.Forward(Point{X: region.MinimumX, Y: transform.data.MinY})
		right, _ := transform.Forward(Point{X: region.MaximumX, Y: transform.data.MaxY})
		appendClippedRect(&prepared, transform.screen, Rect{MinX: min(left.X, right.X), MinY: transform.screen.MinY, MaxX: max(left.X, right.X), MaxY: transform.screen.MaxY}, style, 0)
	}
	return prepared, len(prepared.Rects), nil
}

func appendClippedRect(layer *PreparedLayer, bounds Rect, candidate Rect, style StyleRole, value float64) {
	clipped := Rect{MinX: max(bounds.MinX, candidate.MinX), MinY: max(bounds.MinY, candidate.MinY), MaxX: min(bounds.MaxX, candidate.MaxX), MaxY: min(bounds.MaxY, candidate.MaxY)}
	if !clipped.Valid() {
		return
	}
	x, errX := checkedFloat32(clipped.MinX)
	y, errY := checkedFloat32(clipped.MinY)
	width, errWidth := checkedFloat32(clipped.Width())
	height, errHeight := checkedFloat32(clipped.Height())
	value32, errValue := checkedFloat32(value)
	if errX != nil || errY != nil || errWidth != nil || errHeight != nil || errValue != nil {
		return
	}
	layer.Rects = append(layer.Rects, Rect32{X: x, Y: y, Width: width, Height: height, Style: style, Value: value32})
}

func mustSegment(start Point, end Point, style StyleRole) Segment32 {
	return Segment32{Start: mustPoint32(start), End: mustPoint32(end), Style: style}
}

func mustPoint32(point Point) Point32 {
	converted, err := ToPoint32(point)
	if err != nil {
		panic(fmt.Sprintf("validated chart coordinate conversion failed: %v", err))
	}
	return converted
}
