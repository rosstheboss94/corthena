package table

import "testing"

func FuzzVirtualizeGeometry(f *testing.F) {
	model, err := NewModel(testDataset(100, 8))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(0.0, 0.0, 640.0, 480.0, 24.0, 24.0, uint8(2), uint8(1))
	f.Add(-100.0, 1e12, 1.0, 1.0, 0.0, 0.001, uint8(0), uint8(0))
	f.Fuzz(func(t *testing.T, scrollX float64, scrollY float64, width float64, height float64, headerHeight float64, rowHeight float64, rowOverscan uint8, columnOverscan uint8) {
		window, err := model.Virtualize(WindowRequest{
			ScrollX: scrollX, ScrollY: scrollY, Width: width, Height: height,
			HeaderHeight: headerHeight, RowHeight: rowHeight,
			OverscanRows: int(rowOverscan), OverscanColumns: int(columnOverscan),
		})
		if err != nil {
			return
		}
		if window.MeasuredCells != len(window.Cells) || window.RowStart < 0 || window.RowEnd < window.RowStart || window.RowEnd > 100 {
			t.Fatalf("invalid window: %+v", window)
		}
	})
}
