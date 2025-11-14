package http

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	ext "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
)

type fakeFactory struct{}

func (f *fakeFactory) GenerateFlow(source streams.Source, sink streams.Sink) {
	go func() {
		for v := range source.Out() {
			sink.In() <- v
		}
		if ch, ok := any(sink).(interface{ In() chan any }); ok {
			close(ch.In())
		}
	}()
}

type echoFactory struct{}

func (e *echoFactory) GenerateFlow(source streams.Source, sink streams.Sink) {
	for v := range source.Out() {
		sink.In() <- v
	}
	if ch, ok := sink.(*ext.ChanSink); ok {
		close(ch.Out)
	}
}

type slowFactory struct{}

func (s *slowFactory) GenerateFlow(source streams.Source, sink streams.Sink) {
	for v := range source.Out() {
		time.Sleep(200 * time.Millisecond)
		sink.In() <- v
	}
	if ch, ok := any(sink).(interface{ In() chan any }); ok {
		close(ch.In())
	}
}

func TestNewConfigurationDefaults(t *testing.T) {
	c := NewConfiguration(0, 0, 0, 0, 0)
	if c.Port == 0 {
		t.Fatalf("expected default port to be set")
	}
	if c.MaxInputSize == 0 {
		t.Fatalf("expected default max input size to be set")
	}
}

func TestNewHTTPEvent_ReadBody(t *testing.T) {
	body := "hello"
	req, err := http.NewRequest("POST", "http://example.com", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	ev, err := NewHTTPEvent(req, 1024)
	if err != nil {
		t.Fatalf("NewHTTPEvent error: %v", err)
	}
	if string(ev.GetBody()) != body {
		t.Fatalf("unexpected body: %s", string(ev.GetBody()))
	}
}

func TestHandleHttp_BodyTooLarge(t *testing.T) {
	// validate that NewHTTPEvent truncates body to MaxInputSize
	longBody := "this body is too long"
	req, _ := http.NewRequest("POST", "http://example.com", strings.NewReader(longBody))
	ev, err := NewHTTPEvent(req, 5)
	if err != nil {
		t.Fatalf("expected no error reading body: %v", err)
	}
	if len(ev.GetBody()) != 5 {
		t.Fatalf("expected body to be truncated to 5 bytes, got %d", len(ev.GetBody()))
	}
}

func TestWriteMessage_ContentTypePropagation(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)

	// create event with content-type header
	headers := map[string]string{"content-type": "application/custom"}
	evt, _ := streams.NewGenericEvent([]byte("data"), "", "", headers, nil, 200)

	w := httptest.NewRecorder()
	src.writeMessage(w, evt)
	res := w.Result()
	if res.Header.Get("Content-Type") != "application/custom" {
		t.Fatalf("expected Content-Type propagated, got %s", res.Header.Get("Content-Type"))
	}
}

// errReadCloser simulates a body that errors on Read
type errReadCloser struct{}

func (e *errReadCloser) Read(p []byte) (n int, err error) { return 0, errors.New("read error") }
func (e *errReadCloser) Close() error                     { return nil }

func TestNewHTTPEvent_ReadError(t *testing.T) {
	req := &http.Request{Body: &errReadCloser{}}
	_, err := NewHTTPEvent(req, 1024)
	if err == nil {
		t.Fatalf("expected error from NewHTTPEvent when body read fails")
	}
}

func TestHTTPEvent_Getters(t *testing.T) {
	body := "ok"
	req := httptest.NewRequest(http.MethodPost, "http://example.com/path?q=1&x=a", strings.NewReader(body))
	req.Header.Set("X-Test", "v")
	ev, err := NewHTTPEvent(req, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.GetURL() == "" {
		t.Fatalf("expected URL")
	}
	if ev.GetMethod() != http.MethodPost {
		t.Fatalf("expected POST method")
	}
	if ev.GetPath() != "/path" {
		t.Fatalf("expected path /path")
	}
	if ev.GetField("q") != "1" {
		t.Fatalf("expected q=1")
	}
	fields := ev.GetFields()
	if fields["x"] != "a" {
		t.Fatalf("expected x=a")
	}
	headers := ev.GetHeaders()
	if headers["X-Test"] != "v" {
		t.Fatalf("expected header X-Test=v")
	}
	if len(ev.GetBody()) != len(body) {
		t.Fatalf("body length mismatch")
	}
}

func TestWriteMessage_ByteAndStringAndDefault(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)

	// []byte
	w := httptest.NewRecorder()
	src.writeMessage(w, []byte("bin"))
	res := w.Result()
	if res.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("expected octet-stream, got %s", res.Header.Get("Content-Type"))
	}

	// string
	w = httptest.NewRecorder()
	src.writeMessage(w, "text")
	res = w.Result()
	if res.Header.Get("Content-Type") != "text/plain" {
		t.Fatalf("expected text/plain, got %s", res.Header.Get("Content-Type"))
	}

	// default (unsupported type) - use an int
	w = httptest.NewRecorder()
	src.writeMessage(w, 12345)
	res = w.Result()
	if res.Header.Get("Content-Type") == "" {
		t.Fatalf("expected content-type to be set even for unsupported type")
	}
}

func TestHTTPSourceProcessor_ConvertAndValidate(t *testing.T) {
	proc := &HTTPSourceProcessor{}
	// valid spec
	spec := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"port": 9090, "read_timeout": 1, "write_timeout": 1, "process_timeout": 1, "max_input_size": 1024}}
	src, err := proc.Convert(spec)
	if err != nil || src == nil {
		t.Fatalf("expected convert to succeed: %v", err)
	}
	// validate should pass
	if err := proc.Validate(spec); err != nil {
		t.Fatalf("expected validate to pass, got %v", err)
	}

	// invalid spec: bad port
	spec2 := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"port": -1}}
	if err := proc.Validate(spec2); err == nil {
		t.Fatalf("expected validate to fail for invalid port")
	}
}

// TestHandleHttp_Post and others from http_handlers_test.go
func TestHandleHttp_Post(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)

	// set a fake factory so chainProcessor has a non-nil factory
	src.factory = &echoFactory{}

	body := `{"value":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	src.handleHttp(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", res.StatusCode)
	}
}

func TestHandleHttp_NonPost(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)
	// prevent nil factory usage later
	src.factory = &echoFactory{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	src.handleHttp(w, req)
	res := w.Result()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res.StatusCode)
	}
}

func TestHandleHttpAsync_Ack(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)

	out := make(chan any, 1)
	sink := ext.NewChanSink(out)

	// Avoid StartAsync (it starts a real HTTP server). Instead set up input and run flow.
	src.factory = &echoFactory{}
	src.input = make(chan any)
	go func() {
		src.factory.GenerateFlow(ext.NewChanSource(src.input), sink)
	}()

	body := `{"v":"a"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	src.handleHttpAsync(w, req)

	// close input to let flow finish
	close(src.input)

	select {
	case <-out:
	default:
	}

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 ack, got %d", res.StatusCode)
	}
}

// TestInitAndStartShutdown calls the HTTPSource.init (via Start) and sends a
// SIGTERM to trigger the shutdown branch so init returns quickly.
func TestInitAndStartShutdown(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)

	done := make(chan struct{})
	go func() {
		// call init directly using a no-op handler
		http.DefaultServeMux = http.NewServeMux()
		src.init(&fakeFactory{}, func(w http.ResponseWriter, r *http.Request) {})
		close(done)
	}()

	// give server a moment to start
	time.Sleep(50 * time.Millisecond)

	// send SIGTERM to trigger shutdown
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatalf("init did not return after SIGTERM")
	}
}

// TestStartAsync_Shutdown ensures StartAsync can be invoked and init will
// return when a shutdown signal is sent.
func TestStartAsync_Shutdown(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)

	done := make(chan struct{})
	go func() {
		// StartAsync will call init which blocks until shutdown
		http.DefaultServeMux = http.NewServeMux()
		src.StartAsync(&fakeFactory{}, nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("StartAsync did not return after SIGTERM")
	}
}

func TestGetTimestamp(t *testing.T) {
	body := "ok"
	req := httptest.NewRequest(http.MethodPost, "http://example.com/path", strings.NewReader(body))
	ev, err := NewHTTPEvent(req, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.GetTimestamp().IsZero() {
		t.Fatalf("expected non-zero timestamp")
	}
}

func TestValidate_NegativeTimeouts(t *testing.T) {
	proc := &HTTPSourceProcessor{}
	// negative read timeout
	spec := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"read_timeout": -1}}
	if err := proc.Validate(spec); err == nil {
		t.Fatalf("expected validate to fail for negative read_timeout")
	}
	// negative write timeout
	spec2 := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"write_timeout": -1}}
	if err := proc.Validate(spec2); err == nil {
		t.Fatalf("expected validate to fail for negative write_timeout")
	}
	// negative process timeout
	spec3 := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"process_timeout": -1}}
	if err := proc.Validate(spec3); err == nil {
		t.Fatalf("expected validate to fail for negative process_timeout")
	}
	// negative max input size
	spec4 := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"max_input_size": -1}}
	if err := proc.Validate(spec4); err == nil {
		t.Fatalf("expected validate to fail for negative max_input_size")
	}
}

func TestStartShutdown(t *testing.T) {
	conf := NewConfiguration(0, 0, 0, 1, 1024)
	src := NewHTTPSource(conf)

	done := make(chan struct{})
	go func() {
		http.DefaultServeMux = http.NewServeMux()
		src.Start(&fakeFactory{})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Start did not return after SIGTERM")
	}
}

func TestHandleHttp_ProcessingTimeout(t *testing.T) {
	// use a configuration with zero ProcessTimeout (no defaults applied)
	conf := &Configuration{Port: 0, ReadTimeout: 0, WriteTimeout: 0, ProcessTimeout: 0, MaxInputSize: 1024}
	src := NewHTTPSource(conf)

	// slowFactory (package-level) delays forwarding so processing exceeds timeout
	src.factory = &slowFactory{}

	body := `{"v":"a"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	src.handleHttp(w, req)
	res := w.Result()
	if res.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 Processing timeout, got %d", res.StatusCode)
	}
}

func TestHandleHttpAsync_ProcessingTimeout(t *testing.T) {
	conf := &Configuration{Port: 0, ReadTimeout: 0, WriteTimeout: 0, ProcessTimeout: 0, MaxInputSize: 1024}
	src := NewHTTPSource(conf)
	src.input = make(chan any)

	body := `{"v":"a"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	src.handleHttpAsync(w, req)
	res := w.Result()
	if res.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 Processing timeout, got %d", res.StatusCode)
	}
}

func TestConvert_Failure(t *testing.T) {
	proc := &HTTPSourceProcessor{}
	// pass a map with wrong types to cause util.Convert to fail
	spec := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"port": "not-a-number"}}
	_, err := proc.Convert(spec)
	if err == nil {
		t.Fatalf("expected Convert to fail for invalid spec type")
	}
}

func TestValidate_PortTooLarge(t *testing.T) {
	proc := &HTTPSourceProcessor{}
	spec := model.InputSpec{Kind: "http", Spec: map[string]interface{}{"port": 70000}}
	if err := proc.Validate(spec); err == nil {
		t.Fatalf("expected validate to fail for port > 65535")
	}
}
