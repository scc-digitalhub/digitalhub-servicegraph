package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

func TestCall_Success(t *testing.T) {
	// start test server that returns 200 and a JSON body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	conf := NewConfiguration(srv.URL, nil, nil, 1)
	wh := NewWebHook(*conf)
	// use server's client to ensure proper transport
	wh.httpClient = *srv.Client()

	// create an event and call
	evt := streams.NewEventFrom([]byte("{}"))
	res := wh.call(evt)
	if res == nil {
		t.Fatalf("expected response event, got nil")
	}
	body := res.GetBody()
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if _, ok := parsed["ok"]; !ok {
		t.Fatalf("expected ok field in response")
	}
}

func TestCall_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer srv.Close()

	conf := NewConfiguration(srv.URL, nil, nil, 1)
	wh := NewWebHook(*conf)
	wh.httpClient = *srv.Client()

	evt := streams.NewEventFrom([]byte("{}"))
	res := wh.call(evt)
	if res == nil {
		t.Fatalf("expected error event, got nil")
	}
	body := res.GetBody()
	if len(body) == 0 {
		t.Fatalf("expected non-empty body for error response")
	}
}

func TestCall_GroundError(t *testing.T) {
	// invalid URL to provoke grounding error (url.Parse should fail)
	conf := NewConfiguration(":", nil, nil, 1)
	wh := NewWebHook(*conf)

	res := wh.call(streams.NewEventFrom(nil))
	if res == nil {
		t.Fatalf("expected error event, got nil")
	}
}

func TestWebHookConverter_Convert(t *testing.T) {
	converter := &WebHookProcessor{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"url":         "http://example.com",
			"params":      map[string]string{"key": "value"},
			"headers":     map[string]string{"Content-Type": "application/json"},
			"parallelism": 2,
		},
	}

	sink, err := converter.Convert(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink == nil {
		t.Fatalf("expected sink, got nil")
	}
	wh, ok := sink.(*WebHook)
	if !ok {
		t.Fatalf("expected WebHook, got %T", sink)
	}
	if wh.conf.URL != "http://example.com" {
		t.Errorf("expected URL %s, got %s", "http://example.com", wh.conf.URL)
	}
	if wh.conf.Params["key"] != "value" {
		t.Errorf("expected param key=value, got %v", wh.conf.Params)
	}
	if wh.conf.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected header Content-Type=application/json, got %v", wh.conf.Headers)
	}
	if wh.conf.Parallelism != 2 {
		t.Errorf("expected parallelism 2, got %d", wh.conf.Parallelism)
	}
}

func TestWebHookConverter_Convert_InvalidSpec(t *testing.T) {
	converter := &WebHookProcessor{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"url":         "http://example.com",
			"parallelism": "invalid", // should be int
		},
	}

	_, err := converter.Convert(spec)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestWebHook_InAndAwaitCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received":true}`))
	}))
	defer srv.Close()

	conf := NewConfiguration(srv.URL, nil, nil, 1)
	wh := NewWebHook(*conf)
	wh.httpClient = *srv.Client()

	// Send data through In channel
	wh.In() <- []byte(`{"test":true}`)
	close(wh.In())

	// Await completion
	wh.AwaitCompletion()

	// Since it's async, we can't easily check the result, but at least cover the methods
}

func TestNewConfiguration(t *testing.T) {
	conf := NewConfiguration("http://test.com", map[string]string{"p1": "v1"}, map[string]string{"h1": "v1"}, 5)
	if conf.URL != "http://test.com" {
		t.Errorf("expected URL http://test.com, got %s", conf.URL)
	}
	if conf.Params["p1"] != "v1" {
		t.Errorf("expected param p1=v1, got %v", conf.Params)
	}
	if conf.Headers["h1"] != "v1" {
		t.Errorf("expected header h1=v1, got %v", conf.Headers)
	}
	if conf.Parallelism != 5 {
		t.Errorf("expected parallelism 5, got %d", conf.Parallelism)
	}
}

func TestNewConfiguration_NilMaps(t *testing.T) {
	conf := NewConfiguration("http://test.com", nil, nil, 0)
	if len(conf.Params) != 0 {
		t.Errorf("expected empty params, got %v", conf.Params)
	}
	if len(conf.Headers) != 0 {
		t.Errorf("expected empty headers, got %v", conf.Headers)
	}
	if conf.Parallelism != 0 {
		t.Errorf("expected parallelism 0, got %d", conf.Parallelism)
	}
}

func TestConfiguration_Ground(t *testing.T) {
	conf := NewConfiguration("http://test.com", map[string]string{"p1": "v1"}, map[string]string{"h1": "v1"}, 1)

	in, err := streams.NewGenericEvent([]byte("body"), "url", "GET", map[string]string{"inh": "inv"}, map[string]string{"inf": "inv"}, 200)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	out, err := conf.Ground(in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.GetURL() != "http://test.com" {
		t.Errorf("expected URL http://test.com, got %s", out.GetURL())
	}
	if out.GetHeaders()["h1"] != "v1" {
		t.Errorf("expected header h1=v1, got %v", out.GetHeaders())
	}
	if out.GetHeaders()["inh"] != "inv" {
		t.Errorf("expected inherited header inh=inv, got %v", out.GetHeaders())
	}
	if out.GetFields()["p1"] != "v1" {
		t.Errorf("expected field p1=v1, got %v", out.GetFields())
	}
	if out.GetFields()["inf"] != "inv" {
		t.Errorf("expected inherited field inf=inv, got %v", out.GetFields())
	}
}

func TestConfiguration_GroundEmpty(t *testing.T) {
	conf := NewConfiguration("http://test.com", nil, nil, 1)

	in, err := streams.NewGenericEvent([]byte("body"), "url", "GET", map[string]string{"inh": "inv"}, map[string]string{"inf": "inv"}, 200)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	out, err := conf.Ground(in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.GetURL() != "http://test.com" {
		t.Errorf("expected URL http://test.com, got %s", out.GetURL())
	}
	if out.GetHeaders()["inh"] != "inv" {
		t.Errorf("expected inherited header inh=inv, got %v", out.GetHeaders())
	}
	if out.GetFields()["inf"] != "inv" {
		t.Errorf("expected inherited field inf=inv, got %v", out.GetFields())
	}
}

func TestConfiguration_Ground_NilHeadersFields(t *testing.T) {
	conf := NewConfiguration("http://test.com", map[string]string{"p1": "v1"}, map[string]string{"h1": "v1"}, 1)

	in, err := streams.NewGenericEvent([]byte("body"), "url", "GET", nil, nil, 200)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	out, err := conf.Ground(in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.GetHeaders()["h1"] != "v1" {
		t.Errorf("expected header h1=v1, got %v", out.GetHeaders())
	}
	if out.GetFields()["p1"] != "v1" {
		t.Errorf("expected field p1=v1, got %v", out.GetFields())
	}
}

func TestConfiguration_Ground_EmptyConfMaps(t *testing.T) {
	conf := NewConfiguration("http://test.com", nil, nil, 1)

	in, err := streams.NewGenericEvent([]byte("body"), "url", "GET", map[string]string{"inh": "inv"}, map[string]string{"inf": "inv"}, 200)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	out, err := conf.Ground(in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.GetHeaders()["inh"] != "inv" {
		t.Errorf("expected inherited header inh=inv, got %v", out.GetHeaders())
	}
	if out.GetFields()["inf"] != "inv" {
		t.Errorf("expected inherited field inf=inv, got %v", out.GetFields())
	}
}

func TestConfiguration_Ground_InvalidURL(t *testing.T) {
	conf := NewConfiguration(":", nil, nil, 1) // invalid URL

	in, err := streams.NewGenericEvent([]byte("body"), "url", "GET", nil, nil, 200)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	_, err = conf.Ground(in)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
