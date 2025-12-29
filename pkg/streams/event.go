// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package streams

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// ErrUnsupported is returned when an unsupported interface on the event is called
var ErrUnsupported = errors.New("event does not support this interface")

// ErrTypeConversion is returned when a type conversion for headers / fields fails
var ErrTypeConversion = errors.New("cannot convert to this type")

type ID string

// Event allows access to the concrete event
type Event interface {

	// GetID returns the ID of the event
	GetID() ID

	// SetID sets the ID of the event
	SetID(ID)

	// GetContentType returns the content type of the body
	GetContentType() string

	// GetBody returns the body of the event
	GetBody() []byte

	// GetBodyObject returns the body of the event as an object
	GetBodyObject() interface{}

	// GetHeader returns the header by name as an string
	GetHeader(string) string

	// GetHeaders loads all headers into a map of string / string
	GetHeaders() map[string]string

	// GetField returns the field by name as an string
	GetField(string) string

	// GetFields loads all fields into a map of string / string
	GetFields() map[string]string

	// GetTimestamp returns when the event originated
	GetTimestamp() time.Time

	// GetPath returns the path of the event
	GetPath() string

	// GetURL returns the URL of the event
	GetURL() string

	// GetMethod returns the method of the event, if applicable
	GetMethod() string

	// GetTopic returns the topic of the event
	GetTopic() string

	// GetStatus returns the status of the event, if applicable
	GetStatus() int

	// GetContext returns the context of the event
	GetContext() context.Context
}

type GenericEvent struct {
	Event
	id      ID
	body    []byte
	method  string
	url     *url.URL
	headers map[string]string
	fields  map[string]string
	status  int
	context context.Context
}

var _ Event = (*GenericEvent)(nil)

func NewGenericEvent(ctx context.Context, body []byte, rawUrl string, method string, headers, fields map[string]string, status int) (*GenericEvent, error) {
	event := &GenericEvent{status: status, context: ctx}
	event.body = body
	event.method = method
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}
	event.url = parsed
	if headers != nil {
		event.headers = headers
	} else {
		event.headers = make(map[string]string)
	}
	if fields != nil {
		event.fields = fields
	} else {
		event.fields = make(map[string]string)
	}
	if event.status == 0 {
		event.status = 200
	}
	return event, nil
}

// GetID returns the ID of the event
func (ae *GenericEvent) GetID() ID {
	return ae.id
}

// SetID sets the ID of the event
func (ae *GenericEvent) SetID(id ID) {
	ae.id = id
}

// GetContentType returns the content type of the body
func (ae *GenericEvent) GetContentType() string {
	if ae.headers != nil {
		if v, ok := ae.headers["content-type"]; ok {
			return v
		}
		if v, ok := ae.headers["Content-Type"]; ok {
			return v
		}
	}
	return ""
}

// GetBody returns the body of the event
func (ae *GenericEvent) GetBody() []byte {
	return ae.body
}

// GetBodyObject returns the body of the event as an object
func (ae *GenericEvent) GetBodyObject() interface{} {
	return ae.GetBody()
}

// GetHeader returns the header by name as an interface{}
func (ae *GenericEvent) GetHeader(key string) string {
	if ae.headers != nil {
		return ae.headers[key]
	}
	return ""
}

// GetHeaders loads all headers into a map of string / interface{}
func (ae *GenericEvent) GetHeaders() map[string]string {
	return ae.headers
}

// GetTimestamp returns when the event originated
func (ae *GenericEvent) GetTimestamp() time.Time {
	return time.Now()
}

// GetPath returns the path of the event
func (ae *GenericEvent) GetPath() string {
	return ae.url.Path
}

// GetURL returns the URL of the event
func (ae *GenericEvent) GetURL() string {
	return ae.url.String()
}

// GetMethod returns the method of the event, if applicable
func (ae *GenericEvent) GetMethod() string {
	return ae.method
}

// GetField returns the field by name as an string
func (ae *GenericEvent) GetField(key string) string {
	if ae.fields != nil {
		return ae.fields[key]
	}
	return ""
}

// GetFields loads all fields into a map of string / string
func (ae *GenericEvent) GetFields() map[string]string {
	return ae.fields
}

// GetTopic returns the topic of the event
func (ae *GenericEvent) GetTopic() string {
	return ""
}

// GetStatus returns the status of the event, if applicable
func (ae *GenericEvent) GetStatus() int {
	return ae.status
}
func (ae *GenericEvent) GetContext() context.Context {
	return ae.context
}

// GenericEventBuilder provides a builder pattern for creating GenericEvent instances incrementally
type GenericEventBuilder struct {
	ctx     context.Context
	body    []byte
	rawUrl  string
	method  string
	headers map[string]string
	fields  map[string]string
	status  int
}

// NewGenericEventBuilder creates a new builder for GenericEvent with the required context
func NewGenericEventBuilder(ctx context.Context) *GenericEventBuilder {
	return &GenericEventBuilder{
		ctx:     ctx,
		headers: make(map[string]string),
		fields:  make(map[string]string),
		status:  200, // default status
	}
}

// WithBody sets the body of the event
func (b *GenericEventBuilder) WithBody(body []byte) *GenericEventBuilder {
	b.body = body
	return b
}

// WithURL sets the URL of the event
func (b *GenericEventBuilder) WithURL(rawUrl string) *GenericEventBuilder {
	b.rawUrl = rawUrl
	return b
}

// WithMethod sets the HTTP method of the event
func (b *GenericEventBuilder) WithMethod(method string) *GenericEventBuilder {
	b.method = method
	return b
}

// WithHeaders sets the headers map of the event
func (b *GenericEventBuilder) WithHeaders(headers map[string]string) *GenericEventBuilder {
	if headers != nil {
		b.headers = headers
	} else {
		b.headers = make(map[string]string)
	}
	return b
}

// AddHeader adds a single header to the event
func (b *GenericEventBuilder) AddHeader(key, value string) *GenericEventBuilder {
	b.headers[key] = value
	return b
}

// WithFields sets the fields map of the event
func (b *GenericEventBuilder) WithFields(fields map[string]string) *GenericEventBuilder {
	if fields != nil {
		b.fields = fields
	} else {
		b.fields = make(map[string]string)
	}
	return b
}

// AddField adds a single field to the event
func (b *GenericEventBuilder) AddField(key, value string) *GenericEventBuilder {
	b.fields[key] = value
	return b
}

// WithStatus sets the status code of the event
func (b *GenericEventBuilder) WithStatus(status int) *GenericEventBuilder {
	b.status = status
	return b
}

// Build creates the GenericEvent instance
func (b *GenericEventBuilder) Build() (*GenericEvent, error) {
	return NewGenericEvent(b.ctx, b.body, b.rawUrl, b.method, b.headers, b.fields, b.status)
}

func NewEventFrom(ctx context.Context, value any) Event {
	switch body := value.(type) {
	case Event:
		return body
	case string:
		event, _ := NewGenericEvent(ctx, []byte(body), "", "", nil, nil, 200)
		return event
	case []byte:
		event, _ := NewGenericEvent(ctx, body, "", "", nil, nil, 200)
		return event
	default:
		event, _ := NewGenericEvent(ctx, []byte(fmt.Sprintf("%v", value)), "", "", nil, nil, 200)
		return event

	}
}

func ExtractContext(value any) context.Context {
	switch body := value.(type) {
	case Event:
		return body.GetContext()
	default:
		return context.Background()
	}
}

func NewErrorEvent(ctx context.Context, err error, status int) Event {
	event, _ := NewGenericEvent(ctx, []byte(fmt.Sprintf("{\"error\": \"%s\"}", err)), "", "", nil, nil, status)
	return event
}
