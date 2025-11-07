package http

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

const (
	defaultServerPort     = 8080
	defaultReadTimeout    = 10 * time.Second
	defaultWriteTimeout   = 10 * time.Second
	defaultProcessTimeout = 30 * time.Second
	defaultMaxInputSize   = 10 * 1024 * 1024 // 10MB in bytes
)

type Configuration struct {
	Port           int
	ReadTimeout    int
	WriteTimeout   int
	ProcessTimeout int
	MaxInputSize   int64
}

type HTTPEvent struct {
	streams.GenericEvent
	r         *http.Request
	body      []byte
	timestamp time.Time
}

func NewHTTPEvent(r *http.Request, maxIputSize int64) (*HTTPEvent, error) {
	event := &HTTPEvent{}

	// Read request body with max size limit
	body, err := io.ReadAll(io.LimitReader(r.Body, maxIputSize))
	if err != nil {
		// http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return nil, errors.New("failed to read request body")
	}
	defer r.Body.Close()
	event.body = body

	event.r = r
	event.timestamp = time.Now()
	return event, nil
}

// GetContentType returns the content type of the body
func (e *HTTPEvent) GetContentType() string {
	return e.GetHeader("Content-Type")
}

// GetBody returns the body of the event
func (e *HTTPEvent) GetBody() []byte {
	return e.body
}

func (e *HTTPEvent) GetURL() string {
	return e.r.URL.String()
}

// GetHeader returns the header by name as an string
func (e *HTTPEvent) GetHeader(key string) string {
	return e.r.Header.Get(key)
}

// GetHeaders loads all headers into a map of string / string
func (e *HTTPEvent) GetHeaders() map[string]string {
	headers := make(map[string]string)
	for key, value := range e.r.Header {
		headers[string(key)] = strings.Join(value, ",")
	}
	return headers
}

// GetPath returns the method of the event, if applicable
func (e *HTTPEvent) GetMethod() string {
	return string(e.r.Method)
}

// GetPath returns the path of the event
func (e *HTTPEvent) GetPath() string {
	return string(e.r.URL.Path)
}

// GetFieldByteSlice returns the field by name as string
func (e *HTTPEvent) GetField(key string) string {
	return e.r.URL.Query().Get(key)
}

// GetFields loads all fields into a map of string / string
func (e *HTTPEvent) GetFields() map[string]string {
	fields := make(map[string]string)
	for key, value := range e.r.URL.Query() {
		fields[string(key)] = strings.Join(value, ",")
	}

	return fields
}

// GetTimestamp returns when the event originated
func (e *HTTPEvent) GetTimestamp() time.Time {
	return e.timestamp
}
func NewConfiguration(port, readTimeout, writeTimeout, processTimeout int, maxInputSize int64) *Configuration {

	newConfiguration := &Configuration{
		Port:           port,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		ProcessTimeout: processTimeout,
		MaxInputSize:   maxInputSize,
	}
	if port == 0 {
		newConfiguration.Port = defaultServerPort
	}
	if readTimeout == 0 {
		newConfiguration.ReadTimeout = int(defaultReadTimeout.Seconds())
	}
	if writeTimeout == 0 {
		newConfiguration.WriteTimeout = int(defaultWriteTimeout.Seconds())
	}
	if processTimeout == 0 {
		newConfiguration.ProcessTimeout = int(defaultProcessTimeout.Seconds())
	}
	if maxInputSize == 0 {
		newConfiguration.MaxInputSize = defaultMaxInputSize
	}

	return newConfiguration
}
