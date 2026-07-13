package workstation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/assets"
	"github.com/rosstheboss94/corthena/internal/frontend/nativeui"
)

type phase5GoldenManifest struct {
	Version int                 `json:"version"`
	Source  string              `json:"source"`
	Entries []phase5GoldenEntry `json:"entries"`
}

type phase5GoldenEntry struct {
	File              string               `json:"file"`
	SHA256            string               `json:"sha256"`
	Metadata          phase5GoldenMetadata `json:"metadata"`
	ChannelTolerance  uint8                `json:"channel_tolerance"`
	MaxDifferentRatio float64              `json:"max_different_ratio"`
}

type phase5GoldenMetadata struct {
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
	Fixture             string    `json:"fixture"`
	LayoutName          string    `json:"layout_name"`
	LayoutStateRevision uint64    `json:"layout_state_revision"`
	AppStateRevision    uint64    `json:"app_state_revision"`
	Backend             string    `json:"backend"`
	BuildIdentity       string    `json:"build_identity"`
	DirtyBuild          bool      `json:"dirty_build"`
}

func TestPhase5GoldenManifest(t *testing.T) {
	directory := filepath.Join("testdata", "phase5-golden")
	contents, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest phase5GoldenManifest
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 1 || manifest.Source != "Go workstation deterministic Phase 5 capture helper" || len(manifest.Entries) != 6 {
		t.Fatal("Phase 5 golden manifest is incomplete")
	}
	wanted := map[[3]int]bool{}
	for _, widthHeight := range [][2]int{{1280, 720}, {1920, 1080}} {
		for _, scale := range []int{100, 150, 200} {
			wanted[[3]int{widthHeight[0], widthHeight[1], scale}] = true
		}
	}
	for _, entry := range manifest.Entries {
		metadata := entry.Metadata
		key := [3]int{metadata.Width, metadata.Height, metadata.ScalePercent}
		if !wanted[key] || metadata.BaselineVersion != 1 || metadata.Seed != 42 || metadata.Workspace != "generic-visualization" || metadata.Scenario != "normal" || metadata.HiddenFrames != 3 || metadata.FontIconFingerprint == "" || metadata.ScenarioClock.UTC() != metadata.ScenarioClock || metadata.Fixture != "phase5-generic-v1" || metadata.LayoutName != "phase5-generic" || metadata.LayoutStateRevision != 1 || metadata.AppStateRevision != 1 || metadata.Backend == "" || metadata.BuildIdentity == "" || !metadata.DirtyBuild || entry.ChannelTolerance != 3 || entry.MaxDifferentRatio != 0.002 {
			t.Fatalf("%s has incomplete or noncanonical metadata", entry.File)
		}
		delete(wanted, key)
		imageBytes, err := os.ReadFile(filepath.Join(directory, entry.File))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(imageBytes)
		if hex.EncodeToString(digest[:]) != entry.SHA256 {
			t.Fatalf("%s checksum does not match manifest", entry.File)
		}
		configuration, err := png.DecodeConfig(bytes.NewReader(imageBytes))
		if err != nil || configuration.Width != metadata.Width || configuration.Height != metadata.Height {
			t.Fatalf("%s dimensions do not match manifest", entry.File)
		}
	}
	if len(wanted) != 0 {
		t.Fatal("Phase 5 golden matrix is incomplete")
	}
}

// TestPhase5GoldenCaptureHelper is an explicit opt-in deterministic legacy Go
// capture helper. Normal test runs never rewrite reviewed baselines.
func TestPhase5GoldenCaptureHelper(t *testing.T) {
	output := os.Getenv("CORTHENA_PHASE5_CAPTURE_PATH")
	if output == "" {
		t.Skip("Phase 5 golden capture helper")
	}
	width, err := strconv.Atoi(os.Getenv("CORTHENA_PHASE5_CAPTURE_WIDTH"))
	if err != nil {
		t.Fatal(err)
	}
	height, err := strconv.Atoi(os.Getenv("CORTHENA_PHASE5_CAPTURE_HEIGHT"))
	if err != nil {
		t.Fatal(err)
	}
	scale, err := strconv.Atoi(os.Getenv("CORTHENA_PHASE5_CAPTURE_SCALE"))
	if err != nil {
		t.Fatal(err)
	}
	assetSet, err := assets.Load()
	if err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	window, err := nativeui.Open(nativeui.Config{Width: int32(width), Height: int32(height), Title: "Corthena Phase 5", Hidden: true}, assetSet)
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	for range 3 {
		if err := window.DrawPhase5GoldenFrame(scale); err != nil {
			t.Fatal(err)
		}
	}
	captured, err := window.CaptureRGBA()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, captured); err != nil {
		t.Fatal(err)
	}
}
