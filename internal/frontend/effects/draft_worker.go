package effects

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/drafts"
)

type draftTask struct {
	load *appstate.LoadExperimentDraftEffect
	save *appstate.PersistExperimentDraftEffect
}

type draftTaskQueue struct {
	mu              sync.Mutex
	limit           int
	tasks           []draftTask
	highestRevision uint64
}

func newDraftTaskQueue(limit int) *draftTaskQueue {
	if limit <= 0 {
		limit = defaultEffectBuffer
	}
	return &draftTaskQueue{limit: limit}
}

func (queue *draftTaskQueue) enqueue(effect appstate.UIEffect) (bool, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	switch effect := effect.(type) {
	case appstate.LoadExperimentDraftEffect:
		if len(queue.tasks) >= queue.limit {
			return false, false
		}
		copy := effect
		queue.tasks = append(queue.tasks, draftTask{load: &copy})
		return true, true
	case appstate.PersistExperimentDraftEffect:
		if effect.Revision <= queue.highestRevision {
			return true, false
		}
		for index := range queue.tasks {
			if queue.tasks[index].save != nil {
				copy := effect
				queue.tasks[index] = draftTask{save: &copy}
				queue.highestRevision = effect.Revision
				return true, true
			}
		}
		if len(queue.tasks) >= queue.limit {
			return false, false
		}
		copy := effect
		queue.tasks = append(queue.tasks, draftTask{save: &copy})
		queue.highestRevision = effect.Revision
		return true, true
	default:
		return false, false
	}
}

func (queue *draftTaskQueue) dequeue() (draftTask, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.tasks) == 0 {
		return draftTask{}, false
	}
	task := queue.tasks[0]
	queue.tasks[0] = draftTask{}
	queue.tasks = queue.tasks[1:]
	return task, true
}

func (runtime *Runtime) enqueueDraftEffect(effect appstate.UIEffect) {
	accepted, notify := runtime.draftTasks.enqueue(effect)
	if !accepted {
		snapshot := appstate.ErrorSnapshot{Code: appstate.ErrorPersistence, Message: "experiment draft autosave queue is full", Retryable: true}
		switch effect := effect.(type) {
		case appstate.LoadExperimentDraftEffect:
			runtime.sendAction(appstate.EffectFailedAction{EffectID: effect.ID, FailedAt: runtime.clock.Now(), Operation: "load experiment draft", Error: snapshot})
		case appstate.PersistExperimentDraftEffect:
			runtime.sendAction(appstate.ExperimentDraftPersistenceFailedAction{EffectID: effect.ID, Revision: effect.Revision, FailedAt: runtime.clock.Now(), Error: snapshot})
		}
		return
	}
	if notify {
		select {
		case runtime.draftWake <- struct{}{}:
		default:
		}
	}
}

func (runtime *Runtime) runDraftWorker() {
	defer runtime.wg.Done()
	for {
		select {
		case <-runtime.ctx.Done():
			return
		case <-runtime.draftWake:
		}
		for {
			task, ok := runtime.draftTasks.dequeue()
			if !ok {
				break
			}
			runtime.handleDraftTask(task)
			if runtime.ctx.Err() != nil {
				return
			}
		}
	}
}

func (runtime *Runtime) handleDraftTask(task draftTask) {
	if task.load != nil {
		result, err := runtime.draftStore.Load(runtime.ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				runtime.sendAction(appstate.EffectFailedAction{EffectID: task.load.ID, FailedAt: runtime.clock.Now(), Operation: "load experiment draft", Error: draftError(err)})
			}
			return
		}
		recovered := result.Source == drafts.LoadDefaultInvalid
		diagnostic := ""
		if recovered {
			diagnostic = "Saved experiment draft was invalid and has been quarantined."
		}
		runtime.sendAction(appstate.ExperimentDraftLoadedAction{
			EffectID: task.load.ID, BaseRevision: task.load.BaseRevision,
			Draft: result.Snapshot.Draft.Clone(), LoadedAt: runtime.clock.Now(),
			Recovered: recovered, Diagnostic: diagnostic,
		})
		return
	}
	if task.save != nil {
		err := runtime.draftStore.Save(runtime.ctx, drafts.Snapshot{Revision: task.save.Revision, Draft: task.save.Draft.Clone()})
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				runtime.sendAction(appstate.ExperimentDraftPersistenceFailedAction{EffectID: task.save.ID, Revision: task.save.Revision, FailedAt: runtime.clock.Now(), Error: draftError(err)})
			}
			return
		}
		runtime.sendAction(appstate.ExperimentDraftPersistedAction{EffectID: task.save.ID, Revision: task.save.Revision, SavedAt: runtime.clock.Now()})
		return
	}
	runtime.sendAction(appstate.EffectFailedAction{FailedAt: runtime.clock.Now(), Operation: "run experiment draft task", Error: appstate.ErrorSnapshot{Code: appstate.ErrorInvariant, Message: "empty experiment draft task"}})
}

func draftError(err error) appstate.ErrorSnapshot {
	return appstate.ErrorSnapshot{Code: appstate.ErrorPersistence, Message: fmt.Sprintf("experiment draft autosave: %v", err), Retryable: true}
}
