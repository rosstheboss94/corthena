package arrowipc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

func TestInspectConvertsSchemaAndShape(t *testing.T) {
	t.Parallel()

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "timestamp", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "symbol", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "close", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"A", "B"}, nil)
	builder.Field(2).(*array.Float64Builder).AppendValues([]float64{10, 0}, []bool{true, false})
	record := builder.NewRecordBatch()
	defer record.Release()

	data := writeIPC(t, schema, record)
	summary, err := Inspect(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if summary.RecordBatches != 1 || summary.Rows != 2 {
		t.Fatalf("shape = %d batches/%d rows, want 1/2", summary.RecordBatches, summary.Rows)
	}
	if len(summary.Fields) != 3 ||
		summary.Fields[0].Kind != KindInt64 ||
		summary.Fields[1].Kind != KindString ||
		summary.Fields[2].Kind != KindFloat64 {
		t.Fatalf("unexpected fields: %+v", summary.Fields)
	}
}

func TestInspectRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "value", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).Append(1)
	record := builder.NewRecordBatch()
	defer record.Release()

	_, err := Inspect(context.Background(), writeIPC(t, schema, record))
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("error = %v, want ErrUnsupportedType", err)
	}
}

func writeIPC(t *testing.T, schema *arrow.Schema, record arrow.RecordBatch) []byte {
	t.Helper()

	var output bytes.Buffer
	writer, err := ipc.NewFileWriter(
		struct{ io.Writer }{&output},
		ipc.WithSchema(schema),
		ipc.WithAllocator(memory.DefaultAllocator),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(record); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
