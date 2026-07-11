package table

import "testing"

func TestVirtualizationWindowsAreBoundedAtFirstMiddleLastAndOverscroll(t *testing.T) {
	t.Parallel()
	model, err := NewModel(testDataset(100_000, 20))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		scrollX float64
		scrollY float64
	}{
		{name: "first"},
		{name: "middle", scrollX: 900, scrollY: 900_000},
		{name: "last", scrollX: 10_000, scrollY: 2_399_700},
		{name: "overscrolled", scrollX: 1_000_000, scrollY: 10_000_000},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			window, err := model.Virtualize(WindowRequest{
				ScrollX: test.scrollX, ScrollY: test.scrollY, Width: 640, Height: 300,
				HeaderHeight: 24, RowHeight: 24, OverscanRows: 2, OverscanColumns: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(window.Columns) > 10 {
				t.Fatalf("visible columns = %d", len(window.Columns))
			}
			if window.RowEnd-window.RowStart > 16 {
				t.Fatalf("visible rows = %d", window.RowEnd-window.RowStart)
			}
			if window.MeasuredCells != len(window.Cells) || window.MeasuredCells > 160 {
				t.Fatalf("measured cells = %d", window.MeasuredCells)
			}
			if len(window.Columns) > 0 && !window.Columns[0].Pinned {
				t.Fatal("pinned identifier column is not retained")
			}
		})
	}
}

func TestResizeColumnClampsTypedLimits(t *testing.T) {
	t.Parallel()
	columns := testDataset(0, 2).Columns
	resized, err := ResizeColumn(columns, columns[1].ID, 10_000)
	if err != nil {
		t.Fatal(err)
	}
	if resized[1].Width != resized[1].MaxWidth || columns[1].Width == resized[1].Width {
		t.Fatalf("resized = %+v source=%+v", resized[1], columns[1])
	}
}

func BenchmarkVirtualizedLargeTable(b *testing.B) {
	const sourceRows = 100_000
	const sourceColumns = 32
	model, err := NewModel(testDataset(sourceRows, sourceColumns))
	if err != nil {
		b.Fatal(err)
	}
	request := WindowRequest{ScrollX: 1200, ScrollY: 1_200_000, Width: 1280, Height: 720, HeaderHeight: 28, RowHeight: 24, OverscanRows: 2, OverscanColumns: 1}
	b.ReportAllocs()
	b.ResetTimer()
	measuredCells := 0
	for range b.N {
		window, err := model.Virtualize(request)
		if err != nil {
			b.Fatal(err)
		}
		measuredCells = window.MeasuredCells
	}
	b.ReportMetric(sourceRows, "source_rows/op")
	b.ReportMetric(sourceColumns, "source_columns/op")
	b.ReportMetric(float64(measuredCells), "measured_cells/op")
}

func testDataset(rows int, columns int) Dataset {
	schema := make([]Column, columns)
	for column := range schema {
		kind := CellInteger
		if column == 0 {
			kind = CellString
		}
		schema[column] = Column{
			ID: ColumnID("column-" + integerString(column)), Title: "Column " + integerString(column), Kind: kind,
			Width: 120, MinWidth: 60, MaxWidth: 300, Pinned: column == 0, Sortable: true,
		}
	}
	values := make([]Row, rows)
	for row := range values {
		cells := make([]Cell, columns)
		for column := range cells {
			if column == 0 {
				cells[column] = Cell{Kind: CellString, String: "row-" + integerString(row)}
			} else {
				cells[column] = Cell{Kind: CellInteger, Integer: int64(row*columns + column)}
			}
		}
		values[row] = Row{ID: RowID("row-" + integerString(row)), Cells: cells, SourceIndex: uint64(row + 1)}
	}
	return Dataset{Columns: schema, Rows: values}
}

func integerString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}
