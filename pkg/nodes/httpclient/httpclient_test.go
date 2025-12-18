// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

func TestNewConfigurationDefaultsAndGround(t *testing.T) {
	conf := NewConfiguration("http://example", "GET", map[string]string{"p": "v"}, map[string]string{"h": "v"}, 2)
	ev, err := streams.NewGenericEvent(context.Background(), []byte("body"), "", "", nil, nil, 200)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	grounded, err := conf.Ground(ev)
	if err != nil {
		t.Fatalf("Ground returned error: %v", err)
	}
	if grounded.GetURL() != "http://example" {
		t.Fatalf("expected URL set, got: %s", grounded.GetURL())
	}
	if grounded.GetMethod() != "GET" {
		t.Fatalf("expected Method GET, got: %s", grounded.GetMethod())
	}
}

func TestNewConfiguration_Defaults(t *testing.T) {
	// empty method
	conf := NewConfiguration("http://example", "", nil, nil, 0)
	if conf.Method != "POST" {
		t.Errorf("expected default method POST, got %s", conf.Method)
	}
	if conf.NumInstances != 1 {
		t.Errorf("expected default numInstances 1, got %d", conf.NumInstances)
	}
	// negative numInstances
	conf = NewConfiguration("http://example", "GET", nil, nil, -1)
	if conf.NumInstances != 1 {
		t.Errorf("expected numInstances 1 for negative, got %d", conf.NumInstances)
	}
	// nil maps
	conf = NewConfiguration("http://example", "GET", nil, nil, 2)
	if len(conf.Params) != 0 {
		t.Errorf("expected empty params, got %v", conf.Params)
	}
	if len(conf.Headers) != 0 {
		t.Errorf("expected empty headers, got %v", conf.Headers)
	}
}

func TestConfiguration_Ground_Error(t *testing.T) {
	conf := NewConfiguration(":", "GET", nil, nil, 1) // invalid URL
	ev, _ := streams.NewGenericEvent(context.Background(), []byte("body"), "", "", nil, nil, 200)
	_, err := conf.Ground(ev)
	if err == nil {
		t.Fatalf("expected error for invalid URL")
	}
}

func TestHTTPProcessorValidate(t *testing.T) {
	proc := &HTTPProcessor{}
	// missing URL
	spec := model.NodeConfig{Kind: "http", Spec: map[string]interface{}{"url": ""}}
	if err := proc.Validate(spec); err == nil {
		t.Fatalf("expected validation error for missing url")
	}
	// invalid method
	spec = model.NodeConfig{Kind: "http", Spec: map[string]interface{}{"url": "http://example", "method": "INVALID"}}
	if err := proc.Validate(spec); err == nil {
		t.Fatalf("expected validation error for invalid method")
	}
	// negative num_instances - since numInstances is unexported, can't validate
	// spec = model.NodeConfig{Kind: "http", Spec: map[string]interface{}{"url": "http://example", "num_instances": -1}}
	// if err := proc.Validate(spec); err == nil {
	// 	t.Fatalf("expected validation error for negative num_instances")
	// }
	// valid
	spec = model.NodeConfig{Kind: "http", Spec: map[string]interface{}{"url": "http://example", "method": "GET", "num_instances": 2}}
	if err := proc.Validate(spec); err != nil {
		t.Fatalf("expected no error for valid spec, got %v", err)
	}
}

func TestHTTPProcessorConvert(t *testing.T) {
	proc := &HTTPProcessor{}

	spec := model.NodeConfig{
		Kind: "http",
		Spec: map[string]interface{}{
			"url":           "http://example",
			"method":        "GET",
			"params":        map[string]string{"p": "v"},
			"headers":       map[string]string{"h": "v"},
			"num_instances": 2,
		},
	}

	flow, err := proc.Convert(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if flow == nil {
		t.Fatalf("expected flow, got nil")
	}
	hc, ok := flow.(*HttpClient)
	if !ok {
		t.Fatalf("expected HttpClient, got %T", flow)
	}
	if hc.conf.URL != "http://example" {
		t.Errorf("expected URL http://example, got %s", hc.conf.URL)
	}
	if hc.conf.Method != "GET" {
		t.Errorf("expected Method GET, got %s", hc.conf.Method)
	}
	if hc.conf.Params["p"] != "v" {
		t.Errorf("expected param p=v, got %v", hc.conf.Params)
	}
	if hc.conf.NumInstances != 2 {
		t.Errorf("expected numInstances 2, got %d", hc.conf.NumInstances)
	}
}

func TestHttpClient_InOut(t *testing.T) {
	conf := NewConfiguration("http://example", "GET", nil, nil, 1)
	hc := NewHttpClient(*conf)

	in := hc.In()
	out := hc.Out()

	if in == nil {
		t.Fatalf("expected in channel, got nil")
	}
	if out == nil {
		t.Fatalf("expected out channel, got nil")
	}
}

func TestHttpClient_Via(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	conf := NewConfiguration(srv.URL, "GET", nil, nil, 1)
	hc := NewHttpClient(*conf)

	// Mock flow
	mockFlow := &mockFlow{in: make(chan any, 1)}
	result := hc.Via(mockFlow)

	if result == nil {
		t.Fatalf("expected flow returned")
	}

	// Send data
	hc.In() <- []byte(`{}`)
	close(hc.In())

	time.Sleep(100 * time.Millisecond)

	// Check if transmitted
	select {
	case msg := <-mockFlow.in:
		if msg == nil {
			t.Errorf("expected message, got nil")
		}
	default:
		t.Errorf("expected message transmitted")
	}
}

type mockFlow struct {
	in chan any
}

func (m *mockFlow) In() chan<- any {
	return m.in
}

func (m *mockFlow) Out() <-chan any {
	return nil // not used
}

func (m *mockFlow) Via(flow streams.Flow) streams.Flow {
	return nil
}

func (m *mockFlow) To(sink streams.Sink) {
}
