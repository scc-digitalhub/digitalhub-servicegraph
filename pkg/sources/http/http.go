// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

type HTTPSource struct {
	Conf     *Configuration
	factory  sources.FlowFactory
	logger   *slog.Logger
	input    chan any

	// listenAddr is populated once the underlying TCP listener is bound.
	// Tests that use port 0 read this to discover the OS-assigned port.
	mu         sync.RWMutex
	listenAddr string
	server     *http.Server
}

func (s *HTTPSource) init(factory sources.FlowFactory, handleHttp func(w http.ResponseWriter, r *http.Request)) error {
	s.factory = factory

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Create server with timeouts
	serverPort := fmt.Sprintf(":%d", s.Conf.Port)

	mux := http.NewServeMux()
	withOtel := util.IsOTelTracingEnabled()
	if withOtel {
		// Set up OpenTelemetry.
		otelShutdown, err := util.SetupOTelSDK(ctx)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, otelShutdown(context.Background()))
		}()
		mux.Handle("/", otelhttp.NewHandler(http.HandlerFunc(handleHttp), "HTTPSource"))
		s.logger.Info("OpenTelemetry instrumentation enabled for HTTP source")
	} else {
		mux.HandleFunc("/", handleHttp)
		s.logger.Info("OpenTelemetry instrumentation disabled for HTTP source")
	}

	server := &http.Server{
		Addr:         serverPort,
		ReadTimeout:  time.Duration(s.Conf.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.Conf.WriteTimeout) * time.Second,
		Handler:      mux,
	}

	// Bind the listener up front so a bind failure is returned synchronously
	// (avoids log.Fatalf from a goroutine and lets tests use port 0).
	ln, err := net.Listen("tcp", serverPort)
	if err != nil {
		return fmt.Errorf("http source: failed to listen on %s: %w", serverPort, err)
	}
	s.mu.Lock()
	s.listenAddr = ln.Addr().String()
	s.server = server
	s.mu.Unlock()
	s.logger.Info("Server is starting", slog.String("addr", s.listenAddr))

	// Channel to listen for errors coming from the server
	serverErrors := make(chan error, 1)

	// Start server
	go func() {
		serverErrors <- server.Serve(ln)
	}()

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http source: server error: %w", err)
		}

	case sig := <-shutdown:
		s.logger.Info("Received signal, starting shutdown", slog.Any("signal", sig))

		// Create context with timeout for shutdown
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Gracefully shutdown the server
		if err := server.Shutdown(shutCtx); err != nil {
			s.logger.Warn("Error during server shutdown", slog.Any("error", err))
			server.Close()
		}
	}
	return nil
}

// ListenAddr returns the address the underlying listener is bound to.
// It is populated once init() begins serving; it is empty before then.
// Useful for tests that bind to port 0.
func (s *HTTPSource) ListenAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listenAddr
}

// Port returns the TCP port the source is listening on, parsed from ListenAddr.
func (s *HTTPSource) Port() int {
	addr := s.ListenAddr()
	if addr == "" {
		return 0
	}
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(p)
	return n
}

// Stop gracefully shuts down the underlying HTTP server. Safe to call before
// the server has started (it is a no-op in that case). Implements the
// sources.Stoppable interface.
func (s *HTTPSource) Stop(ctx context.Context) error {
	s.mu.RLock()
	srv := s.server
	s.mu.RUnlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func NewHTTPSource(conf *Configuration) *HTTPSource {
	source := &HTTPSource{}
	source.Conf = conf
	source.logger = slog.Default()
	source.logger = source.logger.With(slog.Group("source",
		slog.String("name", "http"),
		slog.String("type", "source")))

	return source
}

func (s *HTTPSource) Start(factory sources.FlowFactory) error {
	return s.init(factory, s.handleHttp)
}

func (s *HTTPSource) handleHttp(w http.ResponseWriter, r *http.Request) {
	// Validate method
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.Conf.ProcessTimeout*int(time.Second)))
	defer cancel()

	// Create channels for processing
	inputChan := make(chan any)
	outputChan := make(chan any)
	errorChan := make(chan error, 1)

	// Ensure channels are closed
	defer close(inputChan)
	defer close(errorChan)

	// Start the processor
	go chainProcessor(inputChan, outputChan, s.factory)

	// Read request event
	event, err := NewHTTPEvent(r, s.Conf.MaxInputSize)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	util.AddRequestToSpan(r.Context(), event.GetBody(), event.GetContentType())

	// Send data for processing
	select {
	case inputChan <- event:
	case <-ctx.Done():
		http.Error(w, "Processing timeout", http.StatusGatewayTimeout)
		return
	}

	// Wait for processing result or error
	select {
	case result := <-outputChan:
		s.writeMessage(r.Context(), w, result)
	case err := <-errorChan:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	case <-ctx.Done():
		http.Error(w, "Processing timeout", http.StatusGatewayTimeout)
	}
}

func (s *HTTPSource) writeMessage(ctx context.Context, w http.ResponseWriter, msg any) {
	var contentType string = "application/json"
	var body []byte

	switch v := msg.(type) {
	case streams.Event:
		body = v.GetBody()
		contentType = v.GetContentType()
		for k, val := range v.GetHeaders() {
			w.Header().Set(k, val)
		}
	case []byte:
		contentType = "application/octet-stream"
		body = v
	case string:
		contentType = "text/plain"
		body = []byte(v)
	default:
		s.logger.Warn("Received unsupported message type", slog.Any("type", v))
	}

	util.AddResponseToSpan(ctx, body, contentType)

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (s *HTTPSource) StartAsync(factory sources.FlowFactory, sink streams.Sink) error {
	go func() {
		s.input = make(chan any)
		s.factory.GenerateFlow(
			extension.NewChanSource(s.input),
			sink,
		)
	}()
	return s.init(factory, s.handleHttpAsync)
}

func (s *HTTPSource) handleHttpAsync(w http.ResponseWriter, r *http.Request) {
	// Validate method
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.Conf.ProcessTimeout*int(time.Second)))
	defer cancel()

	newCtx := context.Background()
	tracer := otel.GetTracerProvider().Tracer("")
	spanCtx, _ := tracer.Start(newCtx, "HTTPSourceAsync")

	// Read request event
	event, err := NewHTTPEventAsync(spanCtx, r, s.Conf.MaxInputSize)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	util.AddRequestToSpan(spanCtx, event.GetBody(), event.GetContentType())

	// Send data for processing
	select {
	case s.input <- event:
	case <-ctx.Done():
		http.Error(w, "Processing timeout", http.StatusGatewayTimeout)
		return
	}

	// Acknowledge receipt
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ack"))

}
func chainProcessor(input chan any, output chan any, factory sources.FlowFactory) {

	factory.GenerateFlow(
		extension.NewChanSource(input),
		extension.NewChanSink(output),
	)
}

func init() {
	sources.RegistrySingleton.Register("http", &HTTPSourceProcessor{})
}

type HTTPSourceProcessor struct {
	sources.Converter
	sources.Validator
}

func (c *HTTPSourceProcessor) Convert(input model.InputSpec) (sources.Source, error) {
	conf := &Configuration{}
	err := util.Convert(input.Spec, conf)
	if err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.Port, conf.ReadTimeout, conf.WriteTimeout, conf.ProcessTimeout, conf.MaxInputSize)
	src := NewHTTPSource(conf)
	return src, nil
}
func (c *HTTPSourceProcessor) Validate(input model.InputSpec) error {
	conf := &Configuration{}
	err := util.Convert(input.Spec, conf)
	if err != nil {
		return err
	}
	if conf.Port < 0 || conf.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", conf.Port)
	}
	if conf.ReadTimeout < 0 {
		return fmt.Errorf("invalid read timeout: %d", conf.ReadTimeout)
	}
	if conf.WriteTimeout < 0 {
		return fmt.Errorf("invalid write timeout: %d", conf.WriteTimeout)
	}
	if conf.ProcessTimeout < 0 {
		return fmt.Errorf("invalid process timeout: %d", conf.ProcessTimeout)
	}
	if conf.MaxInputSize < 0 {
		return fmt.Errorf("invalid max input size: %d", conf.MaxInputSize)
	}
	return nil
}
