package table

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	// ErrPagerBusy reports bounded request or worker-queue saturation.
	ErrPagerBusy = errors.New("table pager busy")
	// ErrPagerClosed reports use after shutdown.
	ErrPagerClosed = errors.New("table pager closed")
)

// PageScope identifies one independently ordered table panel.
type PageScope string

// PageRequest is a cancellable generation-safe server pagination request.
type PageRequest struct {
	Scope      PageScope
	Generation uint64
	Cursor     string
	Limit      int
	Sort       []SortSpec
	Filters    []FilterSpec
}

// Clone returns an independent immutable request.
func (request PageRequest) Clone() PageRequest {
	request.Sort = append([]SortSpec(nil), request.Sort...)
	request.Filters = append([]FilterSpec(nil), request.Filters...)
	return request
}

// Page is one immutable server-side table page.
type Page struct {
	Rows       []Row
	NextCursor string
	TotalRows  uint64
}

// Clone returns an independent immutable page.
func (page Page) Clone() Page {
	source := page.Rows
	page.Rows = make([]Row, len(source))
	for index, row := range source {
		page.Rows[index] = row.Clone()
	}
	return page
}

// PageLoader performs server work on an owned background worker.
type PageLoader interface {
	LoadPage(context.Context, PageRequest) (Page, error)
}

// PageResult is published only for the newest generation of a scope.
type PageResult struct {
	Scope      PageScope
	Generation uint64
	Page       Page
	Error      error
}

type pageJob struct {
	id      uint64
	key     string
	request PageRequest
	context context.Context
}

type pageCompletion struct {
	id   uint64
	key  string
	page Page
	err  error
}

type pageWatcher struct {
	scope      PageScope
	generation uint64
}

type pageFlight struct {
	id       uint64
	cancel   context.CancelFunc
	request  PageRequest
	watchers map[PageScope]uint64
}

type pageScopeState struct {
	generation uint64
	key        string
}

// Pager owns bounded request, result, and worker queues. One dispatcher owns
// flight maps and closes all output; worker sends are cancellation-aware.
type Pager struct {
	context    context.Context
	cancel     context.CancelFunc
	loader     PageLoader
	requests   chan PageRequest
	cancels    chan PageScope
	jobs       chan pageJob
	completed  chan pageCompletion
	results    chan PageResult
	dispatchWG sync.WaitGroup
	workerWG   sync.WaitGroup
	closed     atomic.Bool
	dropped    atomic.Uint64
	nextFlight uint64
}

// NewPager starts the fixed worker pool.
func NewPager(parent context.Context, loader PageLoader, workers int, queue int) (*Pager, error) {
	if parent == nil || loader == nil || workers <= 0 || queue <= 0 {
		return nil, errors.New("new table pager: invalid context, loader, worker, or queue configuration")
	}
	ctx, cancel := context.WithCancel(parent)
	pager := &Pager{
		context: ctx, cancel: cancel, loader: loader,
		requests: make(chan PageRequest, queue), cancels: make(chan PageScope, queue),
		jobs: make(chan pageJob, workers), completed: make(chan pageCompletion, workers), results: make(chan PageResult, queue),
	}
	pager.workerWG.Add(workers)
	for range workers {
		go pager.worker()
	}
	pager.dispatchWG.Add(1)
	go pager.dispatch()
	return pager, nil
}

// Submit performs a nonblocking render-thread-safe request send.
func (pager *Pager) Submit(request PageRequest) bool {
	if pager.closed.Load() {
		return false
	}
	request = request.Clone()
	select {
	case pager.requests <- request:
		return true
	default:
		return false
	}
}

// CancelScope nonblockingly cancels invisible page work.
func (pager *Pager) CancelScope(scope PageScope) bool {
	if pager.closed.Load() {
		return false
	}
	select {
	case pager.cancels <- scope:
		return true
	default:
		return false
	}
}

// Results returns the dispatcher-owned result stream.
func (pager *Pager) Results() <-chan PageResult { return pager.results }

// DroppedResults returns the number of results discarded under UI
// backpressure.
func (pager *Pager) DroppedResults() uint64 { return pager.dropped.Load() }

// Close cancels, joins, and closes the result stream. It is idempotent.
func (pager *Pager) Close() error {
	if pager.closed.CompareAndSwap(false, true) {
		pager.cancel()
	}
	pager.dispatchWG.Wait()
	return nil
}

func (pager *Pager) dispatch() {
	defer pager.dispatchWG.Done()
	scopes := make(map[PageScope]pageScopeState)
	flights := make(map[string]*pageFlight)
	defer func() {
		for _, active := range flights {
			active.cancel()
		}
		close(pager.jobs)
		pager.workerWG.Wait()
		close(pager.results)
	}()
	for {
		select {
		case <-pager.context.Done():
			return
		case scope := <-pager.cancels:
			pager.detach(scope, scopes, flights)
			delete(scopes, scope)
		case request := <-pager.requests:
			pager.handleRequest(request, scopes, flights)
		case completion := <-pager.completed:
			pager.handleCompletion(completion, scopes, flights)
		}
	}
}

func (pager *Pager) handleRequest(request PageRequest, scopes map[PageScope]pageScopeState, flights map[string]*pageFlight) {
	if request.Scope == "" || request.Generation == 0 || request.Limit <= 0 || request.Limit > 100_000 {
		pager.publish(PageResult{Scope: request.Scope, Generation: request.Generation, Error: fmt.Errorf("%w: invalid page request", ErrInvalidTable)})
		return
	}
	key := pageRequestKey(request)
	current, exists := scopes[request.Scope]
	if exists && request.Generation <= current.generation {
		return
	}
	pager.detach(request.Scope, scopes, flights)
	scopes[request.Scope] = pageScopeState{generation: request.Generation, key: key}
	if active, found := flights[key]; found {
		active.watchers[request.Scope] = request.Generation
		return
	}
	loadContext, cancel := context.WithCancel(pager.context)
	pager.nextFlight++
	active := &pageFlight{id: pager.nextFlight, cancel: cancel, request: request.Clone(), watchers: map[PageScope]uint64{request.Scope: request.Generation}}
	select {
	case pager.jobs <- pageJob{id: active.id, key: key, request: request.Clone(), context: loadContext}:
		flights[key] = active
	default:
		cancel()
		delete(scopes, request.Scope)
		pager.publish(PageResult{Scope: request.Scope, Generation: request.Generation, Error: ErrPagerBusy})
	}
}

func (pager *Pager) detach(scope PageScope, scopes map[PageScope]pageScopeState, flights map[string]*pageFlight) {
	state, found := scopes[scope]
	if !found {
		return
	}
	if active, exists := flights[state.key]; exists {
		delete(active.watchers, scope)
		if len(active.watchers) == 0 {
			active.cancel()
			delete(flights, state.key)
		}
	}
}

func (pager *Pager) handleCompletion(completion pageCompletion, scopes map[PageScope]pageScopeState, flights map[string]*pageFlight) {
	active, found := flights[completion.key]
	if !found || active.id != completion.id {
		return
	}
	delete(flights, completion.key)
	active.cancel()
	if completion.err == nil {
		completion.err = validatePage(completion.page)
	}
	watchers := make([]pageWatcher, 0, len(active.watchers))
	for scope, generation := range active.watchers {
		watchers = append(watchers, pageWatcher{scope: scope, generation: generation})
	}
	sort.Slice(watchers, func(left int, right int) bool { return watchers[left].scope < watchers[right].scope })
	for _, watcher := range watchers {
		state, exists := scopes[watcher.scope]
		if !exists || state.generation != watcher.generation || state.key != completion.key {
			continue
		}
		pager.publish(PageResult{Scope: watcher.scope, Generation: watcher.generation, Page: completion.page.Clone(), Error: completion.err})
	}
}

func validatePage(page Page) error {
	rowIDs := make(map[RowID]struct{}, len(page.Rows))
	sourceIndexes := make(map[uint64]struct{}, len(page.Rows))
	for index, row := range page.Rows {
		if row.ID == "" {
			return fmt.Errorf("%w: page row %d has empty ID", ErrInvalidTable, index)
		}
		if _, duplicate := rowIDs[row.ID]; duplicate {
			return fmt.Errorf("%w: duplicate page row ID %q", ErrInvalidTable, row.ID)
		}
		if _, duplicate := sourceIndexes[row.SourceIndex]; duplicate {
			return fmt.Errorf("%w: duplicate page source index %d", ErrInvalidTable, row.SourceIndex)
		}
		rowIDs[row.ID] = struct{}{}
		sourceIndexes[row.SourceIndex] = struct{}{}
		for cellIndex, cell := range row.Cells {
			if err := cell.Validate(); err != nil {
				return fmt.Errorf("%w: page row %d cell %d", ErrInvalidTable, index, cellIndex)
			}
		}
	}
	return nil
}

func (pager *Pager) publish(result PageResult) {
	select {
	case pager.results <- result:
	default:
		pager.dropped.Add(1)
	}
}

func (pager *Pager) worker() {
	defer pager.workerWG.Done()
	for job := range pager.jobs {
		page, err := pager.loader.LoadPage(job.context, job.request)
		select {
		case pager.completed <- pageCompletion{id: job.id, key: job.key, page: page, err: err}:
		case <-pager.context.Done():
			return
		case <-job.context.Done():
		}
	}
}

func pageRequestKey(request PageRequest) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "%q|%d", request.Cursor, request.Limit)
	for _, spec := range request.Sort {
		fmt.Fprintf(&builder, "|s:%q:%d:%d", spec.Column, spec.Direction, spec.Nulls)
	}
	for _, spec := range request.Filters {
		fmt.Fprintf(&builder, "|f:%q:%d:%d:%t:%q:%d:%g:%t:%d", spec.Column, spec.Operator, spec.Value.Kind, spec.Value.Null,
			spec.Value.String, spec.Value.Integer, spec.Value.Float, spec.Value.Boolean, spec.Value.Time.UTC().UnixNano())
	}
	return builder.String()
}
