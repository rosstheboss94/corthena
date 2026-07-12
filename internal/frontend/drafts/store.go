// Package drafts owns strict local experiment-draft autosave persistence.
package drafts

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
	filename             = "experiment-draft.json"
	maximumDocumentBytes = 1 << 20
)

var (
	ErrInvalidDocument  = errors.New("invalid experiment draft document")
	ErrStaleRevision    = errors.New("stale experiment draft revision")
	ErrRevisionConflict = errors.New("experiment draft revision conflict")
)

// Snapshot is one immutable local draft revision.
type Snapshot struct {
	Revision uint64
	Draft    appstate.ExperimentDraft
}

// LoadSource identifies saved, missing-default, or invalid-default recovery.
type LoadSource string

const (
	LoadSaved          LoadSource = "saved"
	LoadDefaultMissing LoadSource = "default_missing"
	LoadDefaultInvalid LoadSource = "default_invalid"
)

// LoadResult includes valid data and optional quarantine evidence.
type LoadResult struct {
	Snapshot       Snapshot
	Source         LoadSource
	QuarantinePath string
	DiagnosticHash string
}

type documentWire struct {
	SchemaVersion int                      `json:"schema_version"`
	Revision      uint64                   `json:"revision"`
	Draft         appstate.ExperimentDraft `json:"draft"`
}

// Store persists one versioned draft document.
type Store struct {
	path     string
	defaults appstate.ExperimentDraft
}

// NewStore constructs a side-effect-free store.
func NewStore(path string, defaults appstate.ExperimentDraft) (*Store, error) {
	if path == "" {
		return nil, errors.New("new experiment draft store: path is empty")
	}
	return &Store{path: filepath.Clean(path), defaults: defaults.Clone()}, nil
}

// NewUserStore creates a store in the user's Corthena config directory.
func NewUserStore(defaults appstate.ExperimentDraft) (*Store, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve experiment draft directory: %w", err)
	}
	return NewStore(filepath.Join(directory, applicationDirectory, filename), defaults)
}

// Load returns saved data or defaults and quarantines invalid bytes.
func (store *Store) Load(ctx context.Context) (LoadResult, error) {
	if err := store.ready(ctx); err != nil {
		return LoadResult{}, err
	}
	document, err := readBounded(ctx, store.path)
	if errors.Is(err, os.ErrNotExist) {
		return LoadResult{Snapshot: Snapshot{Revision: store.defaults.Revision, Draft: store.defaults.Clone()}, Source: LoadDefaultMissing}, nil
	}
	if err != nil {
		return LoadResult{}, fmt.Errorf("load experiment draft: %w", err)
	}
	snapshot, decodeErr := Decode(document)
	if decodeErr == nil {
		return LoadResult{Snapshot: snapshot, Source: LoadSaved}, nil
	}
	target, hash, quarantineErr := store.quarantine(ctx, document)
	if quarantineErr != nil {
		return LoadResult{}, fmt.Errorf("load invalid experiment draft: %w; quarantine: %w", decodeErr, quarantineErr)
	}
	return LoadResult{
		Snapshot: Snapshot{Revision: store.defaults.Revision, Draft: store.defaults.Clone()},
		Source:   LoadDefaultInvalid, QuarantinePath: target, DiagnosticHash: hash,
	}, nil
}

// Save atomically writes a newer or identical draft revision.
func (store *Store) Save(ctx context.Context, snapshot Snapshot) error {
	if err := store.ready(ctx); err != nil {
		return err
	}
	document, err := Encode(snapshot)
	if err != nil {
		return err
	}
	existing, readErr := readBounded(ctx, store.path)
	if readErr == nil {
		current, decodeErr := Decode(existing)
		if decodeErr != nil {
			if _, _, err := store.quarantine(ctx, existing); err != nil {
				return err
			}
		} else if current.Revision > snapshot.Revision {
			return fmt.Errorf("%w: persisted %d, attempted %d", ErrStaleRevision, current.Revision, snapshot.Revision)
		} else if current.Revision == snapshot.Revision {
			currentDocument, _ := Encode(current)
			if !bytes.Equal(currentDocument, document) {
				return fmt.Errorf("%w: revision %d", ErrRevisionConflict, snapshot.Revision)
			}
			return nil
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	return atomicWrite(ctx, store.path, document)
}

// Encode returns canonical strict JSON.
func Encode(snapshot Snapshot) ([]byte, error) {
	if snapshot.Revision == 0 || snapshot.Draft.Revision != snapshot.Revision {
		return nil, fmt.Errorf("%w: revision mismatch", ErrInvalidDocument)
	}
	document, err := json.Marshal(documentWire{SchemaVersion: SchemaVersion, Revision: snapshot.Revision, Draft: snapshot.Draft.Clone()})
	if err != nil {
		return nil, fmt.Errorf("encode experiment draft: %w", err)
	}
	return append(document, '\n'), nil
}

// Decode rejects unknown fields, trailing values, schema mismatch, and revision mismatch.
func Decode(document []byte) (Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var wire documentWire
	if err := decoder.Decode(&wire); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Snapshot{}, fmt.Errorf("%w: trailing JSON value", ErrInvalidDocument)
		}
		return Snapshot{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	if wire.SchemaVersion != SchemaVersion || wire.Revision == 0 || wire.Draft.Revision != wire.Revision {
		return Snapshot{}, fmt.Errorf("%w: schema or revision mismatch", ErrInvalidDocument)
	}
	return Snapshot{Revision: wire.Revision, Draft: wire.Draft.Clone()}, nil
}

func (store *Store) quarantine(ctx context.Context, document []byte) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(document)
	hash := hex.EncodeToString(digest[:])
	directory := filepath.Join(filepath.Dir(store.path), "quarantine")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", "", err
	}
	target := filepath.Join(directory, "experiment-draft-"+hash+".invalid.json")
	if _, err := os.Stat(target); err == nil {
		if err := os.Remove(store.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", err
		}
		return target, hash, nil
	}
	if err := os.Rename(store.path, target); err != nil {
		return "", "", err
	}
	return target, hash, nil
}

func (store *Store) ready(ctx context.Context) error {
	if store == nil {
		return errors.New("experiment draft store is nil")
	}
	if ctx == nil {
		return errors.New("experiment draft store context is nil")
	}
	return ctx.Err()
}

func readBounded(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maximumDocumentBytes {
		return nil, fmt.Errorf("%w: document too large", ErrInvalidDocument)
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return document, nil
}

func atomicWrite(ctx context.Context, target string, document []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	directory := filepath.Dir(target)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".experiment-draft.tmp-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(document); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}
