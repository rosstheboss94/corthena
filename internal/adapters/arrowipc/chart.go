package arrowipc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/rosstheboss94/corthena/internal/frontend/chart"
)

// ChartSchemaVersion is the admitted internal chart IPC schema version.
const ChartSchemaVersion = 1

// ChartKind identifies one exact admitted chart IPC schema.
type ChartKind uint8

const (
	ChartContinuous ChartKind = iota + 1
	ChartCandles
)

// ChartData is a native-value-free decoded chart payload.
type ChartData struct {
	Version int
	Kind    ChartKind
	Samples []chart.Sample
	Candles []chart.Candle
}

// DecodeChart validates an exact schema and copies all values into typed,
// client-owned Go slices. It performs no Raylib work and honors cancellation
// between record batches and rows.
func DecodeChart(ctx context.Context, data []byte, version int, kind ChartKind) (decoded ChartData, resultErr error) {
	if ctx == nil {
		return ChartData{}, errors.New("decode chart Arrow IPC: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return ChartData{}, fmt.Errorf("decode chart Arrow IPC: %w", err)
	}
	if version != ChartSchemaVersion {
		return ChartData{}, fmt.Errorf("decode chart Arrow IPC: unsupported schema version %d", version)
	}
	reader, err := ipc.NewFileReader(bytes.NewReader(data), ipc.WithAllocator(memory.DefaultAllocator))
	if err != nil {
		return ChartData{}, fmt.Errorf("open chart Arrow IPC: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close chart Arrow IPC: %w", err))
		}
	}()
	if err := validateChartSchema(reader.Schema(), kind); err != nil {
		return ChartData{}, err
	}
	decoded = ChartData{Version: version, Kind: kind}
	for batchIndex := range reader.NumRecords() {
		if err := ctx.Err(); err != nil {
			return ChartData{}, fmt.Errorf("decode chart Arrow IPC: %w", err)
		}
		record, err := reader.RecordBatch(batchIndex)
		if err != nil {
			return ChartData{}, fmt.Errorf("read chart record batch %d: %w", batchIndex, err)
		}
		if kind == ChartContinuous {
			err = appendContinuous(ctx, record, &decoded.Samples)
		} else {
			err = appendCandles(ctx, record, &decoded.Candles)
		}
		record.Release()
		if err != nil {
			return ChartData{}, fmt.Errorf("decode chart record batch %d: %w", batchIndex, err)
		}
	}
	if err := validateChartData(decoded); err != nil {
		return ChartData{}, fmt.Errorf("validate decoded chart Arrow IPC: %w", err)
	}
	return decoded, nil
}

func validateChartData(decoded ChartData) error {
	var previousIndex uint64
	var previousX float64
	if decoded.Kind == ChartContinuous {
		for index, sample := range decoded.Samples {
			if !finiteChartValue(sample.X) || !finiteChartValue(sample.Y) ||
				(index > 0 && (sample.SourceIndex <= previousIndex || sample.X < previousX)) {
				return fmt.Errorf("continuous row %d is non-finite or unordered", index)
			}
			previousIndex, previousX = sample.SourceIndex, sample.X
		}
		return nil
	}
	for index, candle := range decoded.Candles {
		if !finiteChartValue(candle.X) || !finiteChartValue(candle.Open) || !finiteChartValue(candle.High) ||
			!finiteChartValue(candle.Low) || !finiteChartValue(candle.Close) || !finiteChartValue(candle.Volume) || candle.Volume < 0 ||
			candle.High < max(candle.Open, candle.Close) || candle.Low > min(candle.Open, candle.Close) ||
			(index > 0 && (candle.SourceIndex <= previousIndex || candle.X < previousX)) {
			return fmt.Errorf("candle row %d violates ordered finite OHLCV bounds", index)
		}
		previousIndex, previousX = candle.SourceIndex, candle.X
	}
	return nil
}

func finiteChartValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validateChartSchema(schema *arrow.Schema, kind ChartKind) error {
	continuous := []arrow.Field{
		{Name: "source_index", Type: arrow.PrimitiveTypes.Int64},
		{Name: "x", Type: arrow.PrimitiveTypes.Float64},
		{Name: "y", Type: arrow.PrimitiveTypes.Float64},
	}
	candles := []arrow.Field{
		{Name: "source_index", Type: arrow.PrimitiveTypes.Int64},
		{Name: "x", Type: arrow.PrimitiveTypes.Float64},
		{Name: "open", Type: arrow.PrimitiveTypes.Float64},
		{Name: "high", Type: arrow.PrimitiveTypes.Float64},
		{Name: "low", Type: arrow.PrimitiveTypes.Float64},
		{Name: "close", Type: arrow.PrimitiveTypes.Float64},
		{Name: "volume", Type: arrow.PrimitiveTypes.Float64},
	}
	want := continuous
	if kind == ChartCandles {
		want = candles
	} else if kind != ChartContinuous {
		return fmt.Errorf("decode chart Arrow IPC: unknown chart kind %d", kind)
	}
	got := schema.Fields()
	if len(got) != len(want) {
		return fmt.Errorf("decode chart Arrow IPC: schema has %d fields, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].Name != want[index].Name || !arrow.TypeEqual(got[index].Type, want[index].Type) || got[index].Nullable {
			return fmt.Errorf("decode chart Arrow IPC: field %d is %q/%s nullable=%t, want %q/%s nullable=false",
				index, got[index].Name, got[index].Type, got[index].Nullable, want[index].Name, want[index].Type)
		}
	}
	return nil
}

func appendContinuous(ctx context.Context, record arrow.RecordBatch, output *[]chart.Sample) error {
	indexes, ok := record.Column(0).(*array.Int64)
	if !ok {
		return errors.New("source_index array has unexpected native type")
	}
	xValues, ok := record.Column(1).(*array.Float64)
	if !ok {
		return errors.New("x array has unexpected native type")
	}
	yValues, ok := record.Column(2).(*array.Float64)
	if !ok {
		return errors.New("y array has unexpected native type")
	}
	for row := 0; row < int(record.NumRows()); row++ {
		if row&4095 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		index := indexes.Value(row)
		if index < 0 {
			return fmt.Errorf("negative source index at row %d", row)
		}
		*output = append(*output, chart.Sample{X: xValues.Value(row), Y: yValues.Value(row), SourceIndex: uint64(index)})
	}
	return nil
}

func appendCandles(ctx context.Context, record arrow.RecordBatch, output *[]chart.Candle) error {
	indexes, ok := record.Column(0).(*array.Int64)
	if !ok {
		return errors.New("source_index array has unexpected native type")
	}
	values := make([]*array.Float64, 6)
	for index := range values {
		column, ok := record.Column(index + 1).(*array.Float64)
		if !ok {
			return fmt.Errorf("candle column %d has unexpected native type", index+1)
		}
		values[index] = column
	}
	for row := 0; row < int(record.NumRows()); row++ {
		if row&4095 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		index := indexes.Value(row)
		if index < 0 {
			return fmt.Errorf("negative source index at row %d", row)
		}
		*output = append(*output, chart.Candle{
			SourceIndex: uint64(index), X: values[0].Value(row), Open: values[1].Value(row), High: values[2].Value(row),
			Low: values[3].Value(row), Close: values[4].Value(row), Volume: values[5].Value(row),
		})
	}
	return nil
}
