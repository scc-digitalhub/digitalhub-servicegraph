// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package streams

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"
)

func TestNewGenericEvent_Basics(t *testing.T) {
	ctx := context.Background()

	headers := map[string]string{"content-type": "application/json", "x-k": "v"}
	fields := map[string]string{"f1": "v1"}
	body := []byte("{\"a\":1}")
	e, err := NewGenericEvent(ctx, body, "http://example.com/path?x=1", "POST", headers, fields, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(e.GetBody()) != string(body) {
		t.Fatalf("body mismatch: %s", string(e.GetBody()))
	}
	if e.GetURL() == "" || e.GetPath() != "/path" {
		t.Fatalf("url/path incorrect: %s %s", e.GetURL(), e.GetPath())
	}
	if e.GetMethod() != "POST" {
		t.Fatalf("method incorrect: %s", e.GetMethod())
	}
	if ct := e.GetContentType(); ct != "application/json" {
		t.Fatalf("content-type wrong: %s", ct)
	}
	if got := e.GetField("f1"); got != "v1" {
		t.Fatalf("field wrong: %s", got)
	}
	if e.GetStatus() != 200 {
		t.Fatalf("default status expected 200, got %d", e.GetStatus())
	}
}

func TestNewEventFrom_Types(t *testing.T) {
	ctx := context.Background()
	ev := NewEventFrom(ctx, "hello")
	if string(ev.GetBody()) != "hello" {
		t.Fatalf("string body mismatch: %s", string(ev.GetBody()))
	}

	b := []byte("bytes")
	ev2 := NewEventFrom(ctx, b)
	if string(ev2.GetBody()) != "bytes" {
		t.Fatalf("byte body mismatch: %s", string(ev2.GetBody()))
	}

	orig, _ := NewGenericEvent(ctx, []byte("X"), "", "GET", nil, nil, 201)
	ev3 := NewEventFrom(ctx, orig)
	if ev3.GetStatus() != 201 {
		t.Fatalf("event passthrough status mismatch: %d", ev3.GetStatus())
	}

	ev4 := NewEventFrom(ctx, 123)
	if string(ev4.GetBody()) == "" {
		t.Fatalf("expected non-empty body for int input")
	}
}

func TestNewErrorEvent(t *testing.T) {
	ctx := context.Background()
	err := errors.New("boom")
	ev := NewErrorEvent(ctx, err, 502)
	if ev.GetStatus() != 502 {
		t.Fatalf("error event status mismatch: %d", ev.GetStatus())
	}
	if string(ev.GetBody()) == "" {
		t.Fatalf("error event body expected")
	}
}

func TestGenericEvent_ID_Headers_Fields_Timestamp_And_ErrorURL(t *testing.T) {
	ctx := context.Background()

	// invalid URL returns error
	if _, err := NewGenericEvent(ctx, []byte("x"), ":http://bad-url", "GET", nil, nil, 0); err == nil {
		t.Fatalf("expected error for invalid url")
	}

	headers := map[string]string{"content-type": "text/plain", "h1": "v1"}
	fields := map[string]string{"f1": "fv1"}
	body := []byte("payload")
	ge, err := NewGenericEvent(ctx, body, "http://example.com/hey", "POST", headers, fields, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ID set/get
	ge.SetID(ID("my-id"))
	if ge.GetID() != ID("my-id") {
		t.Fatalf("id mismatch: %v", ge.GetID())
	}

	// headers / fields / content-type / method / path / url
	if ge.GetHeader("h1") != "v1" {
		t.Fatalf("header mismatch")
	}
	if ge.GetField("f1") != "fv1" {
		t.Fatalf("field mismatch")
	}
	if ge.GetContentType() != "text/plain" {
		t.Fatalf("content-type mismatch: %s", ge.GetContentType())
	}
	if ge.GetMethod() != "POST" {
		t.Fatalf("method mismatch: %s", ge.GetMethod())
	}
	if ge.GetPath() != "/hey" {
		t.Fatalf("path mismatch: %s", ge.GetPath())
	}
	if ge.GetURL() == "" {
		t.Fatalf("url expected")
	}

	// GetBodyObject returns the body as []byte
	bo := ge.GetBodyObject()
	if string(bo.([]byte)) != string(body) {
		t.Fatalf("body object mismatch: %v", bo)
	}

	// timestamp is recent
	ts := ge.GetTimestamp()
	if ts.IsZero() {
		t.Fatalf("timestamp should be set")
	}
	if time.Since(ts) > time.Second*5 {
		t.Fatalf("timestamp too old: %v", ts)
	}

	// topic returns empty
	if ge.GetTopic() != "" {
		t.Fatalf("expected empty topic")
	}
}

func TestGenericEvent_EmptyHeadersAndFields(t *testing.T) {
	ctx := context.Background()
	// Create without headers/fields
	ge, err := NewGenericEvent(ctx, []byte("x"), "", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// content-type missing
	if ge.GetContentType() != "" {
		t.Fatalf("expected empty content-type when not set")
	}

	// GetHeaders should be a map (possibly empty)
	headers := ge.GetHeaders()
	if headers == nil {
		t.Fatalf("expected headers map, got nil")
	}

	// unknown header returns empty string
	if ge.GetHeader("nope") != "" {
		t.Fatalf("expected empty for missing header")
	}

	// GetFields should be a map (possibly empty)
	fields := ge.GetFields()
	if fields == nil {
		t.Fatalf("expected fields map, got nil")
	}

	if ge.GetField("nope") != "" {
		t.Fatalf("expected empty for missing field")
	}
}

func TestGetHeaderAndField_Explicit(t *testing.T) {
	ctx := context.Background()
	ge1, _ := NewGenericEvent(ctx, []byte("x"), "", "", map[string]string{"a": "b"}, map[string]string{"f": "g"}, 0)
	if ge1.GetHeader("a") != "b" {
		t.Fatalf("expected header a=b")
	}
	if ge1.GetHeader("missing") != "" {
		t.Fatalf("expected empty for missing header")
	}

	if ge1.GetField("f") != "g" {
		t.Fatalf("expected field f=g")
	}
	if ge1.GetField("missing") != "" {
		t.Fatalf("expected empty for missing field")
	}

	ge2, _ := NewGenericEvent(ctx, []byte("y"), "", "", nil, nil, 0)
	if ge2.GetHeader("any") != "" {
		t.Fatalf("expected empty header for nil headers")
	}
	if ge2.GetField("any") != "" {
		t.Fatalf("expected empty field for nil fields")
	}
}

func TestHeaderField_Stress(t *testing.T) {
	ctx := context.Background()
	headers := map[string]string{"k1": "v1", "content-type": "ct"}
	fields := map[string]string{"ff": "vv"}
	ge, _ := NewGenericEvent(ctx, []byte("z"), "", "", headers, fields, 0)

	for i := 0; i < 10; i++ {
		_ = ge.GetHeader("k1")
		_ = ge.GetHeader("missing")
		_ = ge.GetField("ff")
		_ = ge.GetField("missing")
		_ = ge.GetHeaders()
		_ = ge.GetFields()
	}
}

func TestGenericEvent_NilHeadersFields(t *testing.T) {
	// construct GenericEvent directly with nil headers/fields
	ge := &GenericEvent{
		body: []byte("x"),
		url:  &url.URL{Path: "/p"},
	}

	if ge.GetHeader("any") != "" {
		t.Fatalf("expected empty header when headers nil")
	}
	if ge.GetField("any") != "" {
		t.Fatalf("expected empty field when fields nil")
	}
	if ge.GetHeaders() != nil {
		t.Fatalf("expected nil headers map")
	}
	if ge.GetFields() != nil {
		t.Fatalf("expected nil fields map")
	}
}

func TestGenericEventBuilder(t *testing.T) {
	ctx := context.Background()

	// Test basic builder usage
	event, err := NewGenericEventBuilder(ctx).
		WithBody([]byte("test body")).
		WithURL("http://example.com/test").
		WithMethod("POST").
		AddHeader("content-type", "application/json").
		AddHeader("x-custom", "value").
		AddField("field1", "value1").
		WithStatus(201).
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(event.GetBody()) != "test body" {
		t.Fatalf("body mismatch: %s", string(event.GetBody()))
	}

	if event.GetURL() != "http://example.com/test" {
		t.Fatalf("url mismatch: %s", event.GetURL())
	}

	if event.GetMethod() != "POST" {
		t.Fatalf("method mismatch: %s", event.GetMethod())
	}

	if event.GetContentType() != "application/json" {
		t.Fatalf("content-type mismatch: %s", event.GetContentType())
	}

	if event.GetHeader("x-custom") != "value" {
		t.Fatalf("header mismatch: %s", event.GetHeader("x-custom"))
	}

	if event.GetField("field1") != "value1" {
		t.Fatalf("field mismatch: %s", event.GetField("field1"))
	}

	if event.GetStatus() != 201 {
		t.Fatalf("status mismatch: %d", event.GetStatus())
	}

	// Test default values
	event2, err := NewGenericEventBuilder(ctx).Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event2.GetStatus() != 200 {
		t.Fatalf("default status should be 200, got %d", event2.GetStatus())
	}

	if len(event2.GetHeaders()) != 0 {
		t.Fatalf("expected empty headers map")
	}

	if len(event2.GetFields()) != 0 {
		t.Fatalf("expected empty fields map")
	}
}
