package appstate

import (
	"fmt"
	"strings"
)

func beginJobsQuery(state *AppState, query JobsWorkspaceQuery) (UIEffect, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if query.Generation <= state.JobsWorkspace.Generation {
		return nil, fmt.Errorf("%w: Jobs generation %d is not newer than %d", ErrInvariant, query.Generation, state.JobsWorkspace.Generation)
	}
	state.JobsWorkspace.Generation = query.Generation
	state.JobsWorkspace.Query = query
	state.JobsWorkspace.Scenario = query.Scenario
	state.JobsWorkspace.State = WorkspaceLoading
	state.JobsWorkspace.Stale = state.JobsWorkspace.Snapshot.Query.Generation != 0
	state.JobsWorkspace.Error = ErrorSnapshot{}
	return QueryJobsWorkspaceEffect{ID: EffectID(fmt.Sprintf("jobs-%020d", query.Generation)), Query: query}, nil
}

func refreshJobsWorkspace(state *AppState) (UIEffect, error) {
	scenario := state.JobsWorkspace.Scenario
	if !scenario.Valid() {
		scenario = JobsScenarioSuccess
	}
	generation := state.JobsWorkspace.Generation + 1
	return beginJobsQuery(state, JobsWorkspaceQuery{
		CorrelationID: CorrelationID(fmt.Sprintf("jobs-%020d", generation)),
		Generation:    generation,
		Scenario:      scenario,
	})
}

func applyJobsWorkspaceResponse(state *AppState, message JobsWorkspaceMessage) bool {
	snapshot := message.Snapshot.Clone()
	if snapshot.Query.Generation != state.JobsWorkspace.Generation ||
		snapshot.Query.CorrelationID != state.JobsWorkspace.Query.CorrelationID {
		return false
	}
	state.JobsWorkspace.Query = snapshot.Query
	state.JobsWorkspace.Snapshot = snapshot
	state.JobsWorkspace.State = WorkspaceReady
	state.JobsWorkspace.Stale = false
	state.JobsWorkspace.Error = ErrorSnapshot{}
	state.Jobs = jobSummaries(snapshot.Jobs)
	if !containsJobDetail(snapshot.Jobs, state.JobsWorkspace.SelectedJobID) {
		state.JobsWorkspace.SelectedJobID = preferredJobID(snapshot.Jobs, snapshot.Query.Scenario)
	}
	return true
}

func beginResultsQuery(state *AppState, query ResultsWorkspaceQuery) (UIEffect, error) {
	query = query.Clone()
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if query.Generation <= state.ResultsWorkspace.Generation {
		return nil, fmt.Errorf("%w: Results generation %d is not newer than %d", ErrInvariant, query.Generation, state.ResultsWorkspace.Generation)
	}
	state.ResultsWorkspace.Generation = query.Generation
	state.ResultsWorkspace.Query = query
	state.ResultsWorkspace.Scenario = query.Scenario
	state.ResultsWorkspace.Filter = query.Filter
	state.ResultsWorkspace.State = WorkspaceLoading
	state.ResultsWorkspace.Stale = state.ResultsWorkspace.Snapshot.Query.Generation != 0
	state.ResultsWorkspace.Error = ErrorSnapshot{}
	return QueryResultsWorkspaceEffect{ID: EffectID(fmt.Sprintf("results-%020d", query.Generation)), Query: query}, nil
}

func refreshResultsWorkspace(state *AppState) (UIEffect, error) {
	scenario := state.ResultsWorkspace.Scenario
	if !scenario.Valid() {
		scenario = ResultsScenarioNormal
	}
	generation := state.ResultsWorkspace.Generation + 1
	return beginResultsQuery(state, ResultsWorkspaceQuery{
		CorrelationID: CorrelationID(fmt.Sprintf("results-%020d", generation)),
		Generation:    generation,
		Scenario:      scenario,
		Filter:        state.ResultsWorkspace.Filter,
		RunIDs:        append([]RunID(nil), state.ResultsWorkspace.SelectedRunIDs...),
	})
}

func applyResultsWorkspaceResponse(state *AppState, message ResultsWorkspaceMessage) bool {
	snapshot := message.Snapshot.Clone()
	if snapshot.Query.Generation != state.ResultsWorkspace.Generation ||
		snapshot.Query.CorrelationID != state.ResultsWorkspace.Query.CorrelationID {
		return false
	}
	state.ResultsWorkspace.Query = snapshot.Query.Clone()
	state.ResultsWorkspace.Snapshot = snapshot
	state.ResultsWorkspace.Stale = false
	state.ResultsWorkspace.Error = ErrorSnapshot{}
	state.Results = resultSummaries(snapshot.Runs)
	state.ResultsWorkspace.SelectedRunIDs = existingRunSelection(state.ResultsWorkspace.SelectedRunIDs, snapshot.Runs)
	if len(state.ResultsWorkspace.SelectedRunIDs) == 0 && len(snapshot.Runs) > 0 {
		state.ResultsWorkspace.SelectedRunIDs = []RunID{snapshot.Runs[0].Summary.ID}
	}
	if !containsRunDetail(snapshot.Runs, state.ResultsWorkspace.PrimaryRunID) {
		if len(state.ResultsWorkspace.SelectedRunIDs) > 0 {
			state.ResultsWorkspace.PrimaryRunID = state.ResultsWorkspace.SelectedRunIDs[0]
		} else {
			state.ResultsWorkspace.PrimaryRunID = ""
		}
	}
	if state.ResultsWorkspace.PrimaryRunID != "" {
		state.LinkContext.RunID = state.ResultsWorkspace.PrimaryRunID
	}
	switch {
	case snapshot.Degraded:
		state.ResultsWorkspace.State = WorkspaceDegraded
	case snapshot.Query.Scenario == ResultsScenarioRecovered:
		state.ResultsWorkspace.State = WorkspaceRecovered
	case len(snapshot.Runs) == 0:
		state.ResultsWorkspace.State = WorkspaceEmpty
	default:
		state.ResultsWorkspace.State = WorkspaceReady
	}
	return true
}

func jobSummaries(details []JobDetail) []JobSummary {
	if len(details) == 0 {
		return nil
	}
	output := make([]JobSummary, len(details))
	for index, detail := range details {
		output[index] = detail.Summary.Clone()
	}
	return output
}

func resultSummaries(details []RunResultDetail) []RunResultSummary {
	if len(details) == 0 {
		return nil
	}
	output := make([]RunResultSummary, len(details))
	for index, detail := range details {
		output[index] = detail.Summary.Clone()
	}
	return output
}

func containsJobDetail(details []JobDetail, id JobID) bool {
	for _, detail := range details {
		if detail.Summary.ID == id {
			return true
		}
	}
	return false
}

func preferredJobID(details []JobDetail, scenario JobsScenario) JobID {
	wanted := JobRunning
	switch scenario {
	case JobsScenarioPauseResume:
		wanted = JobPaused
	case JobsScenarioCancellation:
		wanted = JobCancelled
	case JobsScenarioInterruption:
		wanted = JobInterrupted
	case JobsScenarioFailure:
		wanted = JobFailed
	}
	for _, detail := range details {
		if detail.Summary.State == wanted {
			return detail.Summary.ID
		}
	}
	if len(details) > 0 {
		return details[0].Summary.ID
	}
	return ""
}

func containsRunDetail(details []RunResultDetail, id RunID) bool {
	for _, detail := range details {
		if detail.Summary.ID == id {
			return true
		}
	}
	return false
}

func existingRunSelection(selection []RunID, details []RunResultDetail) []RunID {
	output := make([]RunID, 0, len(selection))
	for _, runID := range selection {
		if containsRunDetail(details, runID) {
			output = append(output, runID)
		}
	}
	return output
}

func toggleRunSelection(selection []RunID, runID RunID, toggle bool) ([]RunID, error) {
	if runID == "" {
		return nil, fmt.Errorf("%w: result run ID is empty", ErrInvariant)
	}
	if !toggle {
		return []RunID{runID}, nil
	}
	output := append([]RunID(nil), selection...)
	for index, selected := range output {
		if selected != runID {
			continue
		}
		return append(output[:index], output[index+1:]...), nil
	}
	if len(output) >= 4 {
		return nil, fmt.Errorf("%w: at most four runs may be compared", ErrInvariant)
	}
	return append(output, runID), nil
}

func findJobDetail(details []JobDetail, id JobID) (JobDetail, bool) {
	for _, detail := range details {
		if detail.Summary.ID == id {
			return detail.Clone(), true
		}
	}
	return JobDetail{}, false
}

func upsertJobDetail(input []JobDetail, detail JobDetail) []JobDetail {
	output := make([]JobDetail, len(input))
	for index, current := range input {
		output[index] = current.Clone()
		if current.Summary.ID == detail.Summary.ID {
			output[index] = detail.Clone()
			return output
		}
	}
	return append(output, detail.Clone())
}

func upsertRunDetail(input []RunResultDetail, detail RunResultDetail) []RunResultDetail {
	output := make([]RunResultDetail, len(input))
	for index, current := range input {
		output[index] = current.Clone()
		if current.Summary.ID == detail.Summary.ID {
			output[index] = detail.Clone()
			return output
		}
	}
	return append(output, detail.Clone())
}

func workspaceJobResultFailure(code ErrorCode) WorkspaceLoadState {
	switch code {
	case ErrorJobsCancelled, ErrorResultsCancelled:
		return WorkspaceCancelled
	case ErrorEffectBusy:
		return WorkspaceBusy
	default:
		return WorkspaceFailed
	}
}

func normalizedResultFilter(filter string) string {
	return strings.ToLower(strings.TrimSpace(filter))
}
