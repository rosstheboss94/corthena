// Package rayui exercises the approved Raylib and Raygui bindings.
package rayui

import (
	"errors"
	"fmt"
	"runtime"

	gui "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
	"golang.org/x/sys/windows"
)

// Verify opens a hidden Raylib window, draws one Raygui control, and closes the
// window while the calling goroutine remains locked to one Windows UI thread.
func Verify() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ownerThread := windows.GetCurrentThreadId()
	if ownerThread == 0 {
		return errors.New("identify Raylib UI thread: zero thread ID")
	}
	assertThread := func(operation string) error {
		if current := windows.GetCurrentThreadId(); current != ownerThread {
			return fmt.Errorf(
				"%s: thread changed from %d to %d",
				operation,
				ownerThread,
				current,
			)
		}
		return nil
	}

	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(320, 120, "Corthena Phase 0 compatibility")
	if err := assertThread("initialize Raylib window"); err != nil {
		rl.CloseWindow()
		return err
	}
	if !rl.IsWindowReady() {
		rl.CloseWindow()
		return errors.New("initialize Raylib window: window is not ready")
	}

	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	gui.Button(rl.NewRectangle(80, 40, 160, 32), "Compatibility")
	rl.EndDrawing()
	if err := assertThread("draw Raygui control"); err != nil {
		rl.CloseWindow()
		return err
	}

	rl.CloseWindow()
	return assertThread("close Raylib window")
}
