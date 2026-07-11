// Package preferences owns strict, versioned global frontend preference
// persistence. It performs no rendering and is called only by effect workers.
package preferences

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

const (
	SchemaVersion        = 1
	applicationDirectory = "Corthena"
	filename             = "preferences.json"
	maximumDocumentBytes = 1 << 20
)

var (
	ErrInvalidDocument  = errors.New("invalid preference document")
	ErrStaleRevision    = errors.New("stale preference revision")
	ErrRevisionConflict = errors.New("preference revision conflict")
	ErrDocumentTooLarge = errors.New("preference document too large")
)

// Snapshot is one immutable global preference revision.
type Snapshot struct {
	Revision    uint64
	Preferences appstate.Preferences
}

// LoadSource describes how a preference load resolved.
type LoadSource string

const (
	LoadSaved          LoadSource = "saved"
	LoadDefaultMissing LoadSource = "default_missing"
	LoadDefaultInvalid LoadSource = "default_invalid"
)

// LoadResult contains valid preferences and optional recovery metadata.
type LoadResult struct {
	Snapshot       Snapshot
	Source         LoadSource
	QuarantinePath string
	DiagnosticHash string
}

type documentWire struct {
	SchemaVersion int                    `json:"schema_version"`
	Revision      uint64                 `json:"revision"`
	UIScale       appstate.UIScalePreset `json:"ui_scale_percent"`
}

// Store persists one preference document at a configured path.
type Store struct {
	path     string
	defaults appstate.Preferences
}

// NewStore constructs a side-effect-free preference store.
func NewStore(path string, defaults appstate.Preferences) (*Store, error) {
	if path == "" {
		return nil, errors.New("new preference store: path is empty")
	}
	if err := defaults.Validate(); err != nil {
		return nil, fmt.Errorf("new preference store: invalid defaults: %w", err)
	}
	return &Store{path: filepath.Clean(path), defaults: defaults}, nil
}

// NewUserStore creates a store below the user's Corthena config directory.
func NewUserStore(defaults appstate.Preferences) (*Store, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve preference config directory: %w", err)
	}
	if directory == "" {
		return nil, errors.New("resolve preference config directory: path is empty")
	}
	return NewStore(filepath.Join(directory, applicationDirectory, filename), defaults)
}

// Path returns the configured document path without touching the filesystem.
func (store *Store) Path() string {
	if store == nil {
		return ""
	}
	return store.path
}

// Load returns saved preferences or a valid default fallback. Invalid bytes
// are preserved in a content-addressed quarantine file.
func (store *Store) Load(ctx context.Context) (LoadResult, error) {
	if err := store.ready(ctx); err != nil {
		return LoadResult{}, err
	}
	document, err := readFileBounded(ctx, store.path)
	if errors.Is(err, os.ErrNotExist) {
		return LoadResult{Snapshot: Snapshot{Preferences: store.defaults}, Source: LoadDefaultMissing}, nil
	}
	if errors.Is(err, ErrDocumentTooLarge) {
		quarantinePath, hash, quarantineErr := store.quarantineFile(ctx)
		if quarantineErr != nil {
			return LoadResult{}, quarantineErr
		}
		return LoadResult{
			Snapshot: Snapshot{Preferences: store.defaults}, Source: LoadDefaultInvalid,
			QuarantinePath: quarantinePath, DiagnosticHash: hash,
		}, nil
	}
	if err != nil {
		return LoadResult{}, fmt.Errorf("load preferences: %w", err)
	}
	snapshot, err := Decode(document)
	if err == nil {
		return LoadResult{Snapshot: snapshot, Source: LoadSaved}, nil
	}
	quarantinePath, hash, quarantineErr := store.quarantine(ctx, document)
	if quarantineErr != nil {
		return LoadResult{}, fmt.Errorf("load invalid preferences: %w; quarantine: %w", err, quarantineErr)
	}
	return LoadResult{
		Snapshot: Snapshot{Preferences: store.defaults}, Source: LoadDefaultInvalid,
		QuarantinePath: quarantinePath, DiagnosticHash: hash,
	}, nil
}

// Save atomically writes a validated preference revision.
func (store *Store) Save(ctx context.Context, snapshot Snapshot) error {
	if err := store.ready(ctx); err != nil {
		return err
	}
	document, err := Encode(snapshot)
	if err != nil {
		return err
	}
	existing, err := readFileBounded(ctx, store.path)
	if err == nil {
		current, decodeErr := Decode(existing)
		if decodeErr != nil {
			if _, _, quarantineErr := store.quarantine(ctx, existing); quarantineErr != nil {
				return quarantineErr
			}
		} else {
			if current.Revision > snapshot.Revision {
				return fmt.Errorf("%w: persisted %d, attempted %d", ErrStaleRevision, current.Revision, snapshot.Revision)
			}
			if current.Revision == snapshot.Revision {
				if current.Preferences != snapshot.Preferences {
					return fmt.Errorf("%w: revision %d", ErrRevisionConflict, snapshot.Revision)
				}
				return nil
			}
		}
	} else if errors.Is(err, ErrDocumentTooLarge) {
		if _, _, quarantineErr := store.quarantineFile(ctx); quarantineErr != nil {
			return quarantineErr
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing preferences: %w", err)
	}
	return atomicWrite(ctx, store.path, document)
}

// Encode validates and returns canonical preference JSON.
func Encode(snapshot Snapshot) ([]byte, error) {
	if err := snapshot.Preferences.Validate(); err != nil {
		return nil, fmt.Errorf("encode preferences: %w", err)
	}
	document, err := json.Marshal(documentWire{
		SchemaVersion: SchemaVersion,
		Revision:      snapshot.Revision,
		UIScale:       snapshot.Preferences.UIScale,
	})
	if err != nil {
		return nil, fmt.Errorf("encode preferences: %w", err)
	}
	return append(document, '\n'), nil
}

// Decode strictly validates one preference document.
func Decode(document []byte) (Snapshot, error) {
	if err := rejectDuplicateFields(document); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var wire documentWire
	if err := decoder.Decode(&wire); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode: %v", ErrInvalidDocument, err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	if wire.SchemaVersion != SchemaVersion {
		return Snapshot{}, fmt.Errorf("%w: schema version %d", ErrInvalidDocument, wire.SchemaVersion)
	}
	preferences := appstate.Preferences{UIScale: wire.UIScale}
	if err := preferences.Validate(); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	return Snapshot{Revision: wire.Revision, Preferences: preferences}, nil
}

func (store *Store) quarantine(ctx context.Context, document []byte) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(document)
	return store.quarantineDigest(ctx, digest)
}

func (store *Store) quarantineFile(ctx context.Context) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	file, err := os.Open(store.path)
	if err != nil {
		return "", "", err
	}
	hasher := sha256.New()
	buffer := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return "", "", err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			_, _ = hasher.Write(buffer[:read])
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			_ = file.Close()
			return "", "", readErr
		}
	}
	if err := file.Close(); err != nil {
		return "", "", err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return store.quarantineDigest(ctx, digest)
}

func (store *Store) quarantineDigest(ctx context.Context, digest [sha256.Size]byte) (string, string, error) {
	hash := hex.EncodeToString(digest[:])
	directory := filepath.Join(filepath.Dir(store.path), "quarantine")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", "", fmt.Errorf("create preference quarantine: %w", err)
	}
	target := filepath.Join(directory, "preferences-"+hash+".invalid.json")
	if _, err := os.Stat(target); err == nil {
		if err := os.Remove(store.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("remove duplicate invalid preferences: %w", err)
		}
		return target, hash, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	if err := os.Rename(store.path, target); err != nil {
		return "", "", fmt.Errorf("quarantine invalid preferences: %w", err)
	}
	return target, hash, nil
}

func (store *Store) ready(ctx context.Context) error {
	if store == nil {
		return errors.New("preference store is nil")
	}
	if ctx == nil {
		return errors.New("preference store context is nil")
	}
	return ctx.Err()
}

func atomicWrite(ctx context.Context, target string, document []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := Decode(document); err != nil {
		return fmt.Errorf("refuse unverified preference write: %w", err)
	}
	directory := filepath.Dir(target)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create preference directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".preferences.tmp-")
	if err != nil {
		return fmt.Errorf("create preference temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if err := writeContext(ctx, temporary, document); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush preferences: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close preferences: %w", err)
	}
	closed = true
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return fmt.Errorf("atomically replace preferences: %w", err)
	}
	return nil
}

func readFileBounded(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readContext(ctx, file, maximumDocumentBytes)
}

func readContext(ctx context.Context, reader io.Reader, maximum int) ([]byte, error) {
	var buffer bytes.Buffer
	chunk := make([]byte, 16*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		read, err := reader.Read(chunk)
		if read > 0 {
			if buffer.Len()+read > maximum {
				return nil, fmt.Errorf("%w: document exceeds %d bytes", ErrDocumentTooLarge, maximum)
			}
			_, _ = buffer.Write(chunk[:read])
		}
		if errors.Is(err, io.EOF) {
			return buffer.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
		if read == 0 {
			return nil, io.ErrNoProgress
		}
	}
}

func writeContext(ctx context.Context, writer io.Writer, document []byte) error {
	for offset := 0; offset < len(document); {
		if err := ctx.Err(); err != nil {
			return err
		}
		written, err := writer.Write(document[offset:])
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		offset += written
	}
	return ctx.Err()
}

func requireJSONEnd(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("trailing JSON value")
}

func rejectDuplicateFields(document []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	start, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '{' {
		return errors.New("preference document must be an object")
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		field, ok := token.(string)
		if !ok {
			return errors.New("preference field name is not a string")
		}
		if _, duplicate := seen[field]; duplicate {
			return fmt.Errorf("duplicate field %q", field)
		}
		seen[field] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}
