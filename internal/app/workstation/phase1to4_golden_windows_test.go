package workstation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/golden"
)

const phase1To4GoldenVerifyEnv = "CORTHENA_VERIFY_PHASE1TO4_GOLDENS"

type phase1To4GoldenManifest struct {
	Version int                    `json:"version"`
	Source  string                 `json:"source"`
	Entries []phase1To4GoldenEntry `json:"entries"`
}

type phase1To4GoldenEntry struct {
	Phase             int                     `json:"phase"`
	File              string                  `json:"file"`
	SHA256            string                  `json:"sha256"`
	Metadata          phase1To4GoldenMetadata `json:"metadata"`
	ChannelTolerance  uint8                   `json:"channel_tolerance"`
	MaxDifferentRatio float64                 `json:"max_different_ratio"`
}

type phase1To4GoldenMetadata struct {
	BaselineVersion     int       `json:"baseline_version"`
	Seed                uint64    `json:"seed"`
	Workspace           string    `json:"workspace"`
	Scenario            string    `json:"scenario"`
	Width               int       `json:"width"`
	Height              int       `json:"height"`
	ScalePercent        int       `json:"scale_percent"`
	HiddenFrames        int       `json:"hidden_frames"`
	FontIconFingerprint string    `json:"font_icon_fingerprint"`
	ScenarioClock       time.Time `json:"scenario_clock"`
	LayoutName          string    `json:"layout_name"`
	LayoutStateRevision uint64    `json:"layout_state_revision"`
	Backend             string    `json:"backend"`
}

func TestPhase1To4GoldenBaselineManifest(t *testing.T) {
	directory := filepath.Join("testdata", "phase1to4-golden")
	manifest := loadPhase1To4GoldenManifest(t, filepath.Join(directory, "manifest.json"))
	if manifest.Version != 1 || manifest.Source != "Go workstation hidden capture" || len(manifest.Entries) != 4 {
		t.Fatal("Phase 1--4 golden manifest is incomplete")
	}
	for index, entry := range manifest.Entries {
		if entry.Phase != index+1 {
			t.Fatalf("entry %d phase = %d, want %d", index, entry.Phase, index+1)
		}
		validatePhase1To4GoldenEntry(t, entry)
		path := filepath.Join(directory, entry.File)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", entry.File, err)
		}
		digest := sha256.Sum256(contents)
		if hex.EncodeToString(digest[:]) != entry.SHA256 {
			t.Fatalf("%s checksum does not match manifest", entry.File)
		}
		configuration, err := png.DecodeConfig(bytes.NewReader(contents))
		if err != nil {
			t.Fatalf("decode %s: %v", entry.File, err)
		}
		if configuration.Width != entry.Metadata.Width || configuration.Height != entry.Metadata.Height {
			t.Fatalf("%s dimensions = %dx%d", entry.File, configuration.Width, configuration.Height)
		}
	}
	if os.Getenv(phase1To4GoldenVerifyEnv) == "1" {
		for _, entry := range manifest.Entries {
			verifyPhase1To4GoldenEntry(t, directory, entry)
		}
	}
}

func loadPhase1To4GoldenManifest(t *testing.T, path string) phase1To4GoldenManifest {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest phase1To4GoldenManifest
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func validatePhase1To4GoldenEntry(t *testing.T, entry phase1To4GoldenEntry) {
	t.Helper()
	metadata := entry.Metadata
	if entry.File == "" || len(entry.SHA256) != 64 || metadata.BaselineVersion != 1 || metadata.Seed != 42 ||
		metadata.Workspace != "data" || metadata.Scenario != "normal" || metadata.Width != 1280 || metadata.Height != 720 ||
		metadata.ScalePercent != 100 || metadata.HiddenFrames != 30 || metadata.FontIconFingerprint == "" ||
		metadata.ScenarioClock.UTC() != metadata.ScenarioClock || metadata.ScenarioClock.IsZero() ||
		metadata.LayoutName != "isolated-default" || metadata.LayoutStateRevision != 0 || metadata.Backend == "" ||
		entry.ChannelTolerance != 3 || entry.MaxDifferentRatio != 0.002 {
		t.Fatalf("%s has incomplete or noncanonical capture metadata", entry.File)
	}
}

func verifyPhase1To4GoldenEntry(t *testing.T, directory string, entry phase1To4GoldenEntry) {
	t.Helper()
	capture := make(chan *image.RGBA, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runtime.LockOSThread()
	err := Run(ctx, Options{
		Hidden: true, MaxFrames: entry.Metadata.HiddenFrames, DemoSeed: entry.Metadata.Seed,
		Width: int32(entry.Metadata.Width), Height: int32(entry.Metadata.Height),
		InitialWorkspace: appstate.Workspace(entry.Metadata.Workspace),
		InitialUIScale:   appstate.UIScalePreset(entry.Metadata.ScalePercent),
		DataScenario:     appstate.DataScenario(entry.Metadata.Scenario),
		Clock:            appstate.FixedClock{Time: entry.Metadata.ScenarioClock}, Capture: capture,
		LayoutDirectory: t.TempDir(), DisableEvents: true,
	})
	runtime.UnlockOSThread()
	if err != nil {
		t.Fatal(err)
	}
	captured := <-capture
	file, err := os.Open(filepath.Join(directory, entry.File))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	expected, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	difference, err := golden.Compare(expected, captured, golden.Options{ChannelTolerance: entry.ChannelTolerance, MaxDifferentRatio: entry.MaxDifferentRatio})
	if err != nil {
		t.Fatal(err)
	}
	if !difference.Passed {
		t.Fatal(fmt.Errorf("%s golden difference: %d/%d pixels (%.6f), maximum channel delta %d", entry.File, difference.DifferentPixels, difference.TotalPixels, difference.DifferentRatio, difference.MaximumDelta))
	}
}
