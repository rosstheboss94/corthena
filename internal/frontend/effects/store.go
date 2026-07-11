// Package effects owns background UI effects. Render-thread callers enqueue
// typed effects without blocking and drain typed actions produced by workers.
package effects

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/layouts"
	"github.com/rosstheboss94/corthena/internal/frontend/preferences"
)

// LayoutStore owns versioned layout snapshot persistence outside the render
// thread. Implementations must return immutable values from Reload.
type LayoutStore interface {
	Reload(context.Context) (layouts.LoadResult, error)
	Save(context.Context, layouts.Snapshot) error
}

// PreferenceStore owns versioned global preference persistence outside the
// render thread.
type PreferenceStore interface {
	Load(context.Context) (preferences.LoadResult, error)
	Save(context.Context, preferences.Snapshot) error
}

// MemoryPreferenceStore is a deterministic test preference store.
type MemoryPreferenceStore struct {
	mu       sync.Mutex
	snapshot preferences.Snapshot
	present  bool
}

// NewMemoryPreferenceStore creates an empty in-memory preference store.
func NewMemoryPreferenceStore() *MemoryPreferenceStore {
	return &MemoryPreferenceStore{}
}

// Load returns saved preferences or canonical defaults.
func (store *MemoryPreferenceStore) Load(ctx context.Context) (preferences.LoadResult, error) {
	if store == nil {
		return preferences.LoadResult{}, errors.New("memory preference store is nil")
	}
	if ctx == nil {
		return preferences.LoadResult{}, errors.New("memory preference store context is nil")
	}
	if err := ctx.Err(); err != nil {
		return preferences.LoadResult{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.present {
		return preferences.LoadResult{
			Snapshot: preferences.Snapshot{Preferences: appstate.DefaultPreferences()},
			Source:   preferences.LoadDefaultMissing,
		}, nil
	}
	return preferences.LoadResult{Snapshot: store.snapshot, Source: preferences.LoadSaved}, nil
}

// Save retains the newest valid preference revision.
func (store *MemoryPreferenceStore) Save(ctx context.Context, snapshot preferences.Snapshot) error {
	if store == nil {
		return errors.New("memory preference store is nil")
	}
	if ctx == nil {
		return errors.New("memory preference store context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := snapshot.Preferences.Validate(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.present && snapshot.Revision < store.snapshot.Revision {
		return preferences.ErrStaleRevision
	}
	if store.present && snapshot.Revision == store.snapshot.Revision && snapshot.Preferences != store.snapshot.Preferences {
		return preferences.ErrRevisionConflict
	}
	store.snapshot = snapshot
	store.present = true
	return nil
}

// MemoryLayoutStore is a deterministic, revision-aware in-memory layout store
// for tests and the demo workstation.
type MemoryLayoutStore struct {
	mu       sync.Mutex
	snapshot layouts.Snapshot
	saved    bool
}

// NewMemoryLayoutStore creates an empty in-memory layout store.
func NewMemoryLayoutStore() *MemoryLayoutStore {
	return &MemoryLayoutStore{}
}

// Reload returns the last immutable snapshot, or a missing-file result before
// the first save.
func (store *MemoryLayoutStore) Reload(ctx context.Context) (layouts.LoadResult, error) {
	if ctx == nil {
		return layouts.LoadResult{}, errors.New("reload memory layouts: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return layouts.LoadResult{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.saved {
		return layouts.LoadResult{Source: layouts.LoadDefaultMissing}, nil
	}
	return layouts.LoadResult{
		Snapshot: store.snapshot.Clone(),
		Source:   layouts.LoadSaved,
	}, nil
}

// Save validates and stores an immutable snapshot. Older revisions and
// different content at the same revision are rejected.
func (store *MemoryLayoutStore) Save(ctx context.Context, snapshot layouts.Snapshot) error {
	if ctx == nil {
		return errors.New("save memory layouts: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	encoded, err := layouts.Encode(snapshot)
	if err != nil {
		return fmt.Errorf("save memory layouts: %w", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if store.saved {
		if snapshot.Revision < store.snapshot.Revision {
			return fmt.Errorf(
				"%w: persisted %d, attempted %d",
				layouts.ErrStaleRevision,
				store.snapshot.Revision,
				snapshot.Revision,
			)
		}
		if snapshot.Revision == store.snapshot.Revision {
			current, encodeErr := layouts.Encode(store.snapshot)
			if encodeErr != nil {
				return fmt.Errorf("save memory layouts: encode current snapshot: %w", encodeErr)
			}
			if !bytes.Equal(current, encoded) {
				return fmt.Errorf("%w: revision %d", layouts.ErrRevisionConflict, snapshot.Revision)
			}
			return nil
		}
	}
	store.snapshot = snapshot.Clone()
	store.saved = true
	return nil
}

// SaveLayout preserves the Phase 2 single-layout helper while assigning a new
// revision and retaining all other stored workspaces.
func (store *MemoryLayoutStore) SaveLayout(ctx context.Context, layout appstate.WorkspaceLayout) error {
	loaded, err := store.Reload(ctx)
	if err != nil {
		return err
	}
	snapshot := loaded.Snapshot.Clone()
	if snapshot.Revision == math.MaxUint64 {
		return fmt.Errorf("save memory layout: %w at maximum revision", layouts.ErrRevisionConflict)
	}
	snapshot.Revision++
	replaced := false
	for index := range snapshot.Layouts {
		if snapshot.Layouts[index].Workspace == layout.Workspace {
			snapshot.Layouts[index] = layout.Clone()
			replaced = true
			break
		}
	}
	if !replaced {
		snapshot.Layouts = append(snapshot.Layouts, layout.Clone())
	}
	return store.Save(ctx, snapshot)
}

// Layouts returns immutable layout copies in insertion order.
func (store *MemoryLayoutStore) Layouts() []appstate.WorkspaceLayout {
	store.mu.Lock()
	defer store.mu.Unlock()
	return appstate.CloneWorkspaceLayouts(store.snapshot.Layouts)
}
