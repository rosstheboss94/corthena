package chart

import (
	"errors"
	"testing"
)

func TestContinuousLODPreservesStableFirstLastMinimumMaximum(t *testing.T) {
	t.Parallel()
	samples := []Sample{
		{X: 0.1, Y: 5, SourceIndex: 10},
		{X: 0.2, Y: 1, SourceIndex: 11},
		{X: 0.3, Y: 9, SourceIndex: 12},
		{X: 0.4, Y: 9, SourceIndex: 13},
		{X: 0.5, Y: 4, SourceIndex: 14},
	}
	got, stats, err := AggregateContinuous(samples, 0, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{10, 11, 12, 14}
	if len(got) != len(want) {
		t.Fatalf("values = %+v", got)
	}
	for index, source := range want {
		if got[index].SourceIndex != source {
			t.Fatalf("source[%d] = %d, want %d", index, got[index].SourceIndex, source)
		}
	}
	if stats.OutputValues != 4 || stats.Buckets != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestCandleLODHandCalculatedOHLCV(t *testing.T) {
	t.Parallel()
	candles := []Candle{
		{X: 1, Open: 10, High: 13, Low: 9, Close: 12, Volume: 100, SourceIndex: 1},
		{X: 2, Open: 12, High: 15, Low: 8, Close: 9, Volume: 250, SourceIndex: 2},
		{X: 3, Open: 9, High: 11, Low: 7, Close: 10, Volume: 50, SourceIndex: 3},
	}
	got, stats, err := AggregateCandles(candles, 0, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("buckets = %d", len(got))
	}
	bucket := got[0]
	if bucket.Open != 10 || bucket.High != 15 || bucket.Low != 7 || bucket.Close != 10 || bucket.Volume != 400 || bucket.SourceIndex != 1 || bucket.LastSourceIndex != 3 {
		t.Fatalf("bucket = %+v", bucket)
	}
	if stats.OutputValues != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestLODRenderWorkBoundedByViewportWidth(t *testing.T) {
	t.Parallel()
	const sourceCount = 200_000
	const width = 320
	samples := make([]Sample, sourceCount)
	for index := range samples {
		samples[index] = Sample{X: float64(index), Y: float64(index%97) - 48, SourceIndex: uint64(index + 1)}
	}
	values, stats, err := AggregateContinuous(samples, 0, sourceCount-1, width)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) > width*4 || stats.OutputValues > width*4 {
		t.Fatalf("output = %d for width %d", len(values), width)
	}
}

func TestLODRejectsUnstableSourceOrder(t *testing.T) {
	t.Parallel()
	_, _, err := AggregateContinuous([]Sample{{X: 0, Y: 1, SourceIndex: 2}, {X: 1, Y: 2, SourceIndex: 1}}, 0, 1, 10)
	if !errors.Is(err, ErrInvalidData) {
		t.Fatalf("error = %v", err)
	}
}

func BenchmarkContinuousLODMillionPoints(b *testing.B) {
	const sourceCount = 1_000_000
	const width = 1920
	samples := make([]Sample, sourceCount)
	for index := range samples {
		samples[index] = Sample{X: float64(index), Y: float64(index % 101), SourceIndex: uint64(index + 1)}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, _, err := AggregateContinuous(samples, 0, sourceCount-1, width); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(sourceCount, "source_rows/op")
	b.ReportMetric(width*4, "max_render_values/op")
}
