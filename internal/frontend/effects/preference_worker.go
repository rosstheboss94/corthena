package effects

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/preferences"
)

type preferenceTaskKind uint8

const (
	preferenceTaskLoad preferenceTaskKind = iota + 1
	preferenceTaskSave
)

type preferenceTask struct {
	kind preferenceTaskKind
	load appstate.LoadPreferencesEffect
	save appstate.PersistPreferencesEffect
}

type preferenceTaskQueue struct {
	mu              sync.Mutex
	limit           int
	tasks           []preferenceTask
	highestRevision uint64
	hasRevision     bool
}

func newPreferenceTaskQueue(limit int) *preferenceTaskQueue {
	if limit <= 0 {
		limit = defaultEffectBuffer
	}
	return &preferenceTaskQueue{limit: limit}
}

func (queue *preferenceTaskQueue) enqueue(effect appstate.UIEffect) (bool, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	switch effect := effect.(type) {
	case appstate.LoadPreferencesEffect:
		if len(queue.tasks) >= queue.limit {
			return false, false
		}
		queue.tasks = append(queue.tasks, preferenceTask{kind: preferenceTaskLoad, load: effect})
		return true, true
	case appstate.PersistPreferencesEffect:
		if queue.hasRevision && effect.Revision <= queue.highestRevision {
			return true, false
		}
		for index := range queue.tasks {
			if queue.tasks[index].kind == preferenceTaskSave {
				queue.tasks[index] = preferenceTask{kind: preferenceTaskSave, save: effect}
				queue.highestRevision = effect.Revision
				queue.hasRevision = true
				return true, true
			}
		}
		if len(queue.tasks) >= queue.limit {
			return false, false
		}
		queue.tasks = append(queue.tasks, preferenceTask{kind: preferenceTaskSave, save: effect})
		queue.highestRevision = effect.Revision
		queue.hasRevision = true
		return true, true
	default:
		return false, false
	}
}

func (queue *preferenceTaskQueue) dequeue() (preferenceTask, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.tasks) == 0 {
		return preferenceTask{}, false
	}
	task := queue.tasks[0]
	queue.tasks[0] = preferenceTask{}
	queue.tasks = queue.tasks[1:]
	return task, true
}

func (runtime *Runtime) enqueuePreferenceEffect(effect appstate.UIEffect) {
	accepted, notify := runtime.preferenceTasks.enqueue(effect)
	if !accepted {
		runtime.reportPreferenceQueueFull(effect)
		return
	}
	if notify {
		select {
		case runtime.preferenceWake <- struct{}{}:
		default:
		}
	}
}

func (runtime *Runtime) reportPreferenceQueueFull(effect appstate.UIEffect) {
	snapshot := appstate.ErrorSnapshot{Code: appstate.ErrorPersistence, Message: "preference persistence queue is full", Retryable: true}
	switch effect := effect.(type) {
	case appstate.LoadPreferencesEffect:
		runtime.sendAction(appstate.EffectFailedAction{EffectID: effect.ID, FailedAt: runtime.clock.Now(), Operation: "load preferences", Error: snapshot})
	case appstate.PersistPreferencesEffect:
		runtime.sendAction(appstate.PreferencesPersistenceFailedAction{EffectID: effect.ID, Revision: effect.Revision, FailedAt: runtime.clock.Now(), Error: snapshot})
	}
}

func (runtime *Runtime) runPreferenceWorker() {
	defer runtime.wg.Done()
	for {
		select {
		case <-runtime.ctx.Done():
			return
		case <-runtime.preferenceWake:
		}
		for {
			task, ok := runtime.preferenceTasks.dequeue()
			if !ok {
				break
			}
			runtime.handlePreferenceTask(task)
			if runtime.ctx.Err() != nil {
				return
			}
		}
	}
}

func (runtime *Runtime) handlePreferenceTask(task preferenceTask) {
	switch task.kind {
	case preferenceTaskLoad:
		runtime.loadPreferences(task.load)
	case preferenceTaskSave:
		runtime.savePreferences(task.save)
	default:
		runtime.sendAction(appstate.EffectFailedAction{FailedAt: runtime.clock.Now(), Operation: "run preference effect", Error: appstate.ErrorSnapshot{Code: appstate.ErrorInvariant, Message: fmt.Sprintf("unknown preference task kind %d", task.kind)}})
	}
}

func (runtime *Runtime) loadPreferences(effect appstate.LoadPreferencesEffect) {
	result, err := runtime.preferenceStore.Load(runtime.ctx)
	if err != nil {
		if !preferenceOperationCanceled(err) {
			runtime.sendAction(appstate.EffectFailedAction{EffectID: effect.ID, FailedAt: runtime.clock.Now(), Operation: "load preferences", Error: preferenceErrorSnapshot("load preferences", err)})
		}
		return
	}
	recovered := result.Source == preferences.LoadDefaultInvalid
	diagnostic := ""
	if recovered {
		diagnostic = "Saved preferences were invalid; the 125% default was restored."
	}
	runtime.sendAction(appstate.PreferencesLoadedAction{
		EffectID: effect.ID, BaseRevision: effect.BaseRevision,
		Revision: result.Snapshot.Revision, Preferences: result.Snapshot.Preferences,
		LoadedAt: runtime.clock.Now(), Recovered: recovered, Diagnostic: diagnostic,
	})
}

func (runtime *Runtime) savePreferences(effect appstate.PersistPreferencesEffect) {
	err := runtime.preferenceStore.Save(runtime.ctx, preferences.Snapshot{Revision: effect.Revision, Preferences: effect.Preferences})
	if err != nil {
		if !preferenceOperationCanceled(err) {
			runtime.sendAction(appstate.PreferencesPersistenceFailedAction{EffectID: effect.ID, Revision: effect.Revision, FailedAt: runtime.clock.Now(), Error: preferenceErrorSnapshot("save preferences", err)})
		}
		return
	}
	runtime.sendAction(appstate.PreferencesPersistedAction{EffectID: effect.ID, Revision: effect.Revision, SavedAt: runtime.clock.Now()})
}

func preferenceErrorSnapshot(operation string, err error) appstate.ErrorSnapshot {
	return appstate.ErrorSnapshot{Code: appstate.ErrorPersistence, Message: fmt.Sprintf("%s: %v", operation, err), Retryable: true}
}

func preferenceOperationCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
