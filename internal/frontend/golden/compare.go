// Package golden provides first-party deterministic RGBA golden-image
// comparison and baseline metadata validation. Raylib capture remains in the
// UI-thread native adapter; comparison and file I/O run elsewhere.
package golden

import (
	"errors"
	"fmt"
	"image"
	"math"
	"time"
)

var ErrInvalidGolden = errors.New("invalid golden image")

// Metadata records every input required to reproduce one baseline.
type Metadata struct {
	BaselineVersion int       `json:"baseline_version"`
	Seed            uint64    `json:"seed"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	ScalePercent    int       `json:"scale_percent"`
	FontFingerprint string    `json:"font_fingerprint"`
	ScenarioClock   time.Time `json:"scenario_clock"`
	Backend         string    `json:"backend"`
	Scenario        string    `json:"scenario"`
}

// Validate checks reproducibility fields and the required Phase 5 matrix.
func (metadata Metadata) Validate() error {
	validViewport := (metadata.Width == 1280 && metadata.Height == 720) || (metadata.Width == 1920 && metadata.Height == 1080)
	validScale := metadata.ScalePercent == 100 || metadata.ScalePercent == 150 || metadata.ScalePercent == 200
	if metadata.BaselineVersion <= 0 || !validViewport || !validScale || metadata.FontFingerprint == "" || metadata.ScenarioClock.IsZero() || metadata.Backend == "" || metadata.Scenario == "" {
		return fmt.Errorf("%w: incomplete or unsupported metadata", ErrInvalidGolden)
	}
	return nil
}

// Options defines the documented per-channel and differing-pixel tolerances.
type Options struct {
	ChannelTolerance  uint8
	MaxDifferentRatio float64
}

// Difference is deterministic pixel comparison evidence.
type Difference struct {
	Width           int
	Height          int
	TotalPixels     int
	DifferentPixels int
	DifferentRatio  float64
	MaximumDelta    uint8
	Passed          bool
}

// Compare compares decoded pixels without filesystem or native UI work.
func Compare(expected image.Image, actual image.Image, options Options) (Difference, error) {
	if expected == nil || actual == nil || !finite(options.MaxDifferentRatio) || options.MaxDifferentRatio < 0 || options.MaxDifferentRatio > 1 {
		return Difference{}, fmt.Errorf("%w: invalid image or tolerance", ErrInvalidGolden)
	}
	expectedBounds := expected.Bounds()
	actualBounds := actual.Bounds()
	if expectedBounds.Dx() <= 0 || expectedBounds.Dy() <= 0 || expectedBounds.Dx() != actualBounds.Dx() || expectedBounds.Dy() != actualBounds.Dy() {
		return Difference{}, fmt.Errorf("%w: dimension mismatch %v versus %v", ErrInvalidGolden, expectedBounds, actualBounds)
	}
	difference := Difference{Width: expectedBounds.Dx(), Height: expectedBounds.Dy(), TotalPixels: expectedBounds.Dx() * expectedBounds.Dy()}
	for yOffset := 0; yOffset < expectedBounds.Dy(); yOffset++ {
		for xOffset := 0; xOffset < expectedBounds.Dx(); xOffset++ {
			expectedPixel := rgba8(expected.At(expectedBounds.Min.X+xOffset, expectedBounds.Min.Y+yOffset))
			actualPixel := rgba8(actual.At(actualBounds.Min.X+xOffset, actualBounds.Min.Y+yOffset))
			pixelDifferent := false
			for channel := range expectedPixel {
				delta := absoluteDelta(expectedPixel[channel], actualPixel[channel])
				difference.MaximumDelta = max(difference.MaximumDelta, delta)
				if delta > options.ChannelTolerance {
					pixelDifferent = true
				}
			}
			if pixelDifferent {
				difference.DifferentPixels++
			}
		}
	}
	difference.DifferentRatio = float64(difference.DifferentPixels) / float64(difference.TotalPixels)
	difference.Passed = difference.DifferentRatio <= options.MaxDifferentRatio
	return difference, nil
}

func rgba8(value interface {
	RGBA() (uint32, uint32, uint32, uint32)
}) [4]uint8 {
	red, green, blue, alpha := value.RGBA()
	return [4]uint8{uint8((red + 128) / 257), uint8((green + 128) / 257), uint8((blue + 128) / 257), uint8((alpha + 128) / 257)}
}

func absoluteDelta(left uint8, right uint8) uint8 {
	if left >= right {
		return left - right
	}
	return right - left
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
