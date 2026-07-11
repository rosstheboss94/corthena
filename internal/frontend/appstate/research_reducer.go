package appstate

import (
	"fmt"
	"time"
)

const defaultResearchResolution = 1200

func beginResearchQuery(state *AppState, query ResearchQuery) (UIEffect, error) {
	query = query.Clone()
	if err := query.Validate(); err != nil {
		return nil, err
	}
	index := state.researchGroupIndex(query.GroupID)
	current := &state.Research.Groups[index]
	if query.Generation <= current.Generation {
		return nil, fmt.Errorf("%w: research generation %d is not newer than %d", ErrInvariant, query.Generation, current.Generation)
	}
	current.Generation = query.Generation
	current.SelectedFeature = query.SelectedFeatures[0]
	current.Scenario = query.Scenario
	current.Query = query
	current.Stale = current.Snapshot.Query.Generation != 0
	current.State = ResearchLoading
	current.Error = ErrorSnapshot{}
	if query.Scenario == ResearchScenarioReconnecting {
		state.Connection.State = ConnectionDegraded
		state.Connection.Detail = "Research connection is reconnecting"
	}
	return QueryResearchEffect{
		ID:    EffectID(fmt.Sprintf("research-%s-%020d", query.GroupID, query.Generation)),
		Query: query,
	}, nil
}

func refreshResearchGroup(state *AppState, groupID LinkGroupID, context LinkContext, cursor string) (UIEffect, error) {
	if groupID == "" || context.DatasetID == "" || len(context.Symbols) == 0 {
		return nil, nil
	}
	index := state.researchGroupIndex(groupID)
	current := state.Research.Groups[index]
	selectedFeature := current.SelectedFeature
	if selectedFeature == "" {
		selectedFeature = state.Research.SelectedFeature
	}
	scenario := current.Scenario
	if !scenario.Valid() {
		scenario = state.Research.Scenario
	}
	resolution := defaultResearchResolution
	pageSize := 120
	sort := ResearchSortTimeAscending
	filter := ""
	if current.Query.Resolution >= 64 {
		resolution = current.Query.Resolution
	}
	if current.Query.PageSize > 0 {
		pageSize = current.Query.PageSize
	}
	if current.Query.Sort.Valid() {
		sort = current.Query.Sort
	}
	if cursor == "" {
		filter = current.Query.Filter
	}
	generation := current.Generation + 1
	query := ResearchQuery{
		CorrelationID:    CorrelationID(fmt.Sprintf("research-%s-%020d", groupID, generation)),
		GroupID:          groupID,
		Generation:       generation,
		DatasetID:        context.DatasetID,
		Symbols:          cloneSymbols(context.Symbols),
		Interval:         context.Interval,
		TimeRange:        context.TimeRange.Normalize(),
		SelectedFeatures: []FeatureName{selectedFeature},
		Target:           state.Research.Target,
		Resolution:       resolution,
		Cursor:           cursor,
		PageSize:         pageSize,
		Sort:             sort,
		Filter:           filter,
		Scenario:         scenario,
	}
	return beginResearchQuery(state, query)
}

func defaultResearchGroup(layouts []WorkspaceLayout) (LinkGroupID, LinkContext, bool) {
	layout, found := workspaceLayout(layouts, WorkspaceResearch)
	if !found {
		return "", LinkContext{}, false
	}
	for _, group := range layout.LinkGroups {
		if group.Name == "Default" {
			return group.ID, group.Context.Clone(), true
		}
	}
	return "", LinkContext{}, false
}

func researchGroupContext(layouts []WorkspaceLayout, groupID LinkGroupID) (LinkContext, bool) {
	layout, found := workspaceLayout(layouts, WorkspaceResearch)
	if !found {
		return LinkContext{}, false
	}
	for _, group := range layout.LinkGroups {
		if group.ID == groupID {
			return group.Context.Clone(), true
		}
	}
	return LinkContext{}, false
}

func applyResearchResponse(state *AppState, message ResearchResponseMessage) bool {
	snapshot := message.Snapshot.Clone()
	index := state.researchGroupIndex(snapshot.Query.GroupID)
	group := &state.Research.Groups[index]
	if snapshot.Query.Generation != group.Generation || !sameResearchIdentity(snapshot.Query, group.Query) {
		return false
	}
	group.Query = snapshot.Query.Clone()
	group.Snapshot = snapshot
	group.Stale = false
	group.Error = ErrorSnapshot{}
	switch {
	case snapshot.Degraded:
		group.State = ResearchDegraded
	case snapshot.Query.Scenario == ResearchScenarioRecovered:
		group.State = ResearchRecovered
	case len(snapshot.Bars) == 0 && snapshot.Rows.TotalRows == 0:
		group.State = ResearchEmpty
	default:
		group.State = ResearchReady
	}
	return true
}

func sameResearchIdentity(query ResearchQuery, other ResearchQuery) bool {
	return query.GroupID == other.GroupID && query.Generation == other.Generation && query.CorrelationID == other.CorrelationID
}

func researchFailureState(err ErrorSnapshot) ResearchLoadState {
	switch err.Code {
	case ErrorResearchCancelled:
		return ResearchCancelled
	case ErrorEffectBusy:
		return ResearchBusy
	default:
		return ResearchFailed
	}
}

func normalizeResearchRange(timeRange TimeRange) (TimeRange, error) {
	timeRange = timeRange.Normalize()
	if timeRange.Start.IsZero() || timeRange.End.IsZero() || !timeRange.Start.Before(timeRange.End) {
		return TimeRange{}, fmt.Errorf("%w: invalid Research visible range", ErrInvariant)
	}
	const maximum = 100 * 365 * 24 * time.Hour
	if timeRange.End.Sub(timeRange.Start) > maximum {
		return TimeRange{}, fmt.Errorf("%w: Research visible range is too large", ErrInvariant)
	}
	return timeRange, nil
}
