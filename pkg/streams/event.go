package streams

import (
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
}

type GenericEvent struct {
	Event
	id      ID
	body    []byte
	method  string
	url     *url.URL
	headers map[string]string
	fields  map[string]string
}

var _ Event = (*GenericEvent)(nil)

func NewGenericEvent(body []byte, rawUrl string, method string, headers, fields map[string]string) (*GenericEvent, error) {
	event := &GenericEvent{}
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

func NewEventFrom(value interface{}) Event {
	switch body := value.(type) {
	case Event:
		return body
	case string:
		event, _ := NewGenericEvent([]byte(body), "", "", nil, nil)
		return event
	case []byte:
		event, _ := NewGenericEvent(body, "", "", nil, nil)
		return event
	default:
		event, _ := NewGenericEvent([]byte(fmt.Sprintf("%w", value)), "", "", nil, nil)
		return event

	}
}

func NewErrorEvent(err error) Event {
	event, _ := NewGenericEvent([]byte(fmt.Sprintf("{\"error\": \"%s\"}", err)), "", "", nil, nil)
	return event
}
