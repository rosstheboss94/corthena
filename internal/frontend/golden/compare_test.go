package golden

import (
	"image"
	"image/color"
	"testing"
	"time"
)

func TestCompareUsesChannelAndPixelRatioTolerance(t *testing.T) {
	t.Parallel()
	expected := image.NewRGBA(image.Rect(0, 0, 2, 1))
	actual := image.NewRGBA(image.Rect(0, 0, 2, 1))
	expected.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	expected.SetRGBA(1, 0, color.RGBA{R: 100, G: 110, B: 120, A: 255})
	actual.SetRGBA(0, 0, color.RGBA{R: 12, G: 18, B: 31, A: 255})
	actual.SetRGBA(1, 0, color.RGBA{R: 110, G: 110, B: 120, A: 255})
	difference, err := Compare(expected, actual, Options{ChannelTolerance: 2, MaxDifferentRatio: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if !difference.Passed || difference.DifferentPixels != 1 || difference.DifferentRatio != 0.5 || difference.MaximumDelta != 10 {
		t.Fatalf("difference = %+v", difference)
	}
	difference, err = Compare(expected, actual, Options{ChannelTolerance: 2, MaxDifferentRatio: 0.49})
	if err != nil || difference.Passed {
		t.Fatalf("strict difference = %+v err=%v", difference, err)
	}
}

func TestMetadataAdmitsCompleteRequiredGoldenMatrix(t *testing.T) {
	t.Parallel()
	for _, viewport := range [][2]int{{1280, 720}, {1920, 1080}} {
		for _, scale := range []int{100, 150, 200} {
			metadata := Metadata{
				BaselineVersion: 1, Seed: 42, Width: viewport[0], Height: viewport[1], ScalePercent: scale,
				FontFingerprint: "test-fonts", ScenarioClock: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), Backend: "raylib-opengl",
				Scenario: "normal",
			}
			if err := metadata.Validate(); err != nil {
				t.Fatalf("viewport=%v scale=%d: %v", viewport, scale, err)
			}
		}
	}
}
