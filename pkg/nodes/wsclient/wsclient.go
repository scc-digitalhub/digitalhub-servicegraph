package wsclient

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	ws "github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

// Verify WebSocketClient satisfies the Flow interface.
var _ streams.Flow = (*WebSocketClient)(nil)

// Message represents a WebSocket message container.
// Message types are defined in [RFC 6455], section 11.8.
//
// [RFC 6455]: https://www.rfc-editor.org/rfc/rfc6455.html#section-11.8
type Message struct {
	MsgType int
	Payload []byte
}

type WebSocketClient struct {
	conf       Configuration
	in         chan any
	out        chan any
	connection *ws.Conn
	logger     *slog.Logger
}

func NewWebSocketClient(conf Configuration) *WebSocketClient {
	wsClient := &WebSocketClient{
		conf:   conf,
		in:     make(chan any),
		out:    make(chan any),
		logger: slog.Default(),
	}

	// create a new client connection
	conn, _, err := ws.DefaultDialer.Dial(conf.URL, nil)
	if err != nil {
		panic(fmt.Errorf("error connecting to URL: %w", err))
	}
	wsClient.connection = conn

	wsClient.logger = wsClient.logger.With(slog.Group("connector",
		slog.String("name", "websocket"),
		slog.String("type", "sink")))

	go wsClient.stream()
	go wsClient.sink()

	return wsClient
}

// Via asynchronously streams data to the given Flow and returns it.
func (wsc *WebSocketClient) Via(flow streams.Flow) streams.Flow {
	go wsc.transmit(flow)
	return flow
}

// To streams data to the given Sink and blocks until the Sink has completed
// processing all data.
func (wsc *WebSocketClient) To(sink streams.Sink) {
	wsc.transmit(sink)
	sink.AwaitCompletion()
}

// Out returns the output channel of the WebSocketClient operator.
func (wsc *WebSocketClient) Out() <-chan any {
	return wsc.out
}

// In returns the input channel of the WebSocketClient operator.
func (wsc *WebSocketClient) In() chan<- any {
	return wsc.in
}

func (wsc *WebSocketClient) transmit(inlet streams.Inlet) {
	for element := range wsc.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

func (wsc *WebSocketClient) stream() {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for element := range wsc.in {
			wsc.forwardMessage(streams.NewEventFrom(element))
		}
	}()

}

func (wsc *WebSocketClient) sink() {
loop:
	for {
		messageType, payload, err := wsc.connection.ReadMessage()
		if err != nil {
			wsc.logger.Error("error in connection.ReadMessage", slog.Any("error", err))
		} else {
			// exit loop on CloseMessage
			if messageType == ws.CloseMessage {
				break loop
			}
			wsc.out <- streams.NewEventFrom(payload)
		}
	}

	wsc.logger.Info("Closing connector")
	close(wsc.out)

	if err := wsc.connection.Close(); err != nil {
		wsc.logger.Warn("Error in connection.Close", slog.Any("error", err))
	}
}

func (wsc *WebSocketClient) forwardMessage(event streams.Event) {
	body := event.GetBody()

	err := wsc.connection.WriteMessage(wsc.conf.MsgType, body)
	if err != nil {
		wsc.logger.Error("Error processing message", slog.Any("error", err))
	}

}

func init() {
	nodes.RegistrySingleton.Register("websocket", &WSProcessor{})
}

type WSProcessor struct {
	nodes.Converter
	nodes.Validator
}

func (c *WSProcessor) Convert(spec model.NodeConfig) (streams.Flow, error) {
	// marshal to json, unmarshal to config
	data, err := json.Marshal(spec.Spec)
	if err != nil {
		return nil, err
	}
	conf := &Configuration{}
	err = json.Unmarshal(data, conf)
	if err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.URL, conf.Params, conf.Headers, conf.MsgType)
	src := NewWebSocketClient(*conf)
	return src, nil
}

func (c *WSProcessor) Validate(spec model.NodeConfig) error {
	// marshal to json, unmarshal to config
	conf := &Configuration{}
	err := util.Convert(spec.Spec, conf)

	if err != nil {
		return err
	}
	if conf.URL == "" {
		return fmt.Errorf("wsclient node requires a valid URL")
	}
	if conf.MsgType != ws.TextMessage && conf.MsgType != ws.BinaryMessage {
		return fmt.Errorf("wsclient node requires a valid message type")
	}
	return nil
}
