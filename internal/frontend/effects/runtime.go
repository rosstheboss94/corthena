package effects

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

const (
	defaultEffectBuffer = 32
	defaultActionBuffer = 64
	defaultConcurrency  = 4
)

// Config configures effect runtime channel buffers and concurrency.
type Config struct {
	EffectBuffer    int
	ActionBuffer    int
	MaxConcurrent   int
	Clock           appstate.Clock
	PreferenceStore PreferenceStore
	DraftStore      ExperimentDraftStore
}

// Runtime owns background goroutines for client and persistence effects.
type Runtime struct {
	ctx             context.Context
	cancel          context.CancelFunc
	client          appstate.FrontendClient
	store           LayoutStore
	clock           appstate.Clock
	effects         chan appstate.UIEffect
	actions         chan appstate.UIAction
	workers         chan struct{}
	layoutTasks     *layoutTaskQueue
	layoutWake      chan struct{}
	preferenceStore PreferenceStore
	preferenceTasks *preferenceTaskQueue
	preferenceWake  chan struct{}
	draftStore      ExperimentDraftStore
	draftTasks      *draftTaskQueue
	draftWake       chan struct{}
	wg              sync.WaitGroup
	once            sync.Once
	researchMu      sync.Mutex
	researchCancels map[appstate.LinkGroupID]researchCancellation
	workflowMu      sync.Mutex
	workflowCancels map[string]workflowCancellation
	err             error
}

type researchCancellation struct {
	generation uint64
	cancel     context.CancelFunc
}

type workflowCancellation struct {
	generation uint64
	cancel     context.CancelFunc
}

// Start creates and starts a bounded effect runtime. The runtime owns closing
// its action channel during Close.
func Start(
	parent context.Context,
	client appstate.FrontendClient,
	store LayoutStore,
	config Config,
) (*Runtime, error) {
	if parent == nil {
		return nil, errors.New("start frontend effects: parent context is nil")
	}
	if client == nil {
		return nil, errors.New("start frontend effects: client is nil")
	}
	if store == nil {
		return nil, errors.New("start frontend effects: layout store is nil")
	}
	if config.EffectBuffer <= 0 {
		config.EffectBuffer = defaultEffectBuffer
	}
	if config.ActionBuffer <= 0 {
		config.ActionBuffer = defaultActionBuffer
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = defaultConcurrency
	}
	if config.Clock == nil {
		config.Clock = appstate.RealClock{}
	}
	if config.PreferenceStore == nil {
		config.PreferenceStore = NewMemoryPreferenceStore()
	}
	if config.DraftStore == nil {
		config.DraftStore = NewMemoryExperimentDraftStore()
	}
	ctx, cancel := context.WithCancel(parent)
	runtime := &Runtime{
		ctx:             ctx,
		cancel:          cancel,
		client:          client,
		store:           store,
		clock:           config.Clock,
		effects:         make(chan appstate.UIEffect, config.EffectBuffer),
		actions:         make(chan appstate.UIAction, config.ActionBuffer),
		workers:         make(chan struct{}, config.MaxConcurrent),
		layoutTasks:     newLayoutTaskQueue(config.EffectBuffer),
		layoutWake:      make(chan struct{}, 1),
		preferenceStore: config.PreferenceStore,
		preferenceTasks: newPreferenceTaskQueue(config.EffectBuffer),
		preferenceWake:  make(chan struct{}, 1),
		draftStore:      config.DraftStore,
		draftTasks:      newDraftTaskQueue(config.EffectBuffer),
		draftWake:       make(chan struct{}, 1),
		researchCancels: make(map[appstate.LinkGroupID]researchCancellation),
		workflowCancels: make(map[string]workflowCancellation),
	}
	runtime.wg.Add(4)
	go runtime.dispatch()
	go runtime.runLayoutWorker()
	go runtime.runPreferenceWorker()
	go runtime.runDraftWorker()
	return runtime, nil
}

// Actions returns the bounded action channel drained by the render thread.
func (runtime *Runtime) Actions() <-chan appstate.UIAction {
	return runtime.actions
}

// Enqueue submits an effect without blocking the render thread.
func (runtime *Runtime) Enqueue(effect appstate.UIEffect) bool {
	select {
	case <-runtime.ctx.Done():
		return false
	case runtime.effects <- effect:
		return true
	default:
		return false
	}
}

// Close cancels workers, waits for them, closes the action channel, and closes
// the underlying client. It is idempotent.
func (runtime *Runtime) Close() error {
	runtime.once.Do(func() {
		runtime.cancel()
		runtime.wg.Wait()
		close(runtime.actions)
		runtime.err = runtime.client.Close()
	})
	return runtime.err
}

func (runtime *Runtime) dispatch() {
	defer runtime.wg.Done()
	for {
		select {
		case <-runtime.ctx.Done():
			return
		case effect := <-runtime.effects:
			runtime.startEffect(effect)
		}
	}
}

func (runtime *Runtime) startEffect(effect appstate.UIEffect) {
	switch typedEffect := effect.(type) {
	case appstate.LoadLayoutsEffect,
		appstate.PersistLayoutEffect,
		appstate.PersistLayoutsEffect:
		runtime.enqueueLayoutEffect(typedEffect)
		return
	case appstate.LoadPreferencesEffect,
		appstate.PersistPreferencesEffect:
		runtime.enqueuePreferenceEffect(typedEffect)
		return
	case appstate.LoadExperimentDraftEffect,
		appstate.PersistExperimentDraftEffect:
		runtime.enqueueDraftEffect(typedEffect)
		return
	case appstate.QueryResearchEffect:
		runtime.startResearch(typedEffect)
		return
	case appstate.CancelResearchEffect:
		runtime.cancelResearch(typedEffect)
		return
	case appstate.QueryDataWorkspaceEffect,
		appstate.ImportDataEffect,
		appstate.QueryExperimentsEffect,
		appstate.EvaluateExperimentEffect,
		appstate.SubmitExperimentEffect:
		runtime.startWorkflow(typedEffect)
		return
	case appstate.CancelDataEffect,
		appstate.CancelExperimentEffect:
		runtime.cancelWorkflow(typedEffect)
		return
	}
	select {
	case <-runtime.ctx.Done():
		return
	case runtime.workers <- struct{}{}:
		runtime.wg.Add(1)
		go func() {
			defer runtime.wg.Done()
			defer func() { <-runtime.workers }()
			runtime.handle(effect)
		}()
	default:
		runtime.sendAction(appstate.EffectFailedAction{
			EffectID:  effectID(effect),
			FailedAt:  runtime.clock.Now(),
			Operation: "enqueue effect",
			Error: appstate.ErrorSnapshot{
				Code:      appstate.ErrorEffectBusy,
				Message:   "frontend effect runtime is busy",
				Retryable: true,
			},
		})
	}
}

func (runtime *Runtime) startResearch(effect appstate.QueryResearchEffect) {
	if err := effect.Query.Validate(); err != nil {
		runtime.sendAction(appstate.ResearchQueryFailedAction{
			GroupID: effect.Query.GroupID, Generation: effect.Query.Generation, FailedAt: runtime.clock.Now(),
			Error: appstate.ErrorSnapshot{Code: appstate.ErrorResearchFailed, Message: err.Error()},
		})
		return
	}
	runtime.researchMu.Lock()
	if previous, found := runtime.researchCancels[effect.Query.GroupID]; found {
		previous.cancel()
		delete(runtime.researchCancels, effect.Query.GroupID)
	}
	runtime.researchMu.Unlock()
	select {
	case <-runtime.ctx.Done():
		return
	case runtime.workers <- struct{}{}:
	default:
		runtime.sendAction(appstate.ResearchQueryFailedAction{
			GroupID: effect.Query.GroupID, Generation: effect.Query.Generation, FailedAt: runtime.clock.Now(),
			Error: appstate.ErrorSnapshot{Code: appstate.ErrorEffectBusy, Message: "Research request queue is saturated", Retryable: true},
		})
		return
	}
	queryContext, cancel := context.WithCancel(runtime.ctx)
	runtime.researchMu.Lock()
	runtime.researchCancels[effect.Query.GroupID] = researchCancellation{generation: effect.Query.Generation, cancel: cancel}
	runtime.researchMu.Unlock()
	runtime.wg.Add(1)
	go func() {
		defer runtime.wg.Done()
		defer func() { <-runtime.workers }()
		defer cancel()
		message, err := runtime.client.Research(queryContext, effect.Query)
		if err != nil {
			snapshot := appstate.ErrorSnapshot{Code: appstate.ErrorResearchFailed, Message: err.Error(), Retryable: true, CorrelationID: effect.Query.CorrelationID}
			var typedErr interface{ FrontendError() appstate.ErrorSnapshot }
			if errors.As(err, &typedErr) {
				snapshot = typedErr.FrontendError()
			}
			if errors.Is(err, context.Canceled) || errors.Is(queryContext.Err(), context.Canceled) {
				snapshot = appstate.ErrorSnapshot{Code: appstate.ErrorResearchCancelled, Message: "Research request cancelled", Retryable: true, CorrelationID: effect.Query.CorrelationID}
			}
			runtime.sendAction(appstate.ResearchQueryFailedAction{
				GroupID: effect.Query.GroupID, Generation: effect.Query.Generation, FailedAt: runtime.clock.Now(), Error: snapshot,
			})
		} else {
			runtime.sendAction(appstate.ClientMessageAction{Message: message})
		}
		runtime.researchMu.Lock()
		current, found := runtime.researchCancels[effect.Query.GroupID]
		if found && current.generation == effect.Query.Generation {
			delete(runtime.researchCancels, effect.Query.GroupID)
		}
		runtime.researchMu.Unlock()
	}()
}

func (runtime *Runtime) cancelResearch(effect appstate.CancelResearchEffect) {
	runtime.researchMu.Lock()
	current, found := runtime.researchCancels[effect.GroupID]
	cancelled := false
	if found && (effect.Generation == 0 || current.generation == effect.Generation) {
		current.cancel()
		delete(runtime.researchCancels, effect.GroupID)
		cancelled = true
	}
	runtime.researchMu.Unlock()
	if cancelled {
		runtime.sendAction(appstate.ResearchQueryCancelledAction{
			GroupID: effect.GroupID, Generation: current.generation, CancelledAt: runtime.clock.Now(),
		})
	}
}

func (runtime *Runtime) handle(effect appstate.UIEffect) {
	if effect == nil {
		runtime.sendAction(appstate.EffectFailedAction{
			FailedAt:  runtime.clock.Now(),
			Operation: "run effect",
			Error: appstate.ErrorSnapshot{
				Code:      appstate.ErrorInvariant,
				Message:   "nil frontend effect",
				Retryable: false,
			},
		})
		return
	}
	switch effect := effect.(type) {
	case appstate.LoadSnapshotEffect:
		message, err := runtime.client.Snapshot(
			runtime.ctx,
			appstate.SnapshotRequest{CorrelationID: effect.CorrelationID},
		)
		if err != nil {
			runtime.sendFailure(effect.ID, "load snapshot", err)
			return
		}
		runtime.sendAction(appstate.ClientMessageAction{Message: message})
	case appstate.SubscribeClientEventsEffect:
		messages, err := runtime.client.Subscribe(
			runtime.ctx,
			appstate.EventSubscription{Since: effect.Since},
		)
		if err != nil {
			runtime.sendFailure(effect.ID, "subscribe client events", err)
			return
		}
		for {
			select {
			case <-runtime.ctx.Done():
				return
			case message, ok := <-messages:
				if !ok {
					return
				}
				runtime.sendAction(appstate.ClientMessageAction{Message: message})
			}
		}
	default:
		runtime.sendAction(appstate.EffectFailedAction{
			EffectID:  effectID(effect),
			FailedAt:  runtime.clock.Now(),
			Operation: "run effect",
			Error: appstate.ErrorSnapshot{
				Code:      appstate.ErrorInvariant,
				Message:   fmt.Sprintf("unhandled frontend effect %T", effect),
				Retryable: false,
			},
		})
	}
}

func (runtime *Runtime) sendFailure(effectID appstate.EffectID, operation string, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	snapshot := appstate.ErrorSnapshot{
		Code:      appstate.ErrorClientUnavailable,
		Message:   err.Error(),
		Retryable: true,
	}
	var typedErr interface {
		FrontendError() appstate.ErrorSnapshot
	}
	if errors.As(err, &typedErr) {
		snapshot = typedErr.FrontendError()
	}
	runtime.sendAction(appstate.EffectFailedAction{
		EffectID:  effectID,
		FailedAt:  runtime.clock.Now(),
		Operation: operation,
		Error:     snapshot,
	})
}

func (runtime *Runtime) sendAction(action appstate.UIAction) {
	select {
	case <-runtime.ctx.Done():
	case runtime.actions <- action:
	}
}

func effectID(effect appstate.UIEffect) appstate.EffectID {
	switch effect := effect.(type) {
	case appstate.LoadSnapshotEffect:
		return effect.ID
	case appstate.SubscribeClientEventsEffect:
		return effect.ID
	case appstate.LoadLayoutsEffect:
		return effect.ID
	case appstate.PersistLayoutEffect:
		return effect.ID
	case appstate.PersistLayoutsEffect:
		return effect.ID
	case appstate.LoadPreferencesEffect:
		return effect.ID
	case appstate.PersistPreferencesEffect:
		return effect.ID
	case appstate.QueryResearchEffect:
		return effect.ID
	case appstate.CancelResearchEffect:
		return effect.ID
	case appstate.QueryDataWorkspaceEffect:
		return effect.ID
	case appstate.ImportDataEffect:
		return effect.ID
	case appstate.CancelDataEffect:
		return effect.ID
	case appstate.QueryExperimentsEffect:
		return effect.ID
	case appstate.EvaluateExperimentEffect:
		return effect.ID
	case appstate.SubmitExperimentEffect:
		return effect.ID
	case appstate.CancelExperimentEffect:
		return effect.ID
	case appstate.LoadExperimentDraftEffect:
		return effect.ID
	case appstate.PersistExperimentDraftEffect:
		return effect.ID
	default:
		return ""
	}
}
