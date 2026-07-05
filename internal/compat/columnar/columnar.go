// Package columnar exercises the approved Arrow and Parquet boundary.
package columnar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	arrowcsv "github.com/apache/arrow-go/v18/arrow/csv"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

const sampleCSV = `timestamp,symbol,close
1,ALPHA,101.5
2,ALPHA,NULL
3,BETA,99.25
`

var sampleSchema = arrow.NewSchema([]arrow.Field{
	{Name: "timestamp", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	{Name: "symbol", Type: arrow.BinaryTypes.String, Nullable: false},
	{Name: "close", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
}, nil)

// VerifyRoundTrips converts typed CSV input to Zstandard-compressed Parquet and
// Arrow IPC files, reopens both files, and verifies schema, values, and nulls.
func VerifyRoundTrips(ctx context.Context, directory string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("columnar compatibility: %w", err)
	}

	reader := arrowcsv.NewReader(
		strings.NewReader(sampleCSV),
		sampleSchema,
		arrowcsv.WithAllocator(memory.DefaultAllocator),
		arrowcsv.WithChunk(-1),
		arrowcsv.WithHeader(true),
		arrowcsv.WithNullReader(true, "NULL"),
	)
	defer reader.Release()

	if !reader.Next() {
		if err := reader.Err(); err != nil {
			return fmt.Errorf("read sample CSV: %w", err)
		}
		return errors.New("read sample CSV: no record batch")
	}
	record := reader.RecordBatch()
	record.Retain()
	defer record.Release()
	if err := verifyRecord(record); err != nil {
		return fmt.Errorf("verify CSV record: %w", err)
	}
	if reader.Next() {
		return errors.New("read sample CSV: unexpected second record batch")
	}
	if err := reader.Err(); err != nil {
		return fmt.Errorf("finish sample CSV: %w", err)
	}

	if err := verifyParquet(ctx, directory, record); err != nil {
		return err
	}
	if err := verifyIPC(directory, record); err != nil {
		return err
	}
	return nil
}

func verifyParquet(ctx context.Context, directory string, record arrow.RecordBatch) error {
	table := array.NewTableFromRecords(sampleSchema, []arrow.RecordBatch{record})
	defer table.Release()

	path := filepath.Join(directory, "compat.parquet")
	output, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create Parquet file: %w", err)
	}

	properties := parquet.NewWriterProperties(
		parquet.WithAllocator(memory.DefaultAllocator),
		parquet.WithCompression(compress.Codecs.Zstd),
	)
	writeErr := pqarrow.WriteTable(
		table,
		struct{ io.Writer }{output},
		table.NumRows(),
		properties,
		pqarrow.NewArrowWriterProperties(pqarrow.WithAllocator(memory.DefaultAllocator)),
	)
	if writeErr == nil {
		writeErr = output.Sync()
	}
	closeErr := output.Close()
	if writeErr != nil {
		return fmt.Errorf("write Parquet file: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Parquet file: %w", closeErr)
	}

	input, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reopen Parquet file: %w", err)
	}
	reopened, readErr := pqarrow.ReadTable(
		ctx,
		input,
		nil,
		pqarrow.ArrowReadProperties{},
		memory.DefaultAllocator,
	)
	closeErr = input.Close()
	if readErr != nil {
		return fmt.Errorf("read reopened Parquet file: %w", readErr)
	}
	defer reopened.Release()
	if closeErr != nil {
		return fmt.Errorf("close reopened Parquet file: %w", closeErr)
	}
	if err := verifyTable(reopened); err != nil {
		return fmt.Errorf("verify reopened Parquet file: %w", err)
	}
	return nil
}

func verifyIPC(directory string, record arrow.RecordBatch) error {
	path := filepath.Join(directory, "compat.arrow")
	output, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create Arrow IPC file: %w", err)
	}
	writer, err := ipc.NewFileWriter(
		struct{ io.Writer }{output},
		ipc.WithAllocator(memory.DefaultAllocator),
		ipc.WithSchema(sampleSchema),
		ipc.WithZstd(),
	)
	if err != nil {
		_ = output.Close()
		return fmt.Errorf("create Arrow IPC writer: %w", err)
	}
	writeErr := writer.Write(record)
	if writeErr == nil {
		writeErr = writer.Close()
	}
	if writeErr == nil {
		writeErr = output.Sync()
	}
	closeErr := output.Close()
	if writeErr != nil {
		return fmt.Errorf("write Arrow IPC file: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Arrow IPC file: %w", closeErr)
	}

	input, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reopen Arrow IPC file: %w", err)
	}
	reopened, readErr := ipc.NewFileReader(input, ipc.WithAllocator(memory.DefaultAllocator))
	if readErr != nil {
		_ = input.Close()
		return fmt.Errorf("create Arrow IPC reader: %w", readErr)
	}
	if reopened.NumRecords() != 1 {
		_ = reopened.Close()
		_ = input.Close()
		return fmt.Errorf("verify Arrow IPC record count: got %d, want 1", reopened.NumRecords())
	}
	reopenedRecord, readErr := reopened.RecordBatch(0)
	if readErr != nil {
		_ = reopened.Close()
		_ = input.Close()
		return fmt.Errorf("read reopened Arrow IPC record: %w", readErr)
	}
	defer reopenedRecord.Release()
	verifyErr := verifyRecord(reopenedRecord)
	readerCloseErr := reopened.Close()
	fileCloseErr := input.Close()
	if verifyErr != nil {
		return fmt.Errorf("verify reopened Arrow IPC file: %w", verifyErr)
	}
	if readerCloseErr != nil {
		return fmt.Errorf("close Arrow IPC reader: %w", readerCloseErr)
	}
	if fileCloseErr != nil {
		return fmt.Errorf("close reopened Arrow IPC file: %w", fileCloseErr)
	}
	return nil
}

func verifyTable(table arrow.Table) error {
	if err := verifySchema(table.Schema()); err != nil {
		return err
	}
	if table.NumRows() != 3 || table.NumCols() != 3 {
		return fmt.Errorf("shape mismatch: got %dx%d, want 3x3", table.NumRows(), table.NumCols())
	}

	records := array.NewTableReader(table, table.NumRows())
	defer records.Release()
	if !records.Next() {
		return errors.New("table contains no record")
	}
	if err := verifyRecord(records.RecordBatch()); err != nil {
		return err
	}
	if records.Next() {
		return errors.New("table contains an unexpected second record")
	}
	return records.Err()
}

func verifyRecord(record arrow.RecordBatch) error {
	if err := verifySchema(record.Schema()); err != nil {
		return err
	}
	if record.NumRows() != 3 || record.NumCols() != 3 {
		return fmt.Errorf("shape mismatch: got %dx%d, want 3x3", record.NumRows(), record.NumCols())
	}

	timestamps, ok := record.Column(0).(*array.Int64)
	if !ok {
		return fmt.Errorf("timestamp column has type %T, want *array.Int64", record.Column(0))
	}
	symbols, ok := record.Column(1).(*array.String)
	if !ok {
		return fmt.Errorf("symbol column has type %T, want *array.String", record.Column(1))
	}
	closes, ok := record.Column(2).(*array.Float64)
	if !ok {
		return fmt.Errorf("close column has type %T, want *array.Float64", record.Column(2))
	}

	wantTimestamps := [...]int64{1, 2, 3}
	wantSymbols := [...]string{"ALPHA", "ALPHA", "BETA"}
	wantCloses := [...]float64{101.5, 0, 99.25}
	for index := range wantTimestamps {
		if timestamps.IsNull(index) || timestamps.Value(index) != wantTimestamps[index] {
			return fmt.Errorf("timestamp[%d] mismatch", index)
		}
		if symbols.IsNull(index) || symbols.Value(index) != wantSymbols[index] {
			return fmt.Errorf("symbol[%d] mismatch", index)
		}
		if index == 1 {
			if !closes.IsNull(index) {
				return errors.New("close[1] is not null")
			}
			continue
		}
		if closes.IsNull(index) || closes.Value(index) != wantCloses[index] {
			return fmt.Errorf("close[%d] mismatch", index)
		}
	}
	return nil
}

func verifySchema(schema *arrow.Schema) error {
	if schema.NumFields() != sampleSchema.NumFields() {
		return fmt.Errorf(
			"schema field count mismatch: got %d, want %d",
			schema.NumFields(),
			sampleSchema.NumFields(),
		)
	}
	for index, want := range sampleSchema.Fields() {
		got := schema.Field(index)
		if got.Name != want.Name ||
			got.Nullable != want.Nullable ||
			!arrow.TypeEqual(got.Type, want.Type) {
			return fmt.Errorf("schema field %d mismatch: got %v, want %v", index, got, want)
		}
	}
	return nil
}
