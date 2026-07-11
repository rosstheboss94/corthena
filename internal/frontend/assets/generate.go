//go:build ignore

// Command generate creates the deterministic Lucide-derived icon atlas using
// only the Go standard library.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const (
	cellSize  = 24
	iconCount = 4
	scale     = 4
)

type point struct {
	x float64
	y float64
}

func main() {
	highResolution := image.NewRGBA(image.Rect(0, 0, cellSize*iconCount*scale, cellSize*scale))
	drawActivity(highResolution, 0)
	drawDatabase(highResolution, 1)
	drawSearch(highResolution, 2)
	drawSettings(highResolution, 3)

	atlas := downsample(highResolution)
	output, err := os.Create("icons/lucide-atlas.png")
	if err != nil {
		fail(err)
	}
	if err := png.Encode(output, atlas); err != nil {
		_ = output.Close()
		fail(err)
	}
	if err := output.Close(); err != nil {
		fail(err)
	}
}

func drawActivity(canvas *image.RGBA, cell int) {
	drawPolyline(canvas, cell, []point{
		{22, 12}, {19.5, 12}, {15, 22}, {9, 2}, {6.4, 10.5}, {4.5, 12}, {2, 12},
	}, false)
}

func drawDatabase(canvas *image.RGBA, cell int) {
	drawEllipse(canvas, cell, point{12, 5}, 9, 3)
	drawArc(canvas, cell, point{12, 12}, 9, 3, 0, math.Pi)
	drawArc(canvas, cell, point{12, 19}, 9, 3, 0, math.Pi)
	drawLine(canvas, cell, point{3, 5}, point{3, 19})
	drawLine(canvas, cell, point{21, 5}, point{21, 19})
}

func drawSearch(canvas *image.RGBA, cell int) {
	drawEllipse(canvas, cell, point{11, 11}, 8, 8)
	drawLine(canvas, cell, point{16.7, 16.7}, point{21, 21})
}

func drawSettings(canvas *image.RGBA, cell int) {
	gear := make([]point, 16)
	for index := range gear {
		angle := -math.Pi/2 + float64(index)*math.Pi/8
		radius := 9.2
		if index%2 != 0 {
			radius = 7.2
		}
		gear[index] = point{
			x: 12 + math.Cos(angle)*radius,
			y: 12 + math.Sin(angle)*radius,
		}
	}
	drawPolyline(canvas, cell, gear, true)
	drawEllipse(canvas, cell, point{12, 12}, 3, 3)
}

func drawEllipse(canvas *image.RGBA, cell int, center point, radiusX float64, radiusY float64) {
	drawArc(canvas, cell, center, radiusX, radiusY, 0, 2*math.Pi)
}

func drawArc(
	canvas *image.RGBA,
	cell int,
	center point,
	radiusX float64,
	radiusY float64,
	start float64,
	end float64,
) {
	const segments = 64
	points := make([]point, segments+1)
	for index := range points {
		angle := start + (end-start)*float64(index)/segments
		points[index] = point{
			x: center.x + math.Cos(angle)*radiusX,
			y: center.y + math.Sin(angle)*radiusY,
		}
	}
	drawPolyline(canvas, cell, points, false)
}

func drawPolyline(canvas *image.RGBA, cell int, points []point, closed bool) {
	for index := 1; index < len(points); index++ {
		drawLine(canvas, cell, points[index-1], points[index])
	}
	if closed {
		drawLine(canvas, cell, points[len(points)-1], points[0])
	}
}

func drawLine(canvas *image.RGBA, cell int, start point, end point) {
	start.x = (start.x + float64(cell*cellSize)) * scale
	start.y *= scale
	end.x = (end.x + float64(cell*cellSize)) * scale
	end.y *= scale
	radius := float64(scale)

	minX := maxInt(0, int(math.Floor(math.Min(start.x, end.x)-radius)))
	maxX := minInt(canvas.Bounds().Max.X-1, int(math.Ceil(math.Max(start.x, end.x)+radius)))
	minY := maxInt(0, int(math.Floor(math.Min(start.y, end.y)-radius)))
	maxY := minInt(canvas.Bounds().Max.Y-1, int(math.Ceil(math.Max(start.y, end.y)+radius)))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			sample := point{float64(x) + 0.5, float64(y) + 0.5}
			if distanceToSegment(sample, start, end) <= radius {
				canvas.SetRGBA(x, y, color.RGBA{R: 214, G: 220, B: 229, A: 255})
			}
		}
	}
}

func distanceToSegment(sample point, start point, end point) float64 {
	deltaX := end.x - start.x
	deltaY := end.y - start.y
	lengthSquared := deltaX*deltaX + deltaY*deltaY
	if lengthSquared == 0 {
		return math.Hypot(sample.x-start.x, sample.y-start.y)
	}
	projection := ((sample.x-start.x)*deltaX + (sample.y-start.y)*deltaY) / lengthSquared
	projection = math.Max(0, math.Min(1, projection))
	nearestX := start.x + projection*deltaX
	nearestY := start.y + projection*deltaY
	return math.Hypot(sample.x-nearestX, sample.y-nearestY)
}

func downsample(source *image.RGBA) *image.RGBA {
	destination := image.NewRGBA(image.Rect(0, 0, cellSize*iconCount, cellSize))
	for y := range destination.Bounds().Dy() {
		for x := range destination.Bounds().Dx() {
			var red uint32
			var green uint32
			var blue uint32
			var alpha uint32
			for sampleY := range scale {
				for sampleX := range scale {
					pixel := source.RGBAAt(x*scale+sampleX, y*scale+sampleY)
					red += uint32(pixel.R)
					green += uint32(pixel.G)
					blue += uint32(pixel.B)
					alpha += uint32(pixel.A)
				}
			}
			samples := uint32(scale * scale)
			destination.SetRGBA(x, y, color.RGBA{
				R: uint8(red / samples),
				G: uint8(green / samples),
				B: uint8(blue / samples),
				A: uint8(alpha / samples),
			})
		}
	}
	return destination
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
