// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

func TestStdoutSink_ProcessAndAwait(t *testing.T) {
	s := NewStdoutSink()
	// send a simple string and then close the channel to allow the sink to finish
	s.In() <- "hello-stdout"
	close(s.In())
	// wait for completion
	s.AwaitCompletion()
}

func TestStdoutSink_ProcessTypes(t *testing.T) {
	s := NewStdoutSink()
	// send different types
	s.In() <- "string"
	s.In() <- []byte("bytes")
	s.In() <- fmt.Errorf("error")
	ev, _ := streams.NewGenericEvent(context.Background(), []byte("body"), "url", "GET", nil, nil, 200)
	s.In() <- ev
	s.In() <- 123 // default
	close(s.In())
	s.AwaitCompletion()
}

func TestIgnoreSink_NoPanic(t *testing.T) {
	s := NewIgnoreSink()
	s.In() <- "ignored"
	close(s.In())
	s.AwaitCompletion() // cover AwaitCompletion
}

func TestFileSink_WritesToFile(t *testing.T) {
	tmp := os.TempDir() + "/dh_test_output.txt"
	// remove any pre-existing file
	_ = os.Remove(tmp)
	fs := NewFileSink(tmp)
	// send a string and close
	fs.In() <- "file-content"
	close(fs.In())
	// wait for completion
	fs.AwaitCompletion()

	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if !strings.Contains(string(data), "file-content") {
		t.Fatalf("file did not contain expected content: %s", string(data))
	}
	_ = os.Remove(tmp)
}

func TestFileSink_WithEventTypes(t *testing.T) {
	tmp := os.TempDir() + "/dh_test_output_evt.txt"
	_ = os.Remove(tmp)
	fs := NewFileSink(tmp)
	// send a streams.Event (create minimal event)
	ev, err := streams.NewGenericEvent(context.Background(), []byte("evt-body"), "", "", nil, nil, 200)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	// FileSink only writes string/[]byte/Stringer; send the event body bytes
	fs.In() <- ev.GetBody()
	close(fs.In())
	fs.AwaitCompletion()
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if !strings.Contains(string(data), "evt-body") {
		t.Fatalf("file did not contain event body: %s", string(data))
	}
	_ = os.Remove(tmp)
}

type testStringer string

func (t testStringer) String() string {
	return string(t)
}

func TestFileSink_ProcessTypes(t *testing.T) {
	tmp := os.TempDir() + "/dh_test_types.txt"
	_ = os.Remove(tmp)
	fs := NewFileSink(tmp)
	// send different types
	fs.In() <- "string"
	fs.In() <- []byte("bytes")
	fs.In() <- testStringer("stringer")
	fs.In() <- 123 // default, should warn and skip
	close(fs.In())
	fs.AwaitCompletion()

	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "string") || !strings.Contains(content, "bytes") || !strings.Contains(content, "stringer") {
		t.Fatalf("file did not contain expected content: %s", content)
	}
	if strings.Contains(content, "123") {
		t.Fatalf("file should not contain default type: %s", content)
	}
	_ = os.Remove(tmp)
}

func TestFileProcessor_Convert(t *testing.T) {
	converter := &FileProcessor{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"file_name": "test.txt",
		},
	}

	sink, err := converter.Convert(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink == nil {
		t.Fatalf("expected sink, got nil")
	}
	fs, ok := sink.(*FileSink)
	if !ok {
		t.Fatalf("expected FileSink, got %T", sink)
	}
	if fs.fileName != "test.txt" {
		t.Errorf("expected fileName test.txt, got %s", fs.fileName)
	}
}

func TestIgnoreConverter_Convert(t *testing.T) {
	converter := &IgnoreConverter{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{},
	}

	sink, err := converter.Convert(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink == nil {
		t.Fatalf("expected sink, got nil")
	}
	_, ok := sink.(*IgnoreSink)
	if !ok {
		t.Fatalf("expected IgnoreSink, got %T", sink)
	}
}

func TestStdOutConverter_Convert(t *testing.T) {
	converter := &StdOutConverter{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{},
	}

	sink, err := converter.Convert(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink == nil {
		t.Fatalf("expected sink, got nil")
	}
	_, ok := sink.(*StdoutSink)
	if !ok {
		t.Fatalf("expected StdoutSink, got %T", sink)
	}
}

func TestFileProcessor_Validate(t *testing.T) {
	converter := &FileProcessor{}

	// Valid spec
	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"file_name": "test.txt",
		},
	}

	err := converter.Validate(spec)
	if err != nil {
		t.Fatalf("expected no error for valid spec, got %v", err)
	}

	// Invalid spec - missing file_name
	spec.Spec = map[string]interface{}{}
	err = converter.Validate(spec)
	if err == nil {
		t.Fatalf("expected error for missing file_name")
	}

	// Invalid spec - empty file_name
	spec.Spec = map[string]interface{}{
		"file_name": "",
	}
	err = converter.Validate(spec)
	if err == nil {
		t.Fatalf("expected error for empty file_name")
	}
}

func TestIgnoreConverter_Validate(t *testing.T) {
	converter := &IgnoreConverter{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{},
	}

	err := converter.Validate(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestStdOutConverter_Validate(t *testing.T) {
	converter := &StdOutConverter{}

	spec := model.OutputSpec{
		Spec: map[string]interface{}{},
	}

	err := converter.Validate(spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestFileProcessor_Convert_Error(t *testing.T) {
	converter := &FileProcessor{}

	// Invalid spec that can't be converted
	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"file_name": make(chan int), // channels can't be marshaled
		},
	}

	_, err := converter.Convert(spec)
	if err == nil {
		t.Fatalf("expected error for invalid spec")
	}
}

func TestDrainChan(t *testing.T) {
	ch := make(chan any, 3)
	ch <- "test1"
	ch <- "test2"
	ch <- "test3"
	close(ch)

	// drainChan should consume all elements
	drainChan(ch)

	// Channel should be empty and closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("channel should be closed")
		}
	default:
		t.Fatalf("channel should be closed")
	}
}

func TestDrainChan_Nil(t *testing.T) {
	// Should not panic with nil channel
	drainChan(nil)
}

func TestFileSink_WriteError(t *testing.T) {
	// Create a file sink with a directory that doesn't exist and can't be created
	// This will test the error handling in the process method
	tmp := "/nonexistent/directory/test.txt"
	fs := NewFileSink(tmp)

	// Send data that should trigger file creation error
	fs.In() <- "test data"
	close(fs.In())

	// Wait for completion - should handle error gracefully
	fs.AwaitCompletion()
}
