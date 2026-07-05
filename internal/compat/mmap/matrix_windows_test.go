package mmap_test

import (
	"math"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rosstheboss94/corthena/internal/compat/mmap"
)

func TestParallelMappedMatrixRoundTrip(t *testing.T) {
	t.Parallel()

	const (
		rows        = 16
		columns     = 4
		workerCount = 4
	)
	input := make([]float32, rows*columns)
	for row := range rows {
		for column := range columns {
			input[row*columns+column] = float32(row*10 + column)
		}
	}
	path := filepath.Join(t.TempDir(), "matrix.bin")
	if err := mmap.Create(path, rows, columns, input); err != nil {
		t.Fatal(err)
	}

	matrix, err := mmap.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = matrix.Close()
		}
	}()

	// Workers read shared immutable inputs and write one disjoint output range.
	// Each sends one result; the parent closes the buffered channel after all
	// workers terminate.
	results := make(chan error, workerCount)
	var workers sync.WaitGroup
	rowsPerWorker := rows / workerCount
	workers.Add(workerCount)
	for worker := range workerCount {
		start := worker * rowsPerWorker
		end := start + rowsPerWorker
		go func() {
			defer workers.Done()
			output := make([]float64, end-start)
			for row := start; row < end; row++ {
				for column := range columns {
					value, readErr := matrix.Input(row, column)
					if readErr != nil {
						results <- readErr
						return
					}
					output[row-start] += float64(value)
				}
			}
			results <- matrix.WriteOutputRange(start, output)
		}()
	}
	workers.Wait()
	close(results)
	for workerErr := range results {
		if workerErr != nil {
			t.Fatal(workerErr)
		}
	}

	if err := matrix.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := matrix.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true

	reopened, err := mmap.Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.Rows() != rows || reopened.Columns() != columns {
		t.Fatalf(
			"reopened shape is %dx%d, want %dx%d",
			reopened.Rows(),
			reopened.Columns(),
			rows,
			columns,
		)
	}
	for row := range rows {
		var want float64
		for column := range columns {
			want += float64(input[row*columns+column])
		}
		got, err := reopened.Output(row)
		if err != nil {
			t.Fatal(err)
		}
		if math.Float64bits(got) != math.Float64bits(want) {
			t.Fatalf("output[%d] = %v, want %v", row, got, want)
		}
	}
}
