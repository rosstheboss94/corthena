package rayui_test

import (
	"testing"

	"github.com/rosstheboss94/corthena/internal/compat/rayui"
)

func TestLockedThreadWindowAndControl(t *testing.T) {
	if err := rayui.Verify(); err != nil {
		t.Fatal(err)
	}
}
