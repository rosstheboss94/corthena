package appstate

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/chart"
)

func validResearchQuery() ResearchQuery {
	return ResearchQuery{
		CorrelationID: "corr-1", GroupID: "link-default-research", Generation: 1,
		DatasetID: "dataset-1", Symbols: []Symbol{"AAPL"}, Interval: IntervalDaily,
		TimeRange:        TimeRange{Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)},
		SelectedFeatures: []FeatureName{"ret_5"}, Target: TargetSpec{Kind: "forward_open_return", HorizonBars: 5},
		Resolution: 800, PageSize: 100, Sort: ResearchSortTimeAscending, Scenario: ResearchScenarioNormal,
	}
}

func TestResearchQueryValidationAndClone(t *testing.T) {
	query := validResearchQuery()
	if err := query.Validate(); err != nil {
		t.Fatalf("validate query: %v", err)
	}
	clone := query.Clone()
	clone.Symbols[0] = "MSFT"
	clone.SelectedFeatures[0] = "volatility_20"
	if query.Symbols[0] != "AAPL" || query.SelectedFeatures[0] != "ret_5" {
		t.Fatal("query clone shares mutable slices")
	}
	invalid := query
	invalid.TimeRange.Start = invalid.TimeRange.Start.In(time.FixedZone("offset", 3600))
	if !errors.Is(invalid.Validate(), ErrInvalidResearchQuery) {
		t.Fatalf("non-UTC range error = %v", invalid.Validate())
	}
}

func TestResearchRangeOperationsAreDeterministic(t *testing.T) {
	current := TimeRange{Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC)}
	zoomed, err := ResearchZoomRange(current, 0.5, 0.5)
	if err != nil || !zoomed.Start.Equal(time.Date(2025, 1, 3, 12, 0, 0, 0, time.UTC)) || !zoomed.End.Equal(time.Date(2025, 1, 8, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("zoomed = %+v, err %v", zoomed, err)
	}
	selected, err := ResearchSelectRange(current, 0.8, 0.2)
	if err != nil || selected.End.Sub(selected.Start) != 6*24*time.Hour {
		t.Fatalf("selected = %+v, err %v", selected, err)
	}
	panned, err := ResearchPanRange(current, 0.1)
	if err != nil || !panned.Start.Equal(current.Start.Add(24*time.Hour)) {
		t.Fatalf("panned = %+v, err %v", panned, err)
	}
}

func TestResearchReducerRejectsStaleCompletion(t *testing.T) {
	clock := FixedClock{Time: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)}
	state, _, err := NewInitialState(clock, NewSequentialIDSource("research"))
	if err != nil {
		t.Fatalf("initial state: %v", err)
	}
	query := validResearchQuery()
	state, effects, err := Reduce(state, RequestResearchAction{Query: query})
	if err != nil || len(effects) != 1 || state.Research.Groups[0].State != ResearchLoading {
		t.Fatalf("begin query: state %+v effects %d err %v", state.Research.Groups, len(effects), err)
	}
	newer := query.Clone()
	newer.Generation = 2
	newer.CorrelationID = "corr-2"
	state, _, err = Reduce(state, RequestResearchAction{Query: newer})
	if err != nil {
		t.Fatalf("supersede query: %v", err)
	}
	previousConnection := state.Connection
	stale := ResearchResponseMessage{
		Event:    EventEnvelope{ID: "stale-event", Timestamp: clock.Time.Add(time.Hour)},
		Snapshot: ResearchSnapshot{Query: query, Frame: chart.Frame{Generation: 1}, Degraded: true},
	}
	state, _, err = Reduce(state, ClientMessageAction{Message: stale})
	if err != nil {
		t.Fatalf("stale response: %v", err)
	}
	group, _ := state.ResearchGroup(query.GroupID)
	if group.Generation != 2 || group.Snapshot.Query.Generation != 0 || group.State != ResearchLoading {
		t.Fatalf("stale response changed current group: %+v", group)
	}
	if state.Connection != previousConnection {
		t.Fatalf("stale response changed global connection state: got %+v want %+v", state.Connection, previousConnection)
	}
}

func TestResearchActionReplayIsDeterministicAcrossLinkedInteractions(t *testing.T) {
	clock := FixedClock{Time: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)}
	replay := func() AppState {
		state, _, err := NewInitialState(clock, NewSequentialIDSource("research-replay"))
		if err != nil {
			t.Fatal(err)
		}
		state.defaultContextFromSnapshot()
		layoutIndex, _ := state.layoutIndex(WorkspaceResearch)
		layout := state.Layouts[layoutIndex]
		context := validResearchQuery().toLinkContext()
		for index := range layout.LinkGroups {
			layout.LinkGroups[index].Context = context.Clone()
		}
		state.Layouts[layoutIndex] = layout
		groupID := layout.LinkGroups[0].ID
		sourcePanel := layout.Root.(SplitNode).First.(SplitNode).First.(TabStackNode).Panels[0].ID
		actions := []UIAction{
			RequestResearchAction{Query: func() ResearchQuery { query := validResearchQuery(); query.GroupID = groupID; return query }()},
			SetResearchVisibilityAction{GroupID: groupID, ShowOHLCV: true, ShowFeature: true, ShowTarget: true},
			SelectResearchRowAction{GroupID: groupID, RowID: "row-17"},
			SelectResearchRowAction{GroupID: groupID, RowID: "row-19", Toggle: true},
			SetResearchRangeAction{GroupID: groupID, SourcePanelID: sourcePanel, TimeRange: TimeRange{Start: context.TimeRange.Start.Add(24 * time.Hour), End: context.TimeRange.End.Add(-24 * time.Hour)}},
		}
		for _, action := range actions {
			state, _, err = Reduce(state, action)
			if err != nil {
				t.Fatalf("reduce %T: %v", action, err)
			}
		}
		return state
	}
	left := replay()
	right := replay()
	if !reflect.DeepEqual(left, right) {
		t.Fatal("identical Research interaction replay produced different state")
	}
}

func TestResearchLinkGroupsRemainIndependent(t *testing.T) {
	clock := FixedClock{Time: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)}
	state, _, err := NewInitialState(clock, NewSequentialIDSource("links"))
	if err != nil {
		t.Fatalf("initial state: %v", err)
	}
	state.defaultContextFromSnapshot()
	layoutIndex, _ := state.layoutIndex(WorkspaceResearch)
	layout := state.Layouts[layoutIndex]
	base := validResearchQuery().TimeRange
	for index := range layout.LinkGroups {
		layout.LinkGroups[index].Context = validResearchQuery().Clone().toLinkContext()
	}
	state.Layouts[layoutIndex] = layout
	defaultID := layout.LinkGroups[0].ID
	compareID := layout.LinkGroups[1].ID
	defaultQuery := validResearchQuery()
	defaultQuery.GroupID = defaultID
	state, _, err = Reduce(state, RequestResearchAction{Query: defaultQuery})
	if err != nil {
		t.Fatalf("default query: %v", err)
	}
	query := validResearchQuery()
	query.GroupID = compareID
	query.CorrelationID = "compare"
	state, _, err = Reduce(state, RequestResearchAction{Query: query})
	if err != nil {
		t.Fatalf("compare query: %v", err)
	}
	if _, found := state.ResearchGroup(defaultID); found {
		defaultGroup, _ := state.ResearchGroup(defaultID)
		if defaultGroup.Generation != 1 {
			t.Fatalf("compare request mutated default generation: %+v", defaultGroup)
		}
	}
	state, _, err = Reduce(state, SetResearchFeatureAction{GroupID: compareID, Feature: "volatility_20"})
	if err != nil {
		t.Fatalf("compare feature: %v", err)
	}
	defaultGroup, _ := state.ResearchGroup(defaultID)
	compareGroup, _ := state.ResearchGroup(compareID)
	if defaultGroup.SelectedFeature != "ret_5" || compareGroup.SelectedFeature != "volatility_20" {
		t.Fatalf("feature selection crossed link groups: default %q compare %q", defaultGroup.SelectedFeature, compareGroup.SelectedFeature)
	}
	if base != query.TimeRange {
		t.Fatal("query range unexpectedly changed")
	}
}

func (query ResearchQuery) toLinkContext() LinkContext {
	return LinkContext{DatasetID: query.DatasetID, Symbols: cloneSymbols(query.Symbols), Interval: query.Interval, TimeRange: query.TimeRange}
}
