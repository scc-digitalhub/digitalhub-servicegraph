// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type WSSource struct {
	sources.Source
	factory     sources.FlowFactory
	Conf        *Configuration
	connections map[*websocket.Conn]bool
	mutex       *sync.Mutex
	logger      *slog.Logger
	sink        *streams.Sink
}

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewWSSource(conf *Configuration) *WSSource {
	source := &WSSource{}
	source.Conf = conf
	source.logger = slog.Default()
	source.logger = source.logger.With(slog.Group("source",
		slog.String("name", "websocket"),
		slog.String("type", "source")))

	return source
}

func (s *WSSource) init(factory sources.FlowFactory) error {
	s.factory = factory
	serverPort := fmt.Sprintf(":%d", s.Conf.Port)

	s.connections = make(map[*websocket.Conn]bool)
	s.mutex = &sync.Mutex{}

	http.HandleFunc("/ws", s.wsHandler)
	s.logger.Info("WebSocket server started on :8080")
	err := http.ListenAndServe(serverPort, nil)
	if err != nil {
		s.logger.Error("Error starting server:", slog.Any("error", err))
	}
	return nil
}

func (s *WSSource) Start(factory sources.FlowFactory) error {
	return s.init(factory)
}

func (s *WSSource) StartAsync(factory sources.FlowFactory, sink streams.Sink) error {
	s.sink = &sink
	return s.init(factory)
}

func (s *WSSource) wsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Error upgrading:", slog.Any("error", err))
		return
	}
	// defer conn.Close()

	if len(s.connections) >= s.Conf.Capacity {
		s.logger.Error("Capacity limit reached")
		return
	}

	s.mutex.Lock()
	s.connections[conn] = true
	s.mutex.Unlock()

	go s.handleConnection(conn)
}

func (s *WSSource) handleConnection(conn *websocket.Conn) {
	input := make(chan any)
	var output chan any
	var sink streams.Sink
	if s.sink == nil {
		output = make(chan any)
		sink = extension.NewChanSink(output)
		go s.handleOutput(conn, output)
	} else {
		sink = *s.sink
	}

	go func() {
		s.factory.GenerateFlow(
			extension.NewChanSource(input),
			sink,
		)
	}()

	s.logger.Info("Start processing input")
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			s.cleanConnection(conn, err)
			break
		}

		event, err := NewSocketEvent(context.Background(), message, messageType)
		if err != nil {
			s.cleanConnection(conn, err)
			break
		}
		input <- event
		// output <- event
	}
	s.logger.Info("Input processing closed")

}

func (s *WSSource) handleOutput(conn *websocket.Conn, output chan any) {
	s.logger.Info("Start processing output")

	for msg := range output {
		var err error
		switch message := msg.(type) {
		case SocketEvent:
			err = conn.WriteMessage(message.messageType, message.body)
		case *SocketEvent:
			err = conn.WriteMessage(message.messageType, message.body)
		case streams.Event:
			if strings.EqualFold("application/octet-stream", message.GetContentType()) {
				err = conn.WriteMessage(websocket.BinaryMessage, message.GetBody())
			} else {
				err = conn.WriteMessage(websocket.TextMessage, message.GetBody())
			}
		case string:
			err = conn.WriteMessage(websocket.TextMessage, []byte(message))
		case []byte:
			err = conn.WriteMessage(websocket.TextMessage, message)
		default:
			s.logger.Error("Unsuppported message type")
		}

		if err != nil {
			s.logger.Error("Error processing message:", slog.Any("error", err))
		}
	}
	s.logger.Info("Output processing closed")

}

func (s *WSSource) cleanConnection(conn *websocket.Conn, err error) {
	s.logger.Error("Error reading message:", slog.Any("error", err))
	s.mutex.Lock()
	delete(s.connections, conn)
	s.mutex.Unlock()
}

func init() {
	sources.RegistrySingleton.Register("websocket", &WebSocketProcessor{})
}

type WebSocketProcessor struct {
	sources.Converter
	sources.Validator
}

func (c *WebSocketProcessor) Convert(input model.InputSpec) (sources.Source, error) {
	conf := &Configuration{}
	err := util.Convert(input.Spec, conf)
	if err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.Port, conf.Capacity)
	src := NewWSSource(conf)
	return src, nil
}

func (c *WebSocketProcessor) Validate(input model.InputSpec) error {
	conf := &Configuration{}
	err := util.Convert(input.Spec, conf)
	if err != nil {
		return err
	}
	if conf.Port <= 0 || conf.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", conf.Port)
	}
	if conf.Capacity <= 0 {
		return fmt.Errorf("invalid capacity: %d", conf.Capacity)
	}
	return nil
}
