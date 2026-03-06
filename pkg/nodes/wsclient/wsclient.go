// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package wsclient

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
	mu         sync.RWMutex // protects connection during reconnect
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWebSocketClient creates a new WebSocketClient, dialling with exponential
// backoff according to conf.MaxRetries / conf.RetryBackoffMs.
// Returns an error if the initial connection cannot be established.
func NewWebSocketClient(conf Configuration) (*WebSocketClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	logger := slog.Default().With(slog.Group("connector",
		slog.String("name", "websocket"),
		slog.String("type", "node"),
		slog.String("url", conf.URL)))

	initialBackoff := time.Duration(conf.RetryBackoffMs) * time.Millisecond
	conn, err := dialWithBackoff(ctx, logger, conf.URL, conf.MaxRetries, initialBackoff)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("websocket: failed to connect to %s: %w", conf.URL, err)
	}

	wsClient := &WebSocketClient{
		conf:       conf,
		in:         make(chan any),
		out:        make(chan any),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		connection: conn,
	}

	go wsClient.stream()
	go wsClient.sink()

	return wsClient, nil
}

// dialWithBackoff attempts to connect to url, retrying up to maxRetries times
// (0 = no retry, -1 = unlimited) with doubling backoff starting at initialBackoff.
func dialWithBackoff(ctx context.Context, logger *slog.Logger, url string, maxRetries int, initialBackoff time.Duration) (*ws.Conn, error) {
	if initialBackoff <= 0 {
		initialBackoff = time.Second
	}
	backoff := initialBackoff

	for attempt := 1; ; attempt++ {
		conn, _, err := ws.DefaultDialer.DialContext(ctx, url, nil)
		if err == nil {
			return conn, nil
		}
		// Check retry limit: 0 = single attempt only (no retry).
		if maxRetries == 0 || (maxRetries > 0 && attempt > maxRetries) {
			return nil, fmt.Errorf("failed to connect after %d attempt(s): %w", attempt, err)
		}
		logger.Warn("WebSocket dial failed, retrying",
			slog.Int("attempt", attempt),
			slog.Duration("backoff", backoff),
			slog.Any("error", err))
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

// reconnect replaces the current connection with a fresh dial, using the same
// backoff policy as the initial connection.
func (wsc *WebSocketClient) reconnect() error {
	initialBackoff := time.Duration(wsc.conf.RetryBackoffMs) * time.Millisecond
	conn, err := dialWithBackoff(wsc.ctx, wsc.logger, wsc.conf.URL, wsc.conf.MaxRetries, initialBackoff)
	if err != nil {
		return err
	}
	wsc.mu.Lock()
	old := wsc.connection
	wsc.connection = conn
	wsc.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	wsc.logger.Info("WebSocket reconnected successfully")
	return nil
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

// stream reads from the input channel and forwards each element over the
// WebSocket connection. Error events are passed through without sending.
func (wsc *WebSocketClient) stream() {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				wsc.logger.Error("Worker panic recovered", slog.Any("panic", r))
			}
		}()
		for element := range wsc.in {
			// Pass error events downstream without forwarding over the wire.
			if ev, ok := element.(streams.Event); ok && ev.GetStatus() >= 400 {
				wsc.logger.Debug("Passing through error event",
					slog.Int("status", ev.GetStatus()))
				wsc.out <- element
				continue
			}
			body, err := util.ConvertBody(element, wsc.conf.inTemplateObj)
			if err != nil {
				wsc.logger.Error("error processing input template", slog.Any("error", err))
				continue
			}
			if err := wsc.forwardMessage(streams.NewEventFrom(wsc.ctx, body)); err != nil {
				wsc.logger.Error("Failed to forward message, message dropped", slog.Any("error", err))
			}
		}
	}()

	wg.Wait()
}

// sink reads messages arriving from the remote WebSocket endpoint and pushes
// them to the output channel. It reconnects automatically on unexpected errors
// when MaxRetries != 0.
func (wsc *WebSocketClient) sink() {
	defer func() {
		wsc.cancel()
		wsc.mu.RLock()
		conn := wsc.connection
		wsc.mu.RUnlock()
		if conn != nil {
			_ = conn.Close()
		}
		wsc.logger.Info("WebSocket sink closed")
		close(wsc.out)
	}()

	for {
		wsc.mu.RLock()
		conn := wsc.connection
		wsc.mu.RUnlock()

		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			// Normal / expected close — exit cleanly.
			if ws.IsCloseError(err, ws.CloseNormalClosure, ws.CloseGoingAway) {
				wsc.logger.Info("WebSocket connection closed normally")
				return
			}
			wsc.logger.Warn("WebSocket read error", slog.Any("error", err))
			// Attempt reconnect if configured.
			if wsc.conf.MaxRetries == 0 {
				wsc.logger.Error("Reconnects disabled (max_retries=0), closing sink")
				return
			}
			if reconErr := wsc.reconnect(); reconErr != nil {
				wsc.logger.Error("Reconnect failed, closing sink", slog.Any("error", reconErr))
				return
			}
			continue
		}
		if messageType == ws.CloseMessage {
			wsc.logger.Info("Received WebSocket CloseMessage")
			return
		}
		payloadObj, err := util.ConvertBody(payload, wsc.conf.outTemplateObj)
		if err != nil {
			wsc.logger.Error("error processing output template", slog.Any("error", err))
			continue
		}
		wsc.out <- streams.NewEventFrom(wsc.ctx, payloadObj)
	}
}

// forwardMessage writes event body over the WebSocket connection.
// On write failure it triggers one reconnect attempt and retries the write once.
func (wsc *WebSocketClient) forwardMessage(event streams.Event) error {
	body := event.GetBody()

	wsc.mu.RLock()
	conn := wsc.connection
	wsc.mu.RUnlock()

	err := conn.WriteMessage(wsc.conf.MsgType, body)
	if err == nil {
		return nil
	}
	wsc.logger.Warn("WebSocket write failed, attempting reconnect before retry", slog.Any("error", err))

	if wsc.conf.MaxRetries == 0 {
		return fmt.Errorf("write failed and reconnects disabled: %w", err)
	}
	if reconErr := wsc.reconnect(); reconErr != nil {
		return fmt.Errorf("write failed and reconnect unsuccessful: %w", reconErr)
	}
	// One retry after successful reconnect.
	wsc.mu.RLock()
	conn = wsc.connection
	wsc.mu.RUnlock()
	if retryErr := conn.WriteMessage(wsc.conf.MsgType, body); retryErr != nil {
		return fmt.Errorf("write failed after reconnect: %w", retryErr)
	}
	return nil
}

func init() {
	nodes.RegistrySingleton.Register("websocket", &WSProcessor{})
}

type WSProcessor struct {
	nodes.Converter
	nodes.Validator
}

func (c *WSProcessor) Convert(spec *model.NodeConfig) (streams.Flow, error) {
	var newConf *Configuration
	if cached := spec.ConfigCache(); cached != nil {
		newConf = (*cached).(*Configuration)
	} else {
		conf := &Configuration{}
		err := util.Convert(spec.Spec, conf)
		if err != nil {
			return nil, err
		}
		conf.setInputTemplate(conf.InputTemplate)
		conf.setOutputTemplate(conf.OutputTemplate)
		newConf = NewConfiguration(conf.URL, conf.Params, conf.Headers, conf.MsgType)
		newConf.inTemplateObj = conf.inTemplateObj
		newConf.outTemplateObj = conf.outTemplateObj
		newConf.MaxRetries = conf.MaxRetries
		newConf.RetryBackoffMs = conf.RetryBackoffMs
		spec.SetConfigCache(newConf)
	}
	src, err := NewWebSocketClient(*newConf)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func (c *WSProcessor) Validate(spec *model.NodeConfig) error {
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
