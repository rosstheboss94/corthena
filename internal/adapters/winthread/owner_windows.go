// Package winthread isolates Windows thread identity behind a typed owner.
package winthread

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// ErrWrongThread identifies a call made from a thread other than the owner.
var ErrWrongThread = errors.New("wrong Windows thread")

// ID is an opaque Windows thread identifier.
type ID uint32

// Owner records the Windows thread that owns a native resource.
type Owner struct {
	id ID
}

// Capture records the calling Windows thread as an owner.
func Capture() (Owner, error) {
	id := ID(windows.GetCurrentThreadId())
	if id == 0 {
		return Owner{}, errors.New("capture Windows thread owner: zero thread ID")
	}
	return Owner{id: id}, nil
}

// Check verifies that operation is running on the recorded owner thread.
func (owner Owner) Check(operation string) error {
	current := ID(windows.GetCurrentThreadId())
	if current != owner.id {
		return fmt.Errorf(
			"%s: %w: owner %d, current %d",
			operation,
			ErrWrongThread,
			owner.id,
			current,
		)
	}
	return nil
}
