package effects_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/effects"
	"github.com/rosstheboss94/corthena/internal/frontend/layouts"
	"github.com/rosstheboss94/corthena/internal/frontend/simulator"
)

func TestRuntimeLoadsLayoutsWithRecoveryMetadata(t *testing.T) {
	t.Parallel()

	snapshot := layoutSnapshot(t, 12)
	store := &loadResultStore{result: layouts.LoadResult{
		Snapshot:       snapshot,
		Source:         layouts.LoadDefaultInvalid,
		QuarantinePath: "quarantine/autosave.invalid.json",
		DiagnosticHash: "abc123",
	}}
	runtime, cleanup := startRuntimeWithStore(t, store, effects.Config{})
	defer cleanup()

	if !runtime.Enqueue(appstate.LoadLayoutsEffect{
		ID:           "load-layouts",
		BaseRevision: 4,
		Defaults:     snapshot.Layouts,
		RequestedAt:  fixedTime(),
	}) {
		t.Fatal("Enqueue returned false")
	}

	action := waitAction(t, runtime.Actions())
	loaded, ok := action.(appstate.LayoutsLoadedAction)
	if !ok {
		t.Fatalf("action = %T, want LayoutsLoadedAction", action)
	}
	if loaded.EffectID != "load-layouts" || loaded.BaseRevision != 4 || loaded.Revision != 12 {
		t.Fatalf("load metadata = %#v", loaded)
	}
	if !loaded.Recovered || loaded.Diagnostic == "" {
		t.Fatalf("recovery metadata = recovered %t, diagnostic %q", loaded.Recovered, loaded.Diagnostic)
	}
	if len(loaded.Layouts) != len(snapshot.Layouts) {
		t.Fatalf("loaded layouts = %d, want %d", len(loaded.Layouts), len(snapshot.Layouts))
	}

	snapshot.Layouts[0].HiddenPanels = append(snapshot.Layouts[0].HiddenPanels, appstate.PanelInstanceState{ID: "mutated"})
	if len(loaded.Layouts[0].HiddenPanels) != 0 {
		t.Fatal("loaded action aliases store snapshot")
	}
}

func TestRuntimeUsesEffectDefaultsWhenMemoryStoreIsEmpty(t *testing.T) {
	t.Parallel()

	defaults := layoutSnapshot(t, 0)
	runtime, cleanup := startRuntimeWithStore(t, effects.NewMemoryLayoutStore(), effects.Config{})
	defer cleanup()
	if !runtime.Enqueue(appstate.LoadLayoutsEffect{
		ID:       "load-defaults",
		Defaults: defaults.Layouts,
	}) {
		t.Fatal("Enqueue returned false")
	}

	loaded, ok := waitAction(t, runtime.Actions()).(appstate.LayoutsLoadedAction)
	if !ok {
		t.Fatal("action is not LayoutsLoadedAction")
	}
	if loaded.Recovered || loaded.Revision != 0 || len(loaded.Layouts) != len(defaults.Layouts) {
		t.Fatalf("default load = %#v", loaded)
	}
}

func TestRuntimeSerializesAndCoalescesLayoutSaves(t *testing.T) {
	t.Parallel()

	store := newBlockingRecordingStore()
	runtime, cleanup := startRuntimeWithStore(t, store, effects.Config{
		EffectBuffer: 16,
		ActionBuffer: 16,
	})
	defer cleanup()
	snapshot := layoutSnapshot(t, 0)

	enqueueSnapshot(t, runtime, "save-1", 1, snapshot.Layouts)
	waitSignal(t, store.firstStarted, "first layout save")
	enqueueSnapshot(t, runtime, "save-2", 2, snapshot.Layouts)
	enqueueSnapshot(t, runtime, "save-3", 3, snapshot.Layouts)
	if !runtime.Enqueue(appstate.LoadSnapshotEffect{
		ID:            "dispatch-barrier",
		CorrelationID: "dispatch-barrier",
	}) {
		t.Fatal("enqueue dispatch barrier")
	}

	for {
		action := waitAction(t, runtime.Actions())
		clientAction, ok := action.(appstate.ClientMessageAction)
		if !ok {
			continue
		}
		snapshotMessage, ok := clientAction.Message.(appstate.SnapshotMessage)
		if ok && snapshotMessage.Event.CorrelationID == "dispatch-barrier" {
			break
		}
	}
	close(store.releaseFirst)

	persistedRevisions := make([]uint64, 0, 2)
	for len(persistedRevisions) < 2 {
		action := waitAction(t, runtime.Actions())
		persisted, ok := action.(appstate.LayoutPersistedAction)
		if ok {
			persistedRevisions = append(persistedRevisions, persisted.Revision)
		}
	}
	if persistedRevisions[0] != 1 || persistedRevisions[1] != 3 {
		t.Fatalf("persisted revisions = %v, want [1 3]", persistedRevisions)
	}
	if got := store.revisions(); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("store revisions = %v, want [1 3]", got)
	}
	if store.maximumConcurrent() != 1 {
		t.Fatalf("maximum concurrent layout saves = %d, want 1", store.maximumConcurrent())
	}
}

func TestRuntimeReportsRevisionedLayoutFailure(t *testing.T) {
	t.Parallel()

	store := &failingSaveStore{err: layouts.ErrStaleRevision}
	runtime, cleanup := startRuntimeWithStore(t, store, effects.Config{})
	defer cleanup()
	snapshot := layoutSnapshot(t, 0)
	enqueueSnapshot(t, runtime, "save-stale", 9, snapshot.Layouts)

	failed, ok := waitAction(t, runtime.Actions()).(appstate.LayoutPersistenceFailedAction)
	if !ok {
		t.Fatal("action is not LayoutPersistenceFailedAction")
	}
	if failed.EffectID != "save-stale" || failed.Revision != 9 {
		t.Fatalf("failure metadata = %#v", failed)
	}
	if failed.Error.Code != appstate.ErrorPersistence || failed.Error.Retryable {
		t.Fatalf("failure error = %#v", failed.Error)
	}
	if failed.FailedAt != fixedTime() {
		t.Fatalf("failed at = %v, want %v", failed.FailedAt, fixedTime())
	}
}

func TestRuntimeCancelsBlockedLayoutWorker(t *testing.T) {
	t.Parallel()

	store := &cancelBlockingStore{
		startCh: make(chan struct{}),
		stopCh:  make(chan struct{}),
	}
	client, err := simulator.NewDemoCoordinator(simulator.Options{
		Seed:  31,
		Clock: appstate.FixedClock{Time: fixedTime()},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := effects.Start(context.Background(), client, store, effects.Config{
		Clock: appstate.FixedClock{Time: fixedTime()},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := layoutSnapshot(t, 0)
	enqueueSnapshot(t, runtime, "save-blocked", 1, snapshot.Layouts)
	waitSignal(t, store.startCh, "blocked layout save")

	closed := make(chan error, 1)
	go func() {
		closed <- runtime.Close()
	}()
	waitSignal(t, store.stopCh, "canceled layout save")
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not wait for and stop the layout worker")
	}
	if action, ok := <-runtime.Actions(); ok {
		t.Fatalf("unexpected action after cancellation: %T", action)
	}
}

func TestRuntimeConcurrentLayoutEnqueueKeepsHighestRevision(t *testing.T) {
	store := effects.NewMemoryLayoutStore()
	runtime, cleanup := startRuntimeWithStore(t, store, effects.Config{
		EffectBuffer: 128,
		ActionBuffer: 128,
	})
	defer cleanup()
	snapshot := layoutSnapshot(t, 0)

	const saves = 64
	results := make(chan bool, saves)
	var senders sync.WaitGroup
	for revision := uint64(1); revision <= saves; revision++ {
		senders.Add(1)
		go func(revision uint64) {
			defer senders.Done()
			results <- runtime.Enqueue(appstate.PersistLayoutsEffect{
				ID:       appstate.EffectID("concurrent-save"),
				Revision: revision,
				Layouts:  snapshot.Layouts,
			})
		}(revision)
	}
	senders.Wait()
	close(results)
	for accepted := range results {
		if !accepted {
			t.Fatal("concurrent Enqueue returned false")
		}
	}

	for {
		action := waitAction(t, runtime.Actions())
		persisted, ok := action.(appstate.LayoutPersistedAction)
		if ok && persisted.Revision == saves {
			break
		}
	}
	loaded, err := store.Reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Snapshot.Revision != saves {
		t.Fatalf("stored revision = %d, want %d", loaded.Snapshot.Revision, saves)
	}
}

type loadResultStore struct {
	result layouts.LoadResult
}

func (store *loadResultStore) Reload(ctx context.Context) (layouts.LoadResult, error) {
	if err := ctx.Err(); err != nil {
		return layouts.LoadResult{}, err
	}
	result := store.result
	result.Snapshot = result.Snapshot.Clone()
	return result, nil
}

func (*loadResultStore) Save(context.Context, layouts.Snapshot) error {
	return nil
}

type blockingRecordingStore struct {
	mu              sync.Mutex
	saves           []uint64
	active          int
	maxActive       int
	firstStarted    chan struct{}
	releaseFirst    chan struct{}
	firstStartedOne sync.Once
}

func newBlockingRecordingStore() *blockingRecordingStore {
	return &blockingRecordingStore{
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
}

func (*blockingRecordingStore) Reload(context.Context) (layouts.LoadResult, error) {
	return layouts.LoadResult{Source: layouts.LoadDefaultMissing}, nil
}

func (store *blockingRecordingStore) Save(ctx context.Context, snapshot layouts.Snapshot) error {
	store.mu.Lock()
	store.saves = append(store.saves, snapshot.Revision)
	store.active++
	if store.active > store.maxActive {
		store.maxActive = store.active
	}
	first := len(store.saves) == 1
	store.mu.Unlock()

	if first {
		store.firstStartedOne.Do(func() { close(store.firstStarted) })
		select {
		case <-ctx.Done():
			store.finishSave()
			return ctx.Err()
		case <-store.releaseFirst:
		}
	}
	store.finishSave()
	return nil
}

func (store *blockingRecordingStore) finishSave() {
	store.mu.Lock()
	store.active--
	store.mu.Unlock()
}

func (store *blockingRecordingStore) revisions() []uint64 {
	store.mu.Lock()
	defer store.mu.Unlock()
	output := make([]uint64, len(store.saves))
	copy(output, store.saves)
	return output
}

func (store *blockingRecordingStore) maximumConcurrent() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.maxActive
}

type failingSaveStore struct {
	err error
}

func (*failingSaveStore) Reload(context.Context) (layouts.LoadResult, error) {
	return layouts.LoadResult{Source: layouts.LoadDefaultMissing}, nil
}

func (store *failingSaveStore) Save(context.Context, layouts.Snapshot) error {
	return store.err
}

type cancelBlockingStore struct {
	started sync.Once
	stopped sync.Once
	startCh chan struct{}
	stopCh  chan struct{}
}

func (*cancelBlockingStore) Reload(context.Context) (layouts.LoadResult, error) {
	return layouts.LoadResult{Source: layouts.LoadDefaultMissing}, nil
}

func (store *cancelBlockingStore) Save(ctx context.Context, _ layouts.Snapshot) error {
	store.started.Do(func() { close(store.startCh) })
	<-ctx.Done()
	store.stopped.Do(func() { close(store.stopCh) })
	return ctx.Err()
}

func startRuntimeWithStore(
	t *testing.T,
	store effects.LayoutStore,
	config effects.Config,
) (*effects.Runtime, func()) {
	t.Helper()
	client, err := simulator.NewDemoCoordinator(simulator.Options{
		Seed:  29,
		Clock: appstate.FixedClock{Time: fixedTime()},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	config.Clock = appstate.FixedClock{Time: fixedTime()}
	runtime, err := effects.Start(ctx, client, store, config)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	return runtime, func() {
		if err := runtime.Close(); err != nil {
			t.Fatal(err)
		}
		cancel()
	}
}

func layoutSnapshot(t *testing.T, revision uint64) layouts.Snapshot {
	t.Helper()
	layout, err := appstate.DefaultWorkspaceLayout(
		appstate.WorkspaceResearch,
		appstate.NewSequentialIDSource("effects-layout"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return layouts.Snapshot{
		Revision: revision,
		Layouts:  []appstate.WorkspaceLayout{layout},
	}
}

func enqueueSnapshot(
	t *testing.T,
	runtime *effects.Runtime,
	effectID appstate.EffectID,
	revision uint64,
	workspaceLayouts []appstate.WorkspaceLayout,
) {
	t.Helper()
	if !runtime.Enqueue(appstate.PersistLayoutsEffect{
		ID:       effectID,
		Revision: revision,
		Layouts:  workspaceLayouts,
	}) {
		t.Fatalf("enqueue layout revision %d returned false", revision)
	}
}

func waitSignal(t *testing.T, signal <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", operation)
	}
}
