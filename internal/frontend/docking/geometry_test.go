package docking

import (
	"math"
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestCalculateGeometryNestedSplitsAndPhysicalBounds(t *testing.T) {
	t.Parallel()

	chart := testPanel(t, "panel-chart", appstate.PanelOHLCVChart)
	features := testPanel(t, "panel-features", appstate.PanelFeatureBrowser)
	rows := testPanel(t, "panel-rows", appstate.PanelRowTable)
	root := appstate.SplitNode{
		ID:          "split-root",
		Orientation: appstate.SplitHorizontal,
		Ratio:       0.6,
		First: appstate.TabStackNode{
			ID: "stack-chart", Active: chart.ID, Panels: []appstate.PanelInstanceState{chart},
		},
		Second: appstate.SplitNode{
			ID:          "split-secondary",
			Orientation: appstate.SplitVertical,
			Ratio:       0.5,
			First: appstate.TabStackNode{
				ID: "stack-features", Active: features.ID, Panels: []appstate.PanelInstanceState{features},
			},
			Second: appstate.TabStackNode{
				ID: "stack-rows", Active: rows.ID, Panels: []appstate.PanelInstanceState{rows},
			},
		},
	}

	geometry, err := CalculateGeometry(root, Rect{Width: 1200, Height: 800}, GeometryOptions{
		DPIScale:          1.5,
		SplitterThickness: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSize(t, geometry.Minimum, Size{Width: 944, Height: 444})
	assertRect(t, geometry.Host.Physical, Rect{Width: 1800, Height: 1200})
	if len(geometry.Nodes) != 5 {
		t.Fatalf("nodes = %d, want 5", len(geometry.Nodes))
	}
	if len(geometry.Splitters) != 2 {
		t.Fatalf("splitters = %d, want 2", len(geometry.Splitters))
	}

	chartGeometry, found := geometry.Node("stack-chart")
	if !found {
		t.Fatal("chart stack geometry not found")
	}
	assertRect(t, chartGeometry.Bounds.Logical, Rect{Width: 717.6, Height: 800})
	assertRect(t, chartGeometry.Bounds.Physical, Rect{Width: 1076.4, Height: 1200})
	secondary, found := geometry.Node("split-secondary")
	if !found {
		t.Fatal("secondary split geometry not found")
	}
	assertRect(t, secondary.Bounds.Logical, Rect{X: 721.6, Width: 478.4, Height: 800})
	featuresGeometry, found := geometry.Node("stack-features")
	if !found {
		t.Fatal("features stack geometry not found")
	}
	assertRect(t, featuresGeometry.Bounds.Logical, Rect{X: 721.6, Width: 478.4, Height: 398})
	rowsGeometry, found := geometry.Node("stack-rows")
	if !found {
		t.Fatal("rows stack geometry not found")
	}
	assertRect(t, rowsGeometry.Bounds.Logical, Rect{X: 721.6, Y: 402, Width: 478.4, Height: 398})

	rootSplitter, found := geometry.Splitter("split-root")
	if !found {
		t.Fatal("root splitter geometry not found")
	}
	assertRect(t, rootSplitter.Bounds.Logical, Rect{X: 717.6, Width: 4, Height: 800})
	assertRect(t, rootSplitter.Bounds.Physical, Rect{X: 1076.4, Width: 6, Height: 1200})
	assertClose(t, rootSplitter.EffectiveRatio, 0.6)
}

func TestCalculateGeometryUndersizedHostIsDeterministic(t *testing.T) {
	t.Parallel()

	chart := testPanel(t, "panel-chart", appstate.PanelOHLCVChart)
	rows := testPanel(t, "panel-rows", appstate.PanelRowTable)
	root := appstate.SplitNode{
		ID:          "split-root",
		Orientation: appstate.SplitHorizontal,
		Ratio:       0.9,
		First: appstate.TabStackNode{
			ID: "stack-chart", Active: chart.ID, Panels: []appstate.PanelInstanceState{chart},
		},
		Second: appstate.TabStackNode{
			ID: "stack-rows", Active: rows.ID, Panels: []appstate.PanelInstanceState{rows},
		},
	}

	first, err := CalculateGeometry(root, Rect{X: 10, Y: 20, Width: 300, Height: 100}, DefaultGeometryOptions())
	if err != nil {
		t.Fatal(err)
	}
	second, err := CalculateGeometry(root, Rect{X: 10, Y: 20, Width: 300, Height: 100}, DefaultGeometryOptions())
	if err != nil {
		t.Fatal(err)
	}
	firstStack, _ := first.Node("stack-chart")
	secondStack, _ := first.Node("stack-rows")
	wantFirstWidth := 296.0 * 520.0 / (520.0 + 420.0)
	assertClose(t, firstStack.Bounds.Logical.Width, wantFirstWidth)
	assertClose(t, secondStack.Bounds.Logical.Width, 296-wantFirstWidth)
	assertClose(t, firstStack.Bounds.Logical.Width+4+secondStack.Bounds.Logical.Width, 300)
	if len(first.Nodes) != len(second.Nodes) {
		t.Fatalf("repeated calculation node counts differ: %d and %d", len(first.Nodes), len(second.Nodes))
	}
	for index := range first.Nodes {
		assertRect(t, first.Nodes[index].Bounds.Logical, second.Nodes[index].Bounds.Logical)
	}

	tiny, err := CalculateGeometry(root, Rect{Width: 2, Height: 50}, DefaultGeometryOptions())
	if err != nil {
		t.Fatal(err)
	}
	tinyFirst, _ := tiny.Node("stack-chart")
	tinySecond, _ := tiny.Node("stack-rows")
	if tinyFirst.Bounds.Logical.Width < 0 || tinySecond.Bounds.Logical.Width < 0 {
		t.Fatalf("tiny host produced negative panes: %#v %#v", tinyFirst.Bounds.Logical, tinySecond.Bounds.Logical)
	}
	tinySplitter, _ := tiny.Splitter("split-root")
	assertClose(t, tinySplitter.LogicalThickness, 2)
}

func TestCalculateGeometryResolutionAndDPIMatrix(t *testing.T) {
	t.Parallel()

	panel := testPanel(t, "panel-a", appstate.PanelDatasetInspector)
	root := appstate.TabStackNode{ID: "stack-a", Active: panel.ID, Panels: []appstate.PanelInstanceState{panel}}
	tests := []struct {
		name   string
		width  float64
		height float64
		scale  float64
	}{
		{name: "1280x720-100", width: 1280, height: 720, scale: 1},
		{name: "1920x1080-150", width: 1920, height: 1080, scale: 1.5},
		{name: "2560x1440-200", width: 2560, height: 1440, scale: 2},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			logical := Rect{Width: test.width / test.scale, Height: test.height / test.scale}
			geometry, err := CalculateGeometry(root, logical, GeometryOptions{
				DPIScale:          test.scale,
				SplitterThickness: DefaultSplitterThickness,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertRect(t, geometry.Host.Physical, Rect{Width: test.width, Height: test.height})
			node, found := geometry.Node("stack-a")
			if !found {
				t.Fatal("stack geometry not found")
			}
			assertRect(t, node.Bounds.Logical, logical)
			assertRect(t, node.Bounds.Physical, Rect{Width: test.width, Height: test.height})
		})
	}
}

func TestClampSplitRatio(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{name: "NaN", input: math.NaN(), want: 0.5},
		{name: "negative infinity", input: math.Inf(-1), want: MinimumSplitRatio},
		{name: "positive infinity", input: math.Inf(1), want: MaximumSplitRatio},
		{name: "too low", input: -1, want: MinimumSplitRatio},
		{name: "too high", input: 2, want: MaximumSplitRatio},
		{name: "finite", input: 0.625, want: 0.625},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertClose(t, ClampSplitRatio(test.input), test.want)
		})
	}
}

func TestCalculateGeometryClampsStoredRatio(t *testing.T) {
	t.Parallel()

	firstPanel := testPanel(t, "panel-first", appstate.PanelDatasetInspector)
	secondPanel := testPanel(t, "panel-second", appstate.PanelDatasetInspector)
	base := appstate.SplitNode{
		ID:          "split-root",
		Orientation: appstate.SplitHorizontal,
		First: appstate.TabStackNode{
			ID: "stack-first", Active: firstPanel.ID, Panels: []appstate.PanelInstanceState{firstPanel},
		},
		Second: appstate.TabStackNode{
			ID: "stack-second", Active: secondPanel.ID, Panels: []appstate.PanelInstanceState{secondPanel},
		},
	}
	tests := []struct {
		name  string
		ratio float64
		want  float64
	}{
		{name: "NaN", ratio: math.NaN(), want: 0.5},
		{name: "negative infinity", ratio: math.Inf(-1), want: MinimumSplitRatio},
		{name: "positive infinity", ratio: math.Inf(1), want: MaximumSplitRatio},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := base
			root.Ratio = test.ratio
			geometry, err := CalculateGeometry(root, Rect{Width: 10000, Height: 500}, DefaultGeometryOptions())
			if err != nil {
				t.Fatal(err)
			}
			splitter, found := geometry.Splitter("split-root")
			if !found {
				t.Fatal("splitter not found")
			}
			assertClose(t, splitter.EffectiveRatio, test.want)
		})
	}
}

func TestDropTargetRectsAndHitTesting(t *testing.T) {
	t.Parallel()

	host := Rect{X: 10, Y: 20, Width: 100, Height: 80}
	targets, err := DropTargetRects(host)
	if err != nil {
		t.Fatal(err)
	}
	wants := []DropTargetRect{
		{Position: DockLeft, Rect: Rect{X: 10, Y: 20, Width: 25, Height: 80}},
		{Position: DockRight, Rect: Rect{X: 85, Y: 20, Width: 25, Height: 80}},
		{Position: DockTop, Rect: Rect{X: 35, Y: 20, Width: 50, Height: 20}},
		{Position: DockBottom, Rect: Rect{X: 35, Y: 80, Width: 50, Height: 20}},
		{Position: DockCenter, Rect: Rect{X: 35, Y: 40, Width: 50, Height: 40}},
	}
	if len(targets) != len(wants) {
		t.Fatalf("targets = %d, want %d", len(targets), len(wants))
	}
	for index := range wants {
		if targets[index].Position != wants[index].Position {
			t.Fatalf("target %d position = %q, want %q", index, targets[index].Position, wants[index].Position)
		}
		assertRect(t, targets[index].Rect, wants[index].Rect)
	}

	hits := []struct {
		point Point
		want  DockPosition
	}{
		{point: Point{X: 11, Y: 21}, want: DockLeft},
		{point: Point{X: 109, Y: 99}, want: DockRight},
		{point: Point{X: 50, Y: 21}, want: DockTop},
		{point: Point{X: 50, Y: 99}, want: DockBottom},
		{point: Point{X: 50, Y: 50}, want: DockCenter},
	}
	for _, hit := range hits {
		got, found := HitTestDropTargets(targets, hit.point)
		if !found || got != hit.want {
			t.Fatalf("point %#v hit (%q, %t), want %q", hit.point, got, found, hit.want)
		}
	}
	if got, found := HitTestDropTargets(targets, Point{X: 110, Y: 100}); found {
		t.Fatalf("right-bottom boundary hit %q, want no target", got)
	}
}

func TestCalculateGeometryRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	panel := testPanel(t, "panel-a", appstate.PanelDatasetInspector)
	root := appstate.TabStackNode{ID: "stack-a", Active: panel.ID, Panels: []appstate.PanelInstanceState{panel}}
	tests := []struct {
		name    string
		host    Rect
		options GeometryOptions
	}{
		{name: "negative host", host: Rect{Width: -1, Height: 10}, options: DefaultGeometryOptions()},
		{name: "non-finite host", host: Rect{Width: math.NaN(), Height: 10}, options: DefaultGeometryOptions()},
		{name: "zero scale", host: Rect{Width: 10, Height: 10}, options: GeometryOptions{DPIScale: 0, SplitterThickness: 4}},
		{name: "negative splitter", host: Rect{Width: 10, Height: 10}, options: GeometryOptions{DPIScale: 1, SplitterThickness: -1}},
		{name: "physical overflow", host: Rect{Width: math.MaxFloat64, Height: 10}, options: GeometryOptions{DPIScale: 2, SplitterThickness: 4}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := CalculateGeometry(root, test.host, test.options); err == nil {
				t.Fatal("CalculateGeometry succeeded, want error")
			}
		})
	}
	if _, err := CalculateGeometry(nil, Rect{Width: 10, Height: 10}, DefaultGeometryOptions()); err == nil {
		t.Fatal("CalculateGeometry accepted a nil root")
	}
}

func testPanel(t *testing.T, id appstate.PanelID, panelType appstate.PanelType) appstate.PanelInstanceState {
	t.Helper()
	panel, err := appstate.NewPanelInstance(id, panelType, "link-a")
	if err != nil {
		t.Fatal(err)
	}
	return panel
}

func assertRect(t *testing.T, got Rect, want Rect) {
	t.Helper()
	assertClose(t, got.X, want.X)
	assertClose(t, got.Y, want.Y)
	assertClose(t, got.Width, want.Width)
	assertClose(t, got.Height, want.Height)
}

func assertSize(t *testing.T, got Size, want Size) {
	t.Helper()
	assertClose(t, got.Width, want.Width)
	assertClose(t, got.Height, want.Height)
}

func assertClose(t *testing.T, got float64, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}
