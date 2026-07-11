package table

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SortDirection is a closed deterministic sort order.
type SortDirection uint8

const (
	SortAscending SortDirection = iota + 1
	SortDescending
)

// NullOrder explicitly controls null placement independently of direction.
type NullOrder uint8

const (
	NullsFirst NullOrder = iota + 1
	NullsLast
)

// SortSpec is one typed stable sort key.
type SortSpec struct {
	Column    ColumnID
	Direction SortDirection
	Nulls     NullOrder
}

// Sort returns a cloned dataset ordered by typed values, explicit null rules,
// subsequent sort keys, and finally stable SourceIndex.
func Sort(dataset Dataset, specs []SortSpec) (Dataset, error) {
	if err := dataset.Validate(); err != nil {
		return Dataset{}, err
	}
	indexes := make([]int, len(specs))
	for index, spec := range specs {
		columnIndex, found := columnIndex(dataset.Columns, spec.Column)
		if !found || !dataset.Columns[columnIndex].Sortable ||
			(spec.Direction != SortAscending && spec.Direction != SortDescending) ||
			(spec.Nulls != NullsFirst && spec.Nulls != NullsLast) {
			return Dataset{}, fmt.Errorf("%w: invalid sort key %d", ErrInvalidTable, index)
		}
		indexes[index] = columnIndex
	}
	result := dataset.Clone()
	sort.SliceStable(result.Rows, func(left int, right int) bool {
		for index, spec := range specs {
			leftCell := result.Rows[left].Cells[indexes[index]]
			rightCell := result.Rows[right].Cells[indexes[index]]
			comparison := compareCells(leftCell, rightCell, spec.Nulls)
			if comparison == 0 {
				continue
			}
			if spec.Direction == SortDescending && !leftCell.Null && !rightCell.Null {
				comparison = -comparison
			}
			return comparison < 0
		}
		return result.Rows[left].SourceIndex < result.Rows[right].SourceIndex
	})
	return result, nil
}

// FilterOperator is an explicit typed predicate.
type FilterOperator uint8

const (
	FilterEqual FilterOperator = iota + 1
	FilterNotEqual
	FilterLess
	FilterLessEqual
	FilterGreater
	FilterGreaterEqual
	FilterContains
	FilterIsNull
	FilterIsNotNull
)

// FilterSpec is one conjunctive server/local filter.
type FilterSpec struct {
	Column   ColumnID
	Operator FilterOperator
	Value    Cell
}

// Filter applies deterministic conjunctive typed predicates and preserves
// source order and stable row IDs.
func Filter(dataset Dataset, specs []FilterSpec) (Dataset, error) {
	if err := dataset.Validate(); err != nil {
		return Dataset{}, err
	}
	indexes := make([]int, len(specs))
	for index, spec := range specs {
		column, found := columnIndex(dataset.Columns, spec.Column)
		if !found || !validFilter(spec, dataset.Columns[column].Kind) {
			return Dataset{}, fmt.Errorf("%w: invalid filter %d", ErrInvalidTable, index)
		}
		indexes[index] = column
	}
	result := Dataset{Columns: append([]Column(nil), dataset.Columns...), Rows: make([]Row, 0, len(dataset.Rows))}
	for _, row := range dataset.Rows {
		matches := true
		for index, spec := range specs {
			if !matchesFilter(row.Cells[indexes[index]], spec) {
				matches = false
				break
			}
		}
		if matches {
			result.Rows = append(result.Rows, row.Clone())
		}
	}
	return result, nil
}

func validFilter(spec FilterSpec, kind CellKind) bool {
	switch spec.Operator {
	case FilterIsNull, FilterIsNotNull:
		return true
	case FilterContains:
		return kind == CellString && spec.Value.Kind == CellString && !spec.Value.Null
	case FilterEqual, FilterNotEqual, FilterLess, FilterLessEqual, FilterGreater, FilterGreaterEqual:
		return spec.Value.Kind == kind && !spec.Value.Null && spec.Value.Validate() == nil
	default:
		return false
	}
}

func matchesFilter(cell Cell, spec FilterSpec) bool {
	switch spec.Operator {
	case FilterIsNull:
		return cell.Null
	case FilterIsNotNull:
		return !cell.Null
	case FilterContains:
		return !cell.Null && strings.Contains(cell.String, spec.Value.String)
	}
	if cell.Null {
		return false
	}
	comparison := compareCells(cell, spec.Value, NullsLast)
	switch spec.Operator {
	case FilterEqual:
		return comparison == 0
	case FilterNotEqual:
		return comparison != 0
	case FilterLess:
		return comparison < 0
	case FilterLessEqual:
		return comparison <= 0
	case FilterGreater:
		return comparison > 0
	case FilterGreaterEqual:
		return comparison >= 0
	default:
		return false
	}
}

func compareCells(left Cell, right Cell, nulls NullOrder) int {
	if left.Null || right.Null {
		if left.Null && right.Null {
			return 0
		}
		if left.Null {
			if nulls == NullsFirst {
				return -1
			}
			return 1
		}
		if nulls == NullsFirst {
			return 1
		}
		return -1
	}
	switch left.Kind {
	case CellString:
		return strings.Compare(left.String, right.String)
	case CellInteger:
		return compareOrdered(left.Integer, right.Integer)
	case CellFloat:
		return compareOrdered(left.Float, right.Float)
	case CellBoolean:
		return compareOrdered(boolInt(left.Boolean), boolInt(right.Boolean))
	case CellTime:
		return compareOrdered(left.Time.UnixNano(), right.Time.UnixNano())
	default:
		return 0
	}
}

func compareOrdered[T ~int | ~int64 | ~float64](left T, right T) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func columnIndex(columns []Column, id ColumnID) (int, bool) {
	for index, column := range columns {
		if column.ID == id {
			return index, true
		}
	}
	return 0, false
}

// Selection follows stable row IDs through view changes.
type Selection struct {
	IDs    []RowID
	Anchor RowID
}

// Clone returns an independent selection.
func (selection Selection) Clone() Selection {
	selection.IDs = append([]RowID(nil), selection.IDs...)
	return selection
}

// Contains reports whether a stable ID is selected.
func (selection Selection) Contains(id RowID) bool {
	for _, selected := range selection.IDs {
		if selected == id {
			return true
		}
	}
	return false
}

// Select applies single, toggle, or range selection against current visible
// row order.
func Select(selection Selection, rows []Row, id RowID, toggle bool, extend bool) (Selection, error) {
	position, found := rowPosition(rows, id)
	if !found {
		return selection, fmt.Errorf("%w: selected row %q is missing", ErrInvalidTable, id)
	}
	next := selection.Clone()
	if extend && next.Anchor != "" {
		anchor, anchorFound := rowPosition(rows, next.Anchor)
		if anchorFound {
			start, end := min(anchor, position), max(anchor, position)
			next.IDs = make([]RowID, 0, end-start+1)
			for index := start; index <= end; index++ {
				next.IDs = append(next.IDs, rows[index].ID)
			}
			return next, nil
		}
	}
	if toggle {
		for index, selected := range next.IDs {
			if selected == id {
				next.IDs = append(next.IDs[:index], next.IDs[index+1:]...)
				next.Anchor = id
				return next, nil
			}
		}
		next.IDs = append(next.IDs, id)
		next.Anchor = id
		return next, nil
	}
	return Selection{IDs: []RowID{id}, Anchor: id}, nil
}

// MoveSelection applies deterministic keyboard movement in current row order.
func MoveSelection(selection Selection, rows []Row, delta int, extend bool) (Selection, error) {
	if len(rows) == 0 {
		return Selection{}, nil
	}
	current := 0
	if len(selection.IDs) > 0 {
		if position, found := rowPosition(rows, selection.IDs[len(selection.IDs)-1]); found {
			current = position
		}
	}
	target := min(max(current+delta, 0), len(rows)-1)
	return Select(selection, rows, rows[target].ID, false, extend)
}

// PruneSelection drops IDs no longer present while retaining relative order.
func PruneSelection(selection Selection, rows []Row) Selection {
	present := make(map[RowID]struct{}, len(rows))
	for _, row := range rows {
		present[row.ID] = struct{}{}
	}
	next := Selection{Anchor: selection.Anchor, IDs: make([]RowID, 0, len(selection.IDs))}
	for _, id := range selection.IDs {
		if _, found := present[id]; found {
			next.IDs = append(next.IDs, id)
		}
	}
	if _, found := present[next.Anchor]; !found {
		next.Anchor = ""
	}
	return next
}

// CopySelection returns bounded TSV output in current row and requested column
// order. It fails without returning partial output when maxBytes is exceeded.
func CopySelection(dataset Dataset, selection Selection, columns []ColumnID, includeHeader bool, maxBytes int) (string, error) {
	if err := dataset.Validate(); err != nil {
		return "", err
	}
	if maxBytes <= 0 {
		return "", fmt.Errorf("%w: limit must be positive", ErrCopyLimit)
	}
	indexes := make([]int, len(columns))
	for index, id := range columns {
		column, found := columnIndex(dataset.Columns, id)
		if !found {
			return "", fmt.Errorf("%w: unknown copy column %q", ErrInvalidTable, id)
		}
		indexes[index] = column
	}
	selected := make(map[RowID]struct{}, len(selection.IDs))
	for _, id := range selection.IDs {
		selected[id] = struct{}{}
	}
	var output bytes.Buffer
	appendLine := func(values []string) error {
		line := strings.Join(values, "\t") + "\n"
		if output.Len()+len(line) > maxBytes {
			return fmt.Errorf("%w: output exceeds %d bytes", ErrCopyLimit, maxBytes)
		}
		output.WriteString(line)
		return nil
	}
	if includeHeader {
		headings := make([]string, len(indexes))
		for index, column := range indexes {
			headings[index] = escapeTSV(dataset.Columns[column].Title)
		}
		if err := appendLine(headings); err != nil {
			return "", err
		}
	}
	for _, row := range dataset.Rows {
		if _, found := selected[row.ID]; !found {
			continue
		}
		values := make([]string, len(indexes))
		for index, column := range indexes {
			values[index] = formatCell(row.Cells[column])
		}
		if err := appendLine(values); err != nil {
			return "", err
		}
	}
	return output.String(), nil
}

func rowPosition(rows []Row, id RowID) (int, bool) {
	for index, row := range rows {
		if row.ID == id {
			return index, true
		}
	}
	return 0, false
}

func formatCell(cell Cell) string {
	if cell.Null {
		return ""
	}
	switch cell.Kind {
	case CellString:
		return escapeTSV(cell.String)
	case CellInteger:
		return strconv.FormatInt(cell.Integer, 10)
	case CellFloat:
		return strconv.FormatFloat(cell.Float, 'g', -1, 64)
	case CellBoolean:
		return strconv.FormatBool(cell.Boolean)
	case CellTime:
		return cell.Time.UTC().Format(time.RFC3339Nano)
	default:
		return ""
	}
}

func escapeTSV(value string) string {
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}
