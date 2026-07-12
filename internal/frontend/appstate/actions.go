package appstate

import "time"

// UIAction is a closed set of typed transitions reduced on the render thread.
type UIAction interface {
	isUIAction()
}

// SelectWorkspaceAction activates a top-level workspace.
type SelectWorkspaceAction struct {
	Workspace Workspace
}

func (SelectWorkspaceAction) isUIAction() {}

// SetLinkContextAction updates the global analytical link context.
type SetLinkContextAction struct {
	Context LinkContext
}

func (SetLinkContextAction) isUIAction() {}

// SetCommandPaletteAction opens or closes the command palette modal.
type SetCommandPaletteAction struct {
	Open bool
}

func (SetCommandPaletteAction) isUIAction() {}

// SetSettingsOpenAction opens or closes the global Settings modal.
type SetSettingsOpenAction struct {
	Open bool
}

func (SetSettingsOpenAction) isUIAction() {}

// SetUIScaleAction applies and asynchronously persists one supported UI size.
type SetUIScaleAction struct {
	Scale UIScalePreset
}

func (SetUIScaleAction) isUIAction() {}

// DismissToastAction removes one visible nonblocking notification.
type DismissToastAction struct {
	ToastID EffectID
}

func (DismissToastAction) isUIAction() {}

// OpenPanelAction opens or focuses a panel in one workspace.
type OpenPanelAction struct {
	Workspace       Workspace
	Panel           PanelInstanceState
	PersistEffectID EffectID
	RequestedAt     time.Time
}

func (OpenPanelAction) isUIAction() {}

// ClosePanelAction closes one panel in a workspace.
type ClosePanelAction struct {
	Workspace       Workspace
	PanelID         PanelID
	PersistEffectID EffectID
	RequestedAt     time.Time
}

func (ClosePanelAction) isUIAction() {}

// ApplyWorkspaceLayoutAction installs one layout produced by the pure docking
// manager. The reducer validates and revisions it before persistence.
type ApplyWorkspaceLayoutAction struct {
	Layout WorkspaceLayout
}

func (ApplyWorkspaceLayoutAction) isUIAction() {}

// ResetWorkspaceLayoutAction restores the canonical layout for one workspace.
type ResetWorkspaceLayoutAction struct {
	Workspace Workspace
}

func (ResetWorkspaceLayoutAction) isUIAction() {}

// AssignPanelLinkGroupAction moves a visible or hidden panel into a named link
// group without changing its stable panel ID.
type AssignPanelLinkGroupAction struct {
	Workspace Workspace
	PanelID   PanelID
	GroupID   LinkGroupID
}

func (AssignPanelLinkGroupAction) isUIAction() {}

// UpsertLinkGroupAction creates or updates a named, colored link group.
type UpsertLinkGroupAction struct {
	Workspace Workspace
	Group     LinkGroup
}

func (UpsertLinkGroupAction) isUIAction() {}

// RemoveLinkGroupAction deletes a link group and reassigns its panels to an
// existing fallback group.
type RemoveLinkGroupAction struct {
	Workspace       Workspace
	GroupID         LinkGroupID
	FallbackGroupID LinkGroupID
}

func (RemoveLinkGroupAction) isUIAction() {}

// UpdateLinkGroupContextAction updates only the context scopes supported by
// the source panel.
type UpdateLinkGroupContextAction struct {
	Workspace     Workspace
	GroupID       LinkGroupID
	SourcePanelID PanelID
	Context       LinkContext
}

func (UpdateLinkGroupContextAction) isUIAction() {}

// RequestResearchAction starts or supersedes a typed linked Research query.
type RequestResearchAction struct {
	Query ResearchQuery
}

func (RequestResearchAction) isUIAction() {}

// SetResearchFeatureAction selects the visible feature and refreshes the
// affected link group without changing stable row selection.
type SetResearchFeatureAction struct {
	GroupID LinkGroupID
	Feature FeatureName
}

func (SetResearchFeatureAction) isUIAction() {}

// SetResearchScenarioAction selects and requests a deterministic demo state.
type SetResearchScenarioAction struct {
	GroupID  LinkGroupID
	Scenario ResearchScenario
}

func (SetResearchScenarioAction) isUIAction() {}

// SetResearchVisibilityAction controls chart layer visibility.
type SetResearchVisibilityAction struct {
	GroupID     LinkGroupID
	ShowOHLCV   bool
	ShowFeature bool
	ShowTarget  bool
}

func (SetResearchVisibilityAction) isUIAction() {}

// SelectResearchRowAction preserves stable-ID table selection across pages.
type SelectResearchRowAction struct {
	GroupID LinkGroupID
	RowID   string
	Toggle  bool
}

func (SelectResearchRowAction) isUIAction() {}

// SetResearchRangeAction updates one link group's visible range from a chart
// pan, zoom, selection, or reset operation.
type SetResearchRangeAction struct {
	GroupID       LinkGroupID
	SourcePanelID PanelID
	TimeRange     TimeRange
}

func (SetResearchRangeAction) isUIAction() {}

// ResearchQueryFailedAction records a generation-specific query failure.
type ResearchQueryFailedAction struct {
	GroupID    LinkGroupID
	Generation uint64
	FailedAt   time.Time
	Error      ErrorSnapshot
}

func (ResearchQueryFailedAction) isUIAction() {}

// ResearchQueryCancelledAction records explicit current-generation cancel.
type ResearchQueryCancelledAction struct {
	GroupID     LinkGroupID
	Generation  uint64
	CancelledAt time.Time
}

func (ResearchQueryCancelledAction) isUIAction() {}

// RequestDataWorkspaceAction starts or supersedes a Data workspace query.
type RequestDataWorkspaceAction struct{ Query DataWorkspaceQuery }

func (RequestDataWorkspaceAction) isUIAction() {}

// SetDataScenarioAction selects a deterministic Data workflow condition.
type SetDataScenarioAction struct{ Scenario DataScenario }

func (SetDataScenarioAction) isUIAction() {}

// SelectDataDatasetAction preserves stable catalog selection.
type SelectDataDatasetAction struct{ DatasetID DatasetID }

func (SelectDataDatasetAction) isUIAction() {}

// SelectDataImportAction preserves stable import queue selection.
type SelectDataImportAction struct{ ImportID string }

func (SelectDataImportAction) isUIAction() {}

// SubmitDataImportAction starts one validated atomic simulated import.
type SubmitDataImportAction struct{ Request DataImportRequest }

func (SubmitDataImportAction) isUIAction() {}

// DataQueryFailedAction records a generation-specific Data failure.
type DataQueryFailedAction struct {
	Generation uint64
	FailedAt   time.Time
	Error      ErrorSnapshot
}

func (DataQueryFailedAction) isUIAction() {}

// DataQueryCancelledAction records explicit current-generation cancellation.
type DataQueryCancelledAction struct {
	Generation  uint64
	CancelledAt time.Time
}

func (DataQueryCancelledAction) isUIAction() {}

// RequestExperimentsAction starts or supersedes an Experiments query.
type RequestExperimentsAction struct{ Query ExperimentQuery }

func (RequestExperimentsAction) isUIAction() {}

// SetExperimentScenarioAction selects deterministic editor behavior.
type SetExperimentScenarioAction struct{ Scenario ExperimentScenario }

func (SetExperimentScenarioAction) isUIAction() {}

// UpdateExperimentDraftAction commits one newer typed local draft revision.
type UpdateExperimentDraftAction struct {
	Draft     ExperimentDraft
	UpdatedAt time.Time
}

func (UpdateExperimentDraftAction) isUIAction() {}

// SelectExperimentSectionAction focuses one configuration tree section.
type SelectExperimentSectionAction struct{ Section ExperimentSection }

func (SelectExperimentSectionAction) isUIAction() {}

// SelectExperimentDefinitionAction focuses one immutable definition.
type SelectExperimentDefinitionAction struct{ ExperimentID ExperimentID }

func (SelectExperimentDefinitionAction) isUIAction() {}

// SubmitExperimentAction requests immutable idempotent submission.
type SubmitExperimentAction struct{ Command SubmitExperimentCommand }

func (SubmitExperimentAction) isUIAction() {}

// ExperimentQueryFailedAction records query/evaluation/submission failure.
type ExperimentQueryFailedAction struct {
	Generation uint64
	FailedAt   time.Time
	Error      ErrorSnapshot
}

func (ExperimentQueryFailedAction) isUIAction() {}

// ExperimentQueryCancelledAction records explicit workflow cancellation.
type ExperimentQueryCancelledAction struct {
	Generation  uint64
	CancelledAt time.Time
}

func (ExperimentQueryCancelledAction) isUIAction() {}

// ExperimentDraftLoadedAction applies a non-stale local draft load.
type ExperimentDraftLoadedAction struct {
	EffectID     EffectID
	BaseRevision uint64
	Draft        ExperimentDraft
	LoadedAt     time.Time
	Recovered    bool
	Diagnostic   string
}

func (ExperimentDraftLoadedAction) isUIAction() {}

// ExperimentDraftPersistedAction records successful local autosave.
type ExperimentDraftPersistedAction struct {
	EffectID EffectID
	Revision uint64
	SavedAt  time.Time
}

func (ExperimentDraftPersistedAction) isUIAction() {}

// ExperimentDraftPersistenceFailedAction records retryable autosave failure.
type ExperimentDraftPersistenceFailedAction struct {
	EffectID EffectID
	Revision uint64
	FailedAt time.Time
	Error    ErrorSnapshot
}

func (ExperimentDraftPersistenceFailedAction) isUIAction() {}

// RequestJobsWorkspaceAction starts or supersedes a Jobs query.
type RequestJobsWorkspaceAction struct{ Query JobsWorkspaceQuery }

func (RequestJobsWorkspaceAction) isUIAction() {}

// SetJobsScenarioAction selects one deterministic lifecycle demonstration.
type SetJobsScenarioAction struct{ Scenario JobsScenario }

func (SetJobsScenarioAction) isUIAction() {}

// SelectJobAction preserves stable job selection across queue refreshes.
type SelectJobAction struct{ JobID JobID }

func (SelectJobAction) isUIAction() {}

// ControlJobAction dispatches one legal typed job transition.
type ControlJobAction struct{ Command JobControlCommand }

func (ControlJobAction) isUIAction() {}

// JobsQueryFailedAction records a generation-specific Jobs failure.
type JobsQueryFailedAction struct {
	Generation uint64
	FailedAt   time.Time
	Error      ErrorSnapshot
}

func (JobsQueryFailedAction) isUIAction() {}

// JobsQueryCancelledAction records current-generation cancellation.
type JobsQueryCancelledAction struct {
	Generation  uint64
	CancelledAt time.Time
}

func (JobsQueryCancelledAction) isUIAction() {}

// RequestResultsWorkspaceAction starts or supersedes a Results query.
type RequestResultsWorkspaceAction struct{ Query ResultsWorkspaceQuery }

func (RequestResultsWorkspaceAction) isUIAction() {}

// SetResultsScenarioAction selects a deterministic Results workflow condition.
type SetResultsScenarioAction struct{ Scenario ResultsScenario }

func (SetResultsScenarioAction) isUIAction() {}

// SelectResultRunAction sets or toggles a stable comparison run.
type SelectResultRunAction struct {
	RunID  RunID
	Toggle bool
}

func (SelectResultRunAction) isUIAction() {}

// SetResultsFilterAction updates the run browser filter and refreshes results.
type SetResultsFilterAction struct{ Filter string }

func (SetResultsFilterAction) isUIAction() {}

// ResultsQueryFailedAction records a generation-specific Results failure.
type ResultsQueryFailedAction struct {
	Generation uint64
	FailedAt   time.Time
	Error      ErrorSnapshot
}

func (ResultsQueryFailedAction) isUIAction() {}

// ResultsQueryCancelledAction records current-generation cancellation.
type ResultsQueryCancelledAction struct {
	Generation  uint64
	CancelledAt time.Time
}

func (ResultsQueryCancelledAction) isUIAction() {}

// LayoutsLoadedAction applies an asynchronous startup or named-layout load.
// BaseRevision prevents a late load from overwriting newer local mutations.
type LayoutsLoadedAction struct {
	EffectID     EffectID
	BaseRevision uint64
	Revision     uint64
	Layouts      []WorkspaceLayout
	LoadedAt     time.Time
	Recovered    bool
	Diagnostic   string
	Promote      bool
}

func (LayoutsLoadedAction) isUIAction() {}

// ClientMessageAction applies one immutable client message.
type ClientMessageAction struct {
	Message ClientMessage
}

func (ClientMessageAction) isUIAction() {}

// LayoutPersistedAction records a successful background layout save.
type LayoutPersistedAction struct {
	EffectID EffectID
	Revision uint64
	SavedAt  time.Time
}

func (LayoutPersistedAction) isUIAction() {}

// LayoutPersistenceFailedAction records a failed save for one logical layout
// revision. Older failures cannot replace newer persistence state.
type LayoutPersistenceFailedAction struct {
	EffectID EffectID
	Revision uint64
	FailedAt time.Time
	Error    ErrorSnapshot
}

func (LayoutPersistenceFailedAction) isUIAction() {}

// PreferencesLoadedAction applies an asynchronous startup preference load.
type PreferencesLoadedAction struct {
	EffectID     EffectID
	BaseRevision uint64
	Revision     uint64
	Preferences  Preferences
	LoadedAt     time.Time
	Recovered    bool
	Diagnostic   string
}

func (PreferencesLoadedAction) isUIAction() {}

// PreferencesPersistedAction records a successful preference save.
type PreferencesPersistedAction struct {
	EffectID EffectID
	Revision uint64
	SavedAt  time.Time
}

func (PreferencesPersistedAction) isUIAction() {}

// PreferencesPersistenceFailedAction records a failed preference save.
type PreferencesPersistenceFailedAction struct {
	EffectID EffectID
	Revision uint64
	FailedAt time.Time
	Error    ErrorSnapshot
}

func (PreferencesPersistenceFailedAction) isUIAction() {}

// EffectFailedAction records a typed background effect failure.
type EffectFailedAction struct {
	EffectID  EffectID
	FailedAt  time.Time
	Operation string
	Error     ErrorSnapshot
}

func (EffectFailedAction) isUIAction() {}

// UIEffect is a closed set of background operations requested by reducers.
type UIEffect interface {
	isUIEffect()
}

// LoadSnapshotEffect reconciles initial state through the frontend client.
type LoadSnapshotEffect struct {
	ID            EffectID
	CorrelationID CorrelationID
	RequestedAt   time.Time
}

func (LoadSnapshotEffect) isUIEffect() {}

// SubscribeClientEventsEffect starts the client event subscription.
type SubscribeClientEventsEffect struct {
	ID      EffectID
	Since   EventID
	Started time.Time
}

func (SubscribeClientEventsEffect) isUIEffect() {}

// LoadLayoutsEffect loads the autosaved layout set on a background worker.
type LoadLayoutsEffect struct {
	ID           EffectID
	BaseRevision uint64
	Defaults     []WorkspaceLayout
	RequestedAt  time.Time
}

func (LoadLayoutsEffect) isUIEffect() {}

// PersistLayoutEffect saves one workspace layout off the render thread.
type PersistLayoutEffect struct {
	ID        EffectID
	Workspace Workspace
	Layout    WorkspaceLayout
	Revision  uint64
	Requested time.Time
}

func (PersistLayoutEffect) isUIEffect() {}

// PersistLayoutsEffect atomically saves a complete immutable layout snapshot.
type PersistLayoutsEffect struct {
	ID        EffectID
	Revision  uint64
	Layouts   []WorkspaceLayout
	Requested time.Time
}

func (PersistLayoutsEffect) isUIEffect() {}

// LoadPreferencesEffect loads global preferences on a background worker.
type LoadPreferencesEffect struct {
	ID           EffectID
	BaseRevision uint64
	Defaults     Preferences
	RequestedAt  time.Time
}

func (LoadPreferencesEffect) isUIEffect() {}

// PersistPreferencesEffect saves one immutable preference revision.
type PersistPreferencesEffect struct {
	ID          EffectID
	Revision    uint64
	Preferences Preferences
	Requested   time.Time
}

func (PersistPreferencesEffect) isUIEffect() {}

// QueryResearchEffect performs client preparation off the render thread.
type QueryResearchEffect struct {
	ID    EffectID
	Query ResearchQuery
}

func (QueryResearchEffect) isUIEffect() {}

// CancelResearchEffect cancels hidden or superseded link-group work.
type CancelResearchEffect struct {
	ID         EffectID
	GroupID    LinkGroupID
	Generation uint64
}

func (CancelResearchEffect) isUIEffect() {}

// QueryDataWorkspaceEffect prepares Data catalog/import state off-thread.
type QueryDataWorkspaceEffect struct {
	ID    EffectID
	Query DataWorkspaceQuery
}

func (QueryDataWorkspaceEffect) isUIEffect() {}

// ImportDataEffect performs one simulated atomic import off-thread.
type ImportDataEffect struct {
	ID      EffectID
	Request DataImportRequest
}

func (ImportDataEffect) isUIEffect() {}

// CancelDataEffect cancels the current Data generation.
type CancelDataEffect struct {
	ID         EffectID
	Generation uint64
}

func (CancelDataEffect) isUIEffect() {}

// QueryExperimentsEffect prepares experiment definitions and descriptors.
type QueryExperimentsEffect struct {
	ID    EffectID
	Query ExperimentQuery
}

func (QueryExperimentsEffect) isUIEffect() {}

// EvaluateExperimentEffect validates and estimates a draft off-thread.
type EvaluateExperimentEffect struct {
	ID      EffectID
	Request ExperimentEvaluationRequest
}

func (EvaluateExperimentEffect) isUIEffect() {}

// SubmitExperimentEffect freezes one valid immutable definition.
type SubmitExperimentEffect struct {
	ID      EffectID
	Command SubmitExperimentCommand
}

func (SubmitExperimentEffect) isUIEffect() {}

// CancelExperimentEffect cancels one current experiment generation.
type CancelExperimentEffect struct {
	ID         EffectID
	Generation uint64
}

func (CancelExperimentEffect) isUIEffect() {}

// LoadExperimentDraftEffect loads local draft autosave off-thread.
type LoadExperimentDraftEffect struct {
	ID           EffectID
	BaseRevision uint64
	Defaults     ExperimentDraft
	RequestedAt  time.Time
}

func (LoadExperimentDraftEffect) isUIEffect() {}

// PersistExperimentDraftEffect coalesces one immutable draft revision.
type PersistExperimentDraftEffect struct {
	ID        EffectID
	Revision  uint64
	Draft     ExperimentDraft
	Requested time.Time
}

func (PersistExperimentDraftEffect) isUIEffect() {}

// QueryJobsWorkspaceEffect prepares job queue telemetry off-thread.
type QueryJobsWorkspaceEffect struct {
	ID    EffectID
	Query JobsWorkspaceQuery
}

func (QueryJobsWorkspaceEffect) isUIEffect() {}

// ControlJobEffect sends one validated idempotent job control command.
type ControlJobEffect struct {
	ID      EffectID
	Command JobControlCommand
}

func (ControlJobEffect) isUIEffect() {}

// CancelJobsEffect cancels the current Jobs generation.
type CancelJobsEffect struct {
	ID         EffectID
	Generation uint64
}

func (CancelJobsEffect) isUIEffect() {}

// QueryResultsWorkspaceEffect prepares immutable result comparisons off-thread.
type QueryResultsWorkspaceEffect struct {
	ID    EffectID
	Query ResultsWorkspaceQuery
}

func (QueryResultsWorkspaceEffect) isUIEffect() {}

// CancelResultsEffect cancels the current Results generation.
type CancelResultsEffect struct {
	ID         EffectID
	Generation uint64
}

func (CancelResultsEffect) isUIEffect() {}
