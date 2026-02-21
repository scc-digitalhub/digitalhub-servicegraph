// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package mjpeg

import (
	"context"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

const (
	defaultFrameInterval = 1 // Process every frame
	defaultReadTimeout   = 10 * time.Second
)

// Configuration holds the MJPEG source configuration
type Configuration struct {
	URL           string `json:"url"`
	FrameInterval int    `json:"frame_interval,omitempty"` // Process every Nth frame (1 = all frames, 2 = every 2nd frame, etc.)
	ReadTimeout   int    `json:"read_timeout,omitempty"`   // Read timeout in seconds
}

// MJPEGEvent represents a single JPEG frame from the MJPEG stream
type MJPEGEvent struct {
	streams.GenericEvent
	jpegData  []byte
	frameNum  int64
	timestamp time.Time
	ctx       context.Context
	url       string
}

// NewMJPEGEvent creates a new MJPEG event from JPEG data
func NewMJPEGEvent(ctx context.Context, jpegData []byte, frameNum int64, url string) *MJPEGEvent {
	event := &MJPEGEvent{
		jpegData:  jpegData,
		frameNum:  frameNum,
		timestamp: time.Now(),
		ctx:       ctx,
		url:       url,
	}
	return event
}

// GetContentType returns the content type of the JPEG image
func (e *MJPEGEvent) GetContentType() string {
	return "image/jpeg"
}

// GetBody returns the JPEG image data
func (e *MJPEGEvent) GetBody() []byte {
	return e.jpegData
}

// GetTimestamp returns when the frame was received
func (e *MJPEGEvent) GetTimestamp() time.Time {
	return e.timestamp
}

// GetContext returns the context of the event
func (e *MJPEGEvent) GetContext() context.Context {
	return e.ctx
}

// GetURL returns the source URL
func (e *MJPEGEvent) GetURL() string {
	return e.url
}

// GetFrameNumber returns the frame number
func (e *MJPEGEvent) GetFrameNumber() int64 {
	return e.frameNum
}

// GetHeaders returns custom headers including frame information
func (e *MJPEGEvent) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "image/jpeg",
		"X-Frame-Number": string(rune(e.frameNum)),
	}
}

// GetHeader returns a specific header value
func (e *MJPEGEvent) GetHeader(key string) string {
	headers := e.GetHeaders()
	return headers[key]
}

// NewConfiguration creates a new configuration with defaults
func NewConfiguration(url string, frameInterval, readTimeout int) *Configuration {
	newConfiguration := &Configuration{
		URL:           url,
		FrameInterval: frameInterval,
		ReadTimeout:   readTimeout,
	}
	if frameInterval == 0 {
		newConfiguration.FrameInterval = defaultFrameInterval
	}
	if readTimeout == 0 {
		newConfiguration.ReadTimeout = int(defaultReadTimeout.Seconds())
	}

	return newConfiguration
}
