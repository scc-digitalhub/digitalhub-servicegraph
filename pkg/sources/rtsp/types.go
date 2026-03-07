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
	defaultFrameInterval      = 1
	defaultAudioMaxSize       = 1 * 1024 * 1024
	defaultAudioProcessingInterval = 1000
	defaultAudioChunkSize     = 0 // 0 = disabled; emit full buffer snapshot
	defaultRetryBackoff       = 1000
)

// Configuration holds all RTSP source settings.
type Configuration struct {
	// URL is the RTSP stream address.
	URL string `json:"url"`
	// MediaType selects what to process: "video" or "audio".
	MediaType MediaType `json:"media_type"`
	// FrameInterval: 1 = every frame, N = one out of N frames (video only).
	FrameInterval int `json:"frame_interval,omitempty"`
	// AudioMaxSize: maximum rolling audio buffer size in bytes (audio only).
	AudioMaxSize int `json:"audio_max_size,omitempty"`
	// AudioProcessingInterval: interval in ms at which audio data is emitted (audio only).
	AudioProcessingInterval int `json:"audio_processing_interval,omitempty"`
	// AudioChunkSize: if > 0, emit exactly this many bytes per message in a sliding
	// manner (advancing the read cursor); 0 = emit a full buffer snapshot each interval.
	AudioChunkSize int `json:"audio_chunk_size,omitempty"`
	// MaxRetries: max reconnect attempts; 0 = unlimited.
	MaxRetries int `json:"max_retries,omitempty"`
	// RetryBackoff: initial back-off in ms; doubles each attempt, capped at 30 s.
	RetryBackoff int `json:"retry_backoff,omitempty"`
}

// NewConfiguration returns a Configuration with sensible defaults applied.
func NewConfiguration(url string, mediaType MediaType,
	frameInterval, audioBufBytes, audioFlushMs, audioChunkSize, maxRetries, retryBackoffMs int) *Configuration {
	c := &Configuration{
		URL:                url,
		MediaType:          mediaType,
		FrameInterval:      frameInterval,
		AudioMaxSize:       audioBufBytes,
		AudioProcessingInterval: audioFlushMs,
		AudioChunkSize:     audioChunkSize,
		MaxRetries:         maxRetries,
		RetryBackoff:       retryBackoffMs,
	}
	if c.FrameInterval <= 0 {
		c.FrameInterval = defaultFrameInterval
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
