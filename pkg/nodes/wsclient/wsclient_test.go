// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package wsclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

func TestWebSocketClient_ReceiveAndForward(t *testing.T) {
	upgrader := ws.Upgrader{}
	// server that accepts a connection, sends a message, then closes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		// send message to client
		conn.WriteMessage(ws.TextMessage, []byte("hello-from-server"))
		// close
		conn.WriteMessage(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseNormalClosure, "bye"))
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conf := NewConfiguration(wsURL, nil, nil, ws.TextMessage)
	client, err := NewWebSocketClient(*conf)
	if err != nil {
		t.Fatalf("NewWebSocketClient error: %v", err)
	}

	// read from client.Out channel with timeout
	select {
	case msg := <-client.Out():
		if msg == nil {
			t.Fatalf("expected message event, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for message from server")
	}
}

func TestNewConfigurationCopies(t *testing.T) {
	params := map[string]string{"a": "b"}
	headers := map[string]string{"h": "v"}
	conf := NewConfiguration("ws://example", params, headers, ws.TextMessage)
	if conf.URL != "ws://example" {
		t.Fatalf("expected URL copied, got: %s", conf.URL)
	}
	if conf.MsgType != ws.TextMessage {
		t.Fatalf("expected MsgType set")
	}
}

func TestWSProcessorValidate(t *testing.T) {
	proc := &WSProcessor{}
	spec := model.NodeConfig{Kind: "websocket", Spec: map[string]interface{}{"url": ""}}
	if err := proc.Validate(&spec); err == nil {
		t.Fatalf("expected validation error for missing url")
	}
	// valid
	spec = model.NodeConfig{Kind: "websocket", Spec: map[string]interface{}{"url": "ws://example", "msgType": float64(ws.TextMessage)}}
	if err := proc.Validate(&spec); err != nil {
		t.Fatalf("expected no error for valid spec, got %v", err)
	}
	// invalid msgType
	spec = model.NodeConfig{Kind: "websocket", Spec: map[string]interface{}{"url": "ws://example", "msgType": float64(99)}}
	if err := proc.Validate(&spec); err == nil {
		t.Fatalf("expected validation error for invalid msgType")
	}
}

func TestWSProcessorConvert(t *testing.T) {
	upgrader := ws.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]

	proc := &WSProcessor{}

	spec := model.NodeConfig{
		Kind: "websocket",
		Spec: map[string]interface{}{
			"url":     wsURL,
			"msgType": float64(ws.TextMessage),
		},
	}

	flow, err := proc.Convert(&spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if flow == nil {
		t.Fatalf("expected flow, got nil")
	}
	wsc, ok := flow.(*WebSocketClient)
	if !ok {
		t.Fatalf("expected WebSocketClient, got %T", flow)
	}
	if wsc.conf.URL != wsURL {
		t.Errorf("expected URL %s, got %s", wsURL, wsc.conf.URL)
	}
	// Close
	close(wsc.In())
}

func TestWebSocketClient_Via(t *testing.T) {
	upgrader := ws.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		// Send message
		conn.WriteMessage(ws.TextMessage, []byte("response"))
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]

	conf := NewConfiguration(wsURL, nil, nil, ws.TextMessage)
	wsc, err := NewWebSocketClient(*conf)
	if err != nil {
		t.Fatalf("NewWebSocketClient error: %v", err)
	}

	// Mock flow
	mockFlow := &mockFlow{in: make(chan any, 1)}
	result := wsc.Via(mockFlow)

	if result == nil {
		t.Fatalf("expected flow returned")
	}

	// Check if transmitted
	select {
	case msg := <-mockFlow.in:
		if msg == nil {
			t.Errorf("expected message, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Errorf("expected message transmitted")
	}
	// Close
	close(wsc.In())
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
