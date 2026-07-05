package loopback_test

import (
	"context"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/compat/loopback"
)

func TestHTTPWebSocketCancellationAndShutdown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := loopback.Verify(ctx); err != nil {
		t.Fatal(err)
	}
}
