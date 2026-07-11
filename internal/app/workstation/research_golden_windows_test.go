package workstation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/assets"
	"github.com/rosstheboss94/corthena/internal/frontend/golden"
)

const (
	researchGoldenUpdateEnv = "CORTHENA_UPDATE_RESEARCH_GOLDENS"
	researchGoldenVerifyEnv = "CORTHENA_VERIFY_RESEARCH_GOLDENS"
	researchGoldenBackend   = "windows-raylib-opengl-3.3-amd-rx5700xt"
	researchGoldenChildEnv  = "CORTHENA_RESEARCH_GOLDEN_CHILD"
	researchGoldenPathEnv   = "CORTHENA_RESEARCH_GOLDEN_PATH"
	researchGoldenModeEnv   = "CORTHENA_RESEARCH_GOLDEN_MODE"
)

var researchGoldenClock = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

type researchGoldenManifest struct {
	Version int                   `json:"version"`
	Entries []researchGoldenEntry `json:"entries"`
}

type researchGoldenEntry struct {
	File             string          `json:"file"`
	Metadata         golden.Metadata `json:"metadata"`
	ChannelTolerance uint8           `json:"channel_tolerance"`
	MaxDifferentRate float64         `json:"max_different_ratio"`
}

func TestResearchGoldenBaselineMatrix(t *testing.T) {
	directory := filepath.Join("testdata", "research-golden")
	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	manifest := expectedResearchGoldenManifest(assetSet.Fingerprint())
	if os.Getenv(researchGoldenUpdateEnv) == "1" {
		captureResearchGoldenMatrix(t, directory, manifest, true)
	}
	loaded := loadResearchGoldenManifest(t, filepath.Join(directory, "manifest.json"))
	if !reflect.DeepEqual(loaded, manifest) {
		t.Fatal("Research golden manifest does not match the required scenario, viewport, scale, asset, and tolerance matrix")
	}
	for _, entry := range loaded.Entries {
		if err := entry.Metadata.Validate(); err != nil {
			t.Fatalf("validate %s metadata: %v", entry.File, err)
		}
		file, err := os.Open(filepath.Join(directory, entry.File))
		if err != nil {
			t.Fatalf("open %s: %v", entry.File, err)
		}
		configuration, decodeErr := png.DecodeConfig(file)
		closeErr := file.Close()
		if err := errors.Join(decodeErr, closeErr); err != nil {
			t.Fatalf("decode %s: %v", entry.File, err)
		}
		if configuration.Width != entry.Metadata.Width || configuration.Height != entry.Metadata.Height {
			t.Fatalf("%s dimensions = %dx%d, want %dx%d", entry.File, configuration.Width, configuration.Height, entry.Metadata.Width, entry.Metadata.Height)
		}
	}
	if os.Getenv(researchGoldenVerifyEnv) == "1" {
		captureResearchGoldenMatrix(t, directory, manifest, false)
	}
}

func TestResearchGoldenCaptureHelper(t *testing.T) {
	encoded := os.Getenv(researchGoldenChildEnv)
	if encoded == "" {
		t.Skip("golden capture helper")
	}
	var entry researchGoldenEntry
	if err := json.Unmarshal([]byte(encoded), &entry); err != nil {
		t.Fatal(err)
	}
	captureResearchGoldenEntry(t, os.Getenv(researchGoldenPathEnv), entry, os.Getenv(researchGoldenModeEnv) == "update")
}

func expectedResearchGoldenManifest(fontFingerprint string) researchGoldenManifest {
	scenarios := []string{"normal", "linked_selection", "loading", "failure", "degraded", "recovered"}
	viewports := [][2]int{{1280, 720}, {1920, 1080}}
	scales := []int{100, 150, 200}
	manifest := researchGoldenManifest{Version: 1}
	for _, scenario := range scenarios {
		for _, viewport := range viewports {
			for _, scale := range scales {
				manifest.Entries = append(manifest.Entries, researchGoldenEntry{
					File: fmt.Sprintf("research_%s_%dx%d_%d.png", scenario, viewport[0], viewport[1], scale),
					Metadata: golden.Metadata{
						BaselineVersion: 1,
						Seed:            42,
						Width:           viewport[0],
						Height:          viewport[1],
						ScalePercent:    scale,
						FontFingerprint: fontFingerprint,
						ScenarioClock:   researchGoldenClock,
						Backend:         researchGoldenBackend,
						Scenario:        scenario,
					},
					ChannelTolerance: 3,
					MaxDifferentRate: 0.002,
				})
			}
		}
	}
	return manifest
}

func loadResearchGoldenManifest(t *testing.T, path string) researchGoldenManifest {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest researchGoldenManifest
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func captureResearchGoldenMatrix(t *testing.T, directory string, manifest researchGoldenManifest, update bool) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if update {
		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, '\n')
		if err := os.WriteFile(filepath.Join(directory, "manifest.json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries := append([]researchGoldenEntry(nil), manifest.Entries...)
	sort.Slice(entries, func(left int, right int) bool { return entries[left].File < entries[right].File })
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		entry := entry
		t.Run(entry.File, func(t *testing.T) {
			encoded, err := json.Marshal(entry)
			if err != nil {
				t.Fatal(err)
			}
			mode := "verify"
			if update {
				mode = "update"
			}
			command := exec.Command(executable, "-test.run=^TestResearchGoldenCaptureHelper$", "-test.timeout=30s")
			command.Env = append(os.Environ(),
				researchGoldenChildEnv+"="+string(encoded),
				researchGoldenPathEnv+"="+absoluteDirectory,
				researchGoldenModeEnv+"="+mode,
			)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("capture subprocess: %v\n%s", err, output)
			}
		})
	}
}

func captureResearchGoldenEntry(t *testing.T, directory string, entry researchGoldenEntry, update bool) {
	t.Helper()
	capture := make(chan *image.RGBA, 1)
	processed := make(chan error, 1)
	path := filepath.Join(directory, entry.File)
	go func() {
		captured := <-capture
		if update {
			processed <- encodeResearchGolden(path, captured)
			return
		}
		processed <- compareResearchGolden(path, captured, entry)
	}()
	scenario := appstate.ResearchScenario(entry.Metadata.Scenario)
	linkedSelection := entry.Metadata.Scenario == "linked_selection"
	if linkedSelection {
		scenario = appstate.ResearchScenarioNormal
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := Run(ctx, Options{
		Hidden: true, MaxFrames: 30, DemoSeed: entry.Metadata.Seed,
		Width: int32(entry.Metadata.Width), Height: int32(entry.Metadata.Height),
		InitialWorkspace: appstate.WorkspaceResearch,
		InitialUIScale:   appstate.UIScalePreset(entry.Metadata.ScalePercent),
		ResearchScenario: scenario, ResearchLinkedSelection: linkedSelection,
		Clock: appstate.FixedClock{Time: entry.Metadata.ScenarioClock}, Capture: capture,
		LayoutDirectory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-processed; err != nil {
		t.Fatal(err)
	}
}

func encodeResearchGolden(path string, captured *image.RGBA) (resultErr error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	return png.Encode(file, captured)
}

func compareResearchGolden(path string, captured *image.RGBA, entry researchGoldenEntry) (resultErr error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	expected, err := png.Decode(file)
	if err != nil {
		return err
	}
	difference, err := golden.Compare(expected, captured, golden.Options{
		ChannelTolerance: entry.ChannelTolerance, MaxDifferentRatio: entry.MaxDifferentRate,
	})
	if err != nil {
		return err
	}
	if !difference.Passed {
		return fmt.Errorf("golden difference: %d/%d pixels (%.6f), maximum channel delta %d", difference.DifferentPixels, difference.TotalPixels, difference.DifferentRatio, difference.MaximumDelta)
	}
	return nil
}
