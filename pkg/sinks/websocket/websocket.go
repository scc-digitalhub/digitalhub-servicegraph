// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"fmt"
	"log/slog"

	ws "github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

// Sink represents a WebSocket sink connector.
type WebSocketSink struct {
	conf       Configuration
	connection *ws.Conn
	in         chan any

	done   chan struct{}
	logger *slog.Logger
}

var _ streams.Sink = (*WebSocketSink)(nil)

// NewWebSocketSink creates and returns a new [WebSocketSink] using the default dialer.
func NewWebSocketSink(conf Configuration) (*WebSocketSink, error) {
	return NewWebSocketSinkWithDialer(conf, ws.DefaultDialer)
}

// NewWebSocketSinkWithDialer returns a new [WebSocketSink] using the provided dialer.
func NewWebSocketSinkWithDialer(conf Configuration, dialer *ws.Dialer) (*WebSocketSink, error) {
	// create a new client connection
	conn, _, err := dialer.Dial(conf.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	logger := slog.Default()
	logger = logger.With(slog.Group("connector",
		slog.String("name", "websocket"),
		slog.String("type", "sink")))

	sink := &WebSocketSink{
		conf:       conf,
		connection: conn,
		in:         make(chan any),
		done:       make(chan struct{}),
		logger:     logger,
	}

	// begin processing upstream data
	go sink.process()

	return sink, nil
}

func (s *WebSocketSink) process() {
	defer close(s.done) // signal data processing completion

	for msg := range s.in {
		var err error
		switch message := msg.(type) {
		case streams.Event:
			err = s.connection.WriteMessage(s.conf.MsgType, message.GetBody())
		case string:
			err = s.connection.WriteMessage(s.conf.MsgType, []byte(message))
		case []byte:
			err = s.connection.WriteMessage(s.conf.MsgType, message)
		default:
			s.logger.Error("Unsupported message type",
				slog.String("type", fmt.Sprintf("%T", message)))
		}

		if err != nil {
			s.logger.Error("Error processing message", slog.Any("error", err))
		}
	}

	s.logger.Info("Closing connector")
	if err := s.connection.Close(); err != nil {
		s.logger.Warn("Error in connection.Close", slog.Any("error", err))
	}
}

// In returns the input channel of the Sink connector.
func (s *WebSocketSink) In() chan<- any {
	return s.in
}

// AwaitCompletion blocks until the Sink connector has completed
// processing all the received data.
func (s *WebSocketSink) AwaitCompletion() {
	<-s.done
}

func init() {
	sinks.RegistrySingleton.Register("websocket", &WebSocketProcessor{})
}

type WebSocketProcessor struct {
	sinks.Converter
	sinks.Validator
}

func (c *WebSocketProcessor) Convert(output model.OutputSpec) (streams.Sink, error) {
	// marshal to json, unmarshal to config
	conf := &Configuration{}
	err := util.Convert(output.Spec, conf)
	if err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.URL, conf.Params, conf.Headers, conf.MsgType)
	src, err := NewWebSocketSink(*conf)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func (c *WebSocketProcessor) Validate(spec model.OutputSpec) error {
	conf := &Configuration{}
	err := util.Convert(spec.Spec, conf)
	if err != nil {
		return err
	}
	if conf.URL == "" {
		return fmt.Errorf("url is required for websocket sink")
	}
	return nil
}
