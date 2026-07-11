package effects_test

import (
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
	"github.com/rosstheboss94/corthena/internal/frontend/effects"
)

func TestRuntimeLoadsAndPersistsPreferences(t *testing.T) {
	t.Parallel()

	preferenceStore := effects.NewMemoryPreferenceStore()
	runtime, cleanup := startRuntimeWithStore(t, effects.NewMemoryLayoutStore(), effects.Config{PreferenceStore: preferenceStore})
	defer cleanup()
	if !runtime.Enqueue(appstate.LoadPreferencesEffect{
		ID: "load-preferences", Defaults: appstate.DefaultPreferences(),
	}) {
		t.Fatal("enqueue preference load returned false")
	}
	loaded := waitAction(t, runtime.Actions())
	loadAction, ok := loaded.(appstate.PreferencesLoadedAction)
	if !ok || loadAction.Preferences.UIScale != appstate.DefaultUIScale {
		t.Fatalf("load action = %#v", loaded)
	}

	if !runtime.Enqueue(appstate.PersistPreferencesEffect{
		ID: "save-preferences", Revision: 1,
		Preferences: appstate.Preferences{UIScale: appstate.UIScale175},
	}) {
		t.Fatal("enqueue preference save returned false")
	}
	saved := waitAction(t, runtime.Actions())
	saveAction, ok := saved.(appstate.PreferencesPersistedAction)
	if !ok || saveAction.Revision != 1 {
		t.Fatalf("save action = %#v", saved)
	}
	reloaded, err := preferenceStore.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Snapshot.Preferences.UIScale != appstate.UIScale175 {
		t.Fatalf("saved preferences = %+v", reloaded.Snapshot.Preferences)
	}
}
