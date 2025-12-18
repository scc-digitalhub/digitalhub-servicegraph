// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

const (
	defaultServerPort = 8080
	defaultCapacity   = 10
)

type Configuration struct {
	Port     int `json:"port"`
	Capacity int `json:"capacity"`
}

type SocketEvent struct {
	streams.GenericEvent
	body        []byte
	messageType int
	timestamp   time.Time
	ctx         context.Context
}

func NewSocketEvent(ctx context.Context, body []byte, messageType int) (*SocketEvent, error) {
	event := &SocketEvent{ctx: ctx}

	event.body = body
	event.messageType = messageType
	event.timestamp = time.Now()
	return event, nil
}

// GetContentType returns the content type of the body
func (e *SocketEvent) GetContentType() string {
	if websocket.TextMessage == e.messageType {
		return "text/plain"
	}
	return "application/octet-stream"
}

// GetBody returns the body of the event
func (e *SocketEvent) GetBody() []byte {
	return e.body
}

// GetTimestamp returns when the event originated
func (e *SocketEvent) GetTimestamp() time.Time {
	return e.timestamp
}

// GetContext returns the context of the event
func (e *SocketEvent) GetContext() context.Context {
	return e.ctx
}
func NewConfiguration(port int, capacity int) *Configuration {

	newConfiguration := &Configuration{
		Port:     port,
		Capacity: capacity,
	}
	if port == 0 {
		newConfiguration.Port = defaultServerPort
	}
	if capacity == 0 {
		newConfiguration.Capacity = defaultCapacity
	}

	return newConfiguration
}
