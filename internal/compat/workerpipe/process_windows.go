package workerpipe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	helperMarkerEnvironment = "CORTHENA_WORKER_PIPE_HELPER"
	helperModeEnvironment   = "CORTHENA_WORKER_PIPE_MODE"
	helperReadEnvironment   = "CORTHENA_WORKER_PIPE_READ_HANDLE"
	helperWriteEnvironment  = "CORTHENA_WORKER_PIPE_WRITE_HANDLE"

	// HelperHandshake tells the child helper to complete a typed handshake.
	HelperHandshake = "handshake"
	// HelperPeerExit tells the child helper to exit before sending an event.
	HelperPeerExit = "peer-exit"
)

// Session owns the parent ends of one coordinator-to-worker command pipe and
// one worker-to-coordinator event pipe, plus the child process.
type Session struct {
	commandWriter *os.File
	eventReader   *os.File
	command       *exec.Cmd
}

// IsHelperProcess reports whether the current process was launched by
// StartHelper.
func IsHelperProcess() bool {
	return os.Getenv(helperMarkerEnvironment) == "1"
}

// StartHelper starts a hidden copy of a Go test executable with inherited
// anonymous pipe handles. The child is restricted to the named test.
func StartHelper(
	ctx context.Context,
	executable string,
	testName string,
	mode string,
) (*Session, error) {
	if mode != HelperHandshake && mode != HelperPeerExit {
		return nil, fmt.Errorf("start worker helper: unsupported mode %q", mode)
	}
	workerRead, parentWrite, err := createAnonymousPipe(false, true)
	if err != nil {
		return nil, err
	}
	parentRead, workerWrite, err := createAnonymousPipe(true, false)
	if err != nil {
		_ = windows.CloseHandle(workerRead)
		_ = windows.CloseHandle(parentWrite)
		return nil, err
	}

	commandWriter := os.NewFile(uintptr(parentWrite), "coordinator-command-writer")
	eventReader := os.NewFile(uintptr(parentRead), "coordinator-event-reader")
	if commandWriter == nil || eventReader == nil {
		closeFiles(commandWriter, eventReader)
		_ = windows.CloseHandle(workerRead)
		_ = windows.CloseHandle(workerWrite)
		return nil, errors.New("start worker helper: wrap parent pipe handles")
	}

	command := exec.CommandContext(ctx, executable, "-test.run=^"+testName+"$")
	command.Env = append(
		os.Environ(),
		helperMarkerEnvironment+"=1",
		helperModeEnvironment+"="+mode,
		helperReadEnvironment+"="+strconv.FormatUint(uint64(workerRead), 10),
		helperWriteEnvironment+"="+strconv.FormatUint(uint64(workerWrite), 10),
	)
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		AdditionalInheritedHandles: []syscall.Handle{
			syscall.Handle(workerRead),
			syscall.Handle(workerWrite),
		},
	}
	if err := command.Start(); err != nil {
		closeFiles(commandWriter, eventReader)
		_ = windows.CloseHandle(workerRead)
		_ = windows.CloseHandle(workerWrite)
		return nil, fmt.Errorf("start worker helper: %w", err)
	}

	// The parent must close its copies of the child-only handles immediately
	// after process creation so EOF reflects actual peer closure.
	workerReadCloseErr := windows.CloseHandle(workerRead)
	workerWriteCloseErr := windows.CloseHandle(workerWrite)
	if closeErr := errors.Join(workerReadCloseErr, workerWriteCloseErr); closeErr != nil {
		_ = commandWriter.Close()
		_ = eventReader.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("close child-only pipe handles: %w", closeErr)
	}
	return &Session{
		commandWriter: commandWriter,
		eventReader:   eventReader,
		command:       command,
	}, nil
}

// SendStart sends the typed handshake through the command pipe.
func (session *Session) SendStart(ctx context.Context, start Start) error {
	return WriteStart(ctx, session.commandWriter, start)
}

// ReadReady reads the typed handshake through the event pipe.
func (session *Session) ReadReady(ctx context.Context, jobID JobID) (Ready, error) {
	return ReadReady(ctx, session.eventReader, jobID)
}

// CloseCommands closes the parent command writer, signaling command EOF.
func (session *Session) CloseCommands() error {
	if err := session.commandWriter.Close(); err != nil {
		return fmt.Errorf("close worker command pipe: %w", err)
	}
	return nil
}

// CloseEvents closes the parent event reader.
func (session *Session) CloseEvents() error {
	if err := session.eventReader.Close(); err != nil {
		return fmt.Errorf("close worker event pipe: %w", err)
	}
	return nil
}

// Wait waits for the child process to terminate.
func (session *Session) Wait() error {
	if err := session.command.Wait(); err != nil {
		return fmt.Errorf("wait for worker helper: %w", err)
	}
	return nil
}

// RunHelper opens the inherited handles and executes the selected child path.
func RunHelper(ctx context.Context) error {
	input, output, err := openInheritedPipes()
	if err != nil {
		return err
	}
	defer input.Close()
	defer output.Close()

	switch mode := os.Getenv(helperModeEnvironment); mode {
	case HelperHandshake:
		start, err := ReadStart(ctx, input)
		if err != nil {
			return err
		}
		return WriteReady(ctx, output, Ready{
			JobID:           start.JobID,
			CapabilityToken: start.CapabilityToken,
			ProcessID:       os.Getpid(),
		})
	case HelperPeerExit:
		return nil
	default:
		return fmt.Errorf("run worker helper: unsupported mode %q", mode)
	}
}

func createAnonymousPipe(parentReads bool, parentWrites bool) (windows.Handle, windows.Handle, error) {
	security := windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
	var readHandle windows.Handle
	var writeHandle windows.Handle
	if err := windows.CreatePipe(&readHandle, &writeHandle, &security, 0); err != nil {
		return 0, 0, fmt.Errorf("create Windows anonymous pipe: %w", err)
	}
	if parentReads {
		if err := windows.SetHandleInformation(
			readHandle,
			windows.HANDLE_FLAG_INHERIT,
			0,
		); err != nil {
			_ = windows.CloseHandle(readHandle)
			_ = windows.CloseHandle(writeHandle)
			return 0, 0, fmt.Errorf("make parent read handle non-inheritable: %w", err)
		}
	}
	if parentWrites {
		if err := windows.SetHandleInformation(
			writeHandle,
			windows.HANDLE_FLAG_INHERIT,
			0,
		); err != nil {
			_ = windows.CloseHandle(readHandle)
			_ = windows.CloseHandle(writeHandle)
			return 0, 0, fmt.Errorf("make parent write handle non-inheritable: %w", err)
		}
	}
	return readHandle, writeHandle, nil
}

func openInheritedPipes() (*os.File, *os.File, error) {
	readHandle, err := inheritedHandle(helperReadEnvironment)
	if err != nil {
		return nil, nil, err
	}
	writeHandle, err := inheritedHandle(helperWriteEnvironment)
	if err != nil {
		return nil, nil, err
	}
	input := os.NewFile(uintptr(readHandle), "worker-command-reader")
	output := os.NewFile(uintptr(writeHandle), "worker-event-writer")
	if input == nil || output == nil {
		closeFiles(input, output)
		return nil, nil, errors.New("open inherited worker pipes: wrap handles")
	}
	return input, output, nil
}

func inheritedHandle(name string) (windows.Handle, error) {
	value := os.Getenv(name)
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("open inherited worker pipe %s: invalid handle", name)
	}
	return windows.Handle(parsed), nil
}

func closeFiles(files ...*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}
