package filedialog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadDirectoryFiltersAndSortsEntries(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "z.CSV"), []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "a.csv"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "ignored.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	listing, err := ReadDirectory(context.Background(), Request{
		Directory:  directory,
		Extensions: []string{".csv"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(listing.Entries))
	}
	wantNames := [...]string{"folder", "a.csv", "z.CSV"}
	for index, want := range wantNames {
		if got := listing.Entries[index].Name; got != want {
			t.Fatalf("entry[%d] = %q, want %q", index, got, want)
		}
	}
	if listing.Entries[0].Kind != EntryDirectory {
		t.Fatalf("first entry kind = %d, want directory", listing.Entries[0].Kind)
	}
}

func TestReadDirectoryHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ReadDirectory(ctx, Request{Directory: t.TempDir()})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
