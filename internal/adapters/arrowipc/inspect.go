// Package arrowipc contains Apache Arrow values and converts them to deliberate
// typed frontend boundary values.
package arrowipc

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// ErrUnsupportedType identifies an Arrow field outside the admitted boundary.
var ErrUnsupportedType = errors.New("unsupported Arrow type")

// Kind is a frontend-safe logical column kind.
type Kind string

const (
	// KindBoolean is a Boolean column.
	KindBoolean Kind = "boolean"
	// KindInt64 is a signed 64-bit integer column.
	KindInt64 Kind = "int64"
	// KindFloat64 is a double-precision column.
	KindFloat64 Kind = "float64"
	// KindString is a UTF-8 string column.
	KindString Kind = "string"
	// KindTimestamp is a timestamp column.
	KindTimestamp Kind = "timestamp"
)

// Field is a native-value-free Arrow schema field.
type Field struct {
	Name     string
	Kind     Kind
	Nullable bool
}

// Summary describes an Arrow IPC file without exposing Arrow objects.
type Summary struct {
	Fields        []Field
	RecordBatches int
	Rows          int64
}

// Inspect validates an Arrow IPC file and converts its schema and shape to
// typed first-party values.
func Inspect(ctx context.Context, data []byte) (summary Summary, resultErr error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, fmt.Errorf("inspect Arrow IPC: %w", err)
	}
	reader, err := ipc.NewFileReader(
		bytes.NewReader(data),
		ipc.WithAllocator(memory.DefaultAllocator),
	)
	if err != nil {
		return Summary{}, fmt.Errorf("open Arrow IPC: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close Arrow IPC: %w", err))
		}
	}()

	summary.Fields = make([]Field, reader.Schema().NumFields())
	for index, nativeField := range reader.Schema().Fields() {
		kind, err := convertKind(nativeField.Type)
		if err != nil {
			return Summary{}, fmt.Errorf("inspect Arrow field %q: %w", nativeField.Name, err)
		}
		summary.Fields[index] = Field{
			Name:     nativeField.Name,
			Kind:     kind,
			Nullable: nativeField.Nullable,
		}
	}
	summary.RecordBatches = reader.NumRecords()
	for index := range summary.RecordBatches {
		if err := ctx.Err(); err != nil {
			return Summary{}, fmt.Errorf("inspect Arrow IPC: %w", err)
		}
		record, err := reader.RecordBatch(index)
		if err != nil {
			return Summary{}, fmt.Errorf("read Arrow record batch %d: %w", index, err)
		}
		summary.Rows += record.NumRows()
		record.Release()
	}
	return summary, nil
}

func convertKind(dataType arrow.DataType) (Kind, error) {
	switch dataType.ID() {
	case arrow.BOOL:
		return KindBoolean, nil
	case arrow.INT64:
		return KindInt64, nil
	case arrow.FLOAT64:
		return KindFloat64, nil
	case arrow.STRING:
		return KindString, nil
	case arrow.TIMESTAMP:
		return KindTimestamp, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedType, dataType)
	}
}
