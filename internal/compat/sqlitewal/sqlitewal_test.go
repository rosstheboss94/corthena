package sqlitewal_test

import (
	"context"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/compat/sqlitewal"
)

func TestWALAndConcurrentReaders(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sqlitewal.Verify(ctx, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
