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
	phase9GoldenUpdateEnv = "CORTHENA_UPDATE_PHASE9_GOLDENS"
	phase9GoldenVerifyEnv = "CORTHENA_VERIFY_PHASE9_GOLDENS"
	phase9GoldenChildEnv  = "CORTHENA_PHASE9_GOLDEN_CHILD"
	phase9GoldenPathEnv   = "CORTHENA_PHASE9_GOLDEN_PATH"
	phase9GoldenModeEnv   = "CORTHENA_PHASE9_GOLDEN_MODE"
)

func TestPhase9GoldenBaselineMatrix(t *testing.T) {
	directory := filepath.Join("testdata", "phase9-golden")
	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	manifest := expectedPhase9GoldenManifest(assetSet.Fingerprint())
	if os.Getenv(phase9GoldenUpdateEnv) == "1" {
		capturePhase9GoldenMatrix(t, directory, manifest, true)
	}
	loaded := loadPhase9GoldenManifest(t, filepath.Join(directory, "manifest.json"))
	if !reflect.DeepEqual(loaded, manifest) {
		t.Fatal("Phase 9 golden manifest does not match the required workspace, lifecycle, viewport, scale, asset, and tolerance matrix")
	}
	for _, entry := range loaded.Entries {
		if err := entry.Golden.Metadata.Validate(); err != nil {
			t.Fatalf("validate %s metadata: %v", entry.Golden.File, err)
		}
		file, err := os.Open(filepath.Join(directory, entry.Golden.File))
		if err != nil {
			t.Fatal(err)
		}
		configuration, decodeErr := png.DecodeConfig(file)
		closeErr := file.Close()
		if err := errors.Join(decodeErr, closeErr); err != nil {
			t.Fatal(err)
		}
		if configuration.Width != entry.Golden.Metadata.Width || configuration.Height != entry.Golden.Metadata.Height {
			t.Fatalf("%s dimensions = %dx%d", entry.Golden.File, configuration.Width, configuration.Height)
		}
	}
	if os.Getenv(phase9GoldenVerifyEnv) == "1" {
		capturePhase9GoldenMatrix(t, directory, manifest, false)
	}
}

func TestPhase9GoldenCaptureHelper(t *testing.T) {
	encoded := os.Getenv(phase9GoldenChildEnv)
	if encoded == "" {
		t.Skip("Phase 9 golden capture helper")
	}
	var entry phase7GoldenEntry
	if err := json.Unmarshal([]byte(encoded), &entry); err != nil {
		t.Fatal(err)
	}
	capturePhase9GoldenEntry(t, os.Getenv(phase9GoldenPathEnv), entry, os.Getenv(phase9GoldenModeEnv) == "update")
}

func expectedPhase9GoldenManifest(fontFingerprint string) phase7GoldenManifest {
	workspaceScenarios := []struct {
		workspace appstate.Workspace
		scenarios []string
	}{
		{appstate.WorkspaceModels, []string{"normal", "loading", "failure", "degraded", "recovered"}},
		{appstate.WorkspaceInference, []string{"normal", "loading", "incompatible", "failure", "degraded", "recovered"}},
	}
	viewports := [][2]int{{1280, 720}, {1920, 1080}}
	scales := []int{100, 150, 200}
	manifest := phase7GoldenManifest{Version: 1}
	for _, group := range workspaceScenarios {
		for _, scenario := range group.scenarios {
			for _, viewport := range viewports {
				for _, scale := range scales {
					name := fmt.Sprintf("%s_%s_%dx%d_%d.png", group.workspace, scenario, viewport[0], viewport[1], scale)
					manifest.Entries = append(manifest.Entries, phase7GoldenEntry{
						Workspace: group.workspace,
						Golden: researchGoldenEntry{File: name, Metadata: golden.Metadata{
							BaselineVersion: 1, Seed: 209, Width: viewport[0], Height: viewport[1], ScalePercent: scale,
							FontFingerprint: fontFingerprint, ScenarioClock: researchGoldenClock, Backend: researchGoldenBackend,
							Scenario: string(group.workspace) + "_" + scenario,
						}, ChannelTolerance: 3, MaxDifferentRate: 0.002},
					})
				}
			}
		}
	}
	return manifest
}

func loadPhase9GoldenManifest(t *testing.T, path string) phase7GoldenManifest {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest phase7GoldenManifest
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func capturePhase9GoldenMatrix(t *testing.T, directory string, manifest phase7GoldenManifest, update bool) {
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
	entries := append([]phase7GoldenEntry(nil), manifest.Entries...)
	sort.Slice(entries, func(left int, right int) bool { return entries[left].Golden.File < entries[right].Golden.File })
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
		t.Run(entry.Golden.File, func(t *testing.T) {
			encoded, err := json.Marshal(entry)
			if err != nil {
				t.Fatal(err)
			}
			mode := "verify"
			if update {
				mode = "update"
			}
			command := exec.Command(executable, "-test.run=^TestPhase9GoldenCaptureHelper$", "-test.timeout=30s")
			command.Env = append(os.Environ(), phase9GoldenChildEnv+"="+string(encoded), phase9GoldenPathEnv+"="+absoluteDirectory, phase9GoldenModeEnv+"="+mode)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("capture subprocess: %v\n%s", err, output)
			}
		})
	}
}

func capturePhase9GoldenEntry(t *testing.T, directory string, entry phase7GoldenEntry, update bool) {
	t.Helper()
	capture := make(chan *image.RGBA, 1)
	processed := make(chan error, 1)
	path := filepath.Join(directory, entry.Golden.File)
	go func() {
		captured := <-capture
		if update {
			processed <- encodeResearchGolden(path, captured)
			return
		}
		processed <- compareResearchGolden(path, captured, entry.Golden)
	}()
	modelsScenario := appstate.ModelsScenarioNormal
	inferenceScenario := appstate.InferenceScenarioNormal
	scenario := entry.Golden.Metadata.Scenario
	if entry.Workspace == appstate.WorkspaceModels {
		modelsScenario = appstate.ModelsScenario(scenario[len("models_"):])
	} else {
		inferenceScenario = appstate.InferenceScenario(scenario[len("inference_"):])
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := Run(ctx, Options{Hidden: true, MaxFrames: 30, DemoSeed: entry.Golden.Metadata.Seed,
		Width: int32(entry.Golden.Metadata.Width), Height: int32(entry.Golden.Metadata.Height), InitialWorkspace: entry.Workspace,
		InitialUIScale: appstate.UIScalePreset(entry.Golden.Metadata.ScalePercent), ModelsScenario: modelsScenario, InferenceScenario: inferenceScenario,
		Clock: appstate.FixedClock{Time: entry.Golden.Metadata.ScenarioClock}, Capture: capture, LayoutDirectory: t.TempDir(), DisableEvents: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-processed; err != nil {
		t.Fatal(err)
	}
}
