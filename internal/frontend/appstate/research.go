package appstate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/chart"
	virtualtable "github.com/rosstheboss94/corthena/internal/frontend/table"
)

// ErrInvalidResearchQuery identifies a malformed Research client request.
var ErrInvalidResearchQuery = errors.New("invalid research query")

// ResearchScenario selects a deterministic demo-client condition.
type ResearchScenario string

const (
	ResearchScenarioNormal       ResearchScenario = "normal"
	ResearchScenarioLoading      ResearchScenario = "loading"
	ResearchScenarioEmpty        ResearchScenario = "empty"
	ResearchScenarioFailure      ResearchScenario = "failure"
	ResearchScenarioDegraded     ResearchScenario = "degraded"
	ResearchScenarioReconnecting ResearchScenario = "reconnecting"
	ResearchScenarioRecovered    ResearchScenario = "recovered"
	ResearchScenarioCancelled    ResearchScenario = "cancelled"
	ResearchScenarioQueueFull    ResearchScenario = "queue_saturated"
)

// Valid reports whether scenario is a supported deterministic condition.
func (scenario ResearchScenario) Valid() bool {
	switch scenario {
	case ResearchScenarioNormal, ResearchScenarioLoading, ResearchScenarioEmpty, ResearchScenarioFailure,
		ResearchScenarioDegraded, ResearchScenarioReconnecting, ResearchScenarioRecovered,
		ResearchScenarioCancelled, ResearchScenarioQueueFull:
		return true
	default:
		return false
	}
}

// ResearchScenarios returns the stable scenario-control order.
func ResearchScenarios() []ResearchScenario {
	return []ResearchScenario{
		ResearchScenarioNormal,
		ResearchScenarioLoading,
		ResearchScenarioEmpty,
		ResearchScenarioFailure,
		ResearchScenarioDegraded,
		ResearchScenarioReconnecting,
		ResearchScenarioRecovered,
		ResearchScenarioCancelled,
		ResearchScenarioQueueFull,
	}
}

// ResearchSort is the stable server-side ordering for the row page.
type ResearchSort string

const (
	ResearchSortTimeAscending    ResearchSort = "time_ascending"
	ResearchSortTimeDescending   ResearchSort = "time_descending"
	ResearchSortTargetDescending ResearchSort = "target_descending"
)

// Valid reports whether sort is supported.
func (sort ResearchSort) Valid() bool {
	switch sort {
	case ResearchSortTimeAscending, ResearchSortTimeDescending, ResearchSortTargetDescending:
		return true
	default:
		return false
	}
}

// ResearchQuery is the complete immutable identity of one linked Research
// request. Cursor is an opaque decimal offset in the demo implementation.
type ResearchQuery struct {
	CorrelationID    CorrelationID
	GroupID          LinkGroupID
	Generation       uint64
	DatasetID        DatasetID
	Symbols          []Symbol
	Interval         BarInterval
	TimeRange        TimeRange
	SelectedFeatures []FeatureName
	Target           TargetSpec
	Resolution       int
	Cursor           string
	PageSize         int
	Sort             ResearchSort
	Filter           string
	Scenario         ResearchScenario
}

// Clone returns an independent normalized request.
func (query ResearchQuery) Clone() ResearchQuery {
	query.Symbols = cloneSymbols(query.Symbols)
	query.SelectedFeatures = cloneFeatureNames(query.SelectedFeatures)
	query.TimeRange = query.TimeRange.Normalize()
	return query
}

// Validate rejects ambiguous request identities and invalid combinations.
func (query ResearchQuery) Validate() error {
	if query.CorrelationID == "" || query.GroupID == "" || query.Generation == 0 || query.DatasetID == "" {
		return fmt.Errorf("%w: correlation, group, generation, and dataset are required", ErrInvalidResearchQuery)
	}
	if !query.Interval.Valid() || query.TimeRange.Start.IsZero() || query.TimeRange.End.IsZero() ||
		!query.TimeRange.Start.Before(query.TimeRange.End) {
		return fmt.Errorf("%w: invalid UTC interval or time range", ErrInvalidResearchQuery)
	}
	if query.TimeRange.Start.Location() != time.UTC || query.TimeRange.End.Location() != time.UTC {
		return fmt.Errorf("%w: time range must be normalized to UTC", ErrInvalidResearchQuery)
	}
	if len(query.Symbols) == 0 || len(query.Symbols) > 64 {
		return fmt.Errorf("%w: one to 64 symbols are required", ErrInvalidResearchQuery)
	}
	seenSymbols := make(map[Symbol]struct{}, len(query.Symbols))
	for _, symbol := range query.Symbols {
		if strings.TrimSpace(string(symbol)) == "" {
			return fmt.Errorf("%w: symbol is empty", ErrInvalidResearchQuery)
		}
		if _, duplicate := seenSymbols[symbol]; duplicate {
			return fmt.Errorf("%w: duplicate symbol %q", ErrInvalidResearchQuery, symbol)
		}
		seenSymbols[symbol] = struct{}{}
	}
	seenFeatures := make(map[FeatureName]struct{}, len(query.SelectedFeatures))
	if len(query.SelectedFeatures) == 0 || len(query.SelectedFeatures) > 16 {
		return fmt.Errorf("%w: one to 16 selected features are required", ErrInvalidResearchQuery)
	}
	for _, feature := range query.SelectedFeatures {
		if strings.TrimSpace(string(feature)) == "" {
			return fmt.Errorf("%w: selected feature is empty", ErrInvalidResearchQuery)
		}
		if _, duplicate := seenFeatures[feature]; duplicate {
			return fmt.Errorf("%w: duplicate selected feature %q", ErrInvalidResearchQuery, feature)
		}
		seenFeatures[feature] = struct{}{}
	}
	if err := query.Target.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResearchQuery, err)
	}
	if query.Resolution < 64 || query.Resolution > 8192 || query.PageSize < 1 || query.PageSize > 500 {
		return fmt.Errorf("%w: resolution or page size is outside its bound", ErrInvalidResearchQuery)
	}
	if !query.Sort.Valid() || !query.Scenario.Valid() || len(query.Filter) > 128 {
		return fmt.Errorf("%w: invalid sort, scenario, or filter", ErrInvalidResearchQuery)
	}
	if query.Cursor != "" {
		offset, err := strconv.ParseUint(query.Cursor, 10, 64)
		if err != nil || offset > 10_000_000_000 {
			return fmt.Errorf("%w: invalid cursor", ErrInvalidResearchQuery)
		}
	}
	return nil
}

// Validate checks the frontend target configuration.
func (target TargetSpec) Validate() error {
	if target.Kind != "forward_open_return" || target.HorizonBars < 1 || target.HorizonBars > 1024 {
		return errors.New("target must be a 1-1024 bar forward open return")
	}
	return nil
}

// ResearchBar is a tooltip-ready, chronologically ordered primary-symbol bar.
type ResearchBar struct {
	RowID     string
	Timestamp time.Time
	Symbol    Symbol
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// ResearchFeatureDescriptor describes one compiled demo feature.
type ResearchFeatureDescriptor struct {
	Name        FeatureName
	Version     string
	Lookback    int
	Description string
	Fingerprint string
}

// ResearchValue preserves missing values explicitly at the UI boundary.
type ResearchValue struct {
	Timestamp       time.Time
	TargetTimestamp time.Time
	Value           float64
	Missing         bool
}

// ResearchSeries is one leakage-safe feature series.
type ResearchSeries struct {
	Descriptor ResearchFeatureDescriptor
	Values     []ResearchValue
	Minimum    float64
	Maximum    float64
	Missing    int
}

// ResearchTargetPreview contains only values with a valid future horizon.
type ResearchTargetPreview struct {
	Spec         TargetSpec
	Values       []ResearchValue
	ValidRows    uint64
	ExcludedRows uint64
}

// ResearchBin is one deterministic distribution interval.
type ResearchBin struct {
	Minimum float64
	Maximum float64
	Count   uint64
}

// ResearchDistribution is one feature or target histogram.
type ResearchDistribution struct {
	Name FeatureName
	Bins []ResearchBin
}

// ResearchPage is a server-prepared virtual-table page.
type ResearchPage struct {
	Dataset    virtualtable.Dataset
	Model      virtualtable.Model
	NextCursor string
	TotalRows  uint64
}

// Clone returns an independent page.
func (page ResearchPage) Clone() ResearchPage {
	page.Dataset = page.Dataset.Clone()
	page.Model = page.Model.Clone()
	return page
}

// ResearchSnapshot is an immutable, render-ready Research response.
type ResearchSnapshot struct {
	Query         ResearchQuery
	Frame         chart.Frame
	Bars          []ResearchBar
	Features      []ResearchSeries
	Target        ResearchTargetPreview
	Distributions []ResearchDistribution
	Rows          ResearchPage
	PreparedAt    time.Time
	Degraded      bool
}

// Clone returns a deep independent Research response.
func (snapshot ResearchSnapshot) Clone() ResearchSnapshot {
	snapshot.Query = snapshot.Query.Clone()
	snapshot.Frame = snapshot.Frame.Clone()
	snapshot.Bars = append([]ResearchBar(nil), snapshot.Bars...)
	for index := range snapshot.Bars {
		snapshot.Bars[index].Timestamp = snapshot.Bars[index].Timestamp.UTC()
	}
	if len(snapshot.Features) > 0 {
		features := make([]ResearchSeries, len(snapshot.Features))
		for index, series := range snapshot.Features {
			series.Values = append([]ResearchValue(nil), series.Values...)
			for valueIndex := range series.Values {
				series.Values[valueIndex].Timestamp = series.Values[valueIndex].Timestamp.UTC()
			}
			features[index] = series
		}
		snapshot.Features = features
	}
	snapshot.Target.Values = append([]ResearchValue(nil), snapshot.Target.Values...)
	for index := range snapshot.Target.Values {
		snapshot.Target.Values[index].Timestamp = snapshot.Target.Values[index].Timestamp.UTC()
		if !snapshot.Target.Values[index].TargetTimestamp.IsZero() {
			snapshot.Target.Values[index].TargetTimestamp = snapshot.Target.Values[index].TargetTimestamp.UTC()
		}
	}
	if len(snapshot.Distributions) > 0 {
		distributions := make([]ResearchDistribution, len(snapshot.Distributions))
		for index, distribution := range snapshot.Distributions {
			distribution.Bins = append([]ResearchBin(nil), distribution.Bins...)
			distributions[index] = distribution
		}
		snapshot.Distributions = distributions
	}
	snapshot.Rows = snapshot.Rows.Clone()
	snapshot.PreparedAt = snapshot.PreparedAt.UTC()
	return snapshot
}

// ResearchResponseMessage publishes one complete typed Research result.
type ResearchResponseMessage struct {
	Event    EventEnvelope
	Snapshot ResearchSnapshot
}

func (ResearchResponseMessage) isClientMessage() {}

// ResearchLoadState is one panel-visible asynchronous state.
type ResearchLoadState string

const (
	ResearchIdle      ResearchLoadState = "idle"
	ResearchLoading   ResearchLoadState = "loading"
	ResearchReady     ResearchLoadState = "ready"
	ResearchEmpty     ResearchLoadState = "empty"
	ResearchFailed    ResearchLoadState = "error"
	ResearchDegraded  ResearchLoadState = "degraded"
	ResearchRecovered ResearchLoadState = "recovered"
	ResearchCancelled ResearchLoadState = "cancelled"
	ResearchBusy      ResearchLoadState = "queue_saturated"
)

// ResearchGroupState is one independently generation-ordered link group.
type ResearchGroupState struct {
	GroupID         LinkGroupID
	Generation      uint64
	SelectedFeature FeatureName
	Scenario        ResearchScenario
	ShowOHLCV       bool
	ShowFeature     bool
	ShowTarget      bool
	State           ResearchLoadState
	Stale           bool
	Query           ResearchQuery
	Snapshot        ResearchSnapshot
	Error           ErrorSnapshot
	SelectedRows    []virtualtable.RowID
}

// Clone returns an independent group state.
func (group ResearchGroupState) Clone() ResearchGroupState {
	group.Query = group.Query.Clone()
	group.Snapshot = group.Snapshot.Clone()
	group.SelectedRows = append([]virtualtable.RowID(nil), group.SelectedRows...)
	return group
}

// ResearchWorkspaceState owns pure Research selection and async state.
type ResearchWorkspaceState struct {
	SelectedFeature FeatureName
	Target          TargetSpec
	ShowOHLCV       bool
	ShowFeature     bool
	ShowTarget      bool
	Scenario        ResearchScenario
	Groups          []ResearchGroupState
}

// DefaultResearchWorkspaceState returns the canonical demo configuration.
func DefaultResearchWorkspaceState() ResearchWorkspaceState {
	return ResearchWorkspaceState{
		SelectedFeature: "ret_5",
		Target:          TargetSpec{Kind: "forward_open_return", HorizonBars: 5, LogReturn: false},
		ShowOHLCV:       true,
		ShowFeature:     true,
		ShowTarget:      false,
		Scenario:        ResearchScenarioNormal,
	}
}

// Clone returns an independent workspace state.
func (research ResearchWorkspaceState) Clone() ResearchWorkspaceState {
	if len(research.Groups) > 0 {
		groups := make([]ResearchGroupState, len(research.Groups))
		for index, group := range research.Groups {
			groups[index] = group.Clone()
		}
		research.Groups = groups
	}
	return research
}

// ResearchGroup returns an independent group snapshot.
func (state AppState) ResearchGroup(groupID LinkGroupID) (ResearchGroupState, bool) {
	for _, group := range state.Research.Groups {
		if group.GroupID == groupID {
			return group.Clone(), true
		}
	}
	return ResearchGroupState{}, false
}

func (state *AppState) researchGroupIndex(groupID LinkGroupID) int {
	for index := range state.Research.Groups {
		if state.Research.Groups[index].GroupID == groupID {
			return index
		}
	}
	state.Research.Groups = append(state.Research.Groups, ResearchGroupState{
		GroupID: groupID, State: ResearchIdle,
		SelectedFeature: state.Research.SelectedFeature, Scenario: state.Research.Scenario,
		ShowOHLCV: state.Research.ShowOHLCV, ShowFeature: state.Research.ShowFeature, ShowTarget: state.Research.ShowTarget,
	})
	return len(state.Research.Groups) - 1
}

// ResearchZoomRange returns a UTC range zoomed around a normalized pointer.
func ResearchZoomRange(current TimeRange, center float64, factor float64) (TimeRange, error) {
	current, err := normalizeResearchRange(current)
	if err != nil {
		return TimeRange{}, err
	}
	if center < 0 || center > 1 || factor < 0.05 || factor > 20 {
		return TimeRange{}, fmt.Errorf("%w: invalid Research zoom", ErrInvariant)
	}
	span := current.End.Sub(current.Start)
	newSpan := time.Duration(float64(span) * factor)
	if newSpan < time.Minute {
		newSpan = time.Minute
	}
	anchor := current.Start.Add(time.Duration(float64(span) * center))
	start := anchor.Add(-time.Duration(float64(newSpan) * center))
	return normalizeResearchRange(TimeRange{Start: start, End: start.Add(newSpan)})
}

// ResearchSelectRange maps two normalized chart positions to a UTC range.
func ResearchSelectRange(current TimeRange, first float64, second float64) (TimeRange, error) {
	current, err := normalizeResearchRange(current)
	if err != nil {
		return TimeRange{}, err
	}
	if first < 0 || first > 1 || second < 0 || second > 1 || first == second {
		return TimeRange{}, fmt.Errorf("%w: invalid Research selection", ErrInvariant)
	}
	if first > second {
		first, second = second, first
	}
	span := current.End.Sub(current.Start)
	return normalizeResearchRange(TimeRange{
		Start: current.Start.Add(time.Duration(float64(span) * first)),
		End:   current.Start.Add(time.Duration(float64(span) * second)),
	})
}

// ResearchPanRange shifts a UTC range by a normalized fraction of its span.
func ResearchPanRange(current TimeRange, fraction float64) (TimeRange, error) {
	current, err := normalizeResearchRange(current)
	if err != nil {
		return TimeRange{}, err
	}
	if fraction < -10 || fraction > 10 {
		return TimeRange{}, fmt.Errorf("%w: invalid Research pan", ErrInvariant)
	}
	offset := time.Duration(float64(current.End.Sub(current.Start)) * fraction)
	return normalizeResearchRange(TimeRange{Start: current.Start.Add(offset), End: current.End.Add(offset)})
}
