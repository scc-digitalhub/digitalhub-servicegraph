// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package websocket_test

import (
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	wspkg "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

var testLock sync.Mutex

type testFactory struct {
	eventChan chan any
}

func (f *testFactory) GenerateFlow(source streams.Source, sink streams.Sink) {
	go func() {
		for v := range source.Out() {
			f.eventChan <- v
			// Send a response
			sink.In() <- "response"
		}
		if ch, ok := any(sink).(interface{ In() chan any }); ok {
			close(ch.In())
		}
	}()
}

func TestNewConfigurationDefaults(t *testing.T) {
	c := wspkg.NewConfiguration(0, 0)
	if c.Port == 0 {
		t.Fatalf("expected default port to be set")
	}
	if c.Capacity == 0 {
		t.Fatalf("expected default capacity to be set")
	}
}

func TestNewConfigurationNonDefaults(t *testing.T) {
	c := wspkg.NewConfiguration(9090, 20)
	if c.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", c.Port)
	}
	if c.Capacity != 20 {
		t.Fatalf("expected capacity 20, got %d", c.Capacity)
	}
}

func TestNewSocketEvent(t *testing.T) {
	event, err := wspkg.NewSocketEvent([]byte("test"), websocket.TextMessage)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(event.GetBody()) != "test" {
		t.Fatalf("expected body 'test', got %s", string(event.GetBody()))
	}
	if event.GetContentType() != "text/plain" {
		t.Fatalf("expected 'text/plain', got %s", event.GetContentType())
	}
	if event.GetTimestamp().IsZero() {
		t.Fatalf("expected timestamp set")
	}
}

func TestSocketEventBinary(t *testing.T) {
	event, _ := wspkg.NewSocketEvent([]byte("test"), websocket.BinaryMessage)
	if event.GetContentType() != "application/octet-stream" {
		t.Fatalf("expected 'application/octet-stream', got %s", event.GetContentType())
	}
}

func TestNewWSSource(t *testing.T) {
	conf := wspkg.NewConfiguration(8080, 10)
	src := wspkg.NewWSSource(conf)
	if src.Conf.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", src.Conf.Port)
	}
}

func TestWebSocketProcessorValidate(t *testing.T) {
	processor := &wspkg.WebSocketProcessor{}
	// Valid
	input := model.InputSpec{
		Spec: map[string]interface{}{
			"port":     8080,
			"capacity": 10,
		},
	}
	err := processor.Validate(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Invalid port
	input.Spec["port"] = 0
	err = processor.Validate(input)
	if err == nil {
		t.Fatalf("expected error for invalid port")
	}
	input.Spec["port"] = 8080
	// Invalid capacity
	input.Spec["capacity"] = 0
	err = processor.Validate(input)
	if err == nil {
		t.Fatalf("expected error for invalid capacity")
	}
}

func TestWebSocketProcessorConvert(t *testing.T) {
	processor := &wspkg.WebSocketProcessor{}
	input := model.InputSpec{
		Spec: map[string]interface{}{
			"port":     8080,
			"capacity": 10,
		},
	}
	src, err := processor.Convert(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if src == nil {
		t.Fatalf("expected source not nil")
	}
	wsSrc, ok := src.(*wspkg.WSSource)
	if !ok {
		t.Fatalf("expected WSSource, got %T", src)
	}
	if wsSrc.Conf.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", wsSrc.Conf.Port)
	}
}

func TestWSSource(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()
	conf := wspkg.NewConfiguration(8080, 10)
	src := wspkg.NewWSSource(conf)
	eventChan := make(chan any, 10)
	factory := &testFactory{eventChan: eventChan}
	go src.Start(factory)
	time.Sleep(100 * time.Millisecond)
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()
	// Send binary message
	err = conn.WriteMessage(websocket.BinaryMessage, []byte("test"))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	// Receive response
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if string(msg) != "response" {
		t.Fatalf("expected 'response', got %s", string(msg))
	}
	// Check event received
	select {
	case event := <-eventChan:
		if se, ok := event.(*wspkg.SocketEvent); ok {
			if string(se.GetBody()) != "test" {
				t.Fatalf("expected 'test', got %s", string(se.GetBody()))
			}
			if se.GetContentType() != "application/octet-stream" {
				t.Fatalf("expected 'application/octet-stream', got %s", se.GetContentType())
			}
		} else {
			t.Fatalf("expected SocketEvent, got %T", event)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for event")
	}
	// Close conn to trigger cleanConnection
	conn.Close()
	time.Sleep(100 * time.Millisecond)
}
