package simulator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rosstheboss94/corthena/internal/frontend/appstate"
)

func TestDemoCoordinatorSnapshotIsDeterministic(t *testing.T) {
	t.Parallel()

	clock := appstate.FixedClock{Time: fixedTime()}
	first, err := NewDemoCoordinator(Options{Seed: 42, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	second, err := NewDemoCoordinator(Options{Seed: 42, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := second.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	firstSnapshot, err := first.Snapshot(context.Background(), appstate.SnapshotRequest{
		CorrelationID: "corr-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondSnapshot, err := second.Snapshot(context.Background(), appstate.SnapshotRequest{
		CorrelationID: "corr-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstSnapshot, secondSnapshot) {
		t.Fatalf("snapshots differ\nfirst: %#v\nsecond: %#v", firstSnapshot, secondSnapshot)
	}
}

func TestDemoCoordinatorSnapshotFailureIsTyped(t *testing.T) {
	t.Parallel()

	client, err := NewDemoCoordinator(Options{
		Seed:  7,
		Clock: appstate.FixedClock{Time: fixedTime()},
		Failures: FailureProfile{
			Snapshot: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	_, err = client.Snapshot(context.Background(), appstate.SnapshotRequest{
		CorrelationID: "corr-fail",
	})
	if err == nil {
		t.Fatal("Snapshot succeeded, want typed failure")
	}
	var demoErr DemoError
	if !errors.As(err, &demoErr) {
		t.Fatalf("error = %T, want DemoError", err)
	}
	if got := demoErr.FrontendError().CorrelationID; got != "corr-fail" {
		t.Fatalf("correlation ID = %q, want corr-fail", got)
	}
}

func TestDemoCoordinatorSubscribeFiltersAfterEvent(t *testing.T) {
	t.Parallel()

	client, err := NewDemoCoordinator(Options{
		Seed:  11,
		Clock: appstate.FixedClock{Time: fixedTime()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	messages, err := client.Subscribe(context.Background(), appstate.EventSubscription{
		Since: "evt-demo-job-0002",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, ok := <-messages
	if !ok {
		t.Fatal("event stream closed before first filtered event")
	}
	if got := messageEventID(first); got != "evt-demo-component-0003" {
		t.Fatalf("first event = %s, want evt-demo-component-0003", got)
	}
}

func TestDemoCoordinatorSubscribeCanInterrupt(t *testing.T) {
	t.Parallel()

	client, err := NewDemoCoordinator(Options{
		Seed:  11,
		Clock: appstate.FixedClock{Time: fixedTime()},
		Failures: FailureProfile{
			InterruptEvents:   true,
			EventsBeforeError: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	messages, err := client.Subscribe(context.Background(), appstate.EventSubscription{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := <-messages; !ok {
		t.Fatal("event stream closed before normal event")
	}
	second, ok := <-messages
	if !ok {
		t.Fatal("event stream closed before failure event")
	}
	if _, ok := second.(appstate.ClientFailureMessage); !ok {
		t.Fatalf("second event = %T, want ClientFailureMessage", second)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)
}
