// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ws "github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

func TestWebSocketSink_SendReceive(t *testing.T) {
	upgrader := ws.Upgrader{}
	// echo server that upgrades and reads one message
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read error: %v", err)
			return
		}
		// echo back
		conn.WriteMessage(mt, msg)
	}))
	defer srv.Close()

	// convert http URL to ws URL
	wsURL := "ws" + srv.URL[len("http"):]

	conf := NewConfiguration(wsURL, nil, nil, ws.TextMessage)

	// create sink with default dialer
	sink, err := NewWebSocketSink(*conf)
	if err != nil {
		t.Fatalf("failed to create websocket sink: %v", err)
	}

	// send a message and close input
	sink.In() <- []byte("hello")
	close(sink.In())
	sink.AwaitCompletion()
}

func TestWebSocketSink_ProcessTypes(t *testing.T) {
	upgrader := ws.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]

	conf := NewConfiguration(wsURL, nil, nil, ws.TextMessage)

	sink, err := NewWebSocketSink(*conf)
	if err != nil {
		t.Fatalf("failed to create websocket sink: %v", err)
	}

	// send different types
	sink.In() <- "string"
	sink.In() <- []byte("bytes")
	ev, _ := streams.NewGenericEvent([]byte("body"), "url", "GET", nil, nil, 200)
	sink.In() <- ev
	sink.In() <- 123 // default
	close(sink.In())
	sink.AwaitCompletion()
}

func TestWebSocketConverter_Convert(t *testing.T) {
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

	converter := &WebSocketProcessor{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"url":      wsURL,
			"msg_type": float64(ws.TextMessage),
		},
	}

	sink, err := converter.Convert(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink == nil {
		t.Fatalf("expected sink, got nil")
	}
	wsSink, ok := sink.(*WebSocketSink)
	if !ok {
		t.Fatalf("expected WebSocketSink, got %T", sink)
	}
	if wsSink.conf.URL != wsURL {
		t.Errorf("expected URL %s, got %s", wsURL, wsSink.conf.URL)
	}
	if wsSink.conf.MsgType != ws.TextMessage {
		t.Errorf("expected MsgType %d, got %d", ws.TextMessage, wsSink.conf.MsgType)
	}
	// Close the sink
	close(wsSink.In())
	wsSink.AwaitCompletion()
}

func TestNewConfiguration(t *testing.T) {
	conf := NewConfiguration("ws://test.com", map[string]string{"p1": "v1"}, map[string]string{"h1": "v1"}, ws.TextMessage)
	if conf.URL != "ws://test.com" {
		t.Errorf("expected URL ws://test.com, got %s", conf.URL)
	}
	if conf.Params["p1"] != "v1" {
		t.Errorf("expected param p1=v1, got %v", conf.Params)
	}
	if conf.Headers["h1"] != "v1" {
		t.Errorf("expected header h1=v1, got %v", conf.Headers)
	}
	if conf.MsgType != ws.TextMessage {
		t.Errorf("expected MsgType %d, got %d", ws.TextMessage, conf.MsgType)
	}
}

func TestNewConfiguration_NilMaps(t *testing.T) {
	conf := NewConfiguration("ws://test.com", nil, nil, ws.BinaryMessage)
	if len(conf.Params) != 0 {
		t.Errorf("expected empty params, got %v", conf.Params)
	}
	if len(conf.Headers) != 0 {
		t.Errorf("expected empty headers, got %v", conf.Headers)
	}
	if conf.MsgType != ws.BinaryMessage {
		t.Errorf("expected MsgType %d, got %d", ws.BinaryMessage, conf.MsgType)
	}
}
