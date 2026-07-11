package layouts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

const (
	applicationDirectoryName = "Corthena"
	layoutDirectoryName      = "layouts"
	autosaveFilename         = "autosave.json"
	maximumDocumentBytes     = 16 << 20
	maximumNameBytes         = 128
)

var (
	// ErrStaleRevision identifies a save older than the persisted snapshot.
	ErrStaleRevision = errors.New("stale layout revision")
	// ErrRevisionConflict identifies different snapshots with the same revision.
	ErrRevisionConflict = errors.New("layout revision conflict")
	// ErrInvalidName identifies an unsafe or unsupported named-layout name.
	ErrInvalidName = errors.New("invalid layout name")
	// ErrNotFound identifies an absent named layout.
	ErrNotFound = errors.New("layout not found")
	// ErrDocumentTooLarge identifies input above the bounded layout-document size.
	ErrDocumentTooLarge = errors.New("layout document too large")
)

// LoadSource describes how a snapshot load was resolved.
type LoadSource string

const (
	LoadSaved          LoadSource = "saved"
	LoadMigrated       LoadSource = "migrated"
	LoadDefaultMissing LoadSource = "default_missing"
	LoadDefaultInvalid LoadSource = "default_invalid"
	LoadReset          LoadSource = "reset"
)

// LoadResult includes recovery metadata without returning an invalid snapshot.
type LoadResult struct {
	Snapshot       Snapshot
	Source         LoadSource
	QuarantinePath string
	DiagnosticHash string
}

// NamedSnapshotInfo is stable metadata returned when listing named snapshots.
type NamedSnapshotInfo struct {
	Name       string
	Revision   uint64
	Workspaces []appstate.Workspace
}

// Store persists autosaved and named layout snapshots below one directory.
// Construction is side-effect free; filesystem work starts only in methods
// that accept a context.
type Store struct {
	directory string
	defaults  Snapshot
}

// NewStore constructs a store without reading or creating filesystem paths.
func NewStore(directory string, defaults Snapshot) (*Store, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("new layout store: directory is empty")
	}
	if err := Validate(defaults); err != nil {
		return nil, fmt.Errorf("new layout store: invalid defaults: %w", err)
	}
	return &Store{directory: filepath.Clean(directory), defaults: defaults.Clone()}, nil
}

// NewUserStore constructs a store rooted in the user's Corthena config path.
func NewUserStore(defaults Snapshot) (*Store, error) {
	directory, err := DefaultDirectory()
	if err != nil {
		return nil, err
	}
	return NewStore(directory, defaults)
}

// UserConfigDirectory returns the application directory below os.UserConfigDir.
func UserConfigDirectory() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	if directory == "" {
		return "", errors.New("resolve user config directory: path is empty")
	}
	return filepath.Join(directory, applicationDirectoryName), nil
}

// DefaultDirectory returns the default layout storage directory.
func DefaultDirectory() (string, error) {
	directory, err := UserConfigDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, layoutDirectoryName), nil
}

// Directory returns the configured storage directory without touching it.
func (store *Store) Directory() string {
	if store == nil {
		return ""
	}
	return store.directory
}

// Save atomically replaces the autosaved snapshot after revision checks.
func (store *Store) Save(ctx context.Context, snapshot Snapshot) error {
	if err := store.ready(ctx); err != nil {
		return err
	}
	return store.savePath(ctx, store.autosavePath(), snapshot)
}

// Reload loads the autosave, migrates legacy schema v1, or returns defaults.
// Invalid bytes are moved to a content-addressed quarantine before fallback.
func (store *Store) Reload(ctx context.Context) (LoadResult, error) {
	if err := store.ready(ctx); err != nil {
		return LoadResult{}, err
	}
	return store.loadPath(ctx, store.autosavePath(), true)
}

// SaveNamed atomically saves a named snapshot.
func (store *Store) SaveNamed(ctx context.Context, name string, snapshot Snapshot) error {
	if err := store.ready(ctx); err != nil {
		return err
	}
	path, err := store.namedPath(name)
	if err != nil {
		return err
	}
	return store.savePath(ctx, path, snapshot)
}

// LoadNamed loads and verifies a named snapshot. Invalid documents are
// quarantined and resolve to the configured defaults.
func (store *Store) LoadNamed(ctx context.Context, name string) (LoadResult, error) {
	if err := store.ready(ctx); err != nil {
		return LoadResult{}, err
	}
	path, err := store.namedPath(name)
	if err != nil {
		return LoadResult{}, err
	}
	return store.loadPath(ctx, path, false)
}

// ListNamed returns valid named snapshots ordered by name. Invalid documents
// are quarantined and omitted.
func (store *Store) ListNamed(ctx context.Context) ([]NamedSnapshotInfo, error) {
	if err := store.ready(ctx); err != nil {
		return nil, err
	}
	directory := filepath.Join(store.directory, "named")
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []NamedSnapshotInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list named layouts: %w", err)
	}
	result := make([]NamedSnapshotInfo, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name, err := decodeNamedFilename(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		loaded, err := store.loadPath(ctx, filepath.Join(directory, entry.Name()), false)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if loaded.Source == LoadDefaultInvalid {
			continue
		}
		workspaces := make([]appstate.Workspace, len(loaded.Snapshot.Layouts))
		for index, layout := range loaded.Snapshot.Layouts {
			workspaces[index] = layout.Workspace
		}
		result = append(result, NamedSnapshotInfo{Name: name, Revision: loaded.Snapshot.Revision, Workspaces: workspaces})
	}
	sort.Slice(result, func(first, second int) bool {
		return result[first].Name < result[second].Name
	})
	return result, nil
}

// DeleteNamed removes one named snapshot. Missing names return ErrNotFound.
func (store *Store) DeleteNamed(ctx context.Context, name string) error {
	if err := store.ready(ctx); err != nil {
		return err
	}
	path, err := store.namedPath(name)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	} else if err != nil {
		return fmt.Errorf("delete named layout %q: %w", name, err)
	}
	return nil
}

// Reset atomically replaces the autosave with defaults at a newer revision.
func (store *Store) Reset(ctx context.Context) (LoadResult, error) {
	if err := store.ready(ctx); err != nil {
		return LoadResult{}, err
	}
	path := store.autosavePath()
	reset := store.defaults.Clone()
	document, err := readFileContext(ctx, path)
	if err == nil {
		decoded, decodeErr := decodeDocument(document)
		if decodeErr != nil {
			quarantinePath, hash, quarantineErr := store.quarantine(ctx, path, document)
			if quarantineErr != nil {
				return LoadResult{}, quarantineErr
			}
			encoded, encodeErr := Encode(reset)
			if encodeErr != nil {
				return LoadResult{}, encodeErr
			}
			if err := atomicWriteDocument(ctx, path, encoded); err != nil {
				return LoadResult{}, err
			}
			return LoadResult{Snapshot: reset, Source: LoadReset, QuarantinePath: quarantinePath, DiagnosticHash: hash}, nil
		}
		if decoded.Snapshot.Revision == math.MaxUint64 {
			return LoadResult{}, fmt.Errorf("reset layout: %w at maximum revision", ErrRevisionConflict)
		}
		if reset.Revision <= decoded.Snapshot.Revision {
			reset.Revision = decoded.Snapshot.Revision + 1
		}
	} else if errors.Is(err, ErrDocumentTooLarge) {
		quarantinePath, hash, quarantineErr := store.quarantineFile(ctx, path)
		if quarantineErr != nil {
			return LoadResult{}, quarantineErr
		}
		encoded, encodeErr := Encode(reset)
		if encodeErr != nil {
			return LoadResult{}, encodeErr
		}
		if err := atomicWriteDocument(ctx, path, encoded); err != nil {
			return LoadResult{}, err
		}
		return LoadResult{Snapshot: reset, Source: LoadReset, QuarantinePath: quarantinePath, DiagnosticHash: hash}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return LoadResult{}, fmt.Errorf("reset layout: %w", err)
	}
	encoded, err := Encode(reset)
	if err != nil {
		return LoadResult{}, err
	}
	if err := atomicWriteDocument(ctx, path, encoded); err != nil {
		return LoadResult{}, err
	}
	return LoadResult{Snapshot: reset, Source: LoadReset}, nil
}

// Export writes a canonical checksummed snapshot to a caller-owned writer.
func Export(ctx context.Context, writer io.Writer, snapshot Snapshot) error {
	if ctx == nil {
		return errors.New("export layout: context is nil")
	}
	if writer == nil {
		return errors.New("export layout: writer is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	document, err := Encode(snapshot)
	if err != nil {
		return err
	}
	if err := writeContext(ctx, writer, document); err != nil {
		return fmt.Errorf("export layout: %w", err)
	}
	return nil
}

// Import reads, bounds, verifies, migrates, and validates a layout document.
func Import(ctx context.Context, reader io.Reader) (Snapshot, error) {
	if ctx == nil {
		return Snapshot{}, errors.New("import layout: context is nil")
	}
	if reader == nil {
		return Snapshot{}, errors.New("import layout: reader is nil")
	}
	document, err := readContext(ctx, reader, maximumDocumentBytes)
	if err != nil {
		return Snapshot{}, fmt.Errorf("import layout: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	snapshot, err := Decode(document)
	if err != nil {
		return Snapshot{}, fmt.Errorf("import layout: %w", err)
	}
	return snapshot, nil
}

func (store *Store) savePath(ctx context.Context, path string, snapshot Snapshot) error {
	document, err := Encode(snapshot)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	existing, err := readFileContext(ctx, path)
	if err == nil {
		decoded, decodeErr := decodeDocument(existing)
		if decodeErr != nil {
			if _, _, quarantineErr := store.quarantine(ctx, path, existing); quarantineErr != nil {
				return quarantineErr
			}
		} else {
			if decoded.Snapshot.Revision > snapshot.Revision {
				return fmt.Errorf("%w: persisted %d, attempted %d", ErrStaleRevision, decoded.Snapshot.Revision, snapshot.Revision)
			}
			if decoded.Snapshot.Revision == snapshot.Revision {
				canonicalExisting, encodeErr := Encode(decoded.Snapshot)
				if encodeErr != nil {
					return encodeErr
				}
				if !bytes.Equal(canonicalExisting, document) {
					return fmt.Errorf("%w: revision %d has different content", ErrRevisionConflict, snapshot.Revision)
				}
				if !decoded.Migrated {
					return nil
				}
			}
		}
	} else if errors.Is(err, ErrDocumentTooLarge) {
		if _, _, quarantineErr := store.quarantineFile(ctx, path); quarantineErr != nil {
			return quarantineErr
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing layout: %w", err)
	}
	return atomicWriteDocument(ctx, path, document)
}

func (store *Store) loadPath(ctx context.Context, path string, fallbackMissing bool) (LoadResult, error) {
	document, err := readFileContext(ctx, path)
	if errors.Is(err, os.ErrNotExist) {
		if fallbackMissing {
			return LoadResult{Snapshot: store.defaults.Clone(), Source: LoadDefaultMissing}, nil
		}
		return LoadResult{}, fmt.Errorf("%w: %s", ErrNotFound, filepath.Base(path))
	}
	if errors.Is(err, ErrDocumentTooLarge) {
		quarantinePath, hash, quarantineErr := store.quarantineFile(ctx, path)
		if quarantineErr != nil {
			return LoadResult{}, fmt.Errorf("load oversized layout: quarantine: %w", quarantineErr)
		}
		return LoadResult{
			Snapshot:       store.defaults.Clone(),
			Source:         LoadDefaultInvalid,
			QuarantinePath: quarantinePath,
			DiagnosticHash: hash,
		}, nil
	}
	if err != nil {
		return LoadResult{}, fmt.Errorf("load layout %q: %w", path, err)
	}
	if err := ctx.Err(); err != nil {
		return LoadResult{}, err
	}
	decoded, err := decodeDocument(document)
	if err != nil {
		quarantinePath, hash, quarantineErr := store.quarantine(ctx, path, document)
		if quarantineErr != nil {
			return LoadResult{}, fmt.Errorf("load invalid layout: %w; quarantine: %w", err, quarantineErr)
		}
		return LoadResult{
			Snapshot:       store.defaults.Clone(),
			Source:         LoadDefaultInvalid,
			QuarantinePath: quarantinePath,
			DiagnosticHash: hash,
		}, nil
	}
	if decoded.Migrated {
		current, encodeErr := Encode(decoded.Snapshot)
		if encodeErr != nil {
			return LoadResult{}, encodeErr
		}
		if err := atomicWriteDocument(ctx, path, current); err != nil {
			return LoadResult{}, fmt.Errorf("rewrite migrated layout: %w", err)
		}
		return LoadResult{Snapshot: decoded.Snapshot, Source: LoadMigrated}, nil
	}
	return LoadResult{Snapshot: decoded.Snapshot, Source: LoadSaved}, nil
}

func (store *Store) quarantine(ctx context.Context, source string, document []byte) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(document)
	return store.quarantineDigest(ctx, source, digest)
}

func (store *Store) quarantineFile(ctx context.Context, source string) (string, string, error) {
	digest, err := fileDigestContext(ctx, source)
	if err != nil {
		return "", "", fmt.Errorf("checksum invalid layout: %w", err)
	}
	return store.quarantineDigest(ctx, source, digest)
}

func (store *Store) quarantineDigest(ctx context.Context, source string, digest [sha256.Size]byte) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	hash := hex.EncodeToString(digest[:])
	kind := "named"
	if filepath.Clean(source) == filepath.Clean(store.autosavePath()) {
		kind = "autosave"
	}
	directory := filepath.Join(store.directory, "quarantine")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", "", fmt.Errorf("create layout quarantine: %w", err)
	}
	target := filepath.Join(directory, kind+"-"+hash+".invalid.json")
	if existingDigest, err := fileDigestContext(ctx, target); err == nil {
		if existingDigest != digest {
			return "", "", errors.New("quarantine hash collision")
		}
		if err := os.Remove(source); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("remove duplicate invalid layout: %w", err)
		}
		return target, hash, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect layout quarantine: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if err := os.Rename(source, target); err != nil {
		return "", "", fmt.Errorf("quarantine invalid layout: %w", err)
	}
	return target, hash, nil
}

func fileDigestContext(ctx context.Context, path string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if err := ctx.Err(); err != nil {
		return digest, err
	}
	file, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer file.Close()
	hasher := sha256.New()
	buffer := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return digest, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			_, _ = hasher.Write(buffer[:read])
		}
		if errors.Is(readErr, io.EOF) {
			copy(digest[:], hasher.Sum(nil))
			return digest, nil
		}
		if readErr != nil {
			return digest, readErr
		}
		if read == 0 {
			return digest, io.ErrNoProgress
		}
	}
}

func (store *Store) ready(ctx context.Context) error {
	if store == nil {
		return errors.New("layout store is nil")
	}
	if ctx == nil {
		return errors.New("layout store context is nil")
	}
	return ctx.Err()
}

func (store *Store) autosavePath() string {
	return filepath.Join(store.directory, autosaveFilename)
}

func (store *Store) namedPath(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	encoded := hex.EncodeToString([]byte(name))
	return filepath.Join(store.directory, "named", encoded+".json"), nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("%w: surrounding whitespace", ErrInvalidName)
	}
	if len(name) > maximumNameBytes {
		return fmt.Errorf("%w: name exceeds %d bytes", ErrInvalidName, maximumNameBytes)
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("%w: name is not valid UTF-8", ErrInvalidName)
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return fmt.Errorf("%w: name contains a control character", ErrInvalidName)
		}
	}
	return nil
}

func decodeNamedFilename(encoded string) (string, error) {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	name := string(decoded)
	if err := validateName(name); err != nil {
		return "", err
	}
	return name, nil
}

func atomicWriteDocument(ctx context.Context, target string, document []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := Decode(document); err != nil {
		return fmt.Errorf("refuse atomic write of unverified layout: %w", err)
	}
	directory := filepath.Dir(target)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create layout directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(target)+".tmp-")
	if err != nil {
		return fmt.Errorf("create sibling layout temporary file: %w", err)
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
		return fmt.Errorf("set layout temporary permissions: %w", err)
	}
	if err := writeContext(ctx, temporary, document); err != nil {
		return fmt.Errorf("write layout temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush layout temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close layout temporary file: %w", err)
	}
	closed = true
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return fmt.Errorf("atomically replace layout: %w", err)
	}
	return nil
}

func readFileContext(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	document, err := readContext(ctx, file, maximumDocumentBytes)
	if err != nil {
		return nil, err
	}
	return document, nil
}

func readContext(ctx context.Context, reader io.Reader, maximum int) ([]byte, error) {
	buffer := bytes.NewBuffer(make([]byte, 0, 32*1024))
	chunk := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		read, err := reader.Read(chunk)
		if read > 0 {
			if buffer.Len()+read > maximum {
				return nil, ErrDocumentTooLarge
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
	const chunkSize = 32 * 1024
	for offset := 0; offset < len(document); {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := offset + chunkSize
		if end > len(document) {
			end = len(document)
		}
		written, err := writer.Write(document[offset:end])
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
