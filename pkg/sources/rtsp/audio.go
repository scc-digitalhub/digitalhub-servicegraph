// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package rtsp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	gortsplib "github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// RTSPAudioEvent carries a snapshot of the rolling LPCM audio buffer.
type RTSPAudioEvent struct {
	streams.GenericEvent
	audioData []byte
	timestamp time.Time
	ctx       context.Context
	url       string
}

// NewRTSPAudioEvent creates a new RTSPAudioEvent with a defensive copy of the audio data.
func NewRTSPAudioEvent(ctx context.Context, audioData []byte, url string) *RTSPAudioEvent {
	dataCopy := make([]byte, len(audioData))
	copy(dataCopy, audioData)
	return &RTSPAudioEvent{
		audioData: dataCopy,
		timestamp: time.Now(),
		ctx:       ctx,
		url:       url,
	}
}

func (e *RTSPAudioEvent) GetContentType() string      { return "audio/L16" }
func (e *RTSPAudioEvent) GetBody() []byte             { return e.audioData }
func (e *RTSPAudioEvent) GetTimestamp() time.Time     { return e.timestamp }
func (e *RTSPAudioEvent) GetContext() context.Context { return e.ctx }
func (e *RTSPAudioEvent) GetURL() string              { return e.url }

func (e *RTSPAudioEvent) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "audio/L16",
		"X-Source-URL": e.url,
	}
}

func (e *RTSPAudioEvent) GetHeader(key string) string { return e.GetHeaders()[key] }

// audioBuffer is a mutex-protected rolling byte buffer.
// Oldest bytes are discarded when the buffer exceeds maxBytes.
type audioBuffer struct {
	mu       sync.Mutex
	data     []byte
	maxBytes int
}

func newAudioBuffer(maxBytes int) *audioBuffer {
	return &audioBuffer{maxBytes: maxBytes}
}

// append adds samples to the buffer, trimming the oldest bytes if needed.
func (b *audioBuffer) append(samples []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, samples...)
	if len(b.data) > b.maxBytes {
		b.data = b.data[len(b.data)-b.maxBytes:]
	}
}

// snapshot returns a copy of the current buffer contents.
func (b *audioBuffer) snapshot() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) == 0 {
		return nil
	}
	out := make([]byte, len(b.data))
	copy(out, b.data)
	return out
}

// latestChunk returns a copy of the last chunkSize bytes in the buffer.
// Returns nil when the buffer contains fewer than chunkSize bytes.
func (b *audioBuffer) latestChunk(chunkSize int) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) < chunkSize {
		return nil
	}
	chunk := make([]byte, chunkSize)
	copy(chunk, b.data[len(b.data)-chunkSize:])
	return chunk
}

// setupAudioTrack finds the LPCM track in the RTSP session, registers the RTP
// callback that fills buf, and calls c.Setup on the media.
// It returns an error if no supported audio format is present in the stream.
func setupAudioTrack(conf *Configuration, c *gortsplib.Client,
	desc *description.Session, buf *audioBuffer, logger *slog.Logger) error {

	var lpcmFmt *format.LPCM
	medi := desc.FindFormat(&lpcmFmt)
	if medi == nil {
		return fmt.Errorf("no supported audio format found in RTSP stream (supported: lpcm)")
	}

	rtpDec, err := lpcmFmt.CreateDecoder()
	if err != nil {
		return fmt.Errorf("creating LPCM RTP decoder: %w", err)
	}

	if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return fmt.Errorf("setting up LPCM audio track: %w", err)
	}

	c.OnPacketRTP(medi, lpcmFmt, func(pkt *rtp.Packet) {
		samples, err := rtpDec.Decode(pkt)
		if err != nil {
			logger.Warn("LPCM decode error", slog.Any("error", err))
			return
		}
		if len(samples) > 0 {
			buf.append(samples)
		}
	})

	logger.Info("LPCM audio track set up",
		slog.Int("bit_depth", lpcmFmt.BitDepth),
		slog.Int("channels", lpcmFmt.ChannelCount),
		slog.Int("sample_rate", lpcmFmt.SampleRate))
	return nil
}

// startAudioFlusher starts a goroutine that periodically reads from buf and
// emits RTSPAudioEvents on output.  It stops when ctx is cancelled.
//
// When conf.AudioChunkSize > 0 the flusher checks whether the buffer holds at
// least AudioChunkSize bytes and, if so, emits one event containing the latest
// AudioChunkSize bytes (tail of the buffer).  When AudioChunkSize == 0 it
// falls back to emitting a full snapshot of the buffer on each tick.
func startAudioFlusher(ctx context.Context, conf *Configuration, buf *audioBuffer, output chan<- any) {
	flushInterval := time.Duration(conf.AudioProcessingInterval) * time.Millisecond
	chunkSize := conf.AudioChunkSize
	go func() {
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var data []byte
				if chunkSize > 0 {
					data = buf.latestChunk(chunkSize)
				} else {
					data = buf.snapshot()
				}
				if len(data) == 0 {
					continue
				}
				event := NewRTSPAudioEvent(ctx, data, conf.URL)
				select {
				case output <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}
