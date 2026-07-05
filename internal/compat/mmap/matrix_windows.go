// Package mmap provides the narrow typed Windows memory-map boundary used by
// the Phase 0 compatibility gate.
package mmap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	headerSize    = 32
	formatVersion = 1
)

var fileMagic = [8]byte{'C', 'O', 'R', 'T', 'H', 'M', 'A', 'T'}

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// Matrix is a typed view over a versioned little-endian matrix file. A Matrix
// must not be copied. Callers must finish all reads and exclusive-range writes
// before calling Close.
type Matrix struct {
	_         noCopy
	file      *os.File
	mapping   windows.Handle
	address   uintptr
	data      []byte
	rows      int
	columns   int
	writable  bool
	closeOnce sync.Once
	closeErr  error
}

// Create writes a new matrix with a zeroed float64 output vector.
func Create(path string, rows int, columns int, input []float32) error {
	if rows <= 0 || columns <= 0 {
		return errors.New("create matrix: dimensions must be positive")
	}
	elementCount, err := checkedProduct(rows, columns)
	if err != nil {
		return err
	}
	if len(input) != elementCount {
		return fmt.Errorf("create matrix: got %d input values, want %d", len(input), elementCount)
	}
	size, err := matrixFileSize(rows, columns)
	if err != nil {
		return err
	}
	contents := make([]byte, size)
	copy(contents[:len(fileMagic)], fileMagic[:])
	binary.LittleEndian.PutUint32(contents[8:12], formatVersion)
	binary.LittleEndian.PutUint32(contents[12:16], uint32(rows))
	binary.LittleEndian.PutUint32(contents[16:20], uint32(columns))
	binary.LittleEndian.PutUint32(contents[20:24], uint32(elementCount))
	for index, value := range input {
		offset := headerSize + index*4
		binary.LittleEndian.PutUint32(contents[offset:offset+4], math.Float32bits(value))
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create matrix file: %w", err)
	}
	writeCount, writeErr := file.Write(contents)
	if writeErr == nil && writeCount != len(contents) {
		writeErr = ioErrShortWrite(writeCount, len(contents))
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write matrix file: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close matrix file: %w", closeErr)
	}
	return nil
}

// Open validates and maps a matrix file.
func Open(path string, writable bool) (*Matrix, error) {
	flags := os.O_RDONLY
	if writable {
		flags = os.O_RDWR
	}
	file, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open matrix file: %w", err)
	}

	header := make([]byte, headerSize)
	if _, err := file.ReadAt(header, 0); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("read matrix header: %w", err)
	}
	rows, columns, err := validateHeader(header)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	wantSize, err := matrixFileSize(rows, columns)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat matrix file: %w", err)
	}
	if info.Size() != int64(wantSize) {
		_ = file.Close()
		return nil, fmt.Errorf("validate matrix size: got %d, want %d", info.Size(), wantSize)
	}

	protection := uint32(windows.PAGE_READONLY)
	access := uint32(windows.FILE_MAP_READ)
	if writable {
		protection = windows.PAGE_READWRITE
		access = windows.FILE_MAP_READ | windows.FILE_MAP_WRITE
	}
	mapping, err := windows.CreateFileMapping(
		windows.Handle(file.Fd()),
		nil,
		protection,
		0,
		0,
		nil,
	)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("create Windows file mapping: %w", err)
	}
	address, err := windows.MapViewOfFile(mapping, access, 0, 0, uintptr(wantSize))
	if err != nil {
		_ = windows.CloseHandle(mapping)
		_ = file.Close()
		return nil, fmt.Errorf("map Windows file view: %w", err)
	}

	// MapViewOfFile guarantees a contiguous region of wantSize bytes for the
	// lifetime of the view. The Win32 wrapper represents that foreign pointer as
	// uintptr, so mappedPointer reinterprets its bits without pointer arithmetic.
	// The slice never escapes this adapter and is invalid immediately after
	// UnmapViewOfFile in Close.
	data := unsafe.Slice((*byte)(mappedPointer(address)), wantSize)
	return &Matrix{
		file:     file,
		mapping:  mapping,
		address:  address,
		data:     data,
		rows:     rows,
		columns:  columns,
		writable: writable,
	}, nil
}

// Rows returns the number of matrix rows.
func (matrix *Matrix) Rows() int {
	return matrix.rows
}

// Columns returns the number of matrix columns.
func (matrix *Matrix) Columns() int {
	return matrix.columns
}

// Input returns one float32 input value. Input bytes are immutable after the
// file is published.
func (matrix *Matrix) Input(row int, column int) (float32, error) {
	if row < 0 || row >= matrix.rows || column < 0 || column >= matrix.columns {
		return 0, fmt.Errorf("read matrix input: index [%d,%d] out of range", row, column)
	}
	index := row*matrix.columns + column
	offset := headerSize + index*4
	bits := binary.LittleEndian.Uint32(matrix.data[offset : offset+4])
	return math.Float32frombits(bits), nil
}

// WriteOutputRange writes task-owned float64 output rows. Concurrent calls are
// valid only when their row ranges do not overlap.
func (matrix *Matrix) WriteOutputRange(startRow int, values []float64) error {
	if !matrix.writable {
		return errors.New("write matrix output: mapping is read-only")
	}
	if startRow < 0 || startRow > matrix.rows || len(values) > matrix.rows-startRow {
		return fmt.Errorf(
			"write matrix output: range [%d,%d) out of bounds",
			startRow,
			startRow+len(values),
		)
	}
	outputStart := headerSize + matrix.rows*matrix.columns*4
	for index, value := range values {
		offset := outputStart + (startRow+index)*8
		binary.LittleEndian.PutUint64(matrix.data[offset:offset+8], math.Float64bits(value))
	}
	return nil
}

// Output returns one float64 output value.
func (matrix *Matrix) Output(row int) (float64, error) {
	if row < 0 || row >= matrix.rows {
		return 0, fmt.Errorf("read matrix output: row %d out of range", row)
	}
	outputStart := headerSize + matrix.rows*matrix.columns*4
	offset := outputStart + row*8
	bits := binary.LittleEndian.Uint64(matrix.data[offset : offset+8])
	return math.Float64frombits(bits), nil
}

// Flush writes a writable view through to stable storage.
func (matrix *Matrix) Flush() error {
	if !matrix.writable {
		return errors.New("flush matrix: mapping is read-only")
	}
	if err := windows.FlushViewOfFile(matrix.address, uintptr(len(matrix.data))); err != nil {
		return fmt.Errorf("flush Windows mapped view: %w", err)
	}
	if err := matrix.file.Sync(); err != nil {
		return fmt.Errorf("sync matrix file: %w", err)
	}
	return nil
}

// Close releases the mapped view, mapping handle, and file. It is idempotent.
func (matrix *Matrix) Close() error {
	matrix.closeOnce.Do(func() {
		unmapErr := windows.UnmapViewOfFile(matrix.address)
		matrix.data = nil
		mappingErr := windows.CloseHandle(matrix.mapping)
		fileErr := matrix.file.Close()
		matrix.closeErr = errors.Join(unmapErr, mappingErr, fileErr)
	})
	if matrix.closeErr != nil {
		return fmt.Errorf("close matrix mapping: %w", matrix.closeErr)
	}
	return nil
}

func validateHeader(header []byte) (int, int, error) {
	if len(header) != headerSize {
		return 0, 0, errors.New("validate matrix header: wrong size")
	}
	if string(header[:len(fileMagic)]) != string(fileMagic[:]) {
		return 0, 0, errors.New("validate matrix header: wrong magic")
	}
	if version := binary.LittleEndian.Uint32(header[8:12]); version != formatVersion {
		return 0, 0, fmt.Errorf("validate matrix header: unsupported version %d", version)
	}
	rows := int(binary.LittleEndian.Uint32(header[12:16]))
	columns := int(binary.LittleEndian.Uint32(header[16:20]))
	if rows <= 0 || columns <= 0 {
		return 0, 0, errors.New("validate matrix header: dimensions must be positive")
	}
	elementCount, err := checkedProduct(rows, columns)
	if err != nil {
		return 0, 0, err
	}
	if storedCount := int(binary.LittleEndian.Uint32(header[20:24])); storedCount != elementCount {
		return 0, 0, fmt.Errorf(
			"validate matrix header: got %d elements, want %d",
			storedCount,
			elementCount,
		)
	}
	return rows, columns, nil
}

func matrixFileSize(rows int, columns int) (int, error) {
	elementCount, err := checkedProduct(rows, columns)
	if err != nil {
		return 0, err
	}
	if elementCount > (math.MaxInt-headerSize-rows*8)/4 {
		return 0, errors.New("calculate matrix file size: overflow")
	}
	return headerSize + elementCount*4 + rows*8, nil
}

func checkedProduct(left int, right int) (int, error) {
	if left <= 0 || right <= 0 || left > math.MaxInt/right {
		return 0, errors.New("calculate matrix dimensions: overflow")
	}
	product := left * right
	if uint64(product) > math.MaxUint32 {
		return 0, errors.New("calculate matrix dimensions: exceeds file format")
	}
	return product, nil
}

func ioErrShortWrite(got int, want int) error {
	return fmt.Errorf("short write: got %d bytes, want %d", got, want)
}

func mappedPointer(address uintptr) unsafe.Pointer {
	// address refers to Win32-owned mapped memory, not Go memory. Reinterpreting
	// the bits through an unsafe.Pointer slot avoids retaining a Go pointer as a
	// uintptr across any operation.
	return *(*unsafe.Pointer)(unsafe.Pointer(&address))
}
