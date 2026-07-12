// Package appstate owns the typed frontend state machine used by the Raylib
// shell. It intentionally has no dependency on Raylib, the simulator, storage,
// or network adapters.
package appstate

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvariant identifies an impossible frontend state or variant.
var ErrInvariant = errors.New("frontend invariant violation")

// Clock supplies deterministic timestamps to state construction and actions.
type Clock interface {
	Now() time.Time
}

// RealClock returns UTC wall-clock timestamps.
type RealClock struct{}

// Now returns the current UTC time.
func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

// FixedClock returns one configured timestamp.
type FixedClock struct {
	Time time.Time
}

// Now returns the configured UTC timestamp.
func (clock FixedClock) Now() time.Time {
	return clock.Time.UTC()
}

// IDSource supplies deterministic IDs at UI boundaries before actions are
// reduced.
type IDSource interface {
	NewPanelID() PanelID
	NewEffectID() EffectID
	NewCorrelationID() CorrelationID
}

// SequentialIDSource is a deterministic ID source for demos and tests. It is
// not safe for concurrent use; callers inject IDs before crossing goroutines.
type SequentialIDSource struct {
	prefix string
	next   uint64
}

// NewSequentialIDSource creates a source that emits stable prefixed IDs.
func NewSequentialIDSource(prefix string) *SequentialIDSource {
	if prefix == "" {
		prefix = "ui"
	}
	return &SequentialIDSource{prefix: prefix}
}

// NewPanelID returns the next panel ID.
func (source *SequentialIDSource) NewPanelID() PanelID {
	return PanelID(source.nextID("panel"))
}

// NewEffectID returns the next effect ID.
func (source *SequentialIDSource) NewEffectID() EffectID {
	return EffectID(source.nextID("effect"))
}

// NewCorrelationID returns the next correlation ID.
func (source *SequentialIDSource) NewCorrelationID() CorrelationID {
	return CorrelationID(source.nextID("corr"))
}

func (source *SequentialIDSource) nextID(kind string) string {
	source.next++
	return fmt.Sprintf("%s-%s-%04d", source.prefix, kind, source.next)
}

// Opaque identifier types used by the frontend boundary.
type (
	DatasetID     string
	ExperimentID  string
	JobID         string
	RunID         string
	ModelID       string
	InferenceID   string
	PanelID       string
	DockNodeID    string
	LinkGroupID   string
	EffectID      string
	EventID       string
	CorrelationID string
	Symbol        string
	FeatureName   string
)

// Workspace identifies one of the top-level workstation workspaces.
type Workspace string

const (
	WorkspaceData        Workspace = "data"
	WorkspaceResearch    Workspace = "research"
	WorkspaceExperiments Workspace = "experiments"
	WorkspaceJobs        Workspace = "jobs"
	WorkspaceResults     Workspace = "results"
	WorkspaceModels      Workspace = "models"
	WorkspaceInference   Workspace = "inference"
)

var orderedWorkspaces = []Workspace{
	WorkspaceData,
	WorkspaceResearch,
	WorkspaceExperiments,
	WorkspaceJobs,
	WorkspaceResults,
	WorkspaceModels,
	WorkspaceInference,
}

// Workspaces returns the stable top-tab order.
func Workspaces() []Workspace {
	output := make([]Workspace, len(orderedWorkspaces))
	copy(output, orderedWorkspaces)
	return output
}

// Valid reports whether the workspace is one of the closed variants.
func (workspace Workspace) Valid() bool {
	switch workspace {
	case WorkspaceData,
		WorkspaceResearch,
		WorkspaceExperiments,
		WorkspaceJobs,
		WorkspaceResults,
		WorkspaceModels,
		WorkspaceInference:
		return true
	default:
		return false
	}
}

// BarInterval is a validated display interval label.
type BarInterval string

const (
	IntervalDaily  BarInterval = "1d"
	IntervalHourly BarInterval = "1h"
)

// Valid reports whether the interval is one of the closed variants.
func (interval BarInterval) Valid() bool {
	switch interval {
	case IntervalDaily, IntervalHourly:
		return true
	default:
		return false
	}
}

// TimeRange is an immutable UTC range used for linked views.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Normalize returns the range with UTC timestamps.
func (timeRange TimeRange) Normalize() TimeRange {
	return TimeRange{
		Start: timeRange.Start.UTC(),
		End:   timeRange.End.UTC(),
	}
}

// LinkScope declares which context fields a panel can synchronize.
type LinkScope string

const (
	LinkDataset    LinkScope = "dataset"
	LinkSymbols    LinkScope = "symbols"
	LinkInterval   LinkScope = "interval"
	LinkTimeRange  LinkScope = "time_range"
	LinkExperiment LinkScope = "experiment"
	LinkRun        LinkScope = "run"
	LinkModel      LinkScope = "model"
)

// LinkContext is the synchronized analytical selection shared by linked panels.
type LinkContext struct {
	DatasetID    DatasetID
	Symbols      []Symbol
	Interval     BarInterval
	TimeRange    TimeRange
	ExperimentID ExperimentID
	RunID        RunID
	ModelID      ModelID
}

// Clone returns an independent immutable copy.
func (context LinkContext) Clone() LinkContext {
	context.Symbols = cloneSymbols(context.Symbols)
	context.TimeRange = context.TimeRange.Normalize()
	return context
}

// Component identifies a coordinator/runtime component.
type Component string

const (
	ComponentCoordinator Component = "coordinator"
	ComponentCatalog     Component = "catalog"
	ComponentScheduler   Component = "scheduler"
	ComponentWorkerPool  Component = "worker_pool"
	ComponentCache       Component = "cache"
)

// ComponentState is the health state of one component.
type ComponentState string

const (
	ComponentStarting ComponentState = "starting"
	ComponentHealthy  ComponentState = "healthy"
	ComponentDegraded ComponentState = "degraded"
	ComponentStopping ComponentState = "stopping"
	ComponentFailed   ComponentState = "failed"
)

// ComponentSnapshot is immutable component health received from the client.
type ComponentSnapshot struct {
	Component Component
	State     ComponentState
	Detail    string
	UpdatedAt time.Time
}

// ConnectionState is the frontend client's coordinator connection state.
type ConnectionState string

const (
	ConnectionStarting     ConnectionState = "starting"
	ConnectionConnected    ConnectionState = "connected"
	ConnectionDegraded     ConnectionState = "degraded"
	ConnectionDisconnected ConnectionState = "disconnected"
)

// ConnectionSnapshot describes the client connection and reconciliation cursor.
type ConnectionSnapshot struct {
	State       ConnectionState
	LastEventID EventID
	UpdatedAt   time.Time
	Detail      string
}

// CacheSnapshot describes cache metadata visible to the shell.
type CacheSnapshot struct {
	Generation uint64
	BytesUsed  uint64
	UpdatedAt  time.Time
}

// ErrorCode is a stable machine-readable frontend error code.
type ErrorCode string

const (
	ErrorClientUnavailable   ErrorCode = "client_unavailable"
	ErrorEffectBusy          ErrorCode = "effect_busy"
	ErrorInvariant           ErrorCode = "invariant"
	ErrorPersistence         ErrorCode = "persistence"
	ErrorResearchFailed      ErrorCode = "research_failed"
	ErrorResearchCancelled   ErrorCode = "research_cancelled"
	ErrorDataFailed          ErrorCode = "data_failed"
	ErrorDataCancelled       ErrorCode = "data_cancelled"
	ErrorExperimentFailed    ErrorCode = "experiment_failed"
	ErrorExperimentCancelled ErrorCode = "experiment_cancelled"
	ErrorValidation          ErrorCode = "validation"
)

// ErrorSnapshot is a typed immutable error payload suitable for state.
type ErrorSnapshot struct {
	Code          ErrorCode
	Message       string
	Retryable     bool
	CorrelationID CorrelationID
}

// ToastKind identifies user-visible notification severity.
type ToastKind string

const (
	ToastInfo    ToastKind = "info"
	ToastWarning ToastKind = "warning"
	ToastError   ToastKind = "error"
)

// Toast is a nonblocking notification kept in typed state.
type Toast struct {
	ID        EffectID
	Kind      ToastKind
	Message   string
	CreatedAt time.Time
}

// UIScalePreset is one supported application UI-size percentage.
type UIScalePreset uint16

const (
	UIScale100 UIScalePreset = 100
	UIScale125 UIScalePreset = 125
	UIScale150 UIScalePreset = 150
	UIScale175 UIScalePreset = 175
	UIScale200 UIScalePreset = 200

	// DefaultUIScale is the comfortable initial application UI size.
	DefaultUIScale = UIScale125
)

var orderedUIScales = []UIScalePreset{
	UIScale100,
	UIScale125,
	UIScale150,
	UIScale175,
	UIScale200,
}

// Valid reports whether the preset is supported.
func (scale UIScalePreset) Valid() bool {
	switch scale {
	case UIScale100, UIScale125, UIScale150, UIScale175, UIScale200:
		return true
	default:
		return false
	}
}

// UIScalePresets returns the stable display order.
func UIScalePresets() []UIScalePreset {
	result := make([]UIScalePreset, len(orderedUIScales))
	copy(result, orderedUIScales)
	return result
}

// StepUIScale moves by one preset and clamps at the supported endpoints.
func StepUIScale(current UIScalePreset, direction int) UIScalePreset {
	if !current.Valid() {
		current = DefaultUIScale
	}
	index := 0
	for candidate, scale := range orderedUIScales {
		if scale == current {
			index = candidate
			break
		}
	}
	if direction < 0 && index > 0 {
		index--
	}
	if direction > 0 && index < len(orderedUIScales)-1 {
		index++
	}
	return orderedUIScales[index]
}

// Preferences contains global frontend behavior independent of layouts.
type Preferences struct {
	UIScale UIScalePreset
}

// DefaultPreferences returns the canonical initial frontend preferences.
func DefaultPreferences() Preferences {
	return Preferences{UIScale: DefaultUIScale}
}

// Validate checks that every preference is a closed supported value.
func (preferences Preferences) Validate() error {
	if !preferences.UIScale.Valid() {
		return fmt.Errorf("%w: unsupported UI scale %d", ErrInvariant, preferences.UIScale)
	}
	return nil
}

// OverlayState owns modal and toast state.
type OverlayState struct {
	CommandPaletteOpen bool
	SettingsOpen       bool
	CriticalError      ErrorSnapshot
	Toasts             []Toast
}

// Clone returns an independent copy.
func (overlays OverlayState) Clone() OverlayState {
	overlays.Toasts = cloneToasts(overlays.Toasts)
	return overlays
}

// PersistenceState records layout persistence status.
type PersistenceState struct {
	LastSavedAt       time.Time
	LastSavedRevision uint64
	PendingRevision   uint64
	LastErrorRevision uint64
	LastError         ErrorSnapshot
}

func cloneToasts(input []Toast) []Toast {
	if len(input) == 0 {
		return nil
	}
	output := make([]Toast, len(input))
	for index, toast := range input {
		toast.CreatedAt = toast.CreatedAt.UTC()
		output[index] = toast
	}
	return output
}

// AppState is the complete pure frontend state owned by the render loop.
type AppState struct {
	StartedAt             time.Time
	ActiveWorkspace       Workspace
	LinkContext           LinkContext
	Connection            ConnectionSnapshot
	Components            []ComponentSnapshot
	Cache                 CacheSnapshot
	Layouts               []WorkspaceLayout
	DefaultLayouts        []WorkspaceLayout
	LayoutRevision        uint64
	Preferences           Preferences
	PreferenceRevision    uint64
	Datasets              []DatasetSummary
	Jobs                  []JobSummary
	Results               []RunResultSummary
	Models                []ModelSummary
	Inferences            []InferenceSummary
	Research              ResearchWorkspaceState
	Data                  DataWorkspaceState
	Experiments           ExperimentsWorkspaceState
	Overlays              OverlayState
	Persistence           PersistenceState
	PreferencePersistence PersistenceState
}

// Clone returns an independent copy of the state.
func (state AppState) Clone() AppState {
	state.LinkContext = state.LinkContext.Clone()
	state.Components = cloneComponents(state.Components)
	state.Layouts = cloneLayouts(state.Layouts)
	state.DefaultLayouts = cloneLayouts(state.DefaultLayouts)
	state.Datasets = cloneDatasets(state.Datasets)
	state.Jobs = cloneJobs(state.Jobs)
	state.Results = cloneResults(state.Results)
	state.Models = cloneModels(state.Models)
	state.Inferences = cloneInferences(state.Inferences)
	state.Research = state.Research.Clone()
	state.Data = state.Data.Clone()
	state.Experiments = state.Experiments.Clone()
	state.Overlays = state.Overlays.Clone()
	return state
}

// NewInitialState constructs deterministic initial state and startup effects.
func NewInitialState(clock Clock, ids IDSource) (AppState, []UIEffect, error) {
	if clock == nil {
		return AppState{}, nil, errors.New("new initial frontend state: clock is nil")
	}
	if ids == nil {
		return AppState{}, nil, errors.New("new initial frontend state: ID source is nil")
	}
	now := clock.Now().UTC()
	layouts := make([]WorkspaceLayout, 0, len(orderedWorkspaces))
	for _, workspace := range orderedWorkspaces {
		layout, err := DefaultWorkspaceLayout(workspace, ids)
		if err != nil {
			return AppState{}, nil, err
		}
		layouts = append(layouts, layout)
	}
	state := AppState{
		StartedAt:       now,
		ActiveWorkspace: WorkspaceData,
		LinkContext: LinkContext{
			Interval: IntervalDaily,
			TimeRange: TimeRange{
				Start: now.AddDate(0, -6, 0),
				End:   now,
			},
		},
		Connection: ConnectionSnapshot{
			State:     ConnectionStarting,
			UpdatedAt: now,
			Detail:    "initializing demo coordinator",
		},
		Components: []ComponentSnapshot{
			{
				Component: ComponentCoordinator,
				State:     ComponentStarting,
				Detail:    "starting",
				UpdatedAt: now,
			},
		},
		Layouts:        layouts,
		DefaultLayouts: cloneLayouts(layouts),
		Preferences:    DefaultPreferences(),
		Research:       DefaultResearchWorkspaceState(),
		Data:           DefaultDataWorkspaceState(),
		Experiments:    DefaultExperimentsWorkspaceState(),
	}
	effects := []UIEffect{
		LoadExperimentDraftEffect{
			ID: ids.NewEffectID(), BaseRevision: 0,
			Defaults: ExperimentDraft{}, RequestedAt: now,
		},
		LoadPreferencesEffect{
			ID:           ids.NewEffectID(),
			BaseRevision: state.PreferenceRevision,
			Defaults:     state.Preferences,
			RequestedAt:  now,
		},
		LoadLayoutsEffect{
			ID:           ids.NewEffectID(),
			BaseRevision: state.LayoutRevision,
			Defaults:     cloneLayouts(layouts),
			RequestedAt:  now,
		},
		LoadSnapshotEffect{
			ID:            ids.NewEffectID(),
			CorrelationID: ids.NewCorrelationID(),
			RequestedAt:   now,
		},
		SubscribeClientEventsEffect{
			ID:      ids.NewEffectID(),
			Since:   "",
			Started: now,
		},
	}
	return state, effects, nil
}
