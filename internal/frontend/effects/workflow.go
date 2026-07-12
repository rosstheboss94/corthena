package effects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func (runtime *Runtime) startWorkflow(effect appstate.UIEffect) {
	key, generation, validationErr := workflowIdentity(effect)
	if validationErr != nil {
		runtime.sendWorkflowFailure(key, generation, validationErr, appstate.ErrorValidation, false)
		return
	}
	runtime.workflowMu.Lock()
	if previous, found := runtime.workflowCancels[key]; found {
		previous.cancel()
		delete(runtime.workflowCancels, key)
	}
	runtime.workflowMu.Unlock()
	select {
	case <-runtime.ctx.Done():
		return
	case runtime.workers <- struct{}{}:
	default:
		runtime.sendWorkflowFailure(key, generation, errors.New("workflow queue is saturated"), appstate.ErrorEffectBusy, true)
		return
	}
	ctx, cancel := context.WithCancel(runtime.ctx)
	runtime.workflowMu.Lock()
	runtime.workflowCancels[key] = workflowCancellation{generation: generation, cancel: cancel}
	runtime.workflowMu.Unlock()
	runtime.wg.Add(1)
	go func() {
		defer runtime.wg.Done()
		defer func() { <-runtime.workers }()
		defer cancel()
		message, err := runtime.runWorkflow(ctx, effect)
		if err != nil {
			code := appstate.ErrorDataFailed
			switch {
			case strings.HasPrefix(key, "experiments"):
				code = appstate.ErrorExperimentFailed
			case strings.HasPrefix(key, "jobs"):
				code = appstate.ErrorJobsFailed
			case strings.HasPrefix(key, "results"):
				code = appstate.ErrorResultsFailed
			}
			retryable := true
			var typedErr interface{ FrontendError() appstate.ErrorSnapshot }
			if errors.As(err, &typedErr) {
				snapshot := typedErr.FrontendError()
				code = snapshot.Code
				retryable = snapshot.Retryable
			}
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				code = appstate.ErrorDataCancelled
				switch {
				case strings.HasPrefix(key, "experiments"):
					code = appstate.ErrorExperimentCancelled
				case strings.HasPrefix(key, "jobs"):
					code = appstate.ErrorJobsCancelled
				case strings.HasPrefix(key, "results"):
					code = appstate.ErrorResultsCancelled
				}
			}
			runtime.sendWorkflowFailure(key, generation, err, code, retryable)
		} else {
			runtime.sendAction(appstate.ClientMessageAction{Message: message})
		}
		runtime.workflowMu.Lock()
		current, found := runtime.workflowCancels[key]
		if found && current.generation == generation {
			delete(runtime.workflowCancels, key)
		}
		runtime.workflowMu.Unlock()
	}()
}

func (runtime *Runtime) runWorkflow(ctx context.Context, effect appstate.UIEffect) (appstate.ClientMessage, error) {
	switch effect := effect.(type) {
	case appstate.QueryDataWorkspaceEffect:
		return runtime.client.DataWorkspace(ctx, effect.Query)
	case appstate.ImportDataEffect:
		return runtime.client.ImportData(ctx, effect.Request)
	case appstate.QueryExperimentsEffect:
		return runtime.client.Experiments(ctx, effect.Query)
	case appstate.EvaluateExperimentEffect:
		return runtime.client.EvaluateExperiment(ctx, effect.Request)
	case appstate.SubmitExperimentEffect:
		return runtime.client.SubmitExperiment(ctx, effect.Command)
	case appstate.QueryJobsWorkspaceEffect:
		return runtime.client.JobsWorkspace(ctx, effect.Query)
	case appstate.ControlJobEffect:
		return runtime.client.ControlJob(ctx, effect.Command)
	case appstate.QueryResultsWorkspaceEffect:
		return runtime.client.ResultsWorkspace(ctx, effect.Query)
	default:
		return nil, fmt.Errorf("unsupported workflow effect %T", effect)
	}
}

func workflowIdentity(effect appstate.UIEffect) (string, uint64, error) {
	switch effect := effect.(type) {
	case appstate.QueryDataWorkspaceEffect:
		return "data", effect.Query.Generation, effect.Query.Validate()
	case appstate.ImportDataEffect:
		return "data", effect.Request.Generation, effect.Request.Validate()
	case appstate.QueryExperimentsEffect:
		return "experiments-query", effect.Query.Generation, effect.Query.Validate()
	case appstate.EvaluateExperimentEffect:
		return "experiments-evaluate", effect.Request.Generation, effect.Request.Validate()
	case appstate.SubmitExperimentEffect:
		if effect.Command.CorrelationID == "" || effect.Command.CommandID == "" || effect.Command.Generation == 0 {
			return "experiments-submit", effect.Command.Generation, appstate.ErrInvalidExperiment
		}
		return "experiments-submit", effect.Command.Generation, nil
	case appstate.QueryJobsWorkspaceEffect:
		return "jobs-query", effect.Query.Generation, effect.Query.Validate()
	case appstate.ControlJobEffect:
		return "jobs-control", effect.Command.Generation, effect.Command.Validate()
	case appstate.QueryResultsWorkspaceEffect:
		return "results-query", effect.Query.Generation, effect.Query.Validate()
	default:
		return "", 0, fmt.Errorf("unsupported workflow effect %T", effect)
	}
}

func (runtime *Runtime) sendWorkflowFailure(key string, generation uint64, err error, code appstate.ErrorCode, retryable bool) {
	snapshot := appstate.ErrorSnapshot{Code: code, Message: err.Error(), Retryable: retryable}
	var typedErr interface{ FrontendError() appstate.ErrorSnapshot }
	if errors.As(err, &typedErr) {
		snapshot = typedErr.FrontendError()
		if code == appstate.ErrorDataCancelled || code == appstate.ErrorExperimentCancelled ||
			code == appstate.ErrorJobsCancelled || code == appstate.ErrorResultsCancelled {
			snapshot.Code = code
			snapshot.Message = "workflow request cancelled"
		}
	}
	if strings.HasPrefix(key, "experiments") {
		runtime.sendAction(appstate.ExperimentQueryFailedAction{Generation: generation, FailedAt: runtime.clock.Now(), Error: snapshot})
		return
	}
	if strings.HasPrefix(key, "jobs") {
		runtime.sendAction(appstate.JobsQueryFailedAction{Generation: generation, FailedAt: runtime.clock.Now(), Error: snapshot})
		return
	}
	if strings.HasPrefix(key, "results") {
		runtime.sendAction(appstate.ResultsQueryFailedAction{Generation: generation, FailedAt: runtime.clock.Now(), Error: snapshot})
		return
	}
	runtime.sendAction(appstate.DataQueryFailedAction{Generation: generation, FailedAt: runtime.clock.Now(), Error: snapshot})
}

func (runtime *Runtime) cancelWorkflow(effect appstate.UIEffect) {
	prefix := "data"
	generation := uint64(0)
	if typed, ok := effect.(appstate.CancelDataEffect); ok {
		generation = typed.Generation
	} else if typed, ok := effect.(appstate.CancelExperimentEffect); ok {
		prefix = "experiments"
		generation = typed.Generation
	} else if typed, ok := effect.(appstate.CancelJobsEffect); ok {
		prefix = "jobs"
		generation = typed.Generation
	} else if typed, ok := effect.(appstate.CancelResultsEffect); ok {
		prefix = "results"
		generation = typed.Generation
	}
	runtime.workflowMu.Lock()
	var cancelled []uint64
	for key, current := range runtime.workflowCancels {
		if !strings.HasPrefix(key, prefix) || (generation != 0 && current.generation != generation) {
			continue
		}
		current.cancel()
		cancelled = append(cancelled, current.generation)
		delete(runtime.workflowCancels, key)
	}
	runtime.workflowMu.Unlock()
	for _, currentGeneration := range cancelled {
		switch prefix {
		case "data":
			runtime.sendAction(appstate.DataQueryCancelledAction{Generation: currentGeneration, CancelledAt: runtime.clock.Now()})
		case "experiments":
			runtime.sendAction(appstate.ExperimentQueryCancelledAction{Generation: currentGeneration, CancelledAt: runtime.clock.Now()})
		case "jobs":
			runtime.sendAction(appstate.JobsQueryCancelledAction{Generation: currentGeneration, CancelledAt: runtime.clock.Now()})
		case "results":
			runtime.sendAction(appstate.ResultsQueryCancelledAction{Generation: currentGeneration, CancelledAt: runtime.clock.Now()})
		}
	}
}
