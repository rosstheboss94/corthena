package appstate

import (
	"fmt"
	"time"

	virtualtable "github.com/rosstheboss94/corthena/internal/frontend/table"
)

// Reduce applies one typed action and returns the next state plus background
// effects for the render loop to enqueue without blocking.
func Reduce(state AppState, action UIAction) (AppState, []UIEffect, error) {
	if action == nil {
		return AppState{}, nil, fmt.Errorf("%w: nil UI action", ErrInvariant)
	}
	next := state.Clone()
	switch action := action.(type) {
	case SelectWorkspaceAction:
		if !action.Workspace.Valid() {
			return state, nil, fmt.Errorf("%w: unknown workspace %q", ErrInvariant, action.Workspace)
		}
		previous := next.ActiveWorkspace
		next.ActiveWorkspace = action.Workspace
		var effects []UIEffect
		if previous == WorkspaceResearch && action.Workspace != WorkspaceResearch {
			for _, group := range next.Research.Groups {
				if group.State == ResearchLoading {
					effects = append(effects, CancelResearchEffect{
						ID:      EffectID(fmt.Sprintf("research-cancel-%s-%020d", group.GroupID, group.Generation)),
						GroupID: group.GroupID, Generation: group.Generation,
					})
				}
			}
		}
		if action.Workspace == WorkspaceResearch {
			if groupID, context, found := defaultResearchGroup(next.Layouts); found {
				effect, err := refreshResearchGroup(&next, groupID, context, "")
				if err != nil {
					return state, nil, err
				}
				if effect != nil {
					effects = append(effects, effect)
				}
			}
		}
		return next, effects, nil
	case SetLinkContextAction:
		next.LinkContext = action.Context.Clone()
		next.updateDefaultLinkGroup(action.Context)
		committed, effects, err := commitAllLayouts(state, next, "", time.Time{})
		if err != nil || committed.ActiveWorkspace != WorkspaceResearch {
			return committed, effects, err
		}
		if groupID, context, found := defaultResearchGroup(committed.Layouts); found {
			effect, refreshErr := refreshResearchGroup(&committed, groupID, context, "")
			if refreshErr != nil {
				return state, nil, refreshErr
			}
			if effect != nil {
				effects = append(effects, effect)
			}
		}
		return committed, effects, nil
	case SetCommandPaletteAction:
		next.Overlays.CommandPaletteOpen = action.Open
		if action.Open {
			next.Overlays.SettingsOpen = false
		}
		return next, nil, nil
	case SetSettingsOpenAction:
		next.Overlays.SettingsOpen = action.Open
		if action.Open {
			next.Overlays.CommandPaletteOpen = false
		}
		return next, nil, nil
	case SetUIScaleAction:
		if !action.Scale.Valid() {
			return state, nil, fmt.Errorf("%w: unsupported UI scale %d", ErrInvariant, action.Scale)
		}
		if next.Preferences.UIScale == action.Scale {
			return next, nil, nil
		}
		next.Preferences.UIScale = action.Scale
		return commitPreferenceMutation(state, next)
	case DismissToastAction:
		next.Overlays.Toasts = dismissToast(next.Overlays.Toasts, action.ToastID)
		return next, nil, nil
	case OpenPanelAction:
		layoutIndex, err := next.layoutIndex(action.Workspace)
		if err != nil {
			return state, nil, err
		}
		layout := next.Layouts[layoutIndex].Clone()
		panel := action.Panel.Clone()
		if err := validatePanel(panel); err != nil {
			return state, nil, err
		}
		descriptor, err := PanelDescriptorFor(panel.Type)
		if err != nil {
			return state, nil, err
		}
		panel, layout.HiddenPanels = takeHiddenPanel(layout.HiddenPanels, panel, descriptor.Multiplicity)
		if descriptor.Multiplicity == PanelSingleton {
			var activated bool
			layout.Root, activated = activatePanelByType(layout.Root, panel.Type)
			if activated {
				next.Layouts[layoutIndex] = layout
				return commitLayoutMutation(state, next, action.PersistEffectID, action.RequestedAt)
			}
		}
		layout.Root, err = addPanelToDockRoot(layout.Root, panel)
		if err != nil {
			return state, nil, err
		}
		next.Layouts[layoutIndex] = layout
		return commitLayoutMutation(state, next, action.PersistEffectID, action.RequestedAt)
	case ClosePanelAction:
		layoutIndex, err := next.layoutIndex(action.Workspace)
		if err != nil {
			return state, nil, err
		}
		layout := next.Layouts[layoutIndex].Clone()
		var closed bool
		var closedPanel PanelInstanceState
		layout.Root, closedPanel, closed, err = removePanelFromDockRoot(layout.Root, action.PanelID)
		if err != nil {
			return state, nil, err
		}
		if !closed {
			return next, nil, nil
		}
		if layout.Root == nil {
			layout.Root = TabStackNode{ID: DockNodeID("dock-root-" + string(action.Workspace))}
		}
		layout.HiddenPanels = append(layout.HiddenPanels, closedPanel.Clone())
		if layout.Maximized == action.PanelID {
			layout.Maximized = ""
		}
		next.Layouts[layoutIndex] = layout
		return commitLayoutMutation(state, next, action.PersistEffectID, action.RequestedAt)
	case ApplyWorkspaceLayoutAction:
		layout := action.Layout.Clone()
		if err := ValidateWorkspaceLayout(layout); err != nil {
			return state, nil, err
		}
		layoutIndex, err := next.layoutIndex(layout.Workspace)
		if err != nil {
			return state, nil, err
		}
		next.Layouts[layoutIndex] = layout
		return commitLayoutMutation(state, next, "", time.Time{})
	case ResetWorkspaceLayoutAction:
		layoutIndex, err := next.layoutIndex(action.Workspace)
		if err != nil {
			return state, nil, err
		}
		defaultLayout, found := workspaceLayout(next.DefaultLayouts, action.Workspace)
		if !found {
			return state, nil, fmt.Errorf("%w: missing default layout for workspace %q", ErrInvariant, action.Workspace)
		}
		for index := range defaultLayout.LinkGroups {
			if defaultLayout.LinkGroups[index].Name == "Default" {
				defaultLayout.LinkGroups[index].Context = next.LinkContext.Clone()
			}
		}
		next.Layouts[layoutIndex] = defaultLayout
		return commitLayoutMutation(state, next, "", time.Time{})
	case AssignPanelLinkGroupAction:
		layoutIndex, err := next.layoutIndex(action.Workspace)
		if err != nil {
			return state, nil, err
		}
		layout := next.Layouts[layoutIndex].Clone()
		if !hasLinkGroup(layout.LinkGroups, action.GroupID) {
			return state, nil, fmt.Errorf("%w: unknown link group %q", ErrInvariant, action.GroupID)
		}
		var assigned bool
		layout.Root, assigned, err = assignPanelLinkGroup(layout.Root, action.PanelID, action.GroupID)
		if err != nil {
			return state, nil, err
		}
		if !assigned {
			assigned, err = assignHiddenPanelLinkGroup(layout.HiddenPanels, action.PanelID, action.GroupID)
			if err != nil {
				return state, nil, err
			}
		}
		if !assigned {
			return state, nil, fmt.Errorf("%w: panel %q is missing", ErrInvariant, action.PanelID)
		}
		next.Layouts[layoutIndex] = layout
		return commitLayoutMutation(state, next, "", time.Time{})
	case UpsertLinkGroupAction:
		layoutIndex, err := next.layoutIndex(action.Workspace)
		if err != nil {
			return state, nil, err
		}
		group := action.Group.Clone()
		if group.ID == "" || group.Name == "" || !group.Color.Valid() {
			return state, nil, fmt.Errorf("%w: invalid link group %q", ErrInvariant, group.ID)
		}
		if err := validateLinkContext(group.Context); err != nil {
			return state, nil, err
		}
		layout := next.Layouts[layoutIndex].Clone()
		updated := false
		for index := range layout.LinkGroups {
			if layout.LinkGroups[index].ID == group.ID {
				layout.LinkGroups[index] = group
				updated = true
				break
			}
		}
		if !updated {
			layout.LinkGroups = append(layout.LinkGroups, group)
		}
		next.Layouts[layoutIndex] = layout
		return commitLayoutMutation(state, next, "", time.Time{})
	case RemoveLinkGroupAction:
		if action.GroupID == "" || action.GroupID == action.FallbackGroupID {
			return state, nil, fmt.Errorf("%w: invalid link-group removal", ErrInvariant)
		}
		layoutIndex, err := next.layoutIndex(action.Workspace)
		if err != nil {
			return state, nil, err
		}
		layout := next.Layouts[layoutIndex].Clone()
		if !hasLinkGroup(layout.LinkGroups, action.GroupID) {
			return state, nil, fmt.Errorf("%w: unknown link group %q", ErrInvariant, action.GroupID)
		}
		if !hasLinkGroup(layout.LinkGroups, action.FallbackGroupID) {
			return state, nil, fmt.Errorf("%w: unknown fallback link group %q", ErrInvariant, action.FallbackGroupID)
		}
		groups := make([]LinkGroup, 0, len(layout.LinkGroups)-1)
		for _, group := range layout.LinkGroups {
			if group.ID != action.GroupID {
				groups = append(groups, group.Clone())
			}
		}
		layout.LinkGroups = groups
		layout.Root, err = replacePanelLinkGroup(layout.Root, action.GroupID, action.FallbackGroupID)
		if err != nil {
			return state, nil, err
		}
		for index := range layout.HiddenPanels {
			if layout.HiddenPanels[index].LinkGroup == action.GroupID {
				layout.HiddenPanels[index].LinkGroup = action.FallbackGroupID
			}
		}
		next.Layouts[layoutIndex] = layout
		return commitLayoutMutation(state, next, "", time.Time{})
	case UpdateLinkGroupContextAction:
		layoutIndex, err := next.layoutIndex(action.Workspace)
		if err != nil {
			return state, nil, err
		}
		layout := next.Layouts[layoutIndex].Clone()
		panel, found := findPanel(layout, action.SourcePanelID)
		if !found {
			return state, nil, fmt.Errorf("%w: source panel %q is missing", ErrInvariant, action.SourcePanelID)
		}
		if panel.LinkGroup != action.GroupID {
			return state, nil, fmt.Errorf("%w: panel %q is not assigned to link group %q", ErrInvariant, panel.ID, action.GroupID)
		}
		descriptor, err := PanelDescriptorFor(panel.Type)
		if err != nil {
			return state, nil, err
		}
		updated := false
		for index := range layout.LinkGroups {
			if layout.LinkGroups[index].ID != action.GroupID {
				continue
			}
			merged, err := mergeLinkContext(
				layout.LinkGroups[index].Context,
				action.Context,
				descriptor.SupportedLinks,
			)
			if err != nil {
				return state, nil, err
			}
			layout.LinkGroups[index].Context = merged
			if action.Workspace == next.ActiveWorkspace {
				next.LinkContext = layout.LinkGroups[index].Context.Clone()
			}
			updated = true
			break
		}
		if !updated {
			return state, nil, fmt.Errorf("%w: link group %q is missing", ErrInvariant, action.GroupID)
		}
		next.Layouts[layoutIndex] = layout
		committed, effects, err := commitLayoutMutation(state, next, "", time.Time{})
		if err != nil || action.Workspace != WorkspaceResearch || committed.ActiveWorkspace != WorkspaceResearch {
			return committed, effects, err
		}
		context, found := researchGroupContext(committed.Layouts, action.GroupID)
		if found {
			effect, refreshErr := refreshResearchGroup(&committed, action.GroupID, context, "")
			if refreshErr != nil {
				return state, nil, refreshErr
			}
			if effect != nil {
				effects = append(effects, effect)
			}
		}
		return committed, effects, nil
	case RequestResearchAction:
		effect, err := beginResearchQuery(&next, action.Query)
		if err != nil {
			return state, nil, err
		}
		return next, []UIEffect{effect}, nil
	case SetResearchFeatureAction:
		if action.GroupID == "" || action.Feature == "" {
			return state, nil, fmt.Errorf("%w: Research feature and group are required", ErrInvariant)
		}
		groupIndex := next.researchGroupIndex(action.GroupID)
		next.Research.Groups[groupIndex].SelectedFeature = action.Feature
		context, found := researchGroupContext(next.Layouts, action.GroupID)
		if !found {
			return state, nil, fmt.Errorf("%w: Research link group %q is missing", ErrInvariant, action.GroupID)
		}
		effect, err := refreshResearchGroup(&next, action.GroupID, context, "")
		if err != nil {
			return state, nil, err
		}
		if effect == nil {
			return next, nil, nil
		}
		return next, []UIEffect{effect}, nil
	case SetResearchScenarioAction:
		if !action.Scenario.Valid() {
			return state, nil, fmt.Errorf("%w: invalid Research scenario %q", ErrInvariant, action.Scenario)
		}
		groupIndex := next.researchGroupIndex(action.GroupID)
		next.Research.Groups[groupIndex].Scenario = action.Scenario
		if action.Scenario == ResearchScenarioReconnecting {
			next.Connection.State = ConnectionDegraded
			next.Connection.Detail = "Research connection is reconnecting"
		}
		context, found := researchGroupContext(next.Layouts, action.GroupID)
		if !found {
			return state, nil, fmt.Errorf("%w: Research link group %q is missing", ErrInvariant, action.GroupID)
		}
		effect, err := refreshResearchGroup(&next, action.GroupID, context, "")
		if err != nil {
			return state, nil, err
		}
		if effect == nil {
			return next, nil, nil
		}
		return next, []UIEffect{effect}, nil
	case SetResearchVisibilityAction:
		groupIndex := next.researchGroupIndex(action.GroupID)
		next.Research.Groups[groupIndex].ShowOHLCV = action.ShowOHLCV
		next.Research.Groups[groupIndex].ShowFeature = action.ShowFeature
		next.Research.Groups[groupIndex].ShowTarget = action.ShowTarget
		return next, nil, nil
	case SelectResearchRowAction:
		index := next.researchGroupIndex(action.GroupID)
		group := &next.Research.Groups[index]
		rowID := virtualtable.RowID(action.RowID)
		if rowID == "" {
			return state, nil, fmt.Errorf("%w: Research row ID is empty", ErrInvariant)
		}
		if action.Toggle {
			for selectedIndex, selected := range group.SelectedRows {
				if selected == rowID {
					group.SelectedRows = append(group.SelectedRows[:selectedIndex], group.SelectedRows[selectedIndex+1:]...)
					return next, nil, nil
				}
			}
			group.SelectedRows = append(group.SelectedRows, rowID)
		} else {
			group.SelectedRows = []virtualtable.RowID{rowID}
		}
		return next, nil, nil
	case SetResearchRangeAction:
		normalized, err := normalizeResearchRange(action.TimeRange)
		if err != nil {
			return state, nil, err
		}
		context, found := researchGroupContext(next.Layouts, action.GroupID)
		if !found {
			return state, nil, fmt.Errorf("%w: Research link group %q is missing", ErrInvariant, action.GroupID)
		}
		context.TimeRange = normalized
		updated, effects, err := Reduce(state, UpdateLinkGroupContextAction{
			Workspace: WorkspaceResearch, GroupID: action.GroupID,
			SourcePanelID: action.SourcePanelID, Context: context,
		})
		return updated, effects, err
	case ResearchQueryFailedAction:
		index := next.researchGroupIndex(action.GroupID)
		group := &next.Research.Groups[index]
		if action.Generation != group.Generation {
			return next, nil, nil
		}
		group.State = researchFailureState(action.Error)
		group.Stale = group.Snapshot.Query.Generation != 0
		group.Error = action.Error
		return next, nil, nil
	case ResearchQueryCancelledAction:
		index := next.researchGroupIndex(action.GroupID)
		group := &next.Research.Groups[index]
		if action.Generation != group.Generation {
			return next, nil, nil
		}
		group.State = ResearchCancelled
		group.Stale = group.Snapshot.Query.Generation != 0
		group.Error = ErrorSnapshot{Code: ErrorResearchCancelled, Message: "Research request cancelled", Retryable: true}
		return next, nil, nil
	case ClientMessageAction:
		return reduceClientMessage(next, action.Message)
	case LayoutsLoadedAction:
		if next.LayoutRevision != action.BaseRevision {
			return next, nil, nil
		}
		if err := ValidateWorkspaceLayouts(action.Layouts); err != nil {
			return state, nil, err
		}
		merged, err := mergeLoadedLayouts(next.Layouts, action.Layouts)
		if err != nil {
			return state, nil, err
		}
		next.Layouts = merged
		if action.Recovered && action.Diagnostic != "" {
			next.Overlays.Toasts = append(next.Overlays.Toasts, Toast{
				ID:        action.EffectID,
				Kind:      ToastWarning,
				Message:   action.Diagnostic,
				CreatedAt: action.LoadedAt.UTC(),
			})
		}
		if action.Promote || action.BaseRevision != 0 {
			next.LayoutRevision = action.BaseRevision
			return commitLayoutMutation(state, next, "", action.LoadedAt)
		}
		if action.Revision == ^uint64(0) {
			return state, nil, fmt.Errorf("%w: loaded layout revision is exhausted", ErrInvariant)
		}
		next.LayoutRevision = action.Revision
		next.Persistence.LastSavedRevision = action.Revision
		next.Persistence.PendingRevision = 0
		next.Persistence.LastSavedAt = action.LoadedAt.UTC()
		return next, nil, nil
	case LayoutPersistedAction:
		if action.Revision < next.Persistence.LastSavedRevision {
			return next, nil, nil
		}
		next.Persistence.LastSavedAt = action.SavedAt.UTC()
		next.Persistence.LastSavedRevision = action.Revision
		if action.Revision >= next.Persistence.PendingRevision {
			next.Persistence.PendingRevision = 0
		}
		if action.Revision >= next.Persistence.LastErrorRevision {
			next.Persistence.LastError = ErrorSnapshot{}
			next.Persistence.LastErrorRevision = 0
		}
		return next, nil, nil
	case LayoutPersistenceFailedAction:
		if action.Revision < next.Persistence.LastErrorRevision ||
			action.Revision < next.Persistence.LastSavedRevision {
			return next, nil, nil
		}
		next.Persistence.LastError = action.Error
		next.Persistence.LastErrorRevision = action.Revision
		next.Overlays.Toasts = append(next.Overlays.Toasts, Toast{
			ID:        action.EffectID,
			Kind:      ToastError,
			Message:   action.Error.Message,
			CreatedAt: action.FailedAt.UTC(),
		})
		return next, nil, nil
	case PreferencesLoadedAction:
		if next.PreferenceRevision != action.BaseRevision {
			return next, nil, nil
		}
		if err := action.Preferences.Validate(); err != nil {
			return state, nil, err
		}
		next.Preferences = action.Preferences
		next.PreferenceRevision = action.Revision
		next.PreferencePersistence.LastSavedRevision = action.Revision
		next.PreferencePersistence.PendingRevision = 0
		next.PreferencePersistence.LastSavedAt = action.LoadedAt.UTC()
		if action.Recovered && action.Diagnostic != "" {
			next.Overlays.Toasts = append(next.Overlays.Toasts, Toast{
				ID: action.EffectID, Kind: ToastWarning,
				Message: action.Diagnostic, CreatedAt: action.LoadedAt.UTC(),
			})
		}
		return next, nil, nil
	case PreferencesPersistedAction:
		if action.Revision < next.PreferencePersistence.LastSavedRevision {
			return next, nil, nil
		}
		next.PreferencePersistence.LastSavedAt = action.SavedAt.UTC()
		next.PreferencePersistence.LastSavedRevision = action.Revision
		if action.Revision >= next.PreferencePersistence.PendingRevision {
			next.PreferencePersistence.PendingRevision = 0
		}
		if action.Revision >= next.PreferencePersistence.LastErrorRevision {
			next.PreferencePersistence.LastError = ErrorSnapshot{}
			next.PreferencePersistence.LastErrorRevision = 0
		}
		return next, nil, nil
	case PreferencesPersistenceFailedAction:
		if action.Revision < next.PreferencePersistence.LastErrorRevision ||
			action.Revision < next.PreferencePersistence.LastSavedRevision {
			return next, nil, nil
		}
		next.PreferencePersistence.LastError = action.Error
		next.PreferencePersistence.LastErrorRevision = action.Revision
		next.Overlays.Toasts = append(next.Overlays.Toasts, Toast{
			ID: action.EffectID, Kind: ToastError,
			Message: action.Error.Message, CreatedAt: action.FailedAt.UTC(),
		})
		return next, nil, nil
	case EffectFailedAction:
		next.Persistence.LastError = action.Error
		next.Overlays.Toasts = append(next.Overlays.Toasts, Toast{
			ID:        action.EffectID,
			Kind:      ToastError,
			Message:   action.Error.Message,
			CreatedAt: action.FailedAt.UTC(),
		})
		return next, nil, nil
	default:
		return state, nil, fmt.Errorf("%w: unhandled UI action %T", ErrInvariant, action)
	}
}

func dismissToast(input []Toast, toastID EffectID) []Toast {
	if toastID == "" || len(input) == 0 {
		return cloneToasts(input)
	}
	output := make([]Toast, 0, len(input))
	for _, toast := range input {
		if toast.ID == toastID {
			continue
		}
		output = append(output, toast)
	}
	return output
}

func reduceClientMessage(state AppState, message ClientMessage) (AppState, []UIEffect, error) {
	if message == nil {
		return state, nil, fmt.Errorf("%w: nil client message", ErrInvariant)
	}
	switch message := message.(type) {
	case SnapshotMessage:
		snapshot := message.Clone()
		state.Connection = snapshot.Connection
		state.Components = snapshot.Components
		state.Cache = snapshot.Cache
		state.Datasets = snapshot.Datasets
		state.Jobs = snapshot.Jobs
		state.Results = snapshot.Results
		state.Models = snapshot.Models
		state.Inferences = snapshot.Inferences
		state.applyEventEnvelope(snapshot.Event)
		state.defaultContextFromSnapshot()
		return refreshResearchAfterCatalog(state)
	case DatasetCatalogMessage:
		state.Datasets = cloneDatasets(message.Datasets)
		state.applyEventEnvelope(message.Event)
		state.defaultContextFromSnapshot()
		return refreshResearchAfterCatalog(state)
	case JobUpdateMessage:
		state.Jobs = upsertJob(state.Jobs, message.Job)
		state.applyEventEnvelope(message.Event)
		return state, nil, nil
	case RunResultsMessage:
		state.Results = cloneResults(message.Results)
		state.applyEventEnvelope(message.Event)
		return state, nil, nil
	case ModelRegistryMessage:
		state.Models = cloneModels(message.Models)
		state.applyEventEnvelope(message.Event)
		return state, nil, nil
	case InferenceUpdateMessage:
		state.Inferences = upsertInference(state.Inferences, message.Inference)
		state.applyEventEnvelope(message.Event)
		return state, nil, nil
	case ResearchResponseMessage:
		if !applyResearchResponse(&state, message) {
			return state, nil, nil
		}
		state.applyEventEnvelope(message.Event)
		if message.Snapshot.Degraded {
			state.Connection.State = ConnectionDegraded
			state.Connection.Detail = "Research data is degraded; cached context remains available"
		} else if message.Snapshot.Query.Scenario == ResearchScenarioRecovered {
			state.Connection.State = ConnectionConnected
			state.Connection.Detail = "Research connection recovered and reconciled"
		}
		return state, nil, nil
	case ComponentStatusMessage:
		state.Components = upsertComponent(state.Components, message.Component)
		state.applyEventEnvelope(message.Event)
		return state, nil, nil
	case ClientFailureMessage:
		state.Connection.State = ConnectionDegraded
		if !message.Error.Retryable {
			state.Connection.State = ConnectionDisconnected
		}
		state.Connection.Detail = message.Error.Message
		state.Connection.UpdatedAt = message.Event.Timestamp.UTC()
		state.Overlays.Toasts = append(state.Overlays.Toasts, Toast{
			ID:        EffectID(message.Event.ID),
			Kind:      ToastWarning,
			Message:   message.Error.Message,
			CreatedAt: message.Event.Timestamp.UTC(),
		})
		state.applyEventEnvelope(message.Event)
		return state, nil, nil
	default:
		return state, nil, fmt.Errorf("%w: unhandled client message %T", ErrInvariant, message)
	}
}

func refreshResearchAfterCatalog(state AppState) (AppState, []UIEffect, error) {
	if state.ActiveWorkspace != WorkspaceResearch {
		return state, nil, nil
	}
	groupID, context, found := defaultResearchGroup(state.Layouts)
	if !found || context.DatasetID == "" {
		return state, nil, nil
	}
	effect, err := refreshResearchGroup(&state, groupID, context, "")
	if err != nil {
		return state, nil, err
	}
	if effect == nil {
		return state, nil, nil
	}
	return state, []UIEffect{effect}, nil
}

func commitAllLayouts(
	original AppState,
	candidate AppState,
	effectID EffectID,
	requestedAt time.Time,
) (AppState, []UIEffect, error) {
	return commitLayoutMutation(original, candidate, effectID, requestedAt)
}

func commitLayoutMutation(
	original AppState,
	candidate AppState,
	effectID EffectID,
	requestedAt time.Time,
) (AppState, []UIEffect, error) {
	if err := ValidateWorkspaceLayouts(candidate.Layouts); err != nil {
		return original, nil, err
	}
	if candidate.LayoutRevision == ^uint64(0) {
		return original, nil, fmt.Errorf("%w: layout revision overflow", ErrInvariant)
	}
	candidate.LayoutRevision++
	candidate.Persistence.PendingRevision = candidate.LayoutRevision
	if effectID == "" {
		effectID = EffectID(fmt.Sprintf("layout-save-%020d", candidate.LayoutRevision))
	}
	return candidate, []UIEffect{
		PersistLayoutsEffect{
			ID:        effectID,
			Revision:  candidate.LayoutRevision,
			Layouts:   cloneLayouts(candidate.Layouts),
			Requested: requestedAt.UTC(),
		},
	}, nil
}

func commitPreferenceMutation(original AppState, candidate AppState) (AppState, []UIEffect, error) {
	if err := candidate.Preferences.Validate(); err != nil {
		return original, nil, err
	}
	if candidate.PreferenceRevision == ^uint64(0) {
		return original, nil, fmt.Errorf("%w: preference revision overflow", ErrInvariant)
	}
	candidate.PreferenceRevision++
	candidate.PreferencePersistence.PendingRevision = candidate.PreferenceRevision
	return candidate, []UIEffect{PersistPreferencesEffect{
		ID:          EffectID(fmt.Sprintf("preference-save-%020d", candidate.PreferenceRevision)),
		Revision:    candidate.PreferenceRevision,
		Preferences: candidate.Preferences,
	}}, nil
}

func validatePanel(panel PanelInstanceState) error {
	if panel.ID == "" {
		return fmt.Errorf("%w: panel ID is empty", ErrInvariant)
	}
	if _, err := PanelDescriptorFor(panel.Type); err != nil {
		return err
	}
	return nil
}

func (state *AppState) layoutIndex(workspace Workspace) (int, error) {
	if !workspace.Valid() {
		return 0, fmt.Errorf("%w: unknown workspace %q", ErrInvariant, workspace)
	}
	for index, layout := range state.Layouts {
		if layout.Workspace == workspace {
			return index, nil
		}
	}
	return 0, fmt.Errorf("%w: missing layout for workspace %q", ErrInvariant, workspace)
}

func addPanelToDockRoot(root DockNode, panel PanelInstanceState) (DockNode, error) {
	switch node := root.(type) {
	case TabStackNode:
		node.Panels = append(node.Panels, panel.Clone())
		node.Active = panel.ID
		return node, nil
	case SplitNode:
		first, err := addPanelToDockRoot(node.First, panel)
		if err != nil {
			return nil, err
		}
		node.First = first
		return node, nil
	default:
		return nil, fmt.Errorf("%w: unhandled dock node %T", ErrInvariant, root)
	}
}

func activatePanelByType(root DockNode, panelType PanelType) (DockNode, bool) {
	switch node := root.(type) {
	case TabStackNode:
		for _, panel := range node.Panels {
			if panel.Type == panelType {
				node.Active = panel.ID
				return node, true
			}
		}
		return node, false
	case SplitNode:
		var activated bool
		node.First, activated = activatePanelByType(node.First, panelType)
		if activated {
			return node, true
		}
		node.Second, activated = activatePanelByType(node.Second, panelType)
		return node, activated
	default:
		return root, false
	}
}

func removePanelFromDockRoot(root DockNode, panelID PanelID) (DockNode, PanelInstanceState, bool, error) {
	switch node := root.(type) {
	case TabStackNode:
		panels := make([]PanelInstanceState, 0, len(node.Panels))
		removed := false
		var removedPanel PanelInstanceState
		for _, panel := range node.Panels {
			if panel.ID == panelID {
				removed = true
				removedPanel = panel.Clone()
				continue
			}
			panels = append(panels, panel)
		}
		if !removed {
			return node, PanelInstanceState{}, false, nil
		}
		node.Panels = panels
		if len(node.Panels) == 0 {
			return nil, removedPanel, true, nil
		} else if node.Active == panelID {
			node.Active = node.Panels[0].ID
		}
		return node, removedPanel, true, nil
	case SplitNode:
		first, removedPanel, removed, err := removePanelFromDockRoot(node.First, panelID)
		if err != nil {
			return nil, PanelInstanceState{}, false, err
		}
		if removed {
			if first == nil {
				return cloneDockNode(node.Second), removedPanel, true, nil
			}
			node.First = first
			return node, removedPanel, true, nil
		}
		second, removedPanel, removed, err := removePanelFromDockRoot(node.Second, panelID)
		if err != nil {
			return nil, PanelInstanceState{}, false, err
		}
		if removed && second == nil {
			return cloneDockNode(node.First), removedPanel, true, nil
		}
		node.Second = second
		return node, removedPanel, removed, nil
	default:
		return nil, PanelInstanceState{}, false, fmt.Errorf("%w: unhandled dock node %T", ErrInvariant, root)
	}
}

func takeHiddenPanel(
	hidden []PanelInstanceState,
	requested PanelInstanceState,
	multiplicity PanelMultiplicity,
) (PanelInstanceState, []PanelInstanceState) {
	output := make([]PanelInstanceState, 0, len(hidden))
	selected := requested.Clone()
	found := false
	for _, panel := range hidden {
		matches := panel.ID == requested.ID
		if multiplicity == PanelSingleton {
			matches = panel.Type == requested.Type
		}
		if matches && !found {
			selected = panel.Clone()
			found = true
			continue
		}
		output = append(output, panel.Clone())
	}
	return selected, output
}

func hasLinkGroup(groups []LinkGroup, groupID LinkGroupID) bool {
	for _, group := range groups {
		if group.ID == groupID {
			return true
		}
	}
	return false
}

func assignPanelLinkGroup(root DockNode, panelID PanelID, groupID LinkGroupID) (DockNode, bool, error) {
	switch node := root.(type) {
	case TabStackNode:
		for index := range node.Panels {
			if node.Panels[index].ID != panelID {
				continue
			}
			descriptor, err := PanelDescriptorFor(node.Panels[index].Type)
			if err != nil {
				return nil, false, err
			}
			if len(descriptor.SupportedLinks) == 0 {
				return nil, false, fmt.Errorf("%w: panel %q does not support link groups", ErrInvariant, panelID)
			}
			node.Panels[index].LinkGroup = groupID
			return node, true, nil
		}
		return node, false, nil
	case SplitNode:
		first, assigned, err := assignPanelLinkGroup(node.First, panelID, groupID)
		if err != nil {
			return nil, false, err
		}
		if assigned {
			node.First = first
			return node, true, nil
		}
		second, assigned, err := assignPanelLinkGroup(node.Second, panelID, groupID)
		if err != nil {
			return nil, false, err
		}
		node.Second = second
		return node, assigned, nil
	default:
		return nil, false, fmt.Errorf("%w: unhandled dock node %T", ErrInvariant, root)
	}
}

func replacePanelLinkGroup(root DockNode, from LinkGroupID, to LinkGroupID) (DockNode, error) {
	switch node := root.(type) {
	case TabStackNode:
		for index := range node.Panels {
			if node.Panels[index].LinkGroup == from {
				node.Panels[index].LinkGroup = to
			}
		}
		return node, nil
	case SplitNode:
		first, err := replacePanelLinkGroup(node.First, from, to)
		if err != nil {
			return nil, err
		}
		second, err := replacePanelLinkGroup(node.Second, from, to)
		if err != nil {
			return nil, err
		}
		node.First = first
		node.Second = second
		return node, nil
	default:
		return nil, fmt.Errorf("%w: unhandled dock node %T", ErrInvariant, root)
	}
}

func assignHiddenPanelLinkGroup(
	panels []PanelInstanceState,
	panelID PanelID,
	groupID LinkGroupID,
) (bool, error) {
	for index := range panels {
		if panels[index].ID != panelID {
			continue
		}
		descriptor, err := PanelDescriptorFor(panels[index].Type)
		if err != nil {
			return false, err
		}
		if len(descriptor.SupportedLinks) == 0 {
			return false, fmt.Errorf("%w: panel %q does not support link groups", ErrInvariant, panelID)
		}
		panels[index].LinkGroup = groupID
		return true, nil
	}
	return false, nil
}

func findPanel(layout WorkspaceLayout, panelID PanelID) (PanelInstanceState, bool) {
	if panel, found := findPanelInNode(layout.Root, panelID); found {
		return panel, true
	}
	for _, panel := range layout.HiddenPanels {
		if panel.ID == panelID {
			return panel.Clone(), true
		}
	}
	return PanelInstanceState{}, false
}

func findPanelInNode(root DockNode, panelID PanelID) (PanelInstanceState, bool) {
	switch node := root.(type) {
	case TabStackNode:
		for _, panel := range node.Panels {
			if panel.ID == panelID {
				return panel.Clone(), true
			}
		}
	case SplitNode:
		if panel, found := findPanelInNode(node.First, panelID); found {
			return panel, true
		}
		return findPanelInNode(node.Second, panelID)
	}
	return PanelInstanceState{}, false
}

func mergeLinkContext(current LinkContext, incoming LinkContext, scopes []LinkScope) (LinkContext, error) {
	merged := current.Clone()
	for _, scope := range scopes {
		switch scope {
		case LinkDataset:
			merged.DatasetID = incoming.DatasetID
		case LinkSymbols:
			merged.Symbols = cloneSymbols(incoming.Symbols)
		case LinkInterval:
			merged.Interval = incoming.Interval
		case LinkTimeRange:
			merged.TimeRange = incoming.TimeRange.Normalize()
		case LinkExperiment:
			merged.ExperimentID = incoming.ExperimentID
		case LinkRun:
			merged.RunID = incoming.RunID
		case LinkModel:
			merged.ModelID = incoming.ModelID
		default:
			return LinkContext{}, fmt.Errorf("%w: unsupported link scope %q", ErrInvariant, scope)
		}
	}
	return merged, nil
}

func mergeLoadedLayouts(defaults []WorkspaceLayout, loaded []WorkspaceLayout) ([]WorkspaceLayout, error) {
	output := cloneLayouts(defaults)
	for _, layout := range loaded {
		replaced := false
		for index := range output {
			if output[index].Workspace != layout.Workspace {
				continue
			}
			output[index] = layout.Clone()
			replaced = true
			break
		}
		if !replaced {
			return nil, fmt.Errorf("%w: loaded unknown workspace %q", ErrInvariant, layout.Workspace)
		}
	}
	if err := ValidateWorkspaceLayouts(output); err != nil {
		return nil, err
	}
	return output, nil
}

func workspaceLayout(layouts []WorkspaceLayout, workspace Workspace) (WorkspaceLayout, bool) {
	for _, layout := range layouts {
		if layout.Workspace == workspace {
			return layout.Clone(), true
		}
	}
	return WorkspaceLayout{}, false
}

func (state *AppState) applyEventEnvelope(event EventEnvelope) {
	if event.ID != "" {
		state.Connection.LastEventID = event.ID
	}
	if !event.Timestamp.IsZero() {
		state.Connection.UpdatedAt = event.Timestamp.UTC()
	}
	if state.Connection.State == ConnectionStarting {
		state.Connection.State = ConnectionConnected
	}
}

func (state *AppState) defaultContextFromSnapshot() {
	if state.LinkContext.DatasetID == "" && len(state.Datasets) > 0 {
		dataset := state.Datasets[0]
		state.LinkContext.DatasetID = dataset.ID
		state.LinkContext.Symbols = cloneSymbols(dataset.Symbols)
		state.LinkContext.Interval = dataset.Interval
		state.LinkContext.TimeRange = TimeRange{Start: dataset.Start, End: dataset.End}.Normalize()
		state.updateDefaultLinkGroup(state.LinkContext)
	}
	if state.LinkContext.RunID == "" && len(state.Results) > 0 {
		state.LinkContext.RunID = state.Results[0].ID
	}
	if state.LinkContext.ModelID == "" && len(state.Models) > 0 {
		state.LinkContext.ModelID = state.Models[0].ID
	}
}

func (state *AppState) updateDefaultLinkGroup(context LinkContext) {
	for layoutIndex := range state.Layouts {
		for groupIndex := range state.Layouts[layoutIndex].LinkGroups {
			if state.Layouts[layoutIndex].LinkGroups[groupIndex].Name == "Default" {
				state.Layouts[layoutIndex].LinkGroups[groupIndex].Context = context.Clone()
			}
		}
	}
}

func upsertJob(input []JobSummary, job JobSummary) []JobSummary {
	output := cloneJobs(input)
	job.UpdatedAt = job.UpdatedAt.UTC()
	for index := range output {
		if output[index].ID == job.ID {
			output[index] = job
			return output
		}
	}
	return append(output, job)
}

func upsertInference(input []InferenceSummary, inference InferenceSummary) []InferenceSummary {
	output := cloneInferences(input)
	inference = inference.Clone()
	for index := range output {
		if output[index].ID == inference.ID {
			output[index] = inference
			return output
		}
	}
	return append(output, inference)
}

func upsertComponent(input []ComponentSnapshot, component ComponentSnapshot) []ComponentSnapshot {
	output := cloneComponents(input)
	component.UpdatedAt = component.UpdatedAt.UTC()
	for index := range output {
		if output[index].Component == component.Component {
			output[index] = component
			return output
		}
	}
	return append(output, component)
}
