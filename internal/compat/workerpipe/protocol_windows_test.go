package workerpipe

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func TestInheritedPipeHandshake(t *testing.T) {
	if IsHelperProcess() {
		t.Skip("parent-only compatibility test")
	}
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := StartHelper(ctx, executable, "TestWorkerHelperProcess", HelperHandshake)
	if err != nil {
		t.Fatal(err)
	}

	start := Start{
		JobID:           JobID("job-compatibility"),
		CapabilityToken: "one-time-token",
		LeaseSize:       2,
	}
	if err := session.SendStart(ctx, start); err != nil {
		t.Fatal(err)
	}
	if err := session.CloseCommands(); err != nil {
		t.Fatal(err)
	}
	ready, err := session.ReadReady(ctx, start.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.JobID != start.JobID ||
		ready.CapabilityToken != start.CapabilityToken ||
		ready.ProcessID <= 0 {
		t.Fatalf("unexpected ready message: %+v", ready)
	}
	if err := session.CloseEvents(); err != nil {
		t.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerHelperProcess(t *testing.T) {
	if !IsHelperProcess() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RunHelper(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPeerExitClosesEventPipe(t *testing.T) {
	if IsHelperProcess() {
		t.Skip("parent-only compatibility test")
	}
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := StartHelper(ctx, executable, "TestWorkerHelperProcess", HelperPeerExit)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := session.ReadReady(ctx, JobID("job-compatibility"))
	if readErr == nil || !errors.Is(readErr, io.EOF) {
		t.Fatalf("peer exit error = %v, want EOF", readErr)
	}
	if err := session.CloseCommands(); err != nil {
		t.Fatal(err)
	}
	if err := session.CloseEvents(); err != nil {
		t.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedOversizedAndProtocolMismatch(t *testing.T) {
	t.Parallel()

	t.Run("malformed JSON", func(t *testing.T) {
		frame := framed([]byte("{"))
		_, err := readEnvelopeBlocking(bytes.NewReader(frame))
		if !errors.Is(err, ErrMalformedFrame) {
			t.Fatalf("error = %v, want ErrMalformedFrame", err)
		}
	})

	t.Run("oversized", func(t *testing.T) {
		var prefix [4]byte
		binary.LittleEndian.PutUint32(prefix[:], maxFrameSize+1)
		_, err := readEnvelopeBlocking(bytes.NewReader(prefix[:]))
		if !errors.Is(err, ErrFrameTooLarge) {
			t.Fatalf("error = %v, want ErrFrameTooLarge", err)
		}
	})

	t.Run("protocol mismatch", func(t *testing.T) {
		payload, err := json.Marshal(startPayload{
			CapabilityToken: "token",
			LeaseSize:       1,
		})
		if err != nil {
			t.Fatal(err)
		}
		message := envelope{
			ProtocolVersion: protocolVersion + 1,
			JobID:           JobID("job"),
			Sequence:        1,
			Kind:            kindStart,
			Payload:         payload,
		}
		encoded, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		reader := io.NopCloser(bytes.NewReader(framed(encoded)))
		_, err = ReadStart(context.Background(), reader)
		if !errors.Is(err, ErrProtocolMismatch) {
			t.Fatalf("error = %v, want ErrProtocolMismatch", err)
		}
	})
}

func TestCancelledReadClosesPipe(t *testing.T) {
	t.Parallel()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ReadReady(ctx, reader, JobID("job"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestFrameClosure(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(readyPayload{
		CapabilityToken: "token",
		ProcessID:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(envelope{
		ProtocolVersion: protocolVersion,
		JobID:           JobID("job"),
		Sequence:        1,
		Kind:            kindReady,
		Payload:         payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(framed(encoded))
	if _, err := readEnvelopeBlocking(reader); err != nil {
		t.Fatal(err)
	}
	if _, err := readEnvelopeBlocking(reader); !errors.Is(err, io.EOF) {
		t.Fatalf("second read error = %v, want EOF", err)
	}
}

func framed(payload []byte) []byte {
	frame := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	return frame
}
