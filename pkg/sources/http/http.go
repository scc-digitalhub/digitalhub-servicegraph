package http

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type HTTPSource struct {
	sources.Source
	Conf    *Configuration
	factory sources.FlowFactory
	logger  *slog.Logger
	input   chan any
}

func (s *HTTPSource) init(factory sources.FlowFactory, handleHttp func(w http.ResponseWriter, r *http.Request)) {
	s.factory = factory
	// Create server with timeouts
	serverPort := fmt.Sprintf(":%d", s.Conf.Port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHttp)
	server := &http.Server{
		Addr:         serverPort,
		ReadTimeout:  time.Duration(s.Conf.ReadTimeout * int(time.Second)),
		WriteTimeout: time.Duration(s.Conf.WriteTimeout * int(time.Second)),
		Handler:      mux,
	}

	// Channel to listen for errors coming from the server
	serverErrors := make(chan error, 1)

	// Start server
	go func() {
		log.Printf("Server is starting on %s\n", serverPort)
		serverErrors <- server.ListenAndServe()
	}()

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		log.Fatalf("Error starting server: %v", err)

	case sig := <-shutdown:
		log.Printf("Received signal %v, starting shutdown\n", sig)

		// Create context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Gracefully shutdown the server
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Error during server shutdown: %v\n", err)
			server.Close()
		}
	}
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

func (s *HTTPSource) Start(factory sources.FlowFactory) {
	s.init(factory, s.handleHttp)
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
		s.writeMessage(w, result)
	case err := <-errorChan:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	case <-ctx.Done():
		http.Error(w, "Processing timeout", http.StatusGatewayTimeout)
	}
}

func (s *HTTPSource) writeMessage(w http.ResponseWriter, msg interface{}) {
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
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	w.Write(body)

}

func (s *HTTPSource) StartAsync(factory sources.FlowFactory, sink streams.Sink) {
	go func() {
		s.input = make(chan any)
		s.factory.GenerateFlow(
			extension.NewChanSource(s.input),
			sink,
		)
	}()
	s.init(factory, s.handleHttpAsync)
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

	// Read request event
	event, err := NewHTTPEvent(r, s.Conf.MaxInputSize)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

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
	if conf.Port <= 0 || conf.Port > 65535 {
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
