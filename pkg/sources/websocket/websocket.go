package websocket

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
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

func (s *WSSource) init(factory sources.FlowFactory) {
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

}

func (s *WSSource) Start(factory sources.FlowFactory) {
	s.init(factory)
}

func (s *WSSource) StartAsync(factory sources.FlowFactory, sink streams.Sink) {
	s.sink = &sink
	s.init(factory)
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
	sink := *s.sink
	if sink == nil {
		sink = extension.NewChanSink(output)
		output = make(chan any)
		go s.handleOutput(conn, output)
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

		event, err := NewSocketEvent(message, messageType)
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
