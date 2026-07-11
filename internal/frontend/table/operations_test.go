package table

import (
	"errors"
	"reflect"
	"testing"
)

func TestTypedSortUsesExplicitNullOrderAndStableSourceTieBreak(t *testing.T) {
	t.Parallel()
	dataset := Dataset{
		Columns: []Column{{ID: "value", Title: "Value", Kind: CellFloat, Width: 100, MinWidth: 50, MaxWidth: 200, Sortable: true}},
		Rows: []Row{
			{ID: "third", SourceIndex: 3, Cells: []Cell{{Kind: CellFloat, Float: 2}}},
			{ID: "null", SourceIndex: 4, Cells: []Cell{{Kind: CellFloat, Null: true}}},
			{ID: "first", SourceIndex: 1, Cells: []Cell{{Kind: CellFloat, Float: 2}}},
			{ID: "low", SourceIndex: 2, Cells: []Cell{{Kind: CellFloat, Float: 1}}},
		},
	}
	sorted, err := Sort(dataset, []SortSpec{{Column: "value", Direction: SortDescending, Nulls: NullsFirst}})
	if err != nil {
		t.Fatal(err)
	}
	want := []RowID{"null", "first", "third", "low"}
	if got := rowIDs(sorted.Rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("row order = %v, want %v", got, want)
	}
}

func TestSelectionFollowsIDsAcrossSortFilterPaginationAndUpdates(t *testing.T) {
	t.Parallel()
	dataset := testDataset(8, 3)
	selection, err := Select(Selection{}, dataset.Rows, "row-2", false, false)
	if err != nil {
		t.Fatal(err)
	}
	selection, err = Select(selection, dataset.Rows, "row-5", false, true)
	if err != nil {
		t.Fatal(err)
	}
	wantRange := []RowID{"row-2", "row-3", "row-4", "row-5"}
	if !reflect.DeepEqual(selection.IDs, wantRange) {
		t.Fatalf("range selection = %v", selection.IDs)
	}
	sorted, err := Sort(dataset, []SortSpec{{Column: "column-1", Direction: SortDescending, Nulls: NullsLast}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range wantRange {
		if !selection.Contains(id) {
			t.Fatalf("selection lost %q after sort", id)
		}
	}
	filtered, err := Filter(sorted, []FilterSpec{{Column: "column-1", Operator: FilterGreaterEqual, Value: Cell{Kind: CellInteger, Integer: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	pruned := PruneSelection(selection, filtered.Rows)
	for _, id := range pruned.IDs {
		if !selection.Contains(id) {
			t.Fatalf("prune introduced ID %q", id)
		}
	}
	page := filtered.Rows[:min(2, len(filtered.Rows))]
	prunedPage := PruneSelection(selection, page)
	if len(prunedPage.IDs) > 2 {
		t.Fatalf("page selection = %v", prunedPage.IDs)
	}
	updated := dataset.Clone()
	updated.Rows[2].Cells[1].Integer = 999
	if !selection.Contains(updated.Rows[2].ID) {
		t.Fatal("cell update changed row selection")
	}
}

func TestFilterAndBoundedCopy(t *testing.T) {
	t.Parallel()
	dataset := testDataset(4, 3)
	filtered, err := Filter(dataset, []FilterSpec{{Column: "column-0", Operator: FilterContains, Value: Cell{Kind: CellString, String: "row-2"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Rows) != 1 || filtered.Rows[0].ID != "row-2" {
		t.Fatalf("filtered = %+v", filtered.Rows)
	}
	selection := Selection{IDs: []RowID{"row-1", "row-2"}}
	output, err := CopySelection(dataset, selection, []ColumnID{"column-0", "column-1"}, true, 1024)
	if err != nil {
		t.Fatal(err)
	}
	want := "Column 0\tColumn 1\nrow-1\t4\nrow-2\t7\n"
	if output != want {
		t.Fatalf("copy = %q, want %q", output, want)
	}
	if output, err := CopySelection(dataset, selection, []ColumnID{"column-0", "column-1"}, true, 8); !errors.Is(err, ErrCopyLimit) || output != "" {
		t.Fatalf("bounded copy = %q err=%v", output, err)
	}
}

func TestKeyboardSelectionClampsAtBounds(t *testing.T) {
	t.Parallel()
	rows := testDataset(3, 2).Rows
	selection, err := MoveSelection(Selection{}, rows, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selection.IDs, []RowID{"row-2"}) {
		t.Fatalf("selection = %v", selection.IDs)
	}
	selection, err = MoveSelection(selection, rows, -100, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selection.IDs, []RowID{"row-0"}) {
		t.Fatalf("selection = %v", selection.IDs)
	}
}

func rowIDs(rows []Row) []RowID {
	result := make([]RowID, len(rows))
	for index, row := range rows {
		result[index] = row.ID
	}
	return result
}
