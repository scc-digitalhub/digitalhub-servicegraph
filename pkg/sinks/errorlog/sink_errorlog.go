// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

// Package errorlog provides the built-in "errorlog" sink kind.
// It logs every received event as a structured slog warning, making it the
// default error sink when no explicit error_sink is declared in the graph.
package errorlog

import (
	"fmt"
	"log/slog"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// LogSink logs each event it receives as an slog.Warn entry.
type LogSink struct {
	in   chan any
	done chan struct{}
}

var _ streams.Sink = (*LogSink)(nil)

// NewLogSink returns a new LogSink ready to receive events.
func NewLogSink() *LogSink {
	s := &LogSink{
		in:   make(chan any),
		done: make(chan struct{}),
	}
	go s.process()
	return s
}

// In returns the input channel.
func (s *LogSink) In() chan<- any {
	return s.in
}

// AwaitCompletion blocks until all received events have been logged.
func (s *LogSink) AwaitCompletion() {
	<-s.done
}

func (s *LogSink) process() {
	defer close(s.done)
	for element := range s.in {
		if ev, ok := element.(streams.Event); ok {
			slog.Warn("Error event discarded",
				slog.Int("status", ev.GetStatus()),
				slog.String("body", string(ev.GetBody())))
		} else {
			slog.Warn("Non-event error element discarded",
				slog.String("value", fmt.Sprintf("%v", element)))
		}
	}
}

func init() {
	sinks.RegistrySingleton.Register("errorlog", &LogSinkProcessor{})
}

// LogSinkProcessor adapts LogSink to the sinks registry.
type LogSinkProcessor struct {
	sinks.Converter
	sinks.Validator
}

// Convert creates a new LogSink; no configuration is required.
func (c *LogSinkProcessor) Convert(spec model.OutputSpec) (streams.Sink, error) {
	return NewLogSink(), nil
}

// Validate always succeeds — the log sink has no required configuration.
func (c *LogSinkProcessor) Validate(spec model.OutputSpec) error {
	return nil
}
