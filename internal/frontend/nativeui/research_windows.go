package nativeui

import (
	"fmt"
	"math"
	"strconv"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/chart"
	virtualtable "github.com/rosstheboss94/corthena/internal/frontend/table"
)

type researchUIState struct {
	dragPanel appstate.PanelID
	dragStart point
	dragRange appstate.TimeRange
	dragPan   bool
}

func isResearchPanel(panelType appstate.PanelType) bool {
	switch panelType {
	case appstate.PanelOHLCVChart, appstate.PanelFeatureBrowser, appstate.PanelSeriesInspector,
		appstate.PanelTargetPreview, appstate.PanelDistributions, appstate.PanelRowTable:
		return true
	default:
		return false
	}
}

func (renderer *shellRenderer) drawResearchPanel(state appstate.AppState, panel appstate.PanelInstanceState, bounds rectangle) {
	group, found := state.ResearchGroup(panel.LinkGroup)
	if !found {
		renderer.emptyDock(bounds, "Preparing linked Research request")
		return
	}
	content := bounds
	if panel.Type != appstate.PanelOHLCVChart {
		content.y += renderer.scaleValue(24)
		content.height -= renderer.scaleValue(24)
		renderer.drawResearchScenarioControl(group, bounds)
	}
	if group.State == appstate.ResearchLoading && !group.Stale {
		renderer.emptyDock(content, "Loading deterministic Research data...")
		return
	}
	if (group.State == appstate.ResearchFailed || group.State == appstate.ResearchCancelled || group.State == appstate.ResearchBusy) && !group.Stale {
		renderer.drawResearchFailure(group, content)
		return
	}
	if group.State == appstate.ResearchEmpty {
		renderer.emptyDock(content, "No rows match this Research query")
		return
	}
	switch panel.Type {
	case appstate.PanelOHLCVChart:
		renderer.drawResearchChart(state, panel, group, content)
	case appstate.PanelFeatureBrowser:
		renderer.drawFeatureBrowser(group, content)
	case appstate.PanelSeriesInspector:
		renderer.drawSeriesInspector(group, content)
	case appstate.PanelTargetPreview:
		renderer.drawTargetPreview(group, content)
	case appstate.PanelDistributions:
		renderer.drawResearchDistributions(group, content)
	case appstate.PanelRowTable:
		renderer.drawResearchRows(group, content)
	}
	if group.Stale || group.State == appstate.ResearchDegraded || group.State == appstate.ResearchRecovered {
		renderer.drawResearchStatus(group, bounds)
	}
}

func (renderer *shellRenderer) drawResearchScenarioControl(group appstate.ResearchGroupState, bounds rectangle) {
	button := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(134), y: bounds.y, width: renderer.scaleValue(130), height: renderer.scaleValue(20)}
	renderer.rect(button, tokenRaised)
	renderer.outline(button, tokenDivider)
	renderer.text(renderer.window.interFont, "Scenario: "+string(group.Scenario), point{x: button.x + renderer.scaleValue(6), y: button.y + renderer.scaleValue(4)}, 9, tokenMuted)
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, button) {
		scenarios := appstate.ResearchScenarios()
		next := scenarios[0]
		for index, scenario := range scenarios {
			if scenario == group.Scenario {
				next = scenarios[(index+1)%len(scenarios)]
				break
			}
		}
		renderer.actions = append(renderer.actions, appstate.SetResearchScenarioAction{GroupID: group.GroupID, Scenario: next})
	}
}

func (renderer *shellRenderer) drawResearchFailure(group appstate.ResearchGroupState, bounds rectangle) {
	message := group.Error.Message
	if message == "" {
		message = "Research request failed"
	}
	renderer.textBlock(bounds, []string{string(group.State), message, "Click Retry to request a fresh generation."})
	retry := rectangle{x: bounds.x, y: bounds.y + renderer.scaleValue(72), width: renderer.scaleValue(82), height: renderer.scaleValue(24)}
	renderer.rect(retry, tokenRaised)
	renderer.outline(retry, tokenWarning)
	renderer.text(renderer.window.interFont, "Retry", point{x: retry.x + renderer.scaleValue(20), y: retry.y + renderer.scaleValue(5)}, 10, tokenText)
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, retry) {
		query := group.Query.Clone()
		query.Generation++
		query.CorrelationID = appstate.CorrelationID(fmt.Sprintf("research-%s-%020d", query.GroupID, query.Generation))
		query.Scenario = appstate.ResearchScenarioNormal
		renderer.actions = append(renderer.actions, appstate.RequestResearchAction{Query: query})
	}
}

func (renderer *shellRenderer) drawResearchChart(state appstate.AppState, panel appstate.PanelInstanceState, group appstate.ResearchGroupState, bounds rectangle) {
	toolbarHeight := renderer.scaleValue(24)
	toolbar := rectangle{x: bounds.x, y: bounds.y, width: bounds.width, height: toolbarHeight}
	plot := rectangle{x: bounds.x, y: bounds.y + toolbarHeight, width: bounds.width, height: bounds.height - toolbarHeight}
	renderer.drawChartToolbar(state, panel, group, toolbar)
	renderer.rect(plot, tokenBackground)
	renderer.outline(plot, tokenDivider)
	for index := 1; index < 6; index++ {
		y := plot.y + plot.height*float32(index)/6
		renderer.line(point{x: plot.x, y: y}, point{x: plot.x + plot.width, y: y}, 1, withAlpha(tokenDivider, 130))
	}
	renderer.drawResearchFrame(group.Snapshot.Frame, plot, group)
	renderer.researchChartInteraction(panel, group, plot)
	renderer.drawResearchCrosshair(group, plot)
}

func (renderer *shellRenderer) drawChartToolbar(state appstate.AppState, panel appstate.PanelInstanceState, group appstate.ResearchGroupState, bounds rectangle) {
	x := bounds.x
	toggle := func(label string, active bool, action appstate.SetResearchVisibilityAction) {
		button := rectangle{x: x, y: bounds.y, width: renderer.scaleValue(70), height: bounds.height - renderer.scaleValue(2)}
		color := tokenPanel
		if active {
			color = tokenRaised
		}
		renderer.rect(button, color)
		renderer.outline(button, tokenDivider)
		renderer.text(renderer.window.interFont, label, point{x: button.x + renderer.scaleValue(6), y: button.y + renderer.scaleValue(5)}, 9, tokenText)
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, button) {
			renderer.actions = append(renderer.actions, action)
		}
		x += button.width + renderer.scaleValue(3)
	}
	toggle("Candles", group.ShowOHLCV, appstate.SetResearchVisibilityAction{GroupID: group.GroupID, ShowOHLCV: !group.ShowOHLCV, ShowFeature: group.ShowFeature, ShowTarget: group.ShowTarget})
	toggle("Feature", group.ShowFeature, appstate.SetResearchVisibilityAction{GroupID: group.GroupID, ShowOHLCV: group.ShowOHLCV, ShowFeature: !group.ShowFeature, ShowTarget: group.ShowTarget})
	toggle("Target", group.ShowTarget, appstate.SetResearchVisibilityAction{GroupID: group.GroupID, ShowOHLCV: group.ShowOHLCV, ShowFeature: group.ShowFeature, ShowTarget: !group.ShowTarget})
	reset := rectangle{x: bounds.x + bounds.width - renderer.scaleValue(60), y: bounds.y, width: renderer.scaleValue(58), height: bounds.height - renderer.scaleValue(2)}
	renderer.rect(reset, tokenRaised)
	renderer.outline(reset, tokenDivider)
	renderer.text(renderer.window.interFont, "Reset", point{x: reset.x + renderer.scaleValue(12), y: reset.y + renderer.scaleValue(5)}, 9, tokenText)
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, reset) {
		for _, dataset := range state.Datasets {
			if dataset.ID == group.Query.DatasetID {
				renderer.actions = append(renderer.actions, appstate.SetResearchRangeAction{GroupID: group.GroupID, SourcePanelID: panel.ID, TimeRange: appstate.TimeRange{Start: dataset.Start, End: dataset.End}})
				break
			}
		}
	}
}

func (renderer *shellRenderer) drawResearchFrame(frame chart.Frame, bounds rectangle, group appstate.ResearchGroupState) {
	if frame.Viewport.Width() <= 0 || frame.Viewport.Height() <= 0 {
		return
	}
	mapPoint := func(value chart.Point32) point {
		return point{
			x: bounds.x + float32((float64(value.X)-frame.Viewport.MinX)/frame.Viewport.Width())*bounds.width,
			y: bounds.y + float32((float64(value.Y)-frame.Viewport.MinY)/frame.Viewport.Height())*bounds.height,
		}
	}
	for _, layer := range frame.Layers {
		if (layer.ID == "ohlcv" && !group.ShowOHLCV) || (layer.ID == "feature" && !group.ShowFeature) || (layer.ID == "target" && !group.ShowTarget) {
			continue
		}
		for _, item := range layer.Rects {
			first := mapPoint(chart.Point32{X: item.X, Y: item.Y})
			second := mapPoint(chart.Point32{X: item.X + item.Width, Y: item.Y + item.Height})
			color := chartStyleColor(item.Style)
			if layer.Kind == chart.LayerRegions {
				color = withAlpha(color, 28)
			}
			renderer.rect(rectangle{x: first.x, y: first.y, width: second.x - first.x, height: second.y - first.y}, color)
		}
		for _, segment := range layer.Segments {
			renderer.line(mapPoint(segment.Start), mapPoint(segment.End), renderer.scaleValue(1.1), chartStyleColor(segment.Style))
		}
		for _, marker := range layer.Markers {
			center := mapPoint(marker.Center)
			renderer.window.backend.drawCircle(center, renderer.scaleValue(2.2), chartStyleColor(marker.Style))
		}
	}
}

func (renderer *shellRenderer) researchChartInteraction(panel appstate.PanelInstanceState, group appstate.ResearchGroupState, plot rectangle) {
	if !pointInRectangle(renderer.input.mouse, plot) {
		if renderer.input.leftMouseReleased && renderer.window.researchUI.dragPanel == panel.ID {
			renderer.window.researchUI.dragPanel = ""
		}
		return
	}
	if renderer.input.mouseWheel != 0 {
		center := float64((renderer.input.mouse.x - plot.x) / plot.width)
		factor := math.Pow(1.18, -float64(renderer.input.mouseWheel))
		if next, err := appstate.ResearchZoomRange(group.Query.TimeRange, center, factor); err == nil {
			renderer.actions = append(renderer.actions, appstate.SetResearchRangeAction{GroupID: group.GroupID, SourcePanelID: panel.ID, TimeRange: next})
		}
	}
	if renderer.input.leftMousePressed {
		renderer.window.researchUI = researchUIState{dragPanel: panel.ID, dragStart: renderer.input.mouse, dragRange: group.Query.TimeRange, dragPan: renderer.input.shiftDown}
	}
	if renderer.input.leftMouseReleased && renderer.window.researchUI.dragPanel == panel.ID {
		start := renderer.window.researchUI.dragStart
		delta := renderer.input.mouse.x - start.x
		if math.Abs(float64(delta)) >= float64(renderer.scaleValue(6)) {
			var next appstate.TimeRange
			var err error
			if renderer.window.researchUI.dragPan {
				next, err = appstate.ResearchPanRange(renderer.window.researchUI.dragRange, -float64(delta/plot.width))
			} else {
				first := float64((start.x - plot.x) / plot.width)
				second := float64((renderer.input.mouse.x - plot.x) / plot.width)
				first, second = math.Max(0, math.Min(1, first)), math.Max(0, math.Min(1, second))
				next, err = appstate.ResearchSelectRange(renderer.window.researchUI.dragRange, first, second)
			}
			if err == nil {
				renderer.actions = append(renderer.actions, appstate.SetResearchRangeAction{GroupID: group.GroupID, SourcePanelID: panel.ID, TimeRange: next})
			}
		}
		renderer.window.researchUI.dragPanel = ""
	}
}

func (renderer *shellRenderer) drawResearchCrosshair(group appstate.ResearchGroupState, plot rectangle) {
	if !pointInRectangle(renderer.input.mouse, plot) || len(group.Snapshot.Bars) == 0 {
		return
	}
	x := renderer.input.mouse.x
	y := renderer.input.mouse.y
	renderer.line(point{x: x, y: plot.y}, point{x: x, y: plot.y + plot.height}, 1, withAlpha(tokenText, 100))
	renderer.line(point{x: plot.x, y: y}, point{x: plot.x + plot.width, y: y}, 1, withAlpha(tokenText, 80))
	fraction := float64((x - plot.x) / plot.width)
	index := min(max(int(fraction*float64(len(group.Snapshot.Bars))), 0), len(group.Snapshot.Bars)-1)
	bar := group.Snapshot.Bars[index]
	label := fmt.Sprintf("%s  O %.2f  H %.2f  L %.2f  C %.2f  V %.0f", bar.Timestamp.Format("2006-01-02 15:04"), bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
	tip := rectangle{x: minFloat32(x+renderer.scaleValue(8), plot.x+plot.width-renderer.scaleValue(390)), y: plot.y + renderer.scaleValue(6), width: renderer.scaleValue(382), height: renderer.scaleValue(22)}
	renderer.rect(tip, withAlpha(tokenPanel, 235))
	renderer.outline(tip, tokenDivider)
	renderer.text(renderer.window.monoFont, label, point{x: tip.x + renderer.scaleValue(6), y: tip.y + renderer.scaleValue(5)}, 9, tokenText)
}

func (renderer *shellRenderer) drawFeatureBrowser(group appstate.ResearchGroupState, bounds rectangle) {
	y := bounds.y
	for _, series := range group.Snapshot.Features {
		row := rectangle{x: bounds.x, y: y, width: bounds.width, height: renderer.scaleValue(42)}
		active := series.Descriptor.Name == group.SelectedFeature
		if active {
			renderer.rect(row, tokenRaised)
		}
		renderer.outline(row, tokenDivider)
		renderer.text(renderer.window.interFont, string(series.Descriptor.Name)+"  v"+series.Descriptor.Version, point{x: row.x + renderer.scaleValue(8), y: row.y + renderer.scaleValue(6)}, 11, activeColor(active))
		renderer.text(renderer.window.interFont, fmt.Sprintf("lookback %d  missing %d  %s", series.Descriptor.Lookback, series.Missing, series.Descriptor.Description), point{x: row.x + renderer.scaleValue(8), y: row.y + renderer.scaleValue(23)}, 9, tokenMuted)
		if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, row) && !active {
			renderer.actions = append(renderer.actions, appstate.SetResearchFeatureAction{GroupID: group.GroupID, Feature: series.Descriptor.Name})
		}
		y += row.height + renderer.scaleValue(4)
	}
}

func (renderer *shellRenderer) drawSeriesInspector(group appstate.ResearchGroupState, bounds rectangle) {
	for _, series := range group.Snapshot.Features {
		if series.Descriptor.Name != group.SelectedFeature {
			continue
		}
		latest := "missing"
		for index := len(series.Values) - 1; index >= 0; index-- {
			if !series.Values[index].Missing {
				latest = fmt.Sprintf("%.6f", series.Values[index].Value)
				break
			}
		}
		renderer.textBlock(bounds, []string{
			"Selected series: " + string(series.Descriptor.Name),
			"Semantic version: " + series.Descriptor.Version,
			"Implementation: " + series.Descriptor.Fingerprint,
			fmt.Sprintf("Declared lookback: %d bars", series.Descriptor.Lookback),
			fmt.Sprintf("Range: %.6f to %.6f", series.Minimum, series.Maximum),
			fmt.Sprintf("Explicit missing values: %d", series.Missing),
			"Latest visible value: " + latest,
		})
		return
	}
	renderer.emptyDock(bounds, "Selected feature is unavailable")
}

func (renderer *shellRenderer) drawTargetPreview(group appstate.ResearchGroupState, bounds rectangle) {
	target := group.Snapshot.Target
	kind := "simple"
	if target.Spec.LogReturn {
		kind = "log"
	}
	renderer.textBlock(bounds, []string{
		"Forward open-to-open target",
		fmt.Sprintf("Horizon: %d bars", target.Spec.HorizonBars),
		"Return kind: " + kind,
		"Feature cutoff: bar t close",
		"Reference execution: bar t+1 open",
		fmt.Sprintf("Valid future targets: %d", target.ValidRows),
		fmt.Sprintf("Excluded trailing rows: %d", target.ExcludedRows),
		"Viewport changes do not alter split membership.",
	})
}

func (renderer *shellRenderer) drawResearchDistributions(group appstate.ResearchGroupState, bounds rectangle) {
	selected := group.SelectedFeature
	var feature appstate.ResearchDistribution
	var target appstate.ResearchDistribution
	for _, candidate := range group.Snapshot.Distributions {
		if candidate.Name == selected {
			feature = candidate
		}
		if candidate.Name == "target" {
			target = candidate
		}
	}
	gap := renderer.scaleValue(8)
	width := (bounds.width - gap) / 2
	renderer.drawResearchHistogram(feature, rectangle{x: bounds.x, y: bounds.y, width: width, height: bounds.height})
	renderer.drawResearchHistogram(target, rectangle{x: bounds.x + width + gap, y: bounds.y, width: width, height: bounds.height})
}

func (renderer *shellRenderer) drawResearchHistogram(distribution appstate.ResearchDistribution, bounds rectangle) {
	if len(distribution.Bins) == 0 {
		renderer.emptyDock(bounds, "No finite observations")
		return
	}
	maximum := uint64(1)
	for _, bin := range distribution.Bins {
		maximum = max(maximum, bin.Count)
	}
	barWidth := bounds.width / float32(len(distribution.Bins))
	for index, bin := range distribution.Bins {
		height := (bounds.height - renderer.scaleValue(30)) * float32(bin.Count) / float32(maximum)
		bar := rectangle{x: bounds.x + float32(index)*barWidth + 1, y: bounds.y + bounds.height - height - renderer.scaleValue(18), width: maxFloat32(1, barWidth-2), height: height}
		renderer.rect(bar, tokenCyan)
	}
	renderer.text(renderer.window.interFont, clipText(string(distribution.Name)+" distribution", 24), point{x: bounds.x + renderer.scaleValue(6), y: bounds.y + bounds.height - renderer.scaleValue(14)}, 9, tokenMuted)
}

func (renderer *shellRenderer) drawResearchRows(group appstate.ResearchGroupState, bounds rectangle) {
	toolbar := rectangle{x: bounds.x, y: bounds.y, width: bounds.width, height: renderer.scaleValue(24)}
	tableBounds := rectangle{x: bounds.x, y: bounds.y + toolbar.height, width: bounds.width, height: bounds.height - toolbar.height}
	renderer.text(renderer.window.interFont, fmt.Sprintf("%d leakage-safe rows", group.Snapshot.Rows.TotalRows), point{x: toolbar.x + renderer.scaleValue(4), y: toolbar.y + renderer.scaleValue(5)}, 9, tokenMuted)
	query := group.Query.Clone()
	submit := func(query appstate.ResearchQuery) {
		query.Generation++
		query.CorrelationID = appstate.CorrelationID(fmt.Sprintf("research-%s-%020d", query.GroupID, query.Generation))
		renderer.actions = append(renderer.actions, appstate.RequestResearchAction{Query: query})
	}
	nextButton := rectangle{x: toolbar.x + toolbar.width - renderer.scaleValue(56), y: toolbar.y, width: renderer.scaleValue(54), height: toolbar.height - 2}
	renderer.rect(nextButton, tokenRaised)
	renderer.outline(nextButton, tokenDivider)
	renderer.text(renderer.window.interFont, "Next", point{x: nextButton.x + renderer.scaleValue(12), y: nextButton.y + renderer.scaleValue(5)}, 9, tokenText)
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, nextButton) && group.Snapshot.Rows.NextCursor != "" {
		query.Cursor = group.Snapshot.Rows.NextCursor
		submit(query)
	}
	previousButton := rectangle{x: nextButton.x - renderer.scaleValue(60), y: toolbar.y, width: renderer.scaleValue(56), height: toolbar.height - 2}
	renderer.rect(previousButton, tokenRaised)
	renderer.outline(previousButton, tokenDivider)
	renderer.text(renderer.window.interFont, "Prev", point{x: previousButton.x + renderer.scaleValue(12), y: previousButton.y + renderer.scaleValue(5)}, 9, tokenText)
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, previousButton) && query.Cursor != "" {
		offset, _ := strconv.Atoi(query.Cursor)
		offset = max(0, offset-query.PageSize)
		query.Cursor = ""
		if offset > 0 {
			query.Cursor = strconv.Itoa(offset)
		}
		submit(query)
	}
	sortButton := rectangle{x: previousButton.x - renderer.scaleValue(78), y: toolbar.y, width: renderer.scaleValue(74), height: toolbar.height - 2}
	renderer.rect(sortButton, tokenRaised)
	renderer.outline(sortButton, tokenDivider)
	renderer.text(renderer.window.interFont, "Sort", point{x: sortButton.x + renderer.scaleValue(22), y: sortButton.y + renderer.scaleValue(5)}, 9, tokenText)
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, sortButton) {
		switch query.Sort {
		case appstate.ResearchSortTimeAscending:
			query.Sort = appstate.ResearchSortTimeDescending
		case appstate.ResearchSortTimeDescending:
			query.Sort = appstate.ResearchSortTargetDescending
		default:
			query.Sort = appstate.ResearchSortTimeAscending
		}
		query.Cursor = ""
		submit(query)
	}
	filterButton := rectangle{x: sortButton.x - renderer.scaleValue(86), y: toolbar.y, width: renderer.scaleValue(82), height: toolbar.height - 2}
	renderer.rect(filterButton, tokenRaised)
	renderer.outline(filterButton, tokenDivider)
	filterLabel := "Filter all"
	if query.Filter != "" {
		filterLabel = "Filter " + query.Filter
	}
	renderer.text(renderer.window.interFont, clipText(filterLabel, 13), point{x: filterButton.x + renderer.scaleValue(6), y: filterButton.y + renderer.scaleValue(5)}, 9, tokenText)
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, filterButton) {
		if query.Filter == "" && len(query.Symbols) > 0 {
			query.Filter = string(query.Symbols[0])
		} else {
			query.Filter = ""
		}
		query.Cursor = ""
		submit(query)
	}
	window, err := group.Snapshot.Rows.Model.Virtualize(virtualtable.WindowRequest{
		OriginX: float64(tableBounds.x), OriginY: float64(tableBounds.y), Width: float64(tableBounds.width), Height: float64(tableBounds.height),
		HeaderHeight: float64(renderer.scaleValue(24)), RowHeight: float64(renderer.scaleValue(24)), OverscanRows: 1, OverscanColumns: 1,
	})
	if err != nil {
		renderer.emptyDock(tableBounds, "Invalid Research table viewport")
		return
	}
	selection := virtualtable.Selection{IDs: append([]virtualtable.RowID(nil), group.SelectedRows...)}
	renderer.drawTableWindow(window, selection)
	if (renderer.input.upPressed || renderer.input.downPressed) && len(group.Snapshot.Rows.Dataset.Rows) > 0 {
		rows := group.Snapshot.Rows.Dataset.Rows
		position := 0
		if len(group.SelectedRows) > 0 {
			for index, row := range rows {
				if row.ID == group.SelectedRows[len(group.SelectedRows)-1] {
					position = index
					break
				}
			}
		}
		if renderer.input.upPressed {
			position = max(0, position-1)
		} else {
			position = min(len(rows)-1, position+1)
		}
		renderer.actions = append(renderer.actions, appstate.SelectResearchRowAction{GroupID: group.GroupID, RowID: string(rows[position].ID), Toggle: renderer.input.shiftDown})
	}
	if renderer.input.leftMousePressed && pointInRectangle(renderer.input.mouse, tableBounds) {
		for _, cell := range window.Cells {
			cellBounds := rectangle{x: float32(cell.X), y: float32(cell.Y), width: float32(cell.Width), height: float32(cell.Height)}
			if pointInRectangle(renderer.input.mouse, cellBounds) {
				renderer.actions = append(renderer.actions, appstate.SelectResearchRowAction{GroupID: group.GroupID, RowID: string(cell.RowID), Toggle: renderer.input.shiftDown})
				break
			}
		}
	}
}

func (renderer *shellRenderer) drawResearchStatus(group appstate.ResearchGroupState, bounds rectangle) {
	message := string(group.State)
	color := tokenCyan
	if group.Stale {
		message = "refreshing — showing stale generation"
		color = tokenWarning
	}
	status := rectangle{x: bounds.x + renderer.scaleValue(4), y: bounds.y + renderer.scaleValue(4), width: renderer.scaleValue(float32(len(message))*6 + 14), height: renderer.scaleValue(19)}
	renderer.rect(status, withAlpha(tokenPanel, 235))
	renderer.outline(status, color)
	renderer.text(renderer.window.interFont, message, point{x: status.x + renderer.scaleValue(6), y: status.y + renderer.scaleValue(4)}, 9, color)
}

func activeColor(active bool) colorValue {
	if active {
		return tokenCyan
	}
	return tokenText
}
