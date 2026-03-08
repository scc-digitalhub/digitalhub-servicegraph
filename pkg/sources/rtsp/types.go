// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package rtsp

// MediaType selects whether the source processes video or audio.
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
)

const (
	defaultFrameInterval           = 1
	defaultJPEGQuality             = 80
	defaultAudioMaxSize            = 1 * 1024 * 1024
	defaultAudioProcessingInterval = 1000
	defaultAudioChunkSize          = 0 // 0 = disabled; emit full buffer snapshot
	defaultRetryBackoff            = 1000
)

// Configuration holds all RTSP source settings.
type Configuration struct {
	// URL is the RTSP stream address.
	URL string `json:"url"`
	// MediaType selects what to process: "video" or "audio".
	MediaType MediaType `json:"media_type"`
	// FrameInterval: 1 = every frame, N = one out of N frames (video only).
	FrameInterval int `json:"frame_interval,omitempty"`
	// JPEGQuality controls JPEG encoding quality for H.264 frames (1–100). Default: 80.
	JPEGQuality int `json:"jpeg_quality,omitempty"`
	// AudioMaxSize: maximum rolling audio buffer size in bytes (audio only).
	AudioMaxSize int `json:"audio_max_size,omitempty"`
	// AudioProcessingInterval: interval in ms at which audio data is emitted (audio only).
	AudioProcessingInterval int `json:"audio_processing_interval,omitempty"`
	// AudioChunkSize: if > 0, each event contains the latest AudioChunkSize bytes from
	// the buffer tail; 0 = emit a full buffer snapshot each interval.
	AudioChunkSize int `json:"audio_chunk_size,omitempty"`
	// MaxRetries: max reconnect attempts; 0 = unlimited.
	MaxRetries int `json:"max_retries,omitempty"`
	// RetryBackoff: initial back-off in ms; doubles each attempt, capped at 30 s.
	RetryBackoff int `json:"retry_backoff,omitempty"`
}

// Option is a functional option for NewConfiguration.
type Option func(*Configuration)

// WithFrameInterval sets the frame-skip interval for video (1 = every frame).
func WithFrameInterval(n int) Option {
	return func(c *Configuration) { c.FrameInterval = n }
}

// WithJPEGQuality sets the JPEG encoding quality for H.264 video (1–100).
func WithJPEGQuality(q int) Option {
	return func(c *Configuration) { c.JPEGQuality = q }
}

// WithAudioMaxSize sets the maximum rolling audio buffer size in bytes.
func WithAudioMaxSize(n int) Option {
	return func(c *Configuration) { c.AudioMaxSize = n }
}

// WithAudioProcessingInterval sets the flush interval in milliseconds.
func WithAudioProcessingInterval(ms int) Option {
	return func(c *Configuration) { c.AudioProcessingInterval = ms }
}

// WithAudioChunkSize sets the sliding-window chunk size in bytes (0 = full snapshot).
func WithAudioChunkSize(n int) Option {
	return func(c *Configuration) { c.AudioChunkSize = n }
}

// WithMaxRetries sets the maximum reconnect attempts (0 = unlimited).
func WithMaxRetries(n int) Option {
	return func(c *Configuration) { c.MaxRetries = n }
}

// WithRetryBackoff sets the initial retry back-off in milliseconds.
func WithRetryBackoff(ms int) Option {
	return func(c *Configuration) { c.RetryBackoff = ms }
}

// NewConfiguration returns a Configuration with the supplied options and sensible
// defaults applied for any unset fields.
func NewConfiguration(url string, mediaType MediaType, opts ...Option) *Configuration {
	c := &Configuration{URL: url, MediaType: mediaType}
	for _, opt := range opts {
		opt(c)
	}
	if c.FrameInterval <= 0 {
		c.FrameInterval = defaultFrameInterval
	}
	if c.JPEGQuality <= 0 || c.JPEGQuality > 100 {
		c.JPEGQuality = defaultJPEGQuality
	}
	if c.AudioMaxSize <= 0 {
		c.AudioMaxSize = defaultAudioMaxSize
	}
	if c.AudioProcessingInterval <= 0 {
		c.AudioProcessingInterval = defaultAudioProcessingInterval
	}
	// AudioChunkSize: 0 is a valid setting (disabled), so only apply default if negative.
	if c.AudioChunkSize < 0 {
		c.AudioChunkSize = defaultAudioChunkSize
	}
	if c.RetryBackoff <= 0 {
		c.RetryBackoff = defaultRetryBackoff
	}
	return c
}
