package effects

import (
	"testing"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestPreferenceTaskQueueCoalescesLatestRevision(t *testing.T) {
	t.Parallel()

	queue := newPreferenceTaskQueue(4)
	load := appstate.LoadPreferencesEffect{ID: "load", Defaults: appstate.DefaultPreferences()}
	if accepted, _ := queue.enqueue(load); !accepted {
		t.Fatal("load was rejected")
	}
	for revision, scale := range []appstate.UIScalePreset{appstate.UIScale125, appstate.UIScale150, appstate.UIScale175} {
		accepted, _ := queue.enqueue(appstate.PersistPreferencesEffect{
			ID: appstate.EffectID("save"), Revision: uint64(revision + 1),
			Preferences: appstate.Preferences{UIScale: scale},
		})
		if !accepted {
			t.Fatalf("revision %d was rejected", revision+1)
		}
	}
	first, ok := queue.dequeue()
	if !ok || first.kind != preferenceTaskLoad {
		t.Fatalf("first task = %+v", first)
	}
	second, ok := queue.dequeue()
	if !ok || second.kind != preferenceTaskSave || second.save.Revision != 3 || second.save.Preferences.UIScale != appstate.UIScale175 {
		t.Fatalf("coalesced task = %+v", second)
	}
	if _, ok := queue.dequeue(); ok {
		t.Fatal("queue retained an obsolete preference save")
	}
}
