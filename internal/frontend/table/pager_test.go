package table

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type gatedPageLoader struct {
	calls    atomic.Int64
	started  chan PageRequest
	release  chan struct{}
	canceled chan PageRequest
}

func (loader *gatedPageLoader) LoadPage(ctx context.Context, request PageRequest) (Page, error) {
	loader.calls.Add(1)
	select {
	case loader.started <- request.Clone():
	case <-ctx.Done():
		return Page{}, ctx.Err()
	}
	select {
	case <-loader.release:
		return Page{Rows: []Row{{ID: RowID(request.Cursor + "-row"), SourceIndex: 1}}, NextCursor: request.Cursor + "-next", TotalRows: 10}, nil
	case <-ctx.Done():
		select {
		case loader.canceled <- request.Clone():
		default:
		}
		return Page{}, ctx.Err()
	}
}

func TestPagerDeduplicatesAndPublishesInStableScopeOrder(t *testing.T) {
	t.Parallel()
	loader := &gatedPageLoader{started: make(chan PageRequest, 4), release: make(chan struct{}), canceled: make(chan PageRequest, 4)}
	pager, err := NewPager(context.Background(), loader, 2, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer pager.Close()
	base := PageRequest{Generation: 1, Cursor: "cursor", Limit: 50, Sort: []SortSpec{{Column: "id", Direction: SortAscending, Nulls: NullsLast}}}
	first := base.Clone()
	first.Scope = "b"
	second := base.Clone()
	second.Scope = "a"
	pager.Submit(first)
	pager.Submit(second)
	receivePageRequest(t, loader.started)
	close(loader.release)
	resultA := receivePageResult(t, pager.Results())
	resultB := receivePageResult(t, pager.Results())
	if resultA.Scope != "a" || resultB.Scope != "b" || loader.calls.Load() != 1 {
		t.Fatalf("results = %+v %+v calls=%d", resultA, resultB, loader.calls.Load())
	}
}

func TestPagerCancelsStaleGeneration(t *testing.T) {
	t.Parallel()
	loader := &gatedPageLoader{started: make(chan PageRequest, 4), release: make(chan struct{}), canceled: make(chan PageRequest, 4)}
	pager, _ := NewPager(context.Background(), loader, 2, 8)
	defer pager.Close()
	pager.Submit(PageRequest{Scope: "panel", Generation: 1, Cursor: "same", Limit: 10})
	receivePageRequest(t, loader.started)
	pager.Submit(PageRequest{Scope: "panel", Generation: 2, Cursor: "same", Limit: 10})
	newRequest := receivePageRequest(t, loader.started)
	if newRequest.Cursor != "same" {
		t.Fatalf("new request = %+v", newRequest)
	}
	canceled := receivePageRequest(t, loader.canceled)
	if canceled.Cursor != "same" {
		t.Fatalf("canceled = %+v", canceled)
	}
	close(loader.release)
	result := receivePageResult(t, pager.Results())
	if result.Generation != 2 || result.Page.NextCursor != "same-next" || result.Error != nil {
		t.Fatalf("result = %+v", result)
	}
	select {
	case stale := <-pager.Results():
		t.Fatalf("stale result = %+v", stale)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestPagerCloseCancelsAndClosesResultChannel(t *testing.T) {
	t.Parallel()
	loader := &gatedPageLoader{started: make(chan PageRequest, 1), release: make(chan struct{}), canceled: make(chan PageRequest, 1)}
	pager, _ := NewPager(context.Background(), loader, 1, 2)
	pager.Submit(PageRequest{Scope: "panel", Generation: 1, Cursor: "active", Limit: 10})
	receivePageRequest(t, loader.started)
	if err := pager.Close(); err != nil {
		t.Fatal(err)
	}
	if pager.Submit(PageRequest{Scope: "late", Generation: 1, Limit: 10}) {
		t.Fatal("submit succeeded after close")
	}
	if _, open := <-pager.Results(); open {
		t.Fatal("results remain open")
	}
}

func receivePageRequest(t *testing.T, channel <-chan PageRequest) PageRequest {
	t.Helper()
	select {
	case request := <-channel:
		return request
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for page request")
		return PageRequest{}
	}
}

func receivePageResult(t *testing.T, channel <-chan PageResult) PageResult {
	t.Helper()
	select {
	case result, open := <-channel:
		if !open {
			t.Fatal("page results closed early")
		}
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for page result")
		return PageResult{}
	}
}
