package appstate

import "fmt"

// LayoutSchemaVersion is the current serialized workspace layout schema.
const LayoutSchemaVersion = 2

// PanelType identifies one frontend panel kind.
type PanelType string

const (
	PanelCatalogTable        PanelType = "catalog_table"
	PanelCoverageTimeline    PanelType = "coverage_timeline"
	PanelImportQueue         PanelType = "import_queue"
	PanelDatasetInspector    PanelType = "dataset_inspector"
	PanelImportLogs          PanelType = "import_logs"
	PanelOHLCVChart          PanelType = "ohlcv_chart"
	PanelFeatureBrowser      PanelType = "feature_browser"
	PanelSeriesInspector     PanelType = "series_inspector"
	PanelTargetPreview       PanelType = "target_preview"
	PanelDistributions       PanelType = "distributions"
	PanelRowTable            PanelType = "row_table"
	PanelExperimentList      PanelType = "experiment_list"
	PanelConfigurationTree   PanelType = "configuration_tree"
	PanelPropertyEditor      PanelType = "property_editor"
	PanelExperimentInspector PanelType = "experiment_inspector"
	PanelValidationSummary   PanelType = "validation_summary"
	PanelResourceEstimate    PanelType = "resource_estimate"
	PanelJobQueue            PanelType = "job_queue"
	PanelJobProgress         PanelType = "job_progress"
	PanelLiveMetrics         PanelType = "live_metrics"
	PanelWorkerResources     PanelType = "worker_resources"
	PanelProcessStatus       PanelType = "process_status"
	PanelCheckpointStatus    PanelType = "checkpoint_status"
	PanelJobLogs             PanelType = "job_logs"
	PanelRunBrowser          PanelType = "run_browser"
	PanelMetricComparison    PanelType = "metric_comparison"
	PanelEquityChart         PanelType = "equity_chart"
	PanelFoldTimeline        PanelType = "fold_timeline"
	PanelPredictionOverlay   PanelType = "prediction_overlay"
	PanelResultDistributions PanelType = "result_distributions"
	PanelConfigurationDiff   PanelType = "configuration_diff"
	PanelModelRegistry       PanelType = "model_registry"
	PanelAliasHistory        PanelType = "alias_history"
	PanelArtifactMetadata    PanelType = "artifact_metadata"
	PanelFeatureImportance   PanelType = "feature_importance"
	PanelTreeInspector       PanelType = "tree_inspector"
	PanelModelSelector       PanelType = "model_selector"
	PanelInferenceDataset    PanelType = "inference_dataset"
	PanelRankedScores        PanelType = "ranked_scores"
	PanelScoreDistribution   PanelType = "score_distribution"
	PanelPredictionHistory   PanelType = "prediction_history"
	PanelExportStatus        PanelType = "export_status"
)

// PanelMultiplicity declares whether multiple instances are allowed.
type PanelMultiplicity string

const (
	PanelSingleton PanelMultiplicity = "singleton"
	PanelMultiple  PanelMultiplicity = "multiple"
)

// Size is a logical frontend size.
type Size struct {
	Width  int
	Height int
}

// PanelDescriptor describes one panel kind.
type PanelDescriptor struct {
	Type           PanelType
	Title          string
	Icon           string
	Multiplicity   PanelMultiplicity
	MinimumSize    Size
	SupportedLinks []LinkScope
}

// Clone returns an independent copy.
func (descriptor PanelDescriptor) Clone() PanelDescriptor {
	descriptor.SupportedLinks = cloneLinkScopes(descriptor.SupportedLinks)
	return descriptor
}

// PanelDescriptorFor returns the descriptor for a closed panel type.
func PanelDescriptorFor(panelType PanelType) (PanelDescriptor, error) {
	switch panelType {
	case PanelCatalogTable:
		return descriptor(panelType, "Catalog", "database", PanelSingleton, 360, 220, LinkDataset), nil
	case PanelCoverageTimeline:
		return descriptor(panelType, "Coverage", "activity", PanelSingleton, 320, 180, LinkDataset, LinkSymbols, LinkTimeRange), nil
	case PanelImportQueue:
		return descriptor(panelType, "Import Queue", "list-plus", PanelSingleton, 320, 180), nil
	case PanelDatasetInspector:
		return descriptor(panelType, "Dataset", "info", PanelMultiple, 280, 180, LinkDataset), nil
	case PanelImportLogs:
		return descriptor(panelType, "Import Logs", "scroll-text", PanelSingleton, 320, 160), nil
	case PanelOHLCVChart:
		return descriptor(panelType, "OHLCV", "candlestick-chart", PanelMultiple, 520, 280, LinkDataset, LinkSymbols, LinkInterval, LinkTimeRange), nil
	case PanelFeatureBrowser:
		return descriptor(panelType, "Features", "list-tree", PanelSingleton, 320, 220, LinkDataset), nil
	case PanelSeriesInspector:
		return descriptor(panelType, "Series", "scan-line", PanelMultiple, 280, 180, LinkDataset, LinkSymbols, LinkInterval, LinkTimeRange), nil
	case PanelTargetPreview:
		return descriptor(panelType, "Target", "crosshair", PanelSingleton, 320, 180, LinkDataset, LinkTimeRange), nil
	case PanelDistributions:
		return descriptor(panelType, "Distributions", "bar-chart-3", PanelMultiple, 320, 220, LinkDataset, LinkSymbols, LinkTimeRange), nil
	case PanelRowTable:
		return descriptor(panelType, "Rows", "table", PanelMultiple, 420, 220, LinkDataset, LinkSymbols, LinkInterval, LinkTimeRange), nil
	case PanelExperimentList:
		return descriptor(panelType, "Experiments", "flask-conical", PanelSingleton, 320, 220, LinkExperiment), nil
	case PanelConfigurationTree:
		return descriptor(panelType, "Configuration", "list-tree", PanelSingleton, 320, 220, LinkExperiment), nil
	case PanelPropertyEditor:
		return descriptor(panelType, "Properties", "sliders-horizontal", PanelSingleton, 320, 220, LinkExperiment), nil
	case PanelExperimentInspector:
		return descriptor(panelType, "Inspector", "panel-right", PanelSingleton, 280, 180, LinkExperiment), nil
	case PanelValidationSummary:
		return descriptor(panelType, "Validation", "circle-check", PanelSingleton, 320, 180, LinkExperiment), nil
	case PanelResourceEstimate:
		return descriptor(panelType, "Resources", "cpu", PanelSingleton, 280, 160, LinkExperiment), nil
	case PanelJobQueue:
		return descriptor(panelType, "Jobs", "list-ordered", PanelSingleton, 360, 220), nil
	case PanelJobProgress:
		return descriptor(panelType, "Progress", "gauge", PanelMultiple, 320, 180), nil
	case PanelLiveMetrics:
		return descriptor(panelType, "Metrics", "line-chart", PanelMultiple, 320, 180, LinkRun), nil
	case PanelWorkerResources:
		return descriptor(panelType, "Workers", "cpu", PanelSingleton, 320, 160), nil
	case PanelProcessStatus:
		return descriptor(panelType, "Processes", "server", PanelSingleton, 320, 160), nil
	case PanelCheckpointStatus:
		return descriptor(panelType, "Checkpoints", "save", PanelMultiple, 320, 180), nil
	case PanelJobLogs:
		return descriptor(panelType, "Logs", "scroll-text", PanelMultiple, 320, 180), nil
	case PanelRunBrowser:
		return descriptor(panelType, "Runs", "folder-search", PanelSingleton, 360, 220, LinkRun), nil
	case PanelMetricComparison:
		return descriptor(panelType, "Metrics", "bar-chart-3", PanelMultiple, 320, 220, LinkRun), nil
	case PanelEquityChart:
		return descriptor(panelType, "Equity", "line-chart", PanelMultiple, 420, 220, LinkRun, LinkTimeRange), nil
	case PanelFoldTimeline:
		return descriptor(panelType, "Folds", "calendar-range", PanelMultiple, 320, 180, LinkRun, LinkTimeRange), nil
	case PanelPredictionOverlay:
		return descriptor(panelType, "Predictions", "layers", PanelMultiple, 420, 220, LinkRun, LinkSymbols, LinkTimeRange), nil
	case PanelResultDistributions:
		return descriptor(panelType, "Distributions", "bar-chart-3", PanelMultiple, 320, 220, LinkRun), nil
	case PanelConfigurationDiff:
		return descriptor(panelType, "Config Diff", "diff", PanelMultiple, 360, 220, LinkRun), nil
	case PanelModelRegistry:
		return descriptor(panelType, "Models", "archive", PanelSingleton, 360, 220, LinkModel), nil
	case PanelAliasHistory:
		return descriptor(panelType, "Aliases", "history", PanelSingleton, 320, 180, LinkModel), nil
	case PanelArtifactMetadata:
		return descriptor(panelType, "Artifact", "file-json", PanelMultiple, 320, 180, LinkModel), nil
	case PanelFeatureImportance:
		return descriptor(panelType, "Importance", "list-filter", PanelMultiple, 320, 220, LinkModel), nil
	case PanelTreeInspector:
		return descriptor(panelType, "Tree", "git-branch", PanelMultiple, 360, 220, LinkModel), nil
	case PanelModelSelector:
		return descriptor(panelType, "Model", "archive", PanelSingleton, 320, 180, LinkModel), nil
	case PanelInferenceDataset:
		return descriptor(panelType, "Dataset", "database", PanelSingleton, 320, 180, LinkDataset, LinkModel, LinkTimeRange), nil
	case PanelRankedScores:
		return descriptor(panelType, "Scores", "list-ordered", PanelMultiple, 360, 220, LinkModel, LinkTimeRange), nil
	case PanelScoreDistribution:
		return descriptor(panelType, "Distribution", "bar-chart-3", PanelMultiple, 320, 220, LinkModel, LinkTimeRange), nil
	case PanelPredictionHistory:
		return descriptor(panelType, "History", "history", PanelSingleton, 320, 180, LinkModel), nil
	case PanelExportStatus:
		return descriptor(panelType, "Export", "download", PanelSingleton, 280, 160, LinkModel), nil
	default:
		return PanelDescriptor{}, fmt.Errorf("%w: unknown panel type %q", ErrInvariant, panelType)
	}
}

func descriptor(
	panelType PanelType,
	title string,
	icon string,
	multiplicity PanelMultiplicity,
	width int,
	height int,
	links ...LinkScope,
) PanelDescriptor {
	return PanelDescriptor{
		Type:           panelType,
		Title:          title,
		Icon:           icon,
		Multiplicity:   multiplicity,
		MinimumSize:    Size{Width: width, Height: height},
		SupportedLinks: cloneLinkScopes(links),
	}
}

// PanelViewState is serialized panel-local view state.
type PanelViewState struct {
	Version         int
	CursorRow       int
	SortKey         string
	Filter          string
	TimeRange       TimeRange
	SelectedColumns []string
}

// Clone returns an independent copy.
func (view PanelViewState) Clone() PanelViewState {
	view.TimeRange = view.TimeRange.Normalize()
	view.SelectedColumns = cloneStrings(view.SelectedColumns)
	return view
}

// PanelSettings is typed panel configuration.
type PanelSettings struct {
	Pinned  bool
	Compact bool
	View    PanelViewState
}

// Clone returns an independent copy.
func (settings PanelSettings) Clone() PanelSettings {
	settings.View = settings.View.Clone()
	return settings
}

// PanelInstanceState is one stable panel instance in a dock layout.
type PanelInstanceState struct {
	ID        PanelID
	Type      PanelType
	Title     string
	LinkGroup LinkGroupID
	Settings  PanelSettings
}

// Clone returns an independent copy.
func (panel PanelInstanceState) Clone() PanelInstanceState {
	panel.Settings = panel.Settings.Clone()
	return panel
}

// NewPanelInstance creates a typed panel instance from a descriptor.
func NewPanelInstance(id PanelID, panelType PanelType, linkGroup LinkGroupID) (PanelInstanceState, error) {
	if id == "" {
		return PanelInstanceState{}, errorsForPanel("panel ID is empty")
	}
	descriptor, err := PanelDescriptorFor(panelType)
	if err != nil {
		return PanelInstanceState{}, err
	}
	return PanelInstanceState{
		ID:        id,
		Type:      panelType,
		Title:     descriptor.Title,
		LinkGroup: linkGroup,
		Settings: PanelSettings{
			View: PanelViewState{Version: 1},
		},
	}, nil
}

func errorsForPanel(message string) error {
	return fmt.Errorf("%w: %s", ErrInvariant, message)
}

// LinkGroup connects panels that share synchronized context.
type LinkGroup struct {
	ID      LinkGroupID
	Name    string
	Color   LinkGroupColor
	Context LinkContext
}

// LinkGroupColor identifies the visual swatch for a named link group. The
// group name is always rendered alongside the color so color is not the only
// status indicator.
type LinkGroupColor string

const (
	LinkGroupCyan     LinkGroupColor = "cyan"
	LinkGroupPurple   LinkGroupColor = "purple"
	LinkGroupPositive LinkGroupColor = "positive"
	LinkGroupWarning  LinkGroupColor = "warning"
)

// Valid reports whether the color is one of the closed variants.
func (color LinkGroupColor) Valid() bool {
	switch color {
	case LinkGroupCyan, LinkGroupPurple, LinkGroupPositive, LinkGroupWarning:
		return true
	default:
		return false
	}
}

// Clone returns an independent copy.
func (group LinkGroup) Clone() LinkGroup {
	group.Context = group.Context.Clone()
	return group
}

// SplitOrientation identifies a dock split direction.
type SplitOrientation string

const (
	SplitHorizontal SplitOrientation = "horizontal"
	SplitVertical   SplitOrientation = "vertical"
)

// DockNode is a closed internal interface for dock-tree variants.
type DockNode interface {
	isDockNode()
	Clone() DockNode
}

// SplitNode is a binary dock split.
type SplitNode struct {
	ID          DockNodeID
	Orientation SplitOrientation
	Ratio       float64
	First       DockNode
	Second      DockNode
}

func (SplitNode) isDockNode() {}

// Clone returns an independent copy.
func (node SplitNode) Clone() DockNode {
	node.First = cloneDockNode(node.First)
	node.Second = cloneDockNode(node.Second)
	return node
}

// TabStackNode is a stack of tabbed panels.
type TabStackNode struct {
	ID     DockNodeID
	Active PanelID
	Panels []PanelInstanceState
}

func (TabStackNode) isDockNode() {}

// Clone returns an independent copy.
func (node TabStackNode) Clone() DockNode {
	node.Panels = clonePanels(node.Panels)
	return node
}

// WorkspaceLayout is the persisted dock state for one workspace.
type WorkspaceLayout struct {
	SchemaVersion int
	Workspace     Workspace
	Root          DockNode
	HiddenPanels  []PanelInstanceState
	Maximized     PanelID
	LinkGroups    []LinkGroup
}

// Clone returns an independent copy.
func (layout WorkspaceLayout) Clone() WorkspaceLayout {
	layout.Root = cloneDockNode(layout.Root)
	layout.HiddenPanels = clonePanels(layout.HiddenPanels)
	layout.LinkGroups = cloneLinkGroups(layout.LinkGroups)
	return layout
}

// DefaultWorkspaceLayout returns a deterministic one-stack layout for a workspace.
func DefaultWorkspaceLayout(workspace Workspace, ids IDSource) (WorkspaceLayout, error) {
	if ids == nil {
		return WorkspaceLayout{}, errorsForPanel("ID source is nil")
	}
	if !workspace.Valid() {
		return WorkspaceLayout{}, fmt.Errorf("%w: unknown workspace %q", ErrInvariant, workspace)
	}
	linkGroup := LinkGroupID("link-default-" + string(workspace))
	compareGroup := LinkGroupID("link-compare-" + string(workspace))
	panelTypes, err := defaultPanelTypes(workspace)
	if err != nil {
		return WorkspaceLayout{}, err
	}
	panels := make([]PanelInstanceState, 0, len(panelTypes))
	for _, panelType := range panelTypes {
		panel, err := NewPanelInstance(ids.NewPanelID(), panelType, linkGroup)
		if err != nil {
			return WorkspaceLayout{}, err
		}
		panels = append(panels, panel)
	}
	active := PanelID("")
	if len(panels) > 0 {
		active = panels[0].ID
	}
	var root DockNode = TabStackNode{
		ID:     DockNodeID("dock-root-" + string(workspace)),
		Active: active,
		Panels: panels,
	}
	if workspace == WorkspaceResearch && len(panels) == 6 {
		root = SplitNode{
			ID: DockNodeID("dock-root-research"), Orientation: SplitVertical, Ratio: 0.64,
			First: SplitNode{
				ID: DockNodeID("dock-research-primary"), Orientation: SplitHorizontal, Ratio: 0.68,
				First:  TabStackNode{ID: DockNodeID("dock-research-chart"), Active: panels[0].ID, Panels: clonePanels(panels[0:1])},
				Second: TabStackNode{ID: DockNodeID("dock-research-inspectors"), Active: panels[1].ID, Panels: clonePanels(panels[1:5])},
			},
			Second: TabStackNode{ID: DockNodeID("dock-research-rows"), Active: panels[5].ID, Panels: clonePanels(panels[5:6])},
		}
	}
	if workspace == WorkspaceJobs && len(panels) == 7 {
		root = SplitNode{
			ID: DockNodeID("dock-root-jobs"), Orientation: SplitVertical, Ratio: 0.65,
			First: SplitNode{
				ID: DockNodeID("dock-jobs-primary"), Orientation: SplitHorizontal, Ratio: 0.58,
				First:  TabStackNode{ID: DockNodeID("dock-jobs-queue"), Active: panels[0].ID, Panels: clonePanels(panels[0:1])},
				Second: TabStackNode{ID: DockNodeID("dock-jobs-detail"), Active: panels[1].ID, Panels: clonePanels(panels[1:3])},
			},
			Second: SplitNode{
				ID: DockNodeID("dock-jobs-diagnostics"), Orientation: SplitHorizontal, Ratio: 0.52,
				First:  TabStackNode{ID: DockNodeID("dock-jobs-runtime"), Active: panels[3].ID, Panels: clonePanels(panels[3:6])},
				Second: TabStackNode{ID: DockNodeID("dock-jobs-logs"), Active: panels[6].ID, Panels: clonePanels(panels[6:7])},
			},
		}
	}
	if workspace == WorkspaceResults && len(panels) == 7 {
		root = SplitNode{
			ID: DockNodeID("dock-root-results"), Orientation: SplitVertical, Ratio: 0.64,
			First: SplitNode{
				ID: DockNodeID("dock-results-primary"), Orientation: SplitHorizontal, Ratio: 0.28,
				First:  TabStackNode{ID: DockNodeID("dock-results-browser"), Active: panels[0].ID, Panels: clonePanels(panels[0:1])},
				Second: TabStackNode{ID: DockNodeID("dock-results-charts"), Active: panels[1].ID, Panels: clonePanels(panels[1:5])},
			},
			Second: SplitNode{
				ID: DockNodeID("dock-results-inspectors"), Orientation: SplitHorizontal, Ratio: 0.54,
				First:  TabStackNode{ID: DockNodeID("dock-results-distributions"), Active: panels[5].ID, Panels: clonePanels(panels[5:6])},
				Second: TabStackNode{ID: DockNodeID("dock-results-diff"), Active: panels[6].ID, Panels: clonePanels(panels[6:7])},
			},
		}
	}
	return WorkspaceLayout{
		SchemaVersion: LayoutSchemaVersion,
		Workspace:     workspace,
		Root:          root,
		LinkGroups: []LinkGroup{
			{
				ID:      linkGroup,
				Name:    "Default",
				Color:   LinkGroupCyan,
				Context: LinkContext{Interval: IntervalDaily},
			},
			{
				ID:      compareGroup,
				Name:    "Compare",
				Color:   LinkGroupPurple,
				Context: LinkContext{Interval: IntervalDaily},
			},
		},
	}, nil
}

func defaultPanelTypes(workspace Workspace) ([]PanelType, error) {
	switch workspace {
	case WorkspaceData:
		return []PanelType{
			PanelCatalogTable,
			PanelCoverageTimeline,
			PanelImportQueue,
			PanelDatasetInspector,
			PanelImportLogs,
		}, nil
	case WorkspaceResearch:
		return []PanelType{
			PanelOHLCVChart,
			PanelFeatureBrowser,
			PanelSeriesInspector,
			PanelTargetPreview,
			PanelDistributions,
			PanelRowTable,
		}, nil
	case WorkspaceExperiments:
		return []PanelType{
			PanelExperimentList,
			PanelConfigurationTree,
			PanelPropertyEditor,
			PanelExperimentInspector,
			PanelValidationSummary,
			PanelResourceEstimate,
		}, nil
	case WorkspaceJobs:
		return []PanelType{
			PanelJobQueue,
			PanelJobProgress,
			PanelLiveMetrics,
			PanelWorkerResources,
			PanelProcessStatus,
			PanelCheckpointStatus,
			PanelJobLogs,
		}, nil
	case WorkspaceResults:
		return []PanelType{
			PanelRunBrowser,
			PanelMetricComparison,
			PanelEquityChart,
			PanelFoldTimeline,
			PanelPredictionOverlay,
			PanelResultDistributions,
			PanelConfigurationDiff,
		}, nil
	case WorkspaceModels:
		return []PanelType{
			PanelModelRegistry,
			PanelAliasHistory,
			PanelArtifactMetadata,
			PanelFeatureImportance,
			PanelTreeInspector,
		}, nil
	case WorkspaceInference:
		return []PanelType{
			PanelModelSelector,
			PanelInferenceDataset,
			PanelRankedScores,
			PanelScoreDistribution,
			PanelPredictionHistory,
			PanelExportStatus,
		}, nil
	default:
		return nil, fmt.Errorf("%w: unknown workspace %q", ErrInvariant, workspace)
	}
}

func cloneDockNode(node DockNode) DockNode {
	if node == nil {
		return nil
	}
	return node.Clone()
}

func clonePanels(input []PanelInstanceState) []PanelInstanceState {
	if len(input) == 0 {
		return nil
	}
	output := make([]PanelInstanceState, len(input))
	for index, panel := range input {
		output[index] = panel.Clone()
	}
	return output
}

func cloneLinkGroups(input []LinkGroup) []LinkGroup {
	if len(input) == 0 {
		return nil
	}
	output := make([]LinkGroup, len(input))
	for index, group := range input {
		output[index] = group.Clone()
	}
	return output
}

func cloneLayouts(input []WorkspaceLayout) []WorkspaceLayout {
	if len(input) == 0 {
		return nil
	}
	output := make([]WorkspaceLayout, len(input))
	for index, layout := range input {
		output[index] = layout.Clone()
	}
	return output
}

func cloneStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	output := make([]string, len(input))
	copy(output, input)
	return output
}

func cloneLinkScopes(input []LinkScope) []LinkScope {
	if len(input) == 0 {
		return nil
	}
	output := make([]LinkScope, len(input))
	copy(output, input)
	return output
}
