// Package table provides first-party, Raylib-independent virtualized table
// layout, typed sorting/filtering, stable-ID selection, and bounded copy
// kernels.
package table

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	// ErrInvalidTable reports malformed schema, rows, geometry, or operations.
	ErrInvalidTable = errors.New("invalid virtual table")
	// ErrCopyLimit reports copy output that would exceed its explicit bound.
	ErrCopyLimit = errors.New("table copy byte limit exceeded")
)

// RowID is a stable row identity independent of visible position.
type RowID string

// ColumnID is a stable schema column identity.
type ColumnID string

// CellKind identifies one explicit typed cell representation.
type CellKind uint8

const (
	CellString CellKind = iota + 1
	CellInteger
	CellFloat
	CellBoolean
	CellTime
)

// Valid reports whether kind is supported.
func (kind CellKind) Valid() bool {
	switch kind {
	case CellString, CellInteger, CellFloat, CellBoolean, CellTime:
		return true
	default:
		return false
	}
}

// Cell is a typed value. Null cells retain their declared Kind and ignore the
// associated value field.
type Cell struct {
	Kind    CellKind
	Null    bool
	String  string
	Integer int64
	Float   float64
	Boolean bool
	Time    time.Time
}

// Validate checks a cell against its declared kind.
func (cell Cell) Validate() error {
	if !cell.Kind.Valid() {
		return fmt.Errorf("%w: unknown cell kind %d", ErrInvalidTable, cell.Kind)
	}
	if cell.Kind == CellFloat && !cell.Null && (math.IsNaN(cell.Float) || math.IsInf(cell.Float, 0)) {
		return fmt.Errorf("%w: non-finite float cell", ErrInvalidTable)
	}
	if cell.Kind == CellTime && !cell.Null && cell.Time.IsZero() {
		return fmt.Errorf("%w: zero time cell", ErrInvalidTable)
	}
	return nil
}

// Column is one typed, resizable schema column.
type Column struct {
	ID       ColumnID
	Title    string
	Kind     CellKind
	Width    float64
	MinWidth float64
	MaxWidth float64
	Pinned   bool
	Sortable bool
}

// Row is one immutable table row. SourceIndex is a stable final sort
// tie-breaker.
type Row struct {
	ID          RowID
	Cells       []Cell
	SourceIndex uint64
}

// Clone returns an independent immutable row.
func (row Row) Clone() Row {
	row.Cells = append([]Cell(nil), row.Cells...)
	for index := range row.Cells {
		row.Cells[index].Time = row.Cells[index].Time.UTC()
	}
	return row
}

// Dataset owns a validated typed schema and rows.
type Dataset struct {
	Columns []Column
	Rows    []Row
}

// Clone returns a deep immutable dataset copy.
func (dataset Dataset) Clone() Dataset {
	dataset.Columns = append([]Column(nil), dataset.Columns...)
	source := dataset.Rows
	dataset.Rows = make([]Row, len(source))
	for index, row := range source {
		dataset.Rows[index] = row.Clone()
	}
	return dataset
}

// Validate checks schema uniqueness, row identity, type alignment, and stable
// source-index uniqueness.
func (dataset Dataset) Validate() error {
	if len(dataset.Columns) == 0 {
		return fmt.Errorf("%w: schema is empty", ErrInvalidTable)
	}
	columnIDs := make(map[ColumnID]struct{}, len(dataset.Columns))
	for index, column := range dataset.Columns {
		if column.ID == "" || column.Title == "" || !column.Kind.Valid() || !finite(column.Width) || !finite(column.MinWidth) || !finite(column.MaxWidth) ||
			column.MinWidth <= 0 || column.MaxWidth < column.MinWidth || column.Width < column.MinWidth || column.Width > column.MaxWidth {
			return fmt.Errorf("%w: invalid column %d", ErrInvalidTable, index)
		}
		if _, duplicate := columnIDs[column.ID]; duplicate {
			return fmt.Errorf("%w: duplicate column ID %q", ErrInvalidTable, column.ID)
		}
		columnIDs[column.ID] = struct{}{}
	}
	rowIDs := make(map[RowID]struct{}, len(dataset.Rows))
	sourceIndexes := make(map[uint64]struct{}, len(dataset.Rows))
	for rowIndex, row := range dataset.Rows {
		if row.ID == "" || len(row.Cells) != len(dataset.Columns) {
			return fmt.Errorf("%w: invalid row %d shape", ErrInvalidTable, rowIndex)
		}
		if _, duplicate := rowIDs[row.ID]; duplicate {
			return fmt.Errorf("%w: duplicate row ID %q", ErrInvalidTable, row.ID)
		}
		if _, duplicate := sourceIndexes[row.SourceIndex]; duplicate {
			return fmt.Errorf("%w: duplicate source index %d", ErrInvalidTable, row.SourceIndex)
		}
		rowIDs[row.ID] = struct{}{}
		sourceIndexes[row.SourceIndex] = struct{}{}
		for columnIndex, cell := range row.Cells {
			if err := cell.Validate(); err != nil || cell.Kind != dataset.Columns[columnIndex].Kind {
				return fmt.Errorf("%w: row %d column %d type mismatch", ErrInvalidTable, rowIndex, columnIndex)
			}
		}
	}
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
