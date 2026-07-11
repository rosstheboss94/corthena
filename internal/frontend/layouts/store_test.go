package layouts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStoreIsFilesystemSideEffectFree(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "not-created", "layouts")
	store, err := NewStore(directory, testSnapshot(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	if store.Directory() != directory {
		t.Fatalf("directory = %q, want %q", store.Directory(), directory)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewStore touched filesystem: Stat error = %v", err)
	}
}

func TestStoreSaveReloadAndRevisionGuards(t *testing.T) {
	t.Parallel()

	defaults := testSnapshot(t, 0)
	store, err := NewStore(filepath.Join(t.TempDir(), "layouts"), defaults)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	missing, err := store.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if missing.Source != LoadDefaultMissing || missing.Snapshot.Revision != defaults.Revision {
		t.Fatalf("missing load = %+v", missing)
	}

	saved := testSnapshot(t, 5)
	if err := store.Save(ctx, saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadSaved || loaded.Snapshot.Revision != 5 {
		t.Fatalf("saved load = %+v", loaded)
	}

	stale := testSnapshot(t, 4)
	if err := store.Save(ctx, stale); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale save error = %v, want ErrStaleRevision", err)
	}

	conflict := saved.Clone()
	setFirstPanelTitle(t, &conflict, "Conflicting title")
	if err := store.Save(ctx, conflict); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("conflicting save error = %v, want ErrRevisionConflict", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	newer := testSnapshot(t, 6)
	if err := store.Save(canceled, newer); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled save error = %v, want context.Canceled", err)
	}
	loaded, err = store.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Snapshot.Revision != saved.Revision {
		t.Fatalf("canceled replacement left revision %d, want %d", loaded.Snapshot.Revision, saved.Revision)
	}
}

func TestStoreNamedLifecycleAndReset(t *testing.T) {
	t.Parallel()

	defaults := testSnapshot(t, 0)
	store, err := NewStore(filepath.Join(t.TempDir(), "layouts"), defaults)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snapshot := testSnapshot(t, 8)
	if err := store.SaveNamed(ctx, "Beta layout", snapshot); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveNamed(ctx, "Alpha/layout", snapshot); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveNamed(ctx, "alpha/layout", snapshot); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListNamed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Name != "Alpha/layout" || list[1].Name != "Beta layout" || list[2].Name != "alpha/layout" {
		t.Fatalf("named list = %+v", list)
	}
	if list[0].Revision != snapshot.Revision || len(list[0].Workspaces) != len(snapshot.Layouts) {
		t.Fatalf("named metadata = %+v", list[0])
	}
	loaded, err := store.LoadNamed(ctx, "Alpha/layout")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadSaved || loaded.Snapshot.Revision != 8 {
		t.Fatalf("named load = %+v", loaded)
	}
	if err := store.DeleteNamed(ctx, "Alpha/layout"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadNamed(ctx, "Alpha/layout"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted load error = %v, want ErrNotFound", err)
	}
	if err := store.DeleteNamed(ctx, "Alpha/layout"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete error = %v, want ErrNotFound", err)
	}

	if err := store.Save(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	reset, err := store.Reset(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reset.Source != LoadReset || reset.Snapshot.Revision != snapshot.Revision+1 {
		t.Fatalf("reset = %+v", reset)
	}
	reloaded, err := store.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Snapshot.Revision != reset.Snapshot.Revision {
		t.Fatalf("reloaded reset revision = %d, want %d", reloaded.Snapshot.Revision, reset.Snapshot.Revision)
	}
}

func TestReloadQuarantinesInvalidDocumentWithStableHash(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "layouts"), testSnapshot(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	invalid := []byte(`{"format":"corthena.layout-snapshot","payload":"damaged"}`)
	if err := os.MkdirAll(store.directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.autosavePath(), invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadDefaultInvalid {
		t.Fatalf("source = %q, want %q", loaded.Source, LoadDefaultInvalid)
	}
	digest := sha256.Sum256(invalid)
	wantHash := hex.EncodeToString(digest[:])
	if loaded.DiagnosticHash != wantHash {
		t.Fatalf("hash = %q, want %q", loaded.DiagnosticHash, wantHash)
	}
	preserved, err := os.ReadFile(loaded.QuarantinePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(preserved, invalid) {
		t.Fatal("quarantined bytes changed")
	}
	if _, err := os.Stat(store.autosavePath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid autosave remains: %v", err)
	}

	if err := os.WriteFile(store.autosavePath(), invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := store.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second.QuarantinePath != loaded.QuarantinePath || second.DiagnosticHash != loaded.DiagnosticHash {
		t.Fatalf("stable quarantine changed: first=%+v second=%+v", loaded, second)
	}
}

func TestReloadMigratesAndRewritesLegacyDocument(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "layouts"), testSnapshot(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	legacy := legacyPayload(t, testSnapshot(t, 11))
	if err := os.MkdirAll(store.directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.autosavePath(), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadMigrated || loaded.Snapshot.Revision != 11 {
		t.Fatalf("migrated load = %+v", loaded)
	}
	rewritten, err := os.ReadFile(store.autosavePath())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeDocument(rewritten)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Migrated {
		t.Fatal("legacy autosave was not rewritten to current schema")
	}
}

func TestReloadQuarantinesOversizedDocument(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "layouts"), testSnapshot(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.directory, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(store.autosavePath())
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maximumDocumentBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadDefaultInvalid || loaded.DiagnosticHash == "" {
		t.Fatalf("oversized load = %+v", loaded)
	}
	info, err := os.Stat(loaded.QuarantinePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != maximumDocumentBytes+1 {
		t.Fatalf("quarantine size = %d, want %d", info.Size(), maximumDocumentBytes+1)
	}
}

func TestImportExportTypedRoundTripAndCancellation(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t, 23)
	var output bytes.Buffer
	if err := Export(context.Background(), &output, snapshot); err != nil {
		t.Fatal(err)
	}
	imported, err := Import(context.Background(), bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if imported.Revision != snapshot.Revision {
		t.Fatalf("imported revision = %d, want %d", imported.Revision, snapshot.Revision)
	}

	tampered := append([]byte(nil), output.Bytes()...)
	tampered[len(tampered)/2] ^= 1
	if _, err := Import(context.Background(), bytes.NewReader(tampered)); err == nil {
		t.Fatal("Import accepted tampered document")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Export(canceled, &bytes.Buffer{}, snapshot); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Export error = %v", err)
	}
	if _, err := Import(canceled, bytes.NewReader(output.Bytes())); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Import error = %v", err)
	}
}

func TestUserConfigPathHelpers(t *testing.T) {
	t.Parallel()

	application, err := UserConfigDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(application) != applicationDirectoryName {
		t.Fatalf("application directory = %q", application)
	}
	layouts, err := DefaultDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if layouts != filepath.Join(application, layoutDirectoryName) {
		t.Fatalf("default directory = %q, want %q", layouts, filepath.Join(application, layoutDirectoryName))
	}
}

func setFirstPanelTitle(t testing.TB, snapshot *Snapshot, title string) {
	t.Helper()
	stack := requireTabStack(t, snapshot.Layouts[0].Root)
	stack.Panels[0].Title = title
	snapshot.Layouts[0].Root = stack
}
