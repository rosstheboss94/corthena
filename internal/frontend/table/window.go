package table

import (
	"fmt"
	"math"
)

// WindowRequest describes scroll and viewport geometry in logical pixels.
type WindowRequest struct {
	OriginX         float64
	OriginY         float64
	ScrollX         float64
	ScrollY         float64
	Width           float64
	Height          float64
	HeaderHeight    float64
	RowHeight       float64
	OverscanRows    int
	OverscanColumns int
}

// VisibleColumn is one pinned or horizontally visible column.
type VisibleColumn struct {
	SourceIndex int
	Column      Column
	X           float64
	Width       float64
	Pinned      bool
}

// VisibleCell is one cell the native adapter may measure and render.
type VisibleCell struct {
	RowIndex    int
	ColumnIndex int
	RowID       RowID
	Cell        Cell
	X           float64
	Y           float64
	Width       float64
	Height      float64
}

// Window contains only visible cells plus bounded overscan.
type Window struct {
	HeaderY       float64
	RowStart      int
	RowEnd        int
	Columns       []VisibleColumn
	Cells         []VisibleCell
	MeasuredCells int
	TotalWidth    float64
	TotalHeight   float64
}

// Model is an immutable, prevalidated table snapshot. Construct it off the
// render thread, then call Virtualize per frame without rescanning source rows.
type Model struct {
	dataset Dataset
}

// Clone returns an independent immutable prevalidated model.
func (model Model) Clone() Model {
	return Model{dataset: model.dataset.Clone()}
}

// NewModel validates and owns a deep immutable dataset copy.
func NewModel(dataset Dataset) (Model, error) {
	if err := dataset.Validate(); err != nil {
		return Model{}, err
	}
	return Model{dataset: dataset.Clone()}, nil
}

// Virtualize computes one frame window without work proportional to total
// rows.
func (model Model) Virtualize(request WindowRequest) (Window, error) {
	return virtualizeValidated(model.dataset, request)
}

// Virtualize computes visible row and column windows. Cell work is exactly
// len(visible rows) multiplied by len(visible columns), independent of total
// row count.
func Virtualize(dataset Dataset, request WindowRequest) (Window, error) {
	if err := dataset.Validate(); err != nil {
		return Window{}, err
	}
	return virtualizeValidated(dataset, request)
}

func virtualizeValidated(dataset Dataset, request WindowRequest) (Window, error) {
	if !finite(request.OriginX) || !finite(request.OriginY) || !finite(request.ScrollX) || !finite(request.ScrollY) || !finite(request.Width) || !finite(request.Height) ||
		!finite(request.HeaderHeight) || !finite(request.RowHeight) || request.Width <= 0 || request.Height <= 0 ||
		request.HeaderHeight < 0 || request.RowHeight <= 0 || request.OverscanRows < 0 || request.OverscanColumns < 0 {
		return Window{}, fmt.Errorf("%w: invalid table viewport", ErrInvalidTable)
	}
	request.ScrollX = max(0, request.ScrollX)
	request.ScrollY = max(0, request.ScrollY)
	bodyHeight := max(0, request.Height-request.HeaderHeight)
	firstRow := int(math.Floor(request.ScrollY/request.RowHeight)) - request.OverscanRows
	firstRow = max(0, min(firstRow, len(dataset.Rows)))
	visibleRows := int(math.Ceil(bodyHeight/request.RowHeight)) + 2*request.OverscanRows
	lastRow := min(len(dataset.Rows), firstRow+visibleRows)

	columns, totalWidth := visibleColumns(dataset.Columns, request)
	window := Window{
		HeaderY:  request.OriginY,
		RowStart: firstRow, RowEnd: lastRow, Columns: columns,
		TotalWidth: totalWidth, TotalHeight: request.HeaderHeight + float64(len(dataset.Rows))*request.RowHeight,
	}
	rowCount := lastRow - firstRow
	window.Cells = make([]VisibleCell, 0, rowCount*len(columns))
	for rowIndex := firstRow; rowIndex < lastRow; rowIndex++ {
		row := dataset.Rows[rowIndex]
		y := request.OriginY + request.HeaderHeight + float64(rowIndex)*request.RowHeight - request.ScrollY
		for _, column := range columns {
			window.Cells = append(window.Cells, VisibleCell{
				RowIndex: rowIndex, ColumnIndex: column.SourceIndex, RowID: row.ID,
				Cell: row.Cells[column.SourceIndex], X: column.X, Y: y, Width: column.Width, Height: request.RowHeight,
			})
		}
	}
	window.MeasuredCells = len(window.Cells)
	return window, nil
}

func visibleColumns(columns []Column, request WindowRequest) ([]VisibleColumn, float64) {
	pinnedWidth := 0.0
	totalWidth := 0.0
	result := make([]VisibleColumn, 0, len(columns))
	for index, column := range columns {
		totalWidth += column.Width
		if column.Pinned {
			result = append(result, VisibleColumn{SourceIndex: index, Column: column, X: request.OriginX + pinnedWidth, Width: column.Width, Pinned: true})
			pinnedWidth += column.Width
		}
	}
	left := request.ScrollX
	right := request.ScrollX + max(0, request.Width-pinnedWidth)
	unpinnedX := 0.0
	firstVisible := -1
	lastVisible := -1
	for index, column := range columns {
		if column.Pinned {
			continue
		}
		columnLeft := unpinnedX
		columnRight := unpinnedX + column.Width
		if columnRight >= left && columnLeft <= right {
			if firstVisible < 0 {
				firstVisible = index
			}
			lastVisible = index
		}
		unpinnedX = columnRight
	}
	if firstVisible >= 0 {
		start := max(0, firstVisible-request.OverscanColumns)
		end := min(len(columns)-1, lastVisible+request.OverscanColumns)
		x := 0.0
		for index, column := range columns {
			if column.Pinned {
				continue
			}
			if index >= start && index <= end {
				result = append(result, VisibleColumn{SourceIndex: index, Column: column, X: request.OriginX + pinnedWidth + x - request.ScrollX, Width: column.Width})
			}
			x += column.Width
		}
	}
	return result, totalWidth
}

// ResizeColumn clamps one header resize to its typed limits.
func ResizeColumn(columns []Column, id ColumnID, width float64) ([]Column, error) {
	if !finite(width) {
		return nil, fmt.Errorf("%w: non-finite column width", ErrInvalidTable)
	}
	result := append([]Column(nil), columns...)
	for index := range result {
		if result[index].ID == id {
			result[index].Width = min(max(width, result[index].MinWidth), result[index].MaxWidth)
			return result, nil
		}
	}
	return nil, fmt.Errorf("%w: unknown column %q", ErrInvalidTable, id)
}
