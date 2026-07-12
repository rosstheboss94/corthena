// Package workstation owns the desktop application lifecycle.
package workstation

import (
	"context"
	"errors"
	"fmt"
	"image"
	"path/filepath"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/assets"
	"github.com/rosstheboss94/corthena/internal/frontend/drafts"
	"github.com/rosstheboss94/corthena/internal/frontend/effects"
	"github.com/rosstheboss94/corthena/internal/frontend/layouts"
	"github.com/rosstheboss94/corthena/internal/frontend/nativeui"
	"github.com/rosstheboss94/corthena/internal/frontend/preferences"
	"github.com/rosstheboss94/corthena/internal/frontend/simulator"
)

// Options configures the workstation lifecycle.
type Options struct {
	Hidden    bool
	MaxFrames int
	DemoSeed  uint64
	Width     int32
	Height    int32
	// InitialWorkspace selects a deterministic smoke-test entry workspace. An
	// empty value preserves the normal Data startup.
	InitialWorkspace   appstate.Workspace
	InitialUIScale     appstate.UIScalePreset
	ResearchScenario   appstate.ResearchScenario
	DataScenario       appstate.DataScenario
	ExperimentScenario appstate.ExperimentScenario
	JobsScenario       appstate.JobsScenario
	ResultsScenario    appstate.ResultsScenario
	ModelsScenario     appstate.ModelsScenario
	InferenceScenario  appstate.InferenceScenario
	// DisableEvents omits the unrelated demo event stream for deterministic
	// golden capture while retaining snapshot and workspace effects.
	DisableEvents bool
	// ResearchLinkedSelection applies a deterministic linked box-selection
	// preset after the Research catalog context becomes available.
	ResearchLinkedSelection bool
	// Clock fixes deterministic simulator and reducer time for captures and
	// tests. A nil clock uses real application time.
	Clock appstate.Clock
	// Capture receives an owned framebuffer copy on the final smoke frame. The
	// UI send is nonblocking; a background consumer owns encoding and I/O.
	Capture chan<- *image.RGBA
	// LayoutDirectory overrides the user application-data directory. It is
	// intended for isolated tests; an empty value uses the production path.
	LayoutDirectory string
}

// Run validates assets before native initialization, opens the workstation,
// draws frames on the calling UI thread, and performs a clean shutdown.
func Run(ctx context.Context, options Options) (resultErr error) {
	if options.MaxFrames < 0 {
		return errors.New("run workstation: maximum frame count cannot be negative")
	}
	width := options.Width
	height := options.Height
	if width == 0 {
		width = 1280
	}
	if height == 0 {
		height = 720
	}
	if width <= 0 || height <= 0 {
		return errors.New("run workstation: window dimensions must be positive")
	}
	if options.InitialUIScale != 0 && !options.InitialUIScale.Valid() {
		return fmt.Errorf("run workstation: unsupported initial UI scale %d", options.InitialUIScale)
	}
	if options.ResearchScenario != "" && !options.ResearchScenario.Valid() {
		return fmt.Errorf("run workstation: unsupported Research scenario %q", options.ResearchScenario)
	}
	if options.DataScenario != "" && !options.DataScenario.Valid() {
		return fmt.Errorf("run workstation: unsupported Data scenario %q", options.DataScenario)
	}
	if options.ExperimentScenario != "" && !options.ExperimentScenario.Valid() {
		return fmt.Errorf("run workstation: unsupported Experiments scenario %q", options.ExperimentScenario)
	}
	if options.JobsScenario != "" && !options.JobsScenario.Valid() {
		return fmt.Errorf("run workstation: unsupported Jobs scenario %q", options.JobsScenario)
	}
	if options.ResultsScenario != "" && !options.ResultsScenario.Valid() {
		return fmt.Errorf("run workstation: unsupported Results scenario %q", options.ResultsScenario)
	}
	if options.ModelsScenario != "" && !options.ModelsScenario.Valid() {
		return fmt.Errorf("run workstation: unsupported Models scenario %q", options.ModelsScenario)
	}
	if options.InferenceScenario != "" && !options.InferenceScenario.Valid() {
		return fmt.Errorf("run workstation: unsupported Inference scenario %q", options.InferenceScenario)
	}
	assetSet, err := assets.Load()
	if err != nil {
		return fmt.Errorf("run workstation: %w", err)
	}
	window, err := nativeui.Open(nativeui.Config{
		Width:  width,
		Height: height,
		Title:  "Corthena",
		Hidden: options.Hidden,
		FixedFPS: func() int32 {
			if options.Capture != nil {
				return 60
			}
			return 0
		}(),
	}, assetSet)
	if err != nil {
		return fmt.Errorf("run workstation: %w", err)
	}
	defer func() {
		if err := window.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("run workstation: %w", err))
		}
	}()

	clock := options.Clock
	if clock == nil {
		clock = appstate.RealClock{}
	}
	ids := appstate.NewSequentialIDSource("ui")
	state, startupEffects, err := appstate.NewInitialState(clock, ids)
	if err != nil {
		return fmt.Errorf("run workstation: %w", err)
	}
	if options.DisableEvents {
		filtered := make([]appstate.UIEffect, 0, len(startupEffects))
		for _, effect := range startupEffects {
			if _, subscription := effect.(appstate.SubscribeClientEventsEffect); !subscription {
				filtered = append(filtered, effect)
			}
		}
		startupEffects = filtered
	}
	if options.InitialUIScale != 0 {
		state.Preferences.UIScale = options.InitialUIScale
	}
	if options.ResearchScenario != "" {
		state.Research.Scenario = options.ResearchScenario
	}
	if options.DataScenario != "" {
		state.Data.Scenario = options.DataScenario
	}
	if options.ExperimentScenario != "" {
		state.Experiments.Scenario = options.ExperimentScenario
	}
	if options.JobsScenario != "" {
		state.JobsWorkspace.Scenario = options.JobsScenario
	}
	if options.ResultsScenario != "" {
		state.ResultsWorkspace.Scenario = options.ResultsScenario
	}
	if options.ModelsScenario != "" {
		state.ModelsWorkspace.Scenario = options.ModelsScenario
	}
	if options.InferenceScenario != "" {
		state.InferenceWorkspace.Scenario = options.InferenceScenario
	}
	if options.InitialWorkspace != "" && options.InitialWorkspace != state.ActiveWorkspace {
		var initialEffects []appstate.UIEffect
		var reduceErr error
		state, initialEffects, reduceErr = appstate.Reduce(state, appstate.SelectWorkspaceAction{Workspace: options.InitialWorkspace})
		if reduceErr != nil {
			return fmt.Errorf("run workstation: select initial workspace: %w", reduceErr)
		}
		startupEffects = append(startupEffects, initialEffects...)
	}
	client, err := simulator.NewDemoCoordinator(simulator.Options{
		Seed:  options.DemoSeed,
		Clock: clock,
	})
	if err != nil {
		return fmt.Errorf("run workstation: %w", err)
	}
	layoutDefaults := layouts.Snapshot{
		Revision: state.LayoutRevision,
		Layouts:  appstate.CloneWorkspaceLayouts(state.DefaultLayouts),
	}
	var layoutStore effects.LayoutStore
	if options.LayoutDirectory == "" {
		layoutStore, err = layouts.NewUserStore(layoutDefaults)
	} else {
		layoutStore, err = layouts.NewStore(options.LayoutDirectory, layoutDefaults)
	}
	if err != nil {
		return fmt.Errorf("run workstation: create layout store: %w", err)
	}
	var preferenceStore effects.PreferenceStore
	if options.LayoutDirectory == "" {
		preferenceStore, err = preferences.NewUserStore(state.Preferences)
	} else {
		preferenceStore, err = preferences.NewStore(filepath.Join(options.LayoutDirectory, "preferences.json"), state.Preferences)
	}
	if err != nil {
		return fmt.Errorf("run workstation: create preference store: %w", err)
	}
	var draftStore effects.ExperimentDraftStore
	if options.LayoutDirectory == "" {
		draftStore, err = drafts.NewUserStore(state.Experiments.Draft)
	} else {
		draftStore, err = drafts.NewStore(filepath.Join(options.LayoutDirectory, "experiment-draft.json"), state.Experiments.Draft)
	}
	if err != nil {
		return fmt.Errorf("run workstation: create experiment draft store: %w", err)
	}
	effectRuntime, err := effects.Start(
		ctx,
		client,
		layoutStore,
		effects.Config{Clock: clock, PreferenceStore: preferenceStore, DraftStore: draftStore},
	)
	if err != nil {
		return fmt.Errorf("run workstation: %w", err)
	}
	defer func() {
		if err := effectRuntime.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("run workstation: %w", err))
		}
	}()
	for _, effect := range startupEffects {
		if !effectRuntime.Enqueue(effect) {
			return errors.New("run workstation: startup effect queue is full")
		}
	}

	frames := 0
	linkedSelectionPending := options.ResearchLinkedSelection
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("run workstation: %w", err)
		}
		var reduceErr error
		state, reduceErr = drainActions(state, effectRuntime, clock)
		if reduceErr != nil {
			return fmt.Errorf("run workstation: %w", reduceErr)
		}
		if linkedSelectionPending {
			var applied bool
			state, applied, reduceErr = applyResearchLinkedSelection(state, effectRuntime, clock)
			if reduceErr != nil {
				return fmt.Errorf("run workstation: linked Research selection: %w", reduceErr)
			}
			linkedSelectionPending = !applied
		}
		shouldClose, err := window.ShouldClose()
		if err != nil {
			return fmt.Errorf("run workstation: %w", err)
		}
		if shouldClose {
			return nil
		}
		actions, err := window.DrawShellFrame(state)
		if err != nil {
			return fmt.Errorf("run workstation: %w", err)
		}
		if options.Capture != nil && options.MaxFrames > 0 && frames+1 >= options.MaxFrames {
			captured, captureErr := window.CaptureRGBA()
			if captureErr != nil {
				return fmt.Errorf("run workstation: %w", captureErr)
			}
			select {
			case options.Capture <- captured:
			default:
				return errors.New("run workstation: capture consumer is not ready")
			}
		}
		state, reduceErr = applyActions(state, actions, effectRuntime, clock)
		if reduceErr != nil {
			return fmt.Errorf("run workstation: %w", reduceErr)
		}
		frames++
		if options.MaxFrames > 0 && frames >= options.MaxFrames {
			return nil
		}
	}
}

func applyResearchLinkedSelection(
	state appstate.AppState,
	effectRuntime *effects.Runtime,
	clock appstate.Clock,
) (appstate.AppState, bool, error) {
	if state.ActiveWorkspace != appstate.WorkspaceResearch {
		return state, false, nil
	}
	for _, layout := range state.Layouts {
		if layout.Workspace != appstate.WorkspaceResearch {
			continue
		}
		panelID, found := panelOfType(layout.Root, appstate.PanelOHLCVChart)
		if !found {
			return state, false, errors.New("research OHLCV panel is missing")
		}
		for _, group := range layout.LinkGroups {
			if group.Name != "Default" || group.Context.TimeRange.Start.IsZero() || group.Context.TimeRange.End.IsZero() {
				continue
			}
			span := group.Context.TimeRange.End.Sub(group.Context.TimeRange.Start)
			selected := appstate.TimeRange{
				Start: group.Context.TimeRange.Start.Add(span / 4),
				End:   group.Context.TimeRange.End.Add(-span / 4),
			}
			next, err := applyAction(state, appstate.SetResearchRangeAction{
				GroupID: group.ID, SourcePanelID: panelID, TimeRange: selected,
			}, effectRuntime, clock)
			return next, true, err
		}
	}
	return state, false, nil
}

func panelOfType(node appstate.DockNode, panelType appstate.PanelType) (appstate.PanelID, bool) {
	switch typed := node.(type) {
	case appstate.SplitNode:
		if panelID, found := panelOfType(typed.First, panelType); found {
			return panelID, true
		}
		return panelOfType(typed.Second, panelType)
	case appstate.TabStackNode:
		for _, panel := range typed.Panels {
			if panel.Type == panelType {
				return panel.ID, true
			}
		}
	}
	return "", false
}

func drainActions(
	state appstate.AppState,
	effectRuntime *effects.Runtime,
	clock appstate.Clock,
) (appstate.AppState, error) {
	const maxActionsPerFrame = 32
	for range maxActionsPerFrame {
		select {
		case action, ok := <-effectRuntime.Actions():
			if !ok {
				return state, nil
			}
			next, err := applyAction(state, action, effectRuntime, clock)
			if err != nil {
				return state, err
			}
			state = next
		default:
			return state, nil
		}
	}
	return state, nil
}

func applyActions(
	state appstate.AppState,
	actions []appstate.UIAction,
	effectRuntime *effects.Runtime,
	clock appstate.Clock,
) (appstate.AppState, error) {
	var err error
	for _, action := range actions {
		state, err = applyAction(state, action, effectRuntime, clock)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}

func applyAction(
	state appstate.AppState,
	action appstate.UIAction,
	effectRuntime *effects.Runtime,
	clock appstate.Clock,
) (appstate.AppState, error) {
	next, nextEffects, err := appstate.Reduce(state, action)
	if err != nil {
		return state, err
	}
	state = next
	for _, effect := range nextEffects {
		if !effectRuntime.Enqueue(effect) {
			failed := appstate.EffectFailedAction{
				EffectID:  "",
				FailedAt:  clock.Now(),
				Operation: "enqueue reducer effect",
				Error: appstate.ErrorSnapshot{
					Code:      appstate.ErrorEffectBusy,
					Message:   "frontend effect queue is full",
					Retryable: true,
				},
			}
			next, _, err := appstate.Reduce(state, failed)
			if err != nil {
				return state, err
			}
			state = next
		}
	}
	return state, nil
}
