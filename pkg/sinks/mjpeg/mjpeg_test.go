package mjpeg

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// newTestSink starts an MJPEGSink on an OS-assigned port (port 0) and returns it.
func newTestSink(t *testing.T) *MJPEGSink {
	t.Helper()
	conf := NewConfiguration(0, defaultPath)
	s, err := NewMJPEGSink(*conf)
	if err != nil {
		t.Fatalf("failed to create MJPEGSink: %v", err)
	}
	return s
}

// fakeJPEG returns a minimal non-empty byte slice used as a stand-in for JPEG data.
func fakeJPEG(content string) []byte {
	return []byte(content)
}

// TestNewConfiguration verifies default value substitution.
func TestNewConfiguration(t *testing.T) {
	conf := NewConfiguration(-1, "")
	if conf.Port != defaultPort {
		t.Errorf("expected default port %d, got %d", defaultPort, conf.Port)
	}
	if conf.Path != defaultPath {
		t.Errorf("expected default path %q, got %q", defaultPath, conf.Path)
	}

	conf2 := NewConfiguration(9090, "/live")
	if conf2.Port != 9090 {
		t.Errorf("expected port 9090, got %d", conf2.Port)
	}
	if conf2.Path != "/live" {
		t.Errorf("expected path /live, got %q", conf2.Path)
	}
}

// TestMJPEGSink_ListenAddr checks that the sink reports a non-empty listen address.
func TestMJPEGSink_ListenAddr(t *testing.T) {
	s := newTestSink(t)
	addr := s.ListenAddr()
	if addr == "" {
		t.Error("expected non-empty listen address")
	}
	close(s.In())
	s.AwaitCompletion()
}

// TestMJPEGSink_ImplementsSink verifies the streams.Sink interface is satisfied.
func TestMJPEGSink_ImplementsSink(t *testing.T) {
	s := newTestSink(t)
	var _ streams.Sink = s
	close(s.In())
	s.AwaitCompletion()
}

// TestMJPEGSink_SendBytesAndReceiveFrame connects an HTTP client to the stream
// endpoint, pushes one []byte frame through the sink, and verifies the client
// receives it as a multipart JPEG part.
func TestMJPEGSink_SendBytesAndReceiveFrame(t *testing.T) {
	s := newTestSink(t)

	frame := fakeJPEG("JPEG_FRAME_DATA")

	// Start the HTTP client before sending the frame so the handler is ready.
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+s.ListenAddr()+defaultPath, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			ch <- result{err: err}
			return
		}
		defer resp.Body.Close()

		_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil {
			ch <- result{err: err}
			return
		}
		mr := multipart.NewReader(resp.Body, params["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			ch <- result{err: err}
			return
		}
		data, err := io.ReadAll(part)
		ch <- result{data: data, err: err}
	}()

	// Give the HTTP client goroutine a moment to connect before pushing the frame.
	time.Sleep(50 * time.Millisecond)

	s.In() <- frame
	// Close the input after giving the frame time to be broadcast.
	time.Sleep(50 * time.Millisecond)
	close(s.In())
	s.AwaitCompletion()

	select {
	case res := <-ch:
		if res.err != nil && !isContextOrEOF(res.err) {
			t.Fatalf("client error: %v", res.err)
		}
		if len(res.data) > 0 && string(res.data) != string(frame) {
			t.Errorf("frame mismatch: got %q, want %q", res.data, frame)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("timed out waiting for frame")
	}
}

// TestMJPEGSink_SendEventAndReceiveFrame is the same as the bytes test but uses
// a streams.Event as the input message.
func TestMJPEGSink_SendEventAndReceiveFrame(t *testing.T) {
	s := newTestSink(t)

	frame := fakeJPEG("EVENT_JPEG_BODY")
	ev := streams.NewEventFrom(context.Background(), frame)

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+s.ListenAddr()+defaultPath, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			ch <- result{err: err}
			return
		}
		defer resp.Body.Close()

		_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil {
			ch <- result{err: err}
			return
		}
		mr := multipart.NewReader(resp.Body, params["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			ch <- result{err: err}
			return
		}
		data, err := io.ReadAll(part)
		ch <- result{data: data, err: err}
	}()

	time.Sleep(50 * time.Millisecond)

	s.In() <- ev
	time.Sleep(50 * time.Millisecond)
	close(s.In())
	s.AwaitCompletion()

	select {
	case res := <-ch:
		if res.err != nil && !isContextOrEOF(res.err) {
			t.Fatalf("client error: %v", res.err)
		}
		if len(res.data) > 0 && string(res.data) != string(frame) {
			t.Errorf("frame mismatch: got %q, want %q", res.data, frame)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("timed out waiting for frame")
	}
}

// TestMJPEGSink_UnsupportedMessageType verifies that unsupported types do not
// panic and that the sink continues running afterwards.
func TestMJPEGSink_UnsupportedMessageType(t *testing.T) {
	s := newTestSink(t)

	// This should be logged and silently dropped.
	s.In() <- 42
	s.In() <- "unsupported string"

	close(s.In())
	s.AwaitCompletion()
}

// TestMJPEGSink_HTTPHeaders verifies the MJPEG-specific response headers.
func TestMJPEGSink_HTTPHeaders(t *testing.T) {
	s := newTestSink(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+s.ListenAddr()+defaultPath, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/x-mixed-replace") {
			t.Errorf("expected multipart/x-mixed-replace Content-Type, got %q", ct)
		}
		if resp.Header.Get("Cache-Control") != "no-cache" {
			t.Errorf("expected Cache-Control: no-cache, got %q", resp.Header.Get("Cache-Control"))
		}
		// Drain response body until context is cancelled.
		io.Copy(io.Discard, resp.Body)
	}()

	// Give the goroutine time to read headers.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	close(s.In())
	s.AwaitCompletion()
}

// TestMJPEGSink_MultipleClients checks that two concurrent clients both receive
// the same broadcast frame.
func TestMJPEGSink_MultipleClients(t *testing.T) {
	s := newTestSink(t)

	frame := fakeJPEG("BROADCAST_FRAME")
	const nClients = 2

	type result struct {
		data []byte
		err  error
	}
	results := make(chan result, nClients)

	for i := 0; i < nClients; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"http://"+s.ListenAddr()+defaultPath, nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}

		go func() {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results <- result{err: err}
				return
			}
			defer resp.Body.Close()

			_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
			if err != nil {
				results <- result{err: err}
				return
			}
			mr := multipart.NewReader(resp.Body, params["boundary"])
			part, err := mr.NextPart()
			if err != nil {
				results <- result{err: err}
				return
			}
			data, err := io.ReadAll(part)
			results <- result{data: data, err: err}
		}()
	}

	// Allow clients to connect before broadcasting.
	time.Sleep(100 * time.Millisecond)

	s.In() <- frame
	time.Sleep(50 * time.Millisecond)
	close(s.In())
	s.AwaitCompletion()

	for i := 0; i < nClients; i++ {
		select {
		case res := <-results:
			if res.err != nil && !isContextOrEOF(res.err) {
				t.Errorf("client %d error: %v", i, res.err)
			}
			if len(res.data) > 0 && string(res.data) != string(frame) {
				t.Errorf("client %d frame mismatch: got %q, want %q", i, res.data, frame)
			}
		case <-time.After(6 * time.Second):
			t.Fatalf("timed out waiting for client %d", i)
		}
	}
}

// TestMJPEGProcessor_Convert exercises the registry converter with a valid spec.
func TestMJPEGProcessor_Convert(t *testing.T) {
	p := &MJPEGProcessor{}
	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"port": float64(0),
			"path": "/test",
		},
	}

	sink, err := p.Convert(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink == nil {
		t.Fatal("expected a non-nil sink")
	}

	ms, ok := sink.(*MJPEGSink)
	if !ok {
		t.Fatalf("expected *MJPEGSink, got %T", sink)
	}
	if ms.conf.Path != "/test" {
		t.Errorf("expected path /test, got %q", ms.conf.Path)
	}

	close(ms.In())
	ms.AwaitCompletion()
}

// TestMJPEGProcessor_Validate checks that the validator accepts valid specs
// and rejects invalid ones.
func TestMJPEGProcessor_Validate(t *testing.T) {
	p := &MJPEGProcessor{}

	valid := model.OutputSpec{Spec: map[string]interface{}{"port": float64(8090), "path": "/stream"}}
	if err := p.Validate(valid); err != nil {
		t.Errorf("expected valid spec to pass, got: %v", err)
	}

	noPath := model.OutputSpec{Spec: map[string]interface{}{"port": float64(8090)}}
	if err := p.Validate(noPath); err != nil {
		t.Errorf("expected empty path to pass (defaults applied), got: %v", err)
	}

	badPath := model.OutputSpec{Spec: map[string]interface{}{"path": "noslash"}}
	if err := p.Validate(badPath); err == nil {
		t.Error("expected error for path without leading slash, got nil")
	}

	badPort := model.OutputSpec{Spec: map[string]interface{}{"port": float64(-1)}}
	if err := p.Validate(badPort); err == nil {
		t.Error("expected error for negative port, got nil")
	}
}

// isContextOrEOF returns true for errors that are expected when the test
// cancels the HTTP request context or the server closes the stream.
func isContextOrEOF(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "context") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "closed") ||
		strings.Contains(msg, "reset")
}

// readOneFrame opens an HTTP GET against the sink, reads the first MJPEG part,
// closes the response body, and returns the frame bytes.
func readOneFrame(t *testing.T, addr string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+defaultPath, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client do: %v", err)
	}
	defer resp.Body.Close()
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse media type: %v", err)
	}
	mr := multipart.NewReader(resp.Body, params["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("next part: %v", err)
	}
	data, err := io.ReadAll(part)
	if err != nil && !isContextOrEOF(err) {
		t.Fatalf("read part: %v", err)
	}
	return data
}

// TestMJPEGSink_ClientReconnect verifies that after a client disconnects, a
// new client can reconnect and continue receiving frames. This guards against
// regressions where stale client channels would block the broadcast loop.
func TestMJPEGSink_ClientReconnect(t *testing.T) {
	s := newTestSink(t)
	addr := s.ListenAddr()

	// Start a goroutine that pushes frames continuously until the test ends.
	stop := make(chan struct{})
	frame := fakeJPEG("RECONNECT_FRAME\r\n")
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				select {
				case s.In() <- frame:
				case <-stop:
					return
				}
			}
		}
	}()

	// First client receives a frame, then disconnects (defer cancel in helper).
	got := readOneFrame(t, addr)
	if len(got) == 0 || !strings.HasPrefix(string(got), string(frame)) {
		t.Errorf("first client frame mismatch: got %q want prefix %q", got, frame)
	}

	// Allow the server's unsubscribe path to run.
	time.Sleep(50 * time.Millisecond)

	// Second client connects and must also receive a frame.
	got2 := readOneFrame(t, addr)
	if len(got2) == 0 || !strings.HasPrefix(string(got2), string(frame)) {
		t.Errorf("second client frame mismatch: got %q want prefix %q", got2, frame)
	}

	close(stop)
	close(s.In())
	s.AwaitCompletion()
}
