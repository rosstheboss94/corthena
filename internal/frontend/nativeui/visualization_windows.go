package nativeui

import (
	"strconv"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/chart"
	virtualtable "github.com/rosstheboss94/corthena/internal/frontend/table"
)

// drawChartFrame is the thin UI-thread adapter from immutable, render-ready
// chart buffers to Raylib primitives. It performs no LOD, sorting, decoding,
// or domain calculation.
func (renderer *shellRenderer) drawChartFrame(frame chart.Frame) {
	for _, layer := range frame.Layers {
		for _, rect := range layer.Rects {
			color := chartStyleColor(rect.Style)
			if layer.Kind == chart.LayerHeatmap {
				color = heatColor(rect.Value)
			} else if layer.Kind == chart.LayerRegions {
				color = withAlpha(color, 30)
			} else if layer.Kind == chart.LayerArea || layer.Kind == chart.LayerDrawdown {
				color = withAlpha(color, 80)
			}
			renderer.rect(rectangle{x: rect.X, y: rect.Y, width: rect.Width, height: rect.Height}, color)
		}
		for _, polygon := range layer.Polygons {
			if len(polygon.Points) < 3 {
				continue
			}
			points := make([]point, len(polygon.Points))
			for index, item := range polygon.Points {
				points[index] = point{x: item.X, y: item.Y}
			}
			if err := renderer.window.check("draw chart polygon"); err != nil {
				renderer.err = err
				return
			}
			renderer.window.backend.drawTriangleFan(points, withAlpha(chartStyleColor(polygon.Style), 72))
		}
		for _, segment := range layer.Segments {
			renderer.line(point{x: segment.Start.X, y: segment.Start.Y}, point{x: segment.End.X, y: segment.End.Y}, renderer.scaleValue(1.25), chartStyleColor(segment.Style))
		}
		for _, marker := range layer.Markers {
			renderer.drawChartMarker(marker)
		}
		for _, label := range layer.Labels {
			renderer.text(renderer.window.monoFont, label.Text, point{x: label.Position.X, y: label.Position.Y}, 10, chartStyleColor(label.Style))
		}
	}
}

func (renderer *shellRenderer) drawChartMarker(marker chart.Marker32) {
	center := point{x: marker.Center.X, y: marker.Center.Y}
	size := marker.Size * renderer.scale
	color := chartStyleColor(marker.Style)
	if err := renderer.window.check("draw chart marker"); err != nil {
		renderer.err = err
		return
	}
	switch marker.Shape {
	case chart.MarkerCircle:
		renderer.window.backend.drawCircle(center, size, color)
	case chart.MarkerTriangleUp:
		renderer.window.backend.drawTriangle(point{x: center.x, y: center.y - size}, point{x: center.x - size, y: center.y + size}, point{x: center.x + size, y: center.y + size}, color)
	case chart.MarkerTriangleDown:
		renderer.window.backend.drawTriangle(point{x: center.x, y: center.y + size}, point{x: center.x + size, y: center.y - size}, point{x: center.x - size, y: center.y - size}, color)
	}
}

// drawTableWindow renders only the cells admitted by the virtualization
// kernel. It never walks source rows or columns.
func (renderer *shellRenderer) drawTableWindow(window virtualtable.Window, selection virtualtable.Selection) {
	for _, column := range window.Columns {
		bounds := rectangle{x: float32(column.X), y: float32(window.HeaderY), width: float32(column.Width), height: renderer.scaleValue(24)}
		renderer.rect(bounds, tokenRaised)
		renderer.outline(bounds, tokenDivider)
		renderer.text(renderer.window.interFont, clipText(column.Column.Title, 18), point{x: bounds.x + renderer.scaleValue(6), y: bounds.y + renderer.scaleValue(5)}, 10, tokenText)
	}
	for _, cell := range window.Cells {
		bounds := rectangle{x: float32(cell.X), y: float32(cell.Y), width: float32(cell.Width), height: float32(cell.Height)}
		if selection.Contains(cell.RowID) {
			renderer.rect(bounds, tokenRaised)
		}
		renderer.outline(bounds, withAlpha(tokenDivider, 120))
		renderer.text(renderer.window.monoFont, clipText(tableCellText(cell.Cell), 24), point{x: bounds.x + renderer.scaleValue(6), y: bounds.y + renderer.scaleValue(5)}, 10, tokenText)
	}
}

func tableCellText(cell virtualtable.Cell) string {
	if cell.Null {
		return "--"
	}
	switch cell.Kind {
	case virtualtable.CellString:
		return cell.String
	case virtualtable.CellInteger:
		return strconv.FormatInt(cell.Integer, 10)
	case virtualtable.CellFloat:
		return strconv.FormatFloat(cell.Float, 'g', 6, 64)
	case virtualtable.CellBoolean:
		return strconv.FormatBool(cell.Boolean)
	case virtualtable.CellTime:
		return cell.Time.UTC().Format(time.RFC3339)
	default:
		return "?"
	}
}

func chartStyleColor(style chart.StyleRole) colorValue {
	switch style {
	case chart.StylePrimary:
		return tokenCyan
	case chart.StyleSecondary:
		return tokenPurple
	case chart.StylePositive:
		return tokenPositive
	case chart.StyleNegative:
		return tokenNegative
	case chart.StyleWarning:
		return tokenWarning
	case chart.StyleTrain:
		return tokenCyan
	case chart.StyleValidation:
		return tokenPurple
	case chart.StyleTest:
		return tokenWarning
	default:
		return tokenMuted
	}
}

func heatColor(value float32) colorValue {
	normalized := minFloat32(maxFloat32(value, 0), 1)
	return colorValue{
		red:   uint8(float32(tokenPurple.red)*(1-normalized) + float32(tokenCyan.red)*normalized),
		green: uint8(float32(tokenPurple.green)*(1-normalized) + float32(tokenCyan.green)*normalized),
		blue:  uint8(float32(tokenPurple.blue)*(1-normalized) + float32(tokenCyan.blue)*normalized),
		alpha: 210,
	}
}
