package chart

import (
	"fmt"
)

// Sample is one ordered continuous-series value. SourceIndex is the stable
// tie-breaker retained through aggregation.
type Sample struct {
	X           float64
	Y           float64
	SourceIndex uint64
}

// Candle is one ordered OHLCV value.
type Candle struct {
	X           float64
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      float64
	SourceIndex uint64
}

// CandleBucket preserves OHLC semantics and the source interval represented
// by one horizontal pixel bucket.
type CandleBucket struct {
	Candle
	LastSourceIndex uint64
}

// LODStats exposes deterministic operation counts for proportional-work
// assertions. OutputValues is the maximum work consumed by rendering.
type LODStats struct {
	SourceValues int
	Buckets      int
	OutputValues int
}

// AggregateContinuous retains first, last, minimum, and maximum samples per
// horizontal pixel bucket, emitted in stable source-index order.
func AggregateContinuous(samples []Sample, visibleMinX float64, visibleMaxX float64, pixelWidth int) ([]Sample, LODStats, error) {
	stats := LODStats{SourceValues: len(samples)}
	if !finite(visibleMinX) || !finite(visibleMaxX) || visibleMaxX <= visibleMinX || pixelWidth <= 0 {
		return nil, stats, fmt.Errorf("%w: invalid continuous LOD viewport", ErrInvalidGeometry)
	}
	if err := validateSamples(samples); err != nil {
		return nil, stats, err
	}
	type bucket struct {
		used    bool
		first   Sample
		last    Sample
		minimum Sample
		maximum Sample
	}
	buckets := make([]bucket, pixelWidth)
	for _, sample := range samples {
		index, visible := horizontalBucket(sample.X, visibleMinX, visibleMaxX, pixelWidth)
		if !visible {
			continue
		}
		current := &buckets[index]
		if !current.used {
			*current = bucket{used: true, first: sample, last: sample, minimum: sample, maximum: sample}
			continue
		}
		current.last = sample
		if sample.Y < current.minimum.Y || (sample.Y == current.minimum.Y && sample.SourceIndex < current.minimum.SourceIndex) {
			current.minimum = sample
		}
		if sample.Y > current.maximum.Y || (sample.Y == current.maximum.Y && sample.SourceIndex < current.maximum.SourceIndex) {
			current.maximum = sample
		}
	}
	output := make([]Sample, 0, pixelWidth*4)
	for index := range buckets {
		current := buckets[index]
		if !current.used {
			continue
		}
		stats.Buckets++
		selected, count := selectContinuousExtrema(current.first, current.last, current.minimum, current.maximum)
		output = append(output, selected[:count]...)
	}
	stats.OutputValues = len(output)
	return output, stats, nil
}

// AggregateCandles returns one OHLCV bucket per occupied horizontal pixel.
func AggregateCandles(candles []Candle, visibleMinX float64, visibleMaxX float64, pixelWidth int) ([]CandleBucket, LODStats, error) {
	stats := LODStats{SourceValues: len(candles)}
	if !finite(visibleMinX) || !finite(visibleMaxX) || visibleMaxX <= visibleMinX || pixelWidth <= 0 {
		return nil, stats, fmt.Errorf("%w: invalid candle LOD viewport", ErrInvalidGeometry)
	}
	if err := validateCandles(candles); err != nil {
		return nil, stats, err
	}
	type candleAccumulator struct {
		used   bool
		bucket CandleBucket
	}
	buckets := make([]candleAccumulator, pixelWidth)
	for _, candle := range candles {
		index, visible := horizontalBucket(candle.X, visibleMinX, visibleMaxX, pixelWidth)
		if !visible {
			continue
		}
		current := &buckets[index]
		if !current.used {
			current.used = true
			current.bucket = CandleBucket{Candle: candle, LastSourceIndex: candle.SourceIndex}
			continue
		}
		current.bucket.High = max(current.bucket.High, candle.High)
		current.bucket.Low = min(current.bucket.Low, candle.Low)
		current.bucket.Close = candle.Close
		current.bucket.Volume += candle.Volume
		current.bucket.LastSourceIndex = candle.SourceIndex
		if !finite(current.bucket.Volume) {
			return nil, stats, fmt.Errorf("%w: aggregated candle volume overflow", ErrInvalidData)
		}
	}
	output := make([]CandleBucket, 0, pixelWidth)
	for _, current := range buckets {
		if !current.used {
			continue
		}
		stats.Buckets++
		output = append(output, current.bucket)
	}
	stats.OutputValues = len(output)
	return output, stats, nil
}

func selectContinuousExtrema(first Sample, last Sample, minimum Sample, maximum Sample) ([4]Sample, int) {
	selected := [4]Sample{first, minimum, maximum, last}
	for index := 1; index < len(selected); index++ {
		for current := index; current > 0 && selected[current].SourceIndex < selected[current-1].SourceIndex; current-- {
			selected[current], selected[current-1] = selected[current-1], selected[current]
		}
	}
	count := 0
	for _, candidate := range selected {
		if count == 0 || selected[count-1].SourceIndex != candidate.SourceIndex {
			selected[count] = candidate
			count++
		}
	}
	return selected, count
}

func horizontalBucket(x float64, minimum float64, maximum float64, width int) (int, bool) {
	if x < minimum || x > maximum {
		return 0, false
	}
	if x == maximum {
		return width - 1, true
	}
	index := int((x - minimum) * float64(width) / (maximum - minimum))
	return min(max(index, 0), width-1), true
}

func validateSamples(samples []Sample) error {
	var previous uint64
	var previousX float64
	for index, sample := range samples {
		if !finite(sample.X) || !finite(sample.Y) {
			return fmt.Errorf("%w: continuous sample %d is non-finite", ErrInvalidData, index)
		}
		if index > 0 && sample.SourceIndex <= previous {
			return fmt.Errorf("%w: continuous source indexes must increase", ErrInvalidData)
		}
		if index > 0 && sample.X < previousX {
			return fmt.Errorf("%w: continuous X values must not decrease", ErrInvalidData)
		}
		previous = sample.SourceIndex
		previousX = sample.X
	}
	return nil
}

func validateCandles(candles []Candle) error {
	var previous uint64
	var previousX float64
	for index, candle := range candles {
		if !finite(candle.X) || !finite(candle.Open) || !finite(candle.High) || !finite(candle.Low) ||
			!finite(candle.Close) || !finite(candle.Volume) || candle.Volume < 0 ||
			candle.High < max(candle.Open, candle.Close) || candle.Low > min(candle.Open, candle.Close) || candle.High < candle.Low {
			return fmt.Errorf("%w: candle %d violates finite OHLCV bounds", ErrInvalidData, index)
		}
		if index > 0 && candle.SourceIndex <= previous {
			return fmt.Errorf("%w: candle source indexes must increase", ErrInvalidData)
		}
		if index > 0 && candle.X < previousX {
			return fmt.Errorf("%w: candle X values must not decrease", ErrInvalidData)
		}
		previous = candle.SourceIndex
		previousX = candle.X
	}
	return nil
}
