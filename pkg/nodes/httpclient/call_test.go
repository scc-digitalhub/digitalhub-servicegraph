package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

func makeClientWithServer(ts *httptest.Server) *HttpClient {
	conf := Configuration{URL: ts.URL, Method: "GET", NumInstances: 1}
	hc := NewHttpClient(conf)
	// replace default http client with one that hits the test server
	hc.httpClient = *ts.Client()
	return hc
}

func TestCall_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok-body")
	}))
	defer ts.Close()

	hc := makeClientWithServer(ts)
	// prepare an event with body
	ev := streams.NewEventFrom([]byte("in"))
	out := hc.call(ev)
	if string(out.GetBody()) != "ok-body" {
		t.Fatalf("expected ok-body, got: %s", string(out.GetBody()))
	}
}

func TestCall_NetworkError(t *testing.T) {
	// create a client that points to a non-routable address to force error
	conf := Configuration{URL: "http://127.0.0.1:0", Method: "GET", NumInstances: 1}
	hc := NewHttpClient(conf)
	// call with an event
	out := hc.call(streams.NewEventFrom([]byte("in")))
	// expect an error event (contains {"error": ...})
	if out.GetStatus() == 200 {
		t.Fatalf("expected non-200 status on network error")
	}
}
