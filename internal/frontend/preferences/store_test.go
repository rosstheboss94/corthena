package preferences

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestStoreRoundTripRevisionAndConstruction(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "preferences.json")
	store, err := NewStore(path, appstate.DefaultPreferences())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewStore touched filesystem: %v", err)
	}
	missing, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if missing.Source != LoadDefaultMissing || missing.Snapshot.Preferences.UIScale != appstate.UIScale125 {
		t.Fatalf("missing result = %+v", missing)
	}

	snapshot := Snapshot{Revision: 4, Preferences: appstate.Preferences{UIScale: appstate.UIScale175}}
	if err := store.Save(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadSaved || loaded.Snapshot != snapshot {
		t.Fatalf("loaded = %+v, want %+v", loaded, snapshot)
	}
	if err := store.Save(context.Background(), Snapshot{Revision: 3, Preferences: snapshot.Preferences}); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale save error = %v", err)
	}
	if err := store.Save(context.Background(), Snapshot{Revision: 4, Preferences: appstate.Preferences{UIScale: appstate.UIScale150}}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("conflicting save error = %v", err)
	}
}

func TestStoreQuarantinesInvalidDocumentAndFallsBack(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "preferences.json")
	store, err := NewStore(path, appstate.DefaultPreferences())
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{"schema_version":1,"revision":2,"ui_scale_percent":123}`)
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadDefaultInvalid || loaded.Snapshot.Preferences.UIScale != appstate.DefaultUIScale {
		t.Fatalf("fallback = %+v", loaded)
	}
	if loaded.QuarantinePath == "" || loaded.DiagnosticHash == "" {
		t.Fatalf("quarantine metadata = %+v", loaded)
	}
	if _, err := os.Stat(loaded.QuarantinePath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid source remains: %v", err)
	}
}

func TestDecodeStrictnessAndCancellation(t *testing.T) {
	t.Parallel()

	for _, document := range [][]byte{
		[]byte(`{"schema_version":1,"revision":0,"ui_scale_percent":125,"unknown":true}`),
		[]byte(`{"schema_version":2,"revision":0,"ui_scale_percent":125}`),
		[]byte(`{"schema_version":1,"revision":0,"ui_scale_percent":125} {}`),
		[]byte(`{"schema_version":1,"revision":0,"ui_scale_percent":125,"ui_scale_percent":150}`),
	} {
		if _, err := Decode(document); !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("Decode(%s) error = %v", document, err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	store, err := NewStore(filepath.Join(t.TempDir(), "preferences.json"), appstate.DefaultPreferences())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled load error = %v", err)
	}
	if err := store.Save(canceled, Snapshot{Preferences: appstate.DefaultPreferences()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled save error = %v", err)
	}
}

func TestStoreQuarantinesOversizedDocument(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "preferences.json")
	store, err := NewStore(path, appstate.DefaultPreferences())
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
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
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != LoadDefaultInvalid || loaded.QuarantinePath == "" {
		t.Fatalf("oversized fallback = %+v", loaded)
	}
}
