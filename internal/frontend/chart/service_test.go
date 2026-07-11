package chart

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type gatedLoader struct {
	calls    atomic.Int64
	started  chan Query
	release  chan struct{}
	canceled chan Query
}

func (loader *gatedLoader) Load(ctx context.Context, query Query) (Frame, error) {
	loader.calls.Add(1)
	select {
	case loader.started <- query:
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	}
	select {
	case <-loader.release:
		return testFrame(1), nil
	case <-ctx.Done():
		select {
		case loader.canceled <- query:
		default:
		}
		return Frame{}, ctx.Err()
	}
}

func TestServiceDeduplicatesEquivalentRequests(t *testing.T) {
	t.Parallel()
	loader := &gatedLoader{started: make(chan Query, 4), release: make(chan struct{}), canceled: make(chan Query, 4)}
	cache, _ := NewFrameCache(1 << 20)
	service, err := NewService(context.Background(), loader, cache, 2, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	query := testQuery("shared")
	if !service.Submit(Request{Scope: "a", Generation: 1, Query: query}) || !service.Submit(Request{Scope: "b", Generation: 1, Query: query}) {
		t.Fatal("submit rejected")
	}
	receiveQuery(t, loader.started)
	close(loader.release)
	first := receiveResult(t, service.Results())
	second := receiveResult(t, service.Results())
	if first.Scope != "a" || second.Scope != "b" || first.Error != nil || second.Error != nil || loader.calls.Load() != 1 {
		t.Fatalf("results = %+v %+v calls=%d", first, second, loader.calls.Load())
	}
}

func TestServiceCancelsOldGenerationAndDropsStaleCompletion(t *testing.T) {
	t.Parallel()
	loader := &gatedLoader{started: make(chan Query, 4), release: make(chan struct{}), canceled: make(chan Query, 4)}
	cache, _ := NewFrameCache(1 << 20)
	service, err := NewService(context.Background(), loader, cache, 2, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	oldQuery := testQuery("same")
	newQuery := oldQuery
	service.Submit(Request{Scope: "panel", Generation: 1, Query: oldQuery})
	receiveQuery(t, loader.started)
	service.Submit(Request{Scope: "panel", Generation: 2, Query: newQuery})
	startedNew := receiveQuery(t, loader.started)
	if startedNew != newQuery {
		t.Fatalf("started = %+v, want new query", startedNew)
	}
	canceled := receiveQuery(t, loader.canceled)
	if canceled != oldQuery {
		t.Fatalf("canceled = %+v, want old query", canceled)
	}
	close(loader.release)
	result := receiveResult(t, service.Results())
	if result.Generation != 2 || result.Query != newQuery || result.Error != nil {
		t.Fatalf("result = %+v", result)
	}
	select {
	case stale := <-service.Results():
		t.Fatalf("unexpected stale result: %+v", stale)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestServiceUsesCacheAndShutsDownWithoutLeak(t *testing.T) {
	t.Parallel()
	loader := &gatedLoader{started: make(chan Query, 2), release: make(chan struct{}), canceled: make(chan Query, 2)}
	close(loader.release)
	cache, _ := NewFrameCache(1 << 20)
	service, err := NewService(context.Background(), loader, cache, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	query := testQuery("cached")
	service.Submit(Request{Scope: "one", Generation: 1, Query: query})
	first := receiveResult(t, service.Results())
	if first.Error != nil {
		t.Fatal(first.Error)
	}
	service.Submit(Request{Scope: "two", Generation: 1, Query: query})
	second := receiveResult(t, service.Results())
	if !second.FromCache || loader.calls.Load() != 1 {
		t.Fatalf("cache result = %+v calls=%d", second, loader.calls.Load())
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if service.Submit(Request{Scope: "late", Generation: 1, Query: query}) {
		t.Fatal("submit succeeded after close")
	}
	if _, open := <-service.Results(); open {
		t.Fatal("result channel remains open after close")
	}
}

func TestServiceRejectsInvalidRequest(t *testing.T) {
	t.Parallel()
	loader := &gatedLoader{started: make(chan Query, 1), release: make(chan struct{}), canceled: make(chan Query, 1)}
	cache, _ := NewFrameCache(1 << 20)
	service, _ := NewService(context.Background(), loader, cache, 1, 2)
	defer service.Close()
	service.Submit(Request{})
	result := receiveResult(t, service.Results())
	if !errors.Is(result.Error, ErrInvalidData) {
		t.Fatalf("error = %v", result.Error)
	}
}

func receiveQuery(t *testing.T, channel <-chan Query) Query {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for query")
		return Query{}
	}
}

func receiveResult(t *testing.T, channel <-chan Result) Result {
	t.Helper()
	select {
	case value, open := <-channel:
		if !open {
			t.Fatal("result channel closed early")
		}
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
		return Result{}
	}
}
