package arrowipc

import (
	"context"
	"errors"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

func TestDecodeContinuousChartIPC(t *testing.T) {
	t.Parallel()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "source_index", Type: arrow.PrimitiveTypes.Int64},
		{Name: "x", Type: arrow.PrimitiveTypes.Float64},
		{Name: "y", Type: arrow.PrimitiveTypes.Float64},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int64Builder).AppendValues([]int64{10, 11}, nil)
	builder.Field(1).(*array.Float64Builder).AppendValues([]float64{1.5, 2.5}, nil)
	builder.Field(2).(*array.Float64Builder).AppendValues([]float64{3.5, 4.5}, nil)
	record := builder.NewRecordBatch()
	defer record.Release()
	decoded, err := DecodeChart(context.Background(), writeIPC(t, schema, record), ChartSchemaVersion, ChartContinuous)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Samples) != 2 || decoded.Samples[1].SourceIndex != 11 || decoded.Samples[1].X != 2.5 || decoded.Samples[1].Y != 4.5 {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestDecodeCandleChartIPC(t *testing.T) {
	t.Parallel()
	fields := []arrow.Field{{Name: "source_index", Type: arrow.PrimitiveTypes.Int64}, {Name: "x", Type: arrow.PrimitiveTypes.Float64},
		{Name: "open", Type: arrow.PrimitiveTypes.Float64}, {Name: "high", Type: arrow.PrimitiveTypes.Float64},
		{Name: "low", Type: arrow.PrimitiveTypes.Float64}, {Name: "close", Type: arrow.PrimitiveTypes.Float64},
		{Name: "volume", Type: arrow.PrimitiveTypes.Float64}}
	schema := arrow.NewSchema(fields, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int64Builder).Append(1)
	values := []float64{100, 10, 12, 9, 11, 250}
	for index, value := range values {
		builder.Field(index + 1).(*array.Float64Builder).Append(value)
	}
	record := builder.NewRecordBatch()
	defer record.Release()
	decoded, err := DecodeChart(context.Background(), writeIPC(t, schema, record), ChartSchemaVersion, ChartCandles)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Candles) != 1 || decoded.Candles[0].Close != 11 || decoded.Candles[0].Volume != 250 {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestDecodeChartRejectsVersionSchemaAndCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DecodeChart(ctx, nil, ChartSchemaVersion, ChartContinuous); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	if _, err := DecodeChart(context.Background(), nil, 999, ChartContinuous); err == nil {
		t.Fatal("unsupported version accepted")
	}
	schema := arrow.NewSchema([]arrow.Field{{Name: "wrong", Type: arrow.PrimitiveTypes.Int64}}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int64Builder).Append(1)
	record := builder.NewRecordBatch()
	defer record.Release()
	if _, err := DecodeChart(context.Background(), writeIPC(t, schema, record), ChartSchemaVersion, ChartContinuous); err == nil {
		t.Fatal("wrong schema accepted")
	}
}
