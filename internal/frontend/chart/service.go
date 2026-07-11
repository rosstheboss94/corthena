package chart

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// ScopeID identifies one independently generation-ordered chart consumer.
type ScopeID string

// Loader performs decoding, aggregation, and frame preparation on a worker
// goroutine. Implementations must honor context cancellation.
type Loader interface {
	Load(context.Context, Query) (Frame, error)
}

// Request is one monotonically generated chart request.
type Request struct {
	Scope      ScopeID
	Generation uint64
	Query      Query
}

// Result is one immutable worker result. Results are published only when the
// scope still owns the same generation and query.
type Result struct {
	Scope      ScopeID
	Generation uint64
	Query      Query
	Frame      Frame
	FromCache  bool
	Error      error
}

type loadJob struct {
	id      uint64
	query   Query
	context context.Context
}

type loadCompletion struct {
	id    uint64
	query Query
	frame Frame
	err   error
}

type scopeState struct {
	generation uint64
	query      Query
}

type flight struct {
	id       uint64
	cancel   context.CancelFunc
	watchers map[ScopeID]uint64
}

type resultWatcher struct {
	scope      ScopeID
	generation uint64
}

// Service owns a fixed worker pool, bounded request/result channels, request
// deduplication, cancellation, generation filtering, and the frame cache. The
// dispatcher is the sole closer of jobs and results; callers never close
// service channels.
type Service struct {
	context    context.Context
	cancel     context.CancelFunc
	loader     Loader
	cache      *FrameCache
	requests   chan Request
	cancels    chan ScopeID
	jobs       chan loadJob
	completed  chan loadCompletion
	results    chan Result
	dispatchWG sync.WaitGroup
	workerWG   sync.WaitGroup
	closed     atomic.Bool
	dropped    atomic.Uint64
	nextFlight uint64
}

// NewService starts a bounded owned worker pool.
func NewService(parent context.Context, loader Loader, cache *FrameCache, workers int, queue int) (*Service, error) {
	if parent == nil || loader == nil || cache == nil || workers <= 0 || queue <= 0 {
		return nil, errors.New("new chart service: context, loader, cache, workers, and queue must be valid")
	}
	ctx, cancel := context.WithCancel(parent)
	service := &Service{
		context: ctx, cancel: cancel, loader: loader, cache: cache,
		requests: make(chan Request, queue), cancels: make(chan ScopeID, queue),
		jobs: make(chan loadJob, workers), completed: make(chan loadCompletion, workers),
		results: make(chan Result, queue),
	}
	service.workerWG.Add(workers)
	for range workers {
		go service.worker()
	}
	service.dispatchWG.Add(1)
	go service.dispatch()
	return service, nil
}

// Submit performs a nonblocking UI-safe send. False reports explicit
// backpressure or shutdown; no goroutine is created by the caller.
func (service *Service) Submit(request Request) bool {
	if service.closed.Load() {
		return false
	}
	select {
	case service.requests <- request:
		return true
	default:
		return false
	}
}

// CancelScope nonblockingly cancels work no longer visible for a scope.
func (service *Service) CancelScope(scope ScopeID) bool {
	if service.closed.Load() {
		return false
	}
	select {
	case service.cancels <- scope:
		return true
	default:
		return false
	}
}

// Results returns the dispatcher-owned bounded output channel.
func (service *Service) Results() <-chan Result { return service.results }

// DroppedResults reports immutable results discarded because the UI result
// queue was saturated.
func (service *Service) DroppedResults() uint64 { return service.dropped.Load() }

// Close cancels all loaders and waits for the dispatcher and workers. It is
// idempotent.
func (service *Service) Close() error {
	if service.closed.CompareAndSwap(false, true) {
		service.cancel()
	}
	service.dispatchWG.Wait()
	return nil
}

func (service *Service) dispatch() {
	defer service.dispatchWG.Done()
	scopes := make(map[ScopeID]scopeState)
	flights := make(map[Query]*flight)
	defer func() {
		for _, active := range flights {
			active.cancel()
		}
		close(service.jobs)
		service.workerWG.Wait()
		close(service.results)
	}()
	for {
		select {
		case <-service.context.Done():
			return
		case scope := <-service.cancels:
			service.detachScope(scope, scopes, flights)
			delete(scopes, scope)
		case request := <-service.requests:
			service.handleRequest(request, scopes, flights)
		case completion := <-service.completed:
			service.handleCompletion(completion, scopes, flights)
		}
	}
}

func (service *Service) handleRequest(request Request, scopes map[ScopeID]scopeState, flights map[Query]*flight) {
	if request.Scope == "" || request.Generation == 0 {
		service.publish(Result{Scope: request.Scope, Generation: request.Generation, Query: request.Query, Error: fmt.Errorf("%w: scope and generation are required", ErrInvalidData)})
		return
	}
	if err := request.Query.Validate(); err != nil {
		service.publish(Result{Scope: request.Scope, Generation: request.Generation, Query: request.Query, Error: err})
		return
	}
	current, exists := scopes[request.Scope]
	if exists && request.Generation <= current.generation {
		return
	}
	service.detachScope(request.Scope, scopes, flights)
	scopes[request.Scope] = scopeState{generation: request.Generation, query: request.Query}
	if frame, found := service.cache.Get(request.Query); found {
		frame.Generation = request.Generation
		service.publish(Result{Scope: request.Scope, Generation: request.Generation, Query: request.Query, Frame: frame, FromCache: true})
		return
	}
	if active, found := flights[request.Query]; found {
		active.watchers[request.Scope] = request.Generation
		return
	}
	loadContext, cancel := context.WithCancel(service.context)
	service.nextFlight++
	active := &flight{id: service.nextFlight, cancel: cancel, watchers: map[ScopeID]uint64{request.Scope: request.Generation}}
	select {
	case service.jobs <- loadJob{id: active.id, query: request.Query, context: loadContext}:
		flights[request.Query] = active
	default:
		cancel()
		delete(scopes, request.Scope)
		service.publish(Result{Scope: request.Scope, Generation: request.Generation, Query: request.Query, Error: ErrServiceBusy})
	}
}

func (service *Service) detachScope(scope ScopeID, scopes map[ScopeID]scopeState, flights map[Query]*flight) {
	current, found := scopes[scope]
	if !found {
		return
	}
	if active, exists := flights[current.query]; exists {
		delete(active.watchers, scope)
		if len(active.watchers) == 0 {
			active.cancel()
			delete(flights, current.query)
		}
	}
}

func (service *Service) handleCompletion(completion loadCompletion, scopes map[ScopeID]scopeState, flights map[Query]*flight) {
	active, found := flights[completion.query]
	if !found || active.id != completion.id {
		return
	}
	delete(flights, completion.query)
	active.cancel()
	if completion.err == nil {
		_ = service.cache.Put(completion.query, completion.frame)
	}
	watchers := make([]resultWatcher, 0, len(active.watchers))
	for scope, generation := range active.watchers {
		watchers = append(watchers, resultWatcher{scope: scope, generation: generation})
	}
	sort.Slice(watchers, func(left int, right int) bool { return watchers[left].scope < watchers[right].scope })
	for _, watcher := range watchers {
		current, currentFound := scopes[watcher.scope]
		if !currentFound || current.generation != watcher.generation || current.query != completion.query {
			continue
		}
		frame := completion.frame.Clone()
		frame.Generation = watcher.generation
		service.publish(Result{Scope: watcher.scope, Generation: watcher.generation, Query: completion.query, Frame: frame, Error: completion.err})
	}
}

func (service *Service) publish(result Result) {
	select {
	case service.results <- result:
	default:
		service.dropped.Add(1)
	}
}

func (service *Service) worker() {
	defer service.workerWG.Done()
	for job := range service.jobs {
		frame, err := service.loader.Load(job.context, job.query)
		completion := loadCompletion{id: job.id, query: job.query, frame: frame, err: err}
		select {
		case service.completed <- completion:
		case <-service.context.Done():
			return
		case <-job.context.Done():
			if !errors.Is(err, context.Canceled) {
				select {
				case service.completed <- loadCompletion{id: job.id, query: job.query, err: job.context.Err()}:
				case <-service.context.Done():
				}
			}
		}
	}
}
