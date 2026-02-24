// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type FolderConfiguration struct {
	FolderPath      string `json:"folder_path"`
	FilenamePattern string `json:"filename_pattern"`
}

// FolderSink represents an outbound connector that writes each streaming event
// to a separate file in a folder using an incremental filename pattern.
type FolderSink struct {
	folderPath      string
	filenamePattern string
	counter         int
	mu              sync.Mutex
	in              chan any
	done            chan struct{}
}

var _ streams.Sink = (*FolderSink)(nil)

// NewFolderSink returns a new FolderSink connector.
func NewFolderSink(folderPath, filenamePattern string) (*FolderSink, error) {
	// Create the folder if it doesn't exist
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	folderSink := &FolderSink{
		folderPath:      folderPath,
		filenamePattern: filenamePattern,
		counter:         0,
		in:              make(chan any),
		done:            make(chan struct{}),
	}

	// asynchronously process stream data
	go folderSink.process()

	return folderSink, nil
}

// generateFilename creates a filename based on the pattern, replacing placeholders
// with actual values. Supported placeholders:
// - {counter} or {index}: incremental counter starting from 1
// - {timestamp}: unix timestamp in seconds
// - {timestamp_ms}: unix timestamp in milliseconds
// - {timestamp_ns}: unix timestamp in nanoseconds
func (fs *FolderSink) generateFilename() string {
	fs.mu.Lock()
	fs.counter++
	counter := fs.counter
	fs.mu.Unlock()

	filename := fs.filenamePattern
	now := time.Now()

	// Replace placeholders
	filename = strings.ReplaceAll(filename, "{counter}", fmt.Sprintf("%d", counter))
	filename = strings.ReplaceAll(filename, "{index}", fmt.Sprintf("%d", counter))
	filename = strings.ReplaceAll(filename, "{timestamp}", fmt.Sprintf("%d", now.Unix()))
	filename = strings.ReplaceAll(filename, "{timestamp_ms}", fmt.Sprintf("%d", now.UnixMilli()))
	filename = strings.ReplaceAll(filename, "{timestamp_ns}", fmt.Sprintf("%d", now.UnixNano()))

	return filepath.Join(fs.folderPath, filename)
}

// process reads data from the input channel and writes each element to a new file.
func (fs *FolderSink) process() {
	defer close(fs.done)

	for element := range fs.in {
		var stringElement string
		switch v := element.(type) {
		case string:
			stringElement = v
		case []byte:
			stringElement = string(v)
		case fmt.Stringer:
			stringElement = v.String()
		case streams.Event:
			util.FinalizeOTelSpans(v.GetContext())
			stringElement = string(v.GetBody())
		default:
			slog.Warn("Discarded stream element",
				slog.String("type", fmt.Sprintf("%T", v)))
			continue
		}

		// Generate a new filename for this event
		filename := fs.generateFilename()

		// Write to the file
		if err := fs.writeToFile(filename, stringElement); err != nil {
			slog.Error("Failed to write to file",
				slog.String("filename", filename),
				slog.Any("error", err))
			// Continue processing other events even if one fails
		}
	}
}

// writeToFile writes the content to a file
func (fs *FolderSink) writeToFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("Failed to close file",
				slog.String("name", filename),
				slog.Any("error", err))
		}
	}()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// In returns the input channel of the FolderSink connector.
func (fs *FolderSink) In() chan<- any {
	return fs.in
}

// AwaitCompletion blocks until the FolderSink has completed processing and
// flushing all data to files.
func (fs *FolderSink) AwaitCompletion() {
	<-fs.done
}

func init() {
	sinks.RegistrySingleton.Register("folder", &FolderProcessor{})
}

type FolderProcessor struct {
	sinks.Converter
	sinks.Validator
}

func (c *FolderProcessor) Convert(output model.OutputSpec) (streams.Sink, error) {
	// marshal to json, unmarshal to config
	conf := &FolderConfiguration{}
	err := util.Convert(output.Spec, conf)
	if err != nil {
		return nil, err
	}
	sink, err := NewFolderSink(conf.FolderPath, conf.FilenamePattern)
	if err != nil {
		return nil, err
	}
	return sink, nil
}

func (c *FolderProcessor) Validate(spec model.OutputSpec) error {
	conf := &FolderConfiguration{}
	err := util.Convert(spec.Spec, conf)
	if err != nil {
		return err
	}
	if conf.FolderPath == "" {
		return fmt.Errorf("folder_path is required for folder sink")
	}
	if conf.FilenamePattern == "" {
		return fmt.Errorf("filename_pattern is required for folder sink")
	}
	return nil
}
