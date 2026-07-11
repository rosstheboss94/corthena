// Package filedialog provides the blocking filesystem boundary for the
// first-party file browser. Callers run it from an owned background goroutine.
package filedialog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// EntryKind classifies a file-browser entry.
type EntryKind uint8

const (
	// EntryDirectory identifies a directory.
	EntryDirectory EntryKind = iota + 1
	// EntryFile identifies a regular file.
	EntryFile
)

// Request describes one directory listing for a file dialog.
type Request struct {
	Directory  string
	Extensions []string
}

// Entry is one typed file-dialog row.
type Entry struct {
	Name     string
	Path     string
	Kind     EntryKind
	Size     int64
	Modified time.Time
}

// Listing is an immutable-by-ownership result returned to the caller.
type Listing struct {
	Directory string
	Entries   []Entry
}

// ReadDirectory performs one cancellable, deterministic directory read.
func ReadDirectory(ctx context.Context, request Request) (Listing, error) {
	if err := ctx.Err(); err != nil {
		return Listing{}, fmt.Errorf("read file-dialog directory: %w", err)
	}
	directory, extensions, err := validateRequest(request)
	if err != nil {
		return Listing{}, err
	}
	nativeEntries, err := os.ReadDir(directory)
	if err != nil {
		return Listing{}, fmt.Errorf("read file-dialog directory %q: %w", directory, err)
	}

	entries := make([]Entry, 0, len(nativeEntries))
	for _, nativeEntry := range nativeEntries {
		if err := ctx.Err(); err != nil {
			return Listing{}, fmt.Errorf("read file-dialog directory: %w", err)
		}
		if nativeEntry.Type()&os.ModeSymlink != 0 {
			continue
		}
		kind := EntryFile
		if nativeEntry.IsDir() {
			kind = EntryDirectory
		} else if !extensionAllowed(nativeEntry.Name(), extensions) {
			continue
		}
		info, err := nativeEntry.Info()
		if err != nil {
			return Listing{}, fmt.Errorf("inspect file-dialog entry %q: %w", nativeEntry.Name(), err)
		}
		entries = append(entries, Entry{
			Name:     nativeEntry.Name(),
			Path:     filepath.Join(directory, nativeEntry.Name()),
			Kind:     kind,
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}
	slices.SortFunc(entries, compareEntries)
	return Listing{Directory: directory, Entries: entries}, nil
}

func validateRequest(request Request) (string, []string, error) {
	if strings.TrimSpace(request.Directory) == "" {
		return "", nil, errors.New("validate file-dialog request: directory is empty")
	}
	directory, err := filepath.Abs(request.Directory)
	if err != nil {
		return "", nil, fmt.Errorf("validate file-dialog directory: %w", err)
	}
	extensions := make([]string, 0, len(request.Extensions))
	for _, extension := range request.Extensions {
		normalized := strings.ToLower(strings.TrimSpace(extension))
		if len(normalized) < 2 ||
			normalized[0] != '.' ||
			strings.ContainsAny(normalized, `/\*?`) {
			return "", nil, fmt.Errorf(
				"validate file-dialog extension %q: expected a suffix such as .csv",
				extension,
			)
		}
		extensions = append(extensions, normalized)
	}
	slices.Sort(extensions)
	extensions = slices.Compact(extensions)
	return filepath.Clean(directory), extensions, nil
}

func extensionAllowed(name string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}
	extension := strings.ToLower(filepath.Ext(name))
	_, found := slices.BinarySearch(extensions, extension)
	return found
}

func compareEntries(left Entry, right Entry) int {
	if left.Kind != right.Kind {
		if left.Kind == EntryDirectory {
			return -1
		}
		return 1
	}
	leftName := strings.ToLower(left.Name)
	rightName := strings.ToLower(right.Name)
	if result := strings.Compare(leftName, rightName); result != 0 {
		return result
	}
	return strings.Compare(left.Name, right.Name)
}
