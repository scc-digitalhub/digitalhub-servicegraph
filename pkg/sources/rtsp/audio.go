// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package rtsp

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"time"

	gortsplib "github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"

	"github.com/pion/rtp"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/rtsp/g711"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// RTSPAudioEvent carries a snapshot of the rolling LPCM audio buffer.
// All payloads are delivered as 16-bit big-endian linear PCM regardless of
// the wire codec (LPCM, G.711 µ-law, or G.711 A-law); the original codec is
// reported in the X-Audio-Codec header so downstream nodes can identify the
// source without inspecting the bytes.
type RTSPAudioEvent struct {
	streams.GenericEvent
	audioData  []byte
	codec      string // "lpcm", "g711-ulaw", "g711-alaw"
	sampleRate int
	bitDepth   int
	channels   int
	timestamp  time.Time
	ctx        context.Context
	url        string
}

// NewRTSPAudioEvent creates a new RTSPAudioEvent with a defensive copy of the
// audio data.  codec identifies the wire format ("lpcm", "g711-ulaw",
// "g711-alaw"); sampleRate, bitDepth and channels describe the expanded PCM.
func NewRTSPAudioEvent(ctx context.Context, audioData []byte, url string,
	codec string, sampleRate, bitDepth, channels int) *RTSPAudioEvent {
	dataCopy := make([]byte, len(audioData))
	copy(dataCopy, audioData)
	return &RTSPAudioEvent{
		audioData:  dataCopy,
		codec:      codec,
		sampleRate: sampleRate,
		bitDepth:   bitDepth,
		channels:   channels,
		timestamp:  time.Now(),
		ctx:        ctx,
		url:        url,
	}
}

func (e *RTSPAudioEvent) GetContentType() string      { return "audio/L16" }
func (e *RTSPAudioEvent) GetBody() []byte             { return e.audioData }
func (e *RTSPAudioEvent) GetTimestamp() time.Time     { return e.timestamp }
func (e *RTSPAudioEvent) GetContext() context.Context { return e.ctx }
func (e *RTSPAudioEvent) GetURL() string              { return e.url }

func (e *RTSPAudioEvent) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type":        "audio/L16",
		"X-Source-URL":        e.url,
		"X-Audio-Codec":       e.codec,
		"X-Audio-Sample-Rate": fmt.Sprintf("%d", e.sampleRate),
		"X-Audio-Bit-Depth":   fmt.Sprintf("%d", e.bitDepth),
		"X-Audio-Channels":    fmt.Sprintf("%d", e.channels),
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

func convertBigEndianToLittleEndian(in []byte, logger *slog.Logger) []byte {
	out := make([]byte, len(in))
	for i := 0; i+1 < len(in); i += 2 {
		v := binary.BigEndian.Uint16(in[i:])
		binary.LittleEndian.PutUint16(out[i:], v)
	}
	return out
}

// audioFormat holds the PCM encoding parameters discovered during track setup.
// They are forwarded to every RTSPAudioEvent so downstream nodes can interpret
// the payload.  bitDepth is always 16 (the expanded L16 representation);
// codec identifies the wire encoding before expansion.
type audioFormat struct {
	codec      string // "lpcm", "g711-ulaw", "g711-alaw"
	sampleRate int
	bitDepth   int // always 16 after expansion
	channels   int
}

// setupAudioTrack probes the RTSP session for a supported audio format,
// registers an RTP callback that fills buf with 16-bit big-endian LPCM, and
// calls c.Setup on the selected media.
//
// Priority: LPCM (direct) → G.711 µ-law → G.711 A-law.
// G.711 samples are expanded to 16-bit linear PCM inline with no external
// tools; the original wire codec is recorded in the returned audioFormat.
func setupAudioTrack(conf *Configuration, c *gortsplib.Client,
	desc *description.Session, buf *audioBuffer, logger *slog.Logger) (audioFormat, error) {

	// ── LPCM (preferred) ────────────────────────────────────────────────────
	var lpcmFmt *format.LPCM
	if medi := desc.FindFormat(&lpcmFmt); medi != nil {
		rtpDec, err := lpcmFmt.CreateDecoder()
		if err != nil {
			return audioFormat{}, fmt.Errorf("creating LPCM RTP decoder: %w", err)
		}
		if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
			return audioFormat{}, fmt.Errorf("setting up LPCM audio track: %w", err)
		}
		c.OnPacketRTP(medi, lpcmFmt, func(pkt *rtp.Packet) {
			samples, err := rtpDec.Decode(pkt)
			samples = convertBigEndianToLittleEndian(samples, logger)
			if err != nil {
				logger.Warn("LPCM decode error", slog.Any("error", err))
				return
			}
			if len(samples) > 0 {
				buf.append(samples)
			}
		})
		af := audioFormat{
			codec:      "lpcm",
			sampleRate: lpcmFmt.SampleRate,
			bitDepth:   lpcmFmt.BitDepth,
			channels:   lpcmFmt.ChannelCount,
		}
		logger.Info("LPCM audio track set up",
			slog.Int("bit_depth", af.bitDepth),
			slog.Int("channels", af.channels),
			slog.Int("sample_rate", af.sampleRate))
		return af, nil
	}

	// ── G.711 µ-law / A-law fallback ────────────────────────────────────────
	var g711Fmt *format.G711
	medi := desc.FindFormat(&g711Fmt)
	if medi == nil {
		return audioFormat{}, fmt.Errorf(
			"no supported audio format found in RTSP stream (supported: lpcm, g711 mu-law/a-law)")
	}

	rtpDec, err := g711Fmt.CreateDecoder()
	if err != nil {
		return audioFormat{}, fmt.Errorf("creating G.711 RTP decoder: %w", err)
	}
	if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return audioFormat{}, fmt.Errorf("setting up G.711 audio track: %w", err)
	}

	// Select the codec-specific expander once at setup time so the RTP
	// callback is branch-free on the hot path.
	var expand func([]byte) []byte
	codecName := "g711-alaw"
	if g711Fmt.MULaw {
		expand = g711.MulawToLPCM
		codecName = "g711-ulaw"
	} else {
		expand = g711.AlawToLPCM
	}

	c.OnPacketRTP(medi, g711Fmt, func(pkt *rtp.Packet) {
		raw, err := rtpDec.Decode(pkt)
		if err != nil {
			logger.Warn("G.711 decode error", slog.Any("error", err))
			return
		}
		if len(raw) > 0 {
			buf.append(expand(raw)) // expand 8-bit log to 16-bit linear
		}
	})

	af := audioFormat{
		codec:      codecName,
		sampleRate: g711Fmt.SampleRate,
		bitDepth:   16, // expanded to L16
		channels:   g711Fmt.ChannelCount,
	}
	logger.Info("G.711 audio track set up",
		slog.String("codec", codecName),
		slog.Int("channels", af.channels),
		slog.Int("sample_rate", af.sampleRate))
	return af, nil
}

// startAudioFlusher starts a goroutine that periodically reads from buf and
// emits RTSPAudioEvents on output.  It stops when ctx is cancelled.
//
// When conf.AudioChunkSize > 0 the flusher checks whether the buffer holds at
// least AudioChunkSize bytes and, if so, emits one event containing the latest
// AudioChunkSize bytes (tail of the buffer).  When AudioChunkSize == 0 it
// falls back to emitting a full snapshot of the buffer on each tick.
//
// af carries the PCM encoding parameters (sample rate, bit depth, channels) that
// are embedded in every emitted event header.
func startAudioFlusher(ctx context.Context, conf *Configuration, buf *audioBuffer, af audioFormat, output chan<- any) {
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
				event := NewRTSPAudioEvent(ctx, data, conf.URL, af.codec, af.sampleRate, af.bitDepth, af.channels)
				select {
				case output <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}
