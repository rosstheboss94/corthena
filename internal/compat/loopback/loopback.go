// Package loopback exercises the approved HTTP and WebSocket boundaries.
package loopback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const operationTimeout = 5 * time.Second

// Health is the typed HTTP compatibility payload.
type Health struct {
	Status string `json:"status"`
}

// Event is the typed WebSocket compatibility payload.
type Event struct {
	ID      string `json:"event_id"`
	Type    string `json:"event_type"`
	Version int    `json:"schema_version"`
}

// Verify starts a loopback-only service and proves HTTP, WebSocket,
// cancellation, and graceful-shutdown behavior.
func Verify(ctx context.Context) error {
	service, err := start(ctx)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			closeCtx, cancel := context.WithTimeout(context.Background(), operationTimeout)
			defer cancel()
			_ = service.close(closeCtx)
		}
	}()

	if err := verifyHTTP(ctx, service.baseURL()); err != nil {
		return err
	}
	if err := verifyWebSocket(ctx, service.baseURL()); err != nil {
		return err
	}
	if err := verifyCancellation(ctx, service); err != nil {
		return err
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	if err := service.close(closeCtx); err != nil {
		return err
	}
	closed = true
	return nil
}

type service struct {
	server         *http.Server
	listener       net.Listener
	serveDone      chan error
	blockStarted   chan struct{}
	cancelObserved chan struct{}
	blockOnce      sync.Once
	cancelOnce     sync.Once
	handlers       sync.WaitGroup
}

func start(ctx context.Context) (*service, error) {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen on IPv4 loopback: %w", err)
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() {
		_ = listener.Close()
		return nil, fmt.Errorf("verify listener address: got %v", listener.Addr())
	}

	service := &service{
		listener:       listener,
		serveDone:      make(chan error, 1),
		blockStarted:   make(chan struct{}),
		cancelObserved: make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", service.track(service.handleHealth))
	mux.HandleFunc("GET /api/v1/events", service.track(service.handleEvents))
	mux.HandleFunc("GET /api/v1/block", service.track(service.handleBlock))
	service.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: operationTimeout,
		ReadTimeout:       operationTimeout,
		WriteTimeout:      operationTimeout,
		IdleTimeout:       operationTimeout,
	}

	// The service goroutine is the sole sender. It sends exactly once and then
	// closes serveDone. close is the sole receiver.
	go func() {
		err := service.server.Serve(listener)
		service.serveDone <- err
		close(service.serveDone)
	}()
	return service, nil
}

func (service *service) track(handler http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		service.handlers.Add(1)
		defer service.handlers.Done()
		handler(writer, request)
	}
}

func (service *service) handleHealth(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(Health{Status: "healthy"}); err != nil {
		return
	}
}

func (service *service) handleEvents(writer http.ResponseWriter, request *http.Request) {
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns: []string{"127.0.0.1:*"},
	})
	if err != nil {
		return
	}
	defer connection.CloseNow()

	payload, err := json.Marshal(Event{
		ID:      "event-1",
		Type:    "compatibility.ready",
		Version: 1,
	})
	if err != nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(request.Context(), operationTimeout)
	defer cancel()
	if err := connection.Write(writeCtx, websocket.MessageText, payload); err != nil {
		return
	}
	_ = connection.Close(websocket.StatusNormalClosure, "complete")
}

func (service *service) handleBlock(_ http.ResponseWriter, request *http.Request) {
	service.blockOnce.Do(func() {
		close(service.blockStarted)
	})
	<-request.Context().Done()
	service.cancelOnce.Do(func() {
		close(service.cancelObserved)
	})
}

func (service *service) baseURL() string {
	return "http://" + service.listener.Addr().String()
}

func (service *service) close(ctx context.Context) error {
	shutdownErr := service.server.Shutdown(ctx)
	serveErr := <-service.serveDone
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return fmt.Errorf("serve loopback HTTP: %w", serveErr)
	}

	handlersDone := make(chan struct{})
	go func() {
		service.handlers.Wait()
		close(handlersDone)
	}()
	select {
	case <-handlersDone:
	case <-ctx.Done():
		return fmt.Errorf("wait for loopback handlers: %w", ctx.Err())
	}
	if shutdownErr != nil {
		return fmt.Errorf("shutdown loopback HTTP: %w", shutdownErr)
	}
	return nil
}

func verifyHTTP(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: operationTimeout}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/health", nil)
	if err != nil {
		return fmt.Errorf("create loopback health request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send loopback health request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("read loopback health status: got %s", response.Status)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1024))
	decoder.DisallowUnknownFields()
	var health Health
	if err := decoder.Decode(&health); err != nil {
		return fmt.Errorf("decode loopback health response: %w", err)
	}
	if health.Status != "healthy" {
		return fmt.Errorf("verify loopback health: got %q", health.Status)
	}
	return nil
}

func verifyWebSocket(ctx context.Context, baseURL string) error {
	websocketURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("parse loopback WebSocket URL: %w", err)
	}
	websocketURL.Scheme = "ws"
	websocketURL.Path = "/api/v1/events"

	dialCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	connection, response, err := websocket.Dial(dialCtx, websocketURL.String(), nil)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		return fmt.Errorf("dial loopback WebSocket: %w", err)
	}
	defer connection.CloseNow()
	connection.SetReadLimit(1024)

	messageType, payload, err := connection.Read(dialCtx)
	if err != nil {
		return fmt.Errorf("read loopback WebSocket event: %w", err)
	}
	if messageType != websocket.MessageText {
		return fmt.Errorf("verify loopback WebSocket message type: got %d", messageType)
	}
	var event Event
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(payload), 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return fmt.Errorf("decode loopback WebSocket event: %w", err)
	}
	if event != (Event{ID: "event-1", Type: "compatibility.ready", Version: 1}) {
		return fmt.Errorf("verify loopback WebSocket event: got %+v", event)
	}
	if err := connection.Close(websocket.StatusNormalClosure, "received"); err != nil &&
		websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		return fmt.Errorf("close loopback WebSocket: %w", err)
	}
	return nil
}

func verifyCancellation(ctx context.Context, service *service) error {
	requestCtx, cancelRequest := context.WithCancel(ctx)
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		service.baseURL()+"/api/v1/block",
		nil,
	)
	if err != nil {
		cancelRequest()
		return fmt.Errorf("create cancellable loopback request: %w", err)
	}

	requestDone := make(chan error, 1)
	client := &http.Client{Timeout: operationTimeout}
	// This goroutine is the sole sender and sends exactly once. This function
	// owns the channel and receives the result before returning.
	go func() {
		response, requestErr := client.Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		requestDone <- requestErr
	}()
	select {
	case <-service.blockStarted:
	case <-ctx.Done():
		cancelRequest()
		return fmt.Errorf("wait for cancellable loopback request: %w", ctx.Err())
	case <-time.After(operationTimeout):
		cancelRequest()
		return errors.New("wait for cancellable loopback request: timeout")
	}
	cancelRequest()

	select {
	case requestErr := <-requestDone:
		if requestErr == nil {
			return errors.New("cancel loopback request: request unexpectedly succeeded")
		}
	case <-ctx.Done():
		return fmt.Errorf("wait for cancelled loopback request: %w", ctx.Err())
	case <-time.After(operationTimeout):
		return errors.New("wait for cancelled loopback request: timeout")
	}

	select {
	case <-service.cancelObserved:
	case <-ctx.Done():
		return fmt.Errorf("wait for server cancellation: %w", ctx.Err())
	case <-time.After(operationTimeout):
		return errors.New("wait for server cancellation: timeout")
	}
	return nil
}
