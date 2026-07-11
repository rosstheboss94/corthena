package workstation

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHiddenWorkstationSmoke(t *testing.T) {
	layoutDirectory := t.TempDir()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Run(ctx, Options{Hidden: true, MaxFrames: 1, LayoutDirectory: layoutDirectory}); err != nil {
		t.Fatal(err)
	}
}

func TestHiddenWorkstationSmokeLargeWindow(t *testing.T) {
	layoutDirectory := t.TempDir()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Run(ctx, Options{
		Hidden: true, MaxFrames: 1, Width: 1920, Height: 1080, LayoutDirectory: layoutDirectory,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkstationRejectsInvalidWindowDimensions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := Run(ctx, Options{Hidden: true, MaxFrames: 1, Width: -1, Height: 720})
	if err == nil {
		t.Fatal("Run succeeded with invalid dimensions")
	}
	if !strings.Contains(err.Error(), "window dimensions must be positive") {
		t.Fatalf("error = %v, want window dimension validation", err)
	}
}
