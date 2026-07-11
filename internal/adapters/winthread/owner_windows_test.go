package winthread

import (
	"errors"
	"runtime"
	"testing"
)

func TestOwnerAcceptsOwningThreadAndRejectsAnother(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	owner, err := Capture()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Check("owner check"); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		result <- owner.Check("other check")
	}()
	if err := <-result; !errors.Is(err, ErrWrongThread) {
		t.Fatalf("error = %v, want ErrWrongThread", err)
	}
}
