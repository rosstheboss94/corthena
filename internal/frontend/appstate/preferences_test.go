package appstate

import (
	"errors"
	"testing"
	"time"
)

func TestUIScalePresetsAndStepping(t *testing.T) {
	t.Parallel()

	want := []UIScalePreset{UIScale100, UIScale125, UIScale150, UIScale175, UIScale200}
	got := UIScalePresets()
	if len(got) != len(want) {
		t.Fatalf("preset count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] || !got[index].Valid() {
			t.Fatalf("preset %d = %d, want %d", index, got[index], want[index])
		}
	}
	if StepUIScale(UIScale100, -1) != UIScale100 || StepUIScale(UIScale200, 1) != UIScale200 {
		t.Fatal("UI scale stepping did not clamp at endpoints")
	}
	if StepUIScale(UIScale125, 1) != UIScale150 || StepUIScale(UIScale125, -1) != UIScale100 {
		t.Fatal("UI scale stepping did not move by one preset")
	}
}

func TestPreferenceReducerPersistsAndRejectsLateLoad(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("preferences"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Preferences.UIScale != DefaultUIScale {
		t.Fatalf("initial scale = %d, want %d", state.Preferences.UIScale, DefaultUIScale)
	}
	next, effects, err := Reduce(state, SetUIScaleAction{Scale: UIScale150})
	if err != nil {
		t.Fatal(err)
	}
	if next.PreferenceRevision != 1 || next.PreferencePersistence.PendingRevision != 1 {
		t.Fatalf("preference revisions = %+v", next.PreferencePersistence)
	}
	if len(effects) != 1 {
		t.Fatalf("effect count = %d, want 1", len(effects))
	}
	persist, ok := effects[0].(PersistPreferencesEffect)
	if !ok || persist.Preferences.UIScale != UIScale150 || persist.Revision != 1 {
		t.Fatalf("persist effect = %#v", effects[0])
	}

	late, lateEffects, err := Reduce(next, PreferencesLoadedAction{
		EffectID: "late-load", BaseRevision: 0, Revision: 9,
		Preferences: Preferences{UIScale: UIScale100}, LoadedAt: fixedTime(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if late.Preferences.UIScale != UIScale150 || len(lateEffects) != 0 {
		t.Fatalf("late load changed preferences: %+v", late.Preferences)
	}
}

func TestPreferenceReducerOverlayAndFailureRules(t *testing.T) {
	t.Parallel()

	state, _, err := NewInitialState(FixedClock{Time: fixedTime()}, NewSequentialIDSource("preference-overlay"))
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = Reduce(state, SetSettingsOpenAction{Open: true})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Overlays.SettingsOpen || state.Overlays.CommandPaletteOpen {
		t.Fatalf("settings overlay = %+v", state.Overlays)
	}
	state, _, err = Reduce(state, SetCommandPaletteAction{Open: true})
	if err != nil {
		t.Fatal(err)
	}
	if state.Overlays.SettingsOpen || !state.Overlays.CommandPaletteOpen {
		t.Fatalf("palette overlay = %+v", state.Overlays)
	}

	if _, _, err := Reduce(state, SetUIScaleAction{Scale: 123}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("invalid scale error = %v", err)
	}
	state.PreferenceRevision = 2
	state.PreferencePersistence.PendingRevision = 2
	failedAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	state, _, err = Reduce(state, PreferencesPersistenceFailedAction{
		EffectID: "preference-failed", Revision: 2, FailedAt: failedAt,
		Error: ErrorSnapshot{Code: ErrorPersistence, Message: "save failed", Retryable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.PreferencePersistence.LastErrorRevision != 2 || len(state.Overlays.Toasts) != 1 {
		t.Fatalf("failed preference state = %+v", state.PreferencePersistence)
	}
}
