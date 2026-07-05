// Package workerpipe exercises the typed framed worker protocol over inherited
// Windows anonymous pipes.
package workerpipe

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	protocolVersion = 1
	maxFrameSize    = 64 * 1024
)

// Stable protocol errors used by callers to classify failed sessions.
var (
	ErrFrameTooLarge    = errors.New("worker frame too large")
	ErrMalformedFrame   = errors.New("malformed worker frame")
	ErrProtocolMismatch = errors.New("worker protocol mismatch")
	ErrWrongJob         = errors.New("worker message has wrong job ID")
	ErrWrongKind        = errors.New("worker message has wrong kind")
)

// JobID is an opaque worker job identifier.
type JobID string

type messageKind string

const (
	kindStart messageKind = "start"
	kindReady messageKind = "ready"
)

// Start is the typed coordinator-to-worker handshake.
type Start struct {
	JobID           JobID
	CapabilityToken string
	LeaseSize       int
}

// Ready is the typed worker-to-coordinator handshake.
type Ready struct {
	JobID           JobID
	CapabilityToken string
	ProcessID       int
}

type envelope struct {
	ProtocolVersion int             `json:"protocol_version"`
	JobID           JobID           `json:"job_id"`
	Sequence        uint64          `json:"sequence"`
	Kind            messageKind     `json:"kind"`
	Payload         json.RawMessage `json:"payload"`
}

type startPayload struct {
	CapabilityToken string `json:"capability_token"`
	LeaseSize       int    `json:"lease_size"`
}

type readyPayload struct {
	CapabilityToken string `json:"capability_token"`
	ProcessID       int    `json:"process_id"`
}

// WriteStart writes the first coordinator message. writer has one owner and is
// closed by this function only when ctx is cancelled to unblock a pending write.
func WriteStart(ctx context.Context, writer io.WriteCloser, start Start) error {
	if start.JobID == "" || start.CapabilityToken == "" || start.LeaseSize <= 0 {
		return errors.New("validate worker start: invalid field")
	}
	payload, err := json.Marshal(startPayload{
		CapabilityToken: start.CapabilityToken,
		LeaseSize:       start.LeaseSize,
	})
	if err != nil {
		return fmt.Errorf("encode worker start payload: %w", err)
	}
	return writeEnvelope(ctx, writer, envelope{
		ProtocolVersion: protocolVersion,
		JobID:           start.JobID,
		Sequence:        1,
		Kind:            kindStart,
		Payload:         payload,
	})
}

// ReadStart reads and validates the first coordinator message. reader has one
// owner and is closed by this function only when ctx is cancelled.
func ReadStart(ctx context.Context, reader io.ReadCloser) (Start, error) {
	message, err := readEnvelope(ctx, reader)
	if err != nil {
		return Start{}, err
	}
	if err := validateEnvelope(message, kindStart); err != nil {
		return Start{}, err
	}
	var payload startPayload
	if err := decodeJSON(message.Payload, &payload); err != nil {
		return Start{}, fmt.Errorf("%w: start payload: %w", ErrMalformedFrame, err)
	}
	if payload.CapabilityToken == "" || payload.LeaseSize <= 0 {
		return Start{}, fmt.Errorf("%w: invalid start payload", ErrMalformedFrame)
	}
	return Start{
		JobID:           message.JobID,
		CapabilityToken: payload.CapabilityToken,
		LeaseSize:       payload.LeaseSize,
	}, nil
}

// WriteReady writes the first worker message. writer has one owner and is
// closed by this function only when ctx is cancelled.
func WriteReady(ctx context.Context, writer io.WriteCloser, ready Ready) error {
	if ready.JobID == "" || ready.CapabilityToken == "" || ready.ProcessID <= 0 {
		return errors.New("validate worker ready: invalid field")
	}
	payload, err := json.Marshal(readyPayload{
		CapabilityToken: ready.CapabilityToken,
		ProcessID:       ready.ProcessID,
	})
	if err != nil {
		return fmt.Errorf("encode worker ready payload: %w", err)
	}
	return writeEnvelope(ctx, writer, envelope{
		ProtocolVersion: protocolVersion,
		JobID:           ready.JobID,
		Sequence:        1,
		Kind:            kindReady,
		Payload:         payload,
	})
}

// ReadReady reads and validates the first worker message. reader has one owner
// and is closed by this function only when ctx is cancelled.
func ReadReady(ctx context.Context, reader io.ReadCloser, wantJob JobID) (Ready, error) {
	message, err := readEnvelope(ctx, reader)
	if err != nil {
		return Ready{}, err
	}
	if err := validateEnvelope(message, kindReady); err != nil {
		return Ready{}, err
	}
	if message.JobID != wantJob {
		return Ready{}, fmt.Errorf("%w: got %q, want %q", ErrWrongJob, message.JobID, wantJob)
	}
	var payload readyPayload
	if err := decodeJSON(message.Payload, &payload); err != nil {
		return Ready{}, fmt.Errorf("%w: ready payload: %w", ErrMalformedFrame, err)
	}
	if payload.CapabilityToken == "" || payload.ProcessID <= 0 {
		return Ready{}, fmt.Errorf("%w: invalid ready payload", ErrMalformedFrame)
	}
	return Ready{
		JobID:           message.JobID,
		CapabilityToken: payload.CapabilityToken,
		ProcessID:       payload.ProcessID,
	}, nil
}

func validateEnvelope(message envelope, wantKind messageKind) error {
	if message.ProtocolVersion != protocolVersion {
		return fmt.Errorf(
			"%w: got %d, want %d",
			ErrProtocolMismatch,
			message.ProtocolVersion,
			protocolVersion,
		)
	}
	if message.JobID == "" || message.Sequence != 1 {
		return fmt.Errorf("%w: invalid envelope field", ErrMalformedFrame)
	}
	if message.Kind != wantKind {
		return fmt.Errorf("%w: got %q, want %q", ErrWrongKind, message.Kind, wantKind)
	}
	return nil
}

func writeEnvelope(ctx context.Context, writer io.WriteCloser, message envelope) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode worker envelope: %w", err)
	}
	if len(payload) > maxFrameSize {
		return ErrFrameTooLarge
	}
	frame := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)

	// The writer goroutine sends exactly one buffered result. The caller owns
	// the channel and closes the pipe on cancellation before joining it.
	result := make(chan error, 1)
	go func() {
		_, writeErr := writer.Write(frame)
		result <- writeErr
	}()
	select {
	case writeErr := <-result:
		if writeErr != nil {
			return fmt.Errorf("write worker frame: %w", writeErr)
		}
		return nil
	case <-ctx.Done():
		_ = writer.Close()
		<-result
		return fmt.Errorf("write worker frame: %w", ctx.Err())
	}
}

func readEnvelope(ctx context.Context, reader io.ReadCloser) (envelope, error) {
	type readResult struct {
		message envelope
		err     error
	}
	// The reader goroutine sends exactly one buffered result. The caller owns
	// the channel and closes the pipe on cancellation before joining it.
	result := make(chan readResult, 1)
	go func() {
		message, err := readEnvelopeBlocking(reader)
		result <- readResult{message: message, err: err}
	}()
	select {
	case read := <-result:
		return read.message, read.err
	case <-ctx.Done():
		_ = reader.Close()
		<-result
		return envelope{}, fmt.Errorf("read worker frame: %w", ctx.Err())
	}
}

func readEnvelopeBlocking(reader io.Reader) (envelope, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return envelope{}, fmt.Errorf("read worker frame length: %w", err)
	}
	length := binary.LittleEndian.Uint32(prefix[:])
	if length == 0 {
		return envelope{}, fmt.Errorf("%w: empty payload", ErrMalformedFrame)
	}
	if length > maxFrameSize {
		return envelope{}, fmt.Errorf("%w: got %d bytes", ErrFrameTooLarge, length)
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(reader, payload); err != nil {
		return envelope{}, fmt.Errorf("read worker frame payload: %w", err)
	}
	var message envelope
	if err := decodeJSON(payload, &message); err != nil {
		return envelope{}, fmt.Errorf("%w: envelope: %w", ErrMalformedFrame, err)
	}
	return message, nil
}

func decodeJSON(payload []byte, destination interface{ workerProtocolDestination() }) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("trailing JSON value")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func (*envelope) workerProtocolDestination()     {}
func (*startPayload) workerProtocolDestination() {}
func (*readyPayload) workerProtocolDestination() {}
