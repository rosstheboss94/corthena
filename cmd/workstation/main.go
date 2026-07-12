package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rosstheboss94/corthena/internal/app/workstation"
	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func main() {
	runtime.LockOSThread()
	exitCode := run()
	runtime.UnlockOSThread()
	os.Exit(exitCode)
}

func run() int {
	smokeFrames := flag.Int(
		"smoke-frames",
		0,
		"close after this many hidden frames; zero runs interactively",
	)
	demoSeed := flag.Uint64(
		"demo-seed",
		1,
		"seed for deterministic demo coordinator data",
	)
	width := flag.Int(
		"width",
		1280,
		"initial workstation window width in logical pixels",
	)
	height := flag.Int(
		"height",
		720,
		"initial workstation window height in logical pixels",
	)
	initialWorkspace := flag.String(
		"workspace",
		"data",
		"initial workspace for interactive or hidden launch",
	)
	capturePath := flag.String(
		"capture",
		"",
		"write the final hidden smoke frame as PNG",
	)
	layoutDirectory := flag.String(
		"layout-directory",
		"",
		"override the user layout directory for isolated smoke verification",
	)
	uiScale := flag.Int("ui-scale", int(appstate.DefaultUIScale), "initial UI scale percentage")
	researchScenario := flag.String("research-scenario", string(appstate.ResearchScenarioNormal), "initial deterministic Research scenario")
	dataScenario := flag.String("data-scenario", string(appstate.DataScenarioNormal), "initial deterministic Data scenario")
	experimentScenario := flag.String("experiment-scenario", string(appstate.ExperimentScenarioNormal), "initial deterministic Experiments scenario")
	researchLinkedSelection := flag.Bool("research-linked-selection", false, "apply a deterministic linked Research range selection")
	scenarioClock := flag.String("scenario-clock", "", "fixed RFC3339 UTC clock for deterministic captures")
	flag.Parse()
	workspace := appstate.Workspace(*initialWorkspace)
	if !workspace.Valid() {
		_, _ = fmt.Fprintf(os.Stderr, "unsupported workspace %q\n", *initialWorkspace)
		return 2
	}
	scale := appstate.UIScalePreset(*uiScale)
	if !scale.Valid() {
		_, _ = fmt.Fprintf(os.Stderr, "unsupported UI scale %d\n", *uiScale)
		return 2
	}
	scenario := appstate.ResearchScenario(*researchScenario)
	if !scenario.Valid() {
		_, _ = fmt.Fprintf(os.Stderr, "unsupported Research scenario %q\n", *researchScenario)
		return 2
	}
	dataCondition := appstate.DataScenario(*dataScenario)
	if !dataCondition.Valid() {
		_, _ = fmt.Fprintf(os.Stderr, "unsupported Data scenario %q\n", *dataScenario)
		return 2
	}
	experimentCondition := appstate.ExperimentScenario(*experimentScenario)
	if !experimentCondition.Valid() {
		_, _ = fmt.Fprintf(os.Stderr, "unsupported Experiments scenario %q\n", *experimentScenario)
		return 2
	}
	var clock appstate.Clock
	if *scenarioClock != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *scenarioClock)
		if err != nil || parsed.Location() != time.UTC {
			_, _ = fmt.Fprintf(os.Stderr, "invalid UTC scenario clock %q\n", *scenarioClock)
			return 2
		}
		clock = appstate.FixedClock{Time: parsed}
	}

	var captures chan *image.RGBA
	var captureDone chan error
	if *capturePath != "" {
		if *smokeFrames <= 0 {
			_, _ = fmt.Fprintln(os.Stderr, "-capture requires positive -smoke-frames")
			return 2
		}
		captures = make(chan *image.RGBA, 1)
		captureDone = make(chan error, 1)
		go func() {
			captured := <-captures
			captureDone <- writeCapture(*capturePath, captured)
		}()
	}
	err := workstation.Run(context.Background(), workstation.Options{
		Hidden:                  *smokeFrames > 0,
		MaxFrames:               *smokeFrames,
		DemoSeed:                *demoSeed,
		Width:                   int32(*width),
		Height:                  int32(*height),
		InitialWorkspace:        workspace,
		Capture:                 captures,
		LayoutDirectory:         *layoutDirectory,
		InitialUIScale:          scale,
		ResearchScenario:        scenario,
		DataScenario:            dataCondition,
		ExperimentScenario:      experimentCondition,
		ResearchLinkedSelection: *researchLinkedSelection,
		Clock:                   clock,
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if captureDone != nil {
		if err := <-captureDone; err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

func writeCapture(path string, captured *image.RGBA) (resultErr error) {
	if captured == nil {
		return fmt.Errorf("write capture: image is nil")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("write capture: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return fmt.Errorf("write capture: %w", err)
	}
	file, err := os.Create(absolute)
	if err != nil {
		return fmt.Errorf("write capture: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, file.Close())
	}()
	if err := png.Encode(file, captured); err != nil {
		return fmt.Errorf("write capture: %w", err)
	}
	return nil
}
