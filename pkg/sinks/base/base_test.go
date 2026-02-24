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
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
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

func TestFolderSink_WritesToMultipleFiles(t *testing.T) {
	tmpDir := os.TempDir() + "/dh_test_folder"
	// Clean up any existing folder
	_ = os.RemoveAll(tmpDir)

	fs, err := NewFolderSink(tmpDir, "event_{counter}.txt")
	if err != nil {
		t.Fatalf("failed to create folder sink: %v", err)
	}

	// Send multiple events
	fs.In() <- "event1"
	fs.In() <- "event2"
	fs.In() <- "event3"
	close(fs.In())

	// Wait for completion
	fs.AwaitCompletion()

	// Verify files were created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read folder: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	// Verify content of first file
	data, err := os.ReadFile(tmpDir + "/event_1.txt")
	if err != nil {
		t.Fatalf("failed to read event_1.txt: %v", err)
	}
	if string(data) != "event1" {
		t.Fatalf("expected 'event1', got '%s'", string(data))
	}

	// Verify content of second file
	data, err = os.ReadFile(tmpDir + "/event_2.txt")
	if err != nil {
		t.Fatalf("failed to read event_2.txt: %v", err)
	}
	if string(data) != "event2" {
		t.Fatalf("expected 'event2', got '%s'", string(data))
	}

	// Clean up
	_ = os.RemoveAll(tmpDir)
}

func TestFolderSink_IndexPattern(t *testing.T) {
	tmpDir := os.TempDir() + "/dh_test_folder_index"
	_ = os.RemoveAll(tmpDir)

	fs, err := NewFolderSink(tmpDir, "data_{index}.json")
	if err != nil {
		t.Fatalf("failed to create folder sink: %v", err)
	}

	fs.In() <- "{\"id\": 1}"
	fs.In() <- "{\"id\": 2}"
	close(fs.In())
	fs.AwaitCompletion()

	// Verify files
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read folder: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	_ = os.RemoveAll(tmpDir)
}

func TestFolderSink_TimestampPattern(t *testing.T) {
	tmpDir := os.TempDir() + "/dh_test_folder_ts"
	_ = os.RemoveAll(tmpDir)

	fs, err := NewFolderSink(tmpDir, "log_{timestamp_ns}.txt")
	if err != nil {
		t.Fatalf("failed to create folder sink: %v", err)
	}

	fs.In() <- "log1"
	fs.In() <- "log2"
	close(fs.In())
	fs.AwaitCompletion()

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read folder: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Filenames should contain "log_" and ".txt"
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "log_") || !strings.HasSuffix(file.Name(), ".txt") {
			t.Fatalf("unexpected filename: %s", file.Name())
		}
	}

	_ = os.RemoveAll(tmpDir)
}

func TestFolderSink_WithEventTypes(t *testing.T) {
	tmpDir := os.TempDir() + "/dh_test_folder_evt"
	_ = os.RemoveAll(tmpDir)

	fs, err := NewFolderSink(tmpDir, "event_{counter}.dat")
	if err != nil {
		t.Fatalf("failed to create folder sink: %v", err)
	}

	// Send different types
	fs.In() <- "string"
	fs.In() <- []byte("bytes")
	ev, _ := streams.NewGenericEvent(context.Background(), []byte("event-body"), "", "", nil, nil, 200)
	fs.In() <- ev
	close(fs.In())
	fs.AwaitCompletion()

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read folder: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	// Verify event file content
	data, err := os.ReadFile(tmpDir + "/event_3.dat")
	if err != nil {
		t.Fatalf("failed to read event_3.dat: %v", err)
	}
	if string(data) != "event-body" {
		t.Fatalf("expected 'event-body', got '%s'", string(data))
	}

	_ = os.RemoveAll(tmpDir)
}

func TestFolderSink_CreatesFolderIfNotExists(t *testing.T) {
	tmpDir := os.TempDir() + "/dh_test_nested/subfolder"
	_ = os.RemoveAll(os.TempDir() + "/dh_test_nested")

	fs, err := NewFolderSink(tmpDir, "file.txt")
	if err != nil {
		t.Fatalf("failed to create folder sink: %v", err)
	}

	fs.In() <- "content"
	close(fs.In())
	fs.AwaitCompletion()

	// Verify folder exists
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatalf("folder was not created")
	}

	_ = os.RemoveAll(os.TempDir() + "/dh_test_nested")
}

func TestFolderProcessor_Convert(t *testing.T) {
	tmpDir := os.TempDir() + "/dh_test_convert"
	_ = os.RemoveAll(tmpDir)

	processor := &FolderProcessor{}
	spec := model.OutputSpec{
		Spec: map[string]interface{}{
			"folder_path":      tmpDir,
			"filename_pattern": "test_{counter}.txt",
		},
	}

	sink, err := processor.Convert(spec)
	if err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	if sink == nil {
		t.Fatalf("sink is nil")
	}

	_ = os.RemoveAll(tmpDir)
}

func TestFolderProcessor_Validate(t *testing.T) {
	processor := &FolderProcessor{}

	tests := []struct {
		name    string
		spec    model.OutputSpec
		wantErr bool
	}{
		{
			name: "valid config",
			spec: model.OutputSpec{
				Spec: map[string]interface{}{
					"folder_path":      "/tmp/test",
					"filename_pattern": "file_{counter}.txt",
				},
			},
			wantErr: false,
		},
		{
			name: "missing folder_path",
			spec: model.OutputSpec{
				Spec: map[string]interface{}{
					"filename_pattern": "file_{counter}.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing filename_pattern",
			spec: model.OutputSpec{
				Spec: map[string]interface{}{
					"folder_path": "/tmp/test",
				},
			},
			wantErr: true,
		},
		{
			name: "empty folder_path",
			spec: model.OutputSpec{
				Spec: map[string]interface{}{
					"folder_path":      "",
					"filename_pattern": "file.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "empty filename_pattern",
			spec: model.OutputSpec{
				Spec: map[string]interface{}{
					"folder_path":      "/tmp/test",
					"filename_pattern": "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.Validate(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFolderSink_Registry(t *testing.T) {
	// Verify the folder sink is registered
	tmpDir := os.TempDir() + "/dh_test_registry"
	_ = os.RemoveAll(tmpDir)

	spec := model.OutputSpec{
		Kind: "folder",
		Spec: map[string]interface{}{
			"folder_path":      tmpDir,
			"filename_pattern": "test_{counter}.txt",
		},
	}

	// Get the processor from the registry
	processor, err := sinks.RegistrySingleton.Get("folder")
	if err != nil {
		t.Fatalf("folder sink not registered: %v", err)
	}
	if processor == nil {
		t.Fatalf("folder sink processor is nil")
	}

	// Validate
	validator, ok := processor.(sinks.Validator)
	if !ok {
		t.Fatalf("processor does not implement Validator")
	}
	if err := validator.Validate(spec); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	// Convert
	converter, ok := processor.(sinks.Converter)
	if !ok {
		t.Fatalf("processor does not implement Converter")
	}
	sink, err := converter.Convert(spec)
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}
	if sink == nil {
		t.Fatalf("sink is nil")
	}

	_ = os.RemoveAll(tmpDir)
}
