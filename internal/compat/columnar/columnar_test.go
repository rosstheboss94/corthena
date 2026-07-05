package columnar_test

import (
	"context"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/compat/columnar"
)

func TestRoundTrips(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := columnar.VerifyRoundTrips(ctx, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
