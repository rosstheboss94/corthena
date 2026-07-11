package effects

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/layouts"
)

type layoutTaskKind uint8

const (
	layoutTaskLoad layoutTaskKind = iota + 1
	layoutTaskLegacySave
	layoutTaskSave
)

type layoutTask struct {
	kind   layoutTaskKind
	load   appstate.LoadLayoutsEffect
	legacy appstate.PersistLayoutEffect
	save   appstate.PersistLayoutsEffect
}

// layoutTaskQueue is written by the dispatcher and drained by exactly one
// layout worker. Its mutex is never held during store I/O or action delivery.
type layoutTaskQueue struct {
	mu              sync.Mutex
	limit           int
	tasks           []layoutTask
	highestRevision uint64
	hasRevision     bool
}

func newLayoutTaskQueue(limit int) *layoutTaskQueue {
	if limit <= 0 {
		limit = defaultEffectBuffer
	}
	return &layoutTaskQueue{limit: limit}
}

func (queue *layoutTaskQueue) enqueue(effect appstate.UIEffect) (accepted bool, notify bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	switch effect := effect.(type) {
	case appstate.LoadLayoutsEffect:
		if len(queue.tasks) >= queue.limit {
			return false, false
		}
		effect.Defaults = appstate.CloneWorkspaceLayouts(effect.Defaults)
		queue.tasks = append(queue.tasks, layoutTask{kind: layoutTaskLoad, load: effect})
		return true, true
	case appstate.PersistLayoutEffect:
		if len(queue.tasks) >= queue.limit {
			return false, false
		}
		effect.Layout = effect.Layout.Clone()
		queue.tasks = append(queue.tasks, layoutTask{kind: layoutTaskLegacySave, legacy: effect})
		return true, true
	case appstate.PersistLayoutsEffect:
		if queue.hasRevision && effect.Revision <= queue.highestRevision {
			return true, false
		}
		effect.Layouts = appstate.CloneWorkspaceLayouts(effect.Layouts)
		for index := range queue.tasks {
			if queue.tasks[index].kind == layoutTaskSave {
				queue.tasks[index] = layoutTask{kind: layoutTaskSave, save: effect}
				queue.highestRevision = effect.Revision
				queue.hasRevision = true
				return true, true
			}
		}
		if len(queue.tasks) >= queue.limit {
			return false, false
		}
		queue.tasks = append(queue.tasks, layoutTask{kind: layoutTaskSave, save: effect})
		queue.highestRevision = effect.Revision
		queue.hasRevision = true
		return true, true
	default:
		return false, false
	}
}

func (queue *layoutTaskQueue) dequeue() (layoutTask, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.tasks) == 0 {
		return layoutTask{}, false
	}
	task := queue.tasks[0]
	queue.tasks[0] = layoutTask{}
	queue.tasks = queue.tasks[1:]
	return task, true
}

func (runtime *Runtime) enqueueLayoutEffect(effect appstate.UIEffect) {
	accepted, notify := runtime.layoutTasks.enqueue(effect)
	if !accepted {
		runtime.reportLayoutQueueFull(effect)
		return
	}
	if !notify {
		return
	}
	select {
	case runtime.layoutWake <- struct{}{}:
	default:
	}
}

func (runtime *Runtime) reportLayoutQueueFull(effect appstate.UIEffect) {
	snapshot := appstate.ErrorSnapshot{
		Code:      appstate.ErrorPersistence,
		Message:   "layout persistence queue is full",
		Retryable: true,
	}
	switch effect := effect.(type) {
	case appstate.PersistLayoutEffect:
		runtime.sendAction(appstate.LayoutPersistenceFailedAction{
			EffectID: effect.ID,
			Revision: effect.Revision,
			FailedAt: runtime.clock.Now(),
			Error:    snapshot,
		})
	case appstate.PersistLayoutsEffect:
		runtime.sendAction(appstate.LayoutPersistenceFailedAction{
			EffectID: effect.ID,
			Revision: effect.Revision,
			FailedAt: runtime.clock.Now(),
			Error:    snapshot,
		})
	case appstate.LoadLayoutsEffect:
		runtime.sendAction(appstate.EffectFailedAction{
			EffectID:  effect.ID,
			FailedAt:  runtime.clock.Now(),
			Operation: "load layouts",
			Error:     snapshot,
		})
	}
}

func (runtime *Runtime) runLayoutWorker() {
	defer runtime.wg.Done()
	for {
		select {
		case <-runtime.ctx.Done():
			return
		case <-runtime.layoutWake:
		}
		for {
			task, ok := runtime.layoutTasks.dequeue()
			if !ok {
				break
			}
			runtime.handleLayoutTask(task)
			if runtime.ctx.Err() != nil {
				return
			}
		}
	}
}

func (runtime *Runtime) handleLayoutTask(task layoutTask) {
	switch task.kind {
	case layoutTaskLoad:
		runtime.loadLayouts(task.load)
	case layoutTaskLegacySave:
		runtime.persistLegacyLayout(task.legacy)
	case layoutTaskSave:
		runtime.persistLayouts(task.save.ID, task.save.Revision, task.save.Layouts)
	default:
		runtime.sendAction(appstate.EffectFailedAction{
			FailedAt:  runtime.clock.Now(),
			Operation: "run layout effect",
			Error: appstate.ErrorSnapshot{
				Code:      appstate.ErrorInvariant,
				Message:   fmt.Sprintf("unknown layout task kind %d", task.kind),
				Retryable: false,
			},
		})
	}
}

func (runtime *Runtime) loadLayouts(effect appstate.LoadLayoutsEffect) {
	result, err := runtime.store.Reload(runtime.ctx)
	if err != nil {
		runtime.sendLayoutLoadFailure(effect.ID, err)
		return
	}
	if result.Source == layouts.LoadDefaultMissing && len(result.Snapshot.Layouts) == 0 {
		result.Snapshot = layouts.Snapshot{
			Layouts: appstate.CloneWorkspaceLayouts(effect.Defaults),
		}
	}
	if err := layouts.Validate(result.Snapshot); err != nil {
		runtime.sendLayoutLoadFailure(effect.ID, fmt.Errorf("validate loaded layouts: %w", err))
		return
	}
	recovered, diagnostic, err := layoutLoadMetadata(result)
	if err != nil {
		runtime.sendLayoutLoadFailure(effect.ID, err)
		return
	}
	runtime.sendAction(appstate.LayoutsLoadedAction{
		EffectID:     effect.ID,
		BaseRevision: effect.BaseRevision,
		Revision:     result.Snapshot.Revision,
		Layouts:      appstate.CloneWorkspaceLayouts(result.Snapshot.Layouts),
		LoadedAt:     runtime.clock.Now(),
		Recovered:    recovered,
		Diagnostic:   diagnostic,
	})
}

func layoutLoadMetadata(result layouts.LoadResult) (bool, string, error) {
	switch result.Source {
	case layouts.LoadSaved, layouts.LoadDefaultMissing, layouts.LoadReset:
		return false, "", nil
	case layouts.LoadMigrated:
		return true, "Saved layout settings were migrated to the current schema.", nil
	case layouts.LoadDefaultInvalid:
		diagnostic := "Saved layout settings were invalid; defaults were restored."
		if result.DiagnosticHash != "" {
			diagnostic = fmt.Sprintf("%s Diagnostic %s.", diagnostic, result.DiagnosticHash)
		}
		return true, diagnostic, nil
	default:
		return false, "", fmt.Errorf("unknown layout load source %q", result.Source)
	}
}

func (runtime *Runtime) persistLegacyLayout(effect appstate.PersistLayoutEffect) {
	if effect.Workspace != "" && effect.Workspace != effect.Layout.Workspace {
		runtime.sendLayoutPersistenceFailure(
			effect.ID,
			effect.Revision,
			fmt.Errorf("workspace %q does not match layout workspace %q", effect.Workspace, effect.Layout.Workspace),
		)
		return
	}
	loaded, err := runtime.store.Reload(runtime.ctx)
	if err != nil {
		runtime.sendLayoutPersistenceFailure(effect.ID, effect.Revision, fmt.Errorf("reload layouts: %w", err))
		return
	}
	snapshot := loaded.Snapshot.Clone()
	replaced := false
	for index := range snapshot.Layouts {
		if snapshot.Layouts[index].Workspace == effect.Layout.Workspace {
			snapshot.Layouts[index] = effect.Layout.Clone()
			replaced = true
			break
		}
	}
	if !replaced {
		snapshot.Layouts = append(snapshot.Layouts, effect.Layout.Clone())
	}
	if effect.Revision != 0 {
		snapshot.Revision = effect.Revision
	} else {
		if snapshot.Revision == math.MaxUint64 {
			runtime.sendLayoutPersistenceFailure(effect.ID, effect.Revision, errors.New("layout revision is at maximum"))
			return
		}
		snapshot.Revision++
	}
	runtime.persistSnapshot(effect.ID, snapshot)
}

func (runtime *Runtime) persistLayouts(
	effectID appstate.EffectID,
	revision uint64,
	workspaceLayouts []appstate.WorkspaceLayout,
) {
	runtime.persistSnapshot(effectID, layouts.Snapshot{
		Revision: revision,
		Layouts:  appstate.CloneWorkspaceLayouts(workspaceLayouts),
	})
}

func (runtime *Runtime) persistSnapshot(effectID appstate.EffectID, snapshot layouts.Snapshot) {
	if err := layouts.Validate(snapshot); err != nil {
		runtime.sendLayoutPersistenceFailure(effectID, snapshot.Revision, fmt.Errorf("validate layouts: %w", err))
		return
	}
	if err := runtime.store.Save(runtime.ctx, snapshot); err != nil {
		runtime.sendLayoutPersistenceFailure(effectID, snapshot.Revision, err)
		return
	}
	runtime.sendAction(appstate.LayoutPersistedAction{
		EffectID: effectID,
		Revision: snapshot.Revision,
		SavedAt:  runtime.clock.Now(),
	})
}

func (runtime *Runtime) sendLayoutLoadFailure(effectID appstate.EffectID, err error) {
	if layoutOperationCanceled(err) {
		return
	}
	runtime.sendAction(appstate.EffectFailedAction{
		EffectID:  effectID,
		FailedAt:  runtime.clock.Now(),
		Operation: "load layouts",
		Error:     layoutErrorSnapshot("load layouts", err),
	})
}

func (runtime *Runtime) sendLayoutPersistenceFailure(
	effectID appstate.EffectID,
	revision uint64,
	err error,
) {
	if layoutOperationCanceled(err) {
		return
	}
	runtime.sendAction(appstate.LayoutPersistenceFailedAction{
		EffectID: effectID,
		Revision: revision,
		FailedAt: runtime.clock.Now(),
		Error:    layoutErrorSnapshot("persist layouts", err),
	})
}

func layoutErrorSnapshot(operation string, err error) appstate.ErrorSnapshot {
	retryable := true
	if errors.Is(err, layouts.ErrStaleRevision) ||
		errors.Is(err, layouts.ErrRevisionConflict) ||
		errors.Is(err, layouts.ErrInvalidSnapshot) ||
		errors.Is(err, layouts.ErrInvalidDocument) {
		retryable = false
	}
	return appstate.ErrorSnapshot{
		Code:      appstate.ErrorPersistence,
		Message:   fmt.Sprintf("%s: %v", operation, err),
		Retryable: retryable,
	}
}

func layoutOperationCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
