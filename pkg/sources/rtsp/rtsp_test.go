// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package rtsp

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
)

// ---- Configuration ---------------------------------------------------------

func TestNewConfigurationDefaults(t *testing.T) {
	c := NewConfiguration("rtsp://cam/live", MediaTypeVideo)
	if c.URL != "rtsp://cam/live" {
		t.Fatalf("expected URL to be set, got %q", c.URL)
	}
	if c.MediaType != MediaTypeVideo {
		t.Fatalf("expected MediaType=video, got %q", c.MediaType)
	}
	if c.FrameInterval != defaultFrameInterval {
		t.Fatalf("expected default FrameInterval=%d, got %d", defaultFrameInterval, c.FrameInterval)
	}
	if c.JPEGQuality != defaultJPEGQuality {
		t.Fatalf("expected default JPEGQuality=%d, got %d", defaultJPEGQuality, c.JPEGQuality)
	}
	if c.AudioMaxSize != defaultAudioMaxSize {
		t.Fatalf("expected default AudioMaxSize=%d, got %d", defaultAudioMaxSize, c.AudioMaxSize)
	}
	if c.AudioProcessingInterval != defaultAudioProcessingInterval {
		t.Fatalf("expected default AudioProcessingInterval=%d, got %d", defaultAudioProcessingInterval, c.AudioProcessingInterval)
	}
	if c.AudioChunkSize != defaultAudioChunkSize {
		t.Fatalf("expected default AudioChunkSize=%d, got %d", defaultAudioChunkSize, c.AudioChunkSize)
	}
	if c.RetryBackoff != defaultRetryBackoff {
		t.Fatalf("expected default RetryBackoff=%d, got %d", defaultRetryBackoff, c.RetryBackoff)
	}
}

func TestNewConfigurationExplicitValues(t *testing.T) {
	c := NewConfiguration("rtsp://cam/live", MediaTypeAudio,
		WithFrameInterval(3),
		WithAudioMaxSize(2048),
		WithAudioProcessingInterval(500),
		WithAudioChunkSize(4096),
		WithMaxRetries(5),
		WithRetryBackoff(2000),
		WithJPEGQuality(70),
	)
	if c.MediaType != MediaTypeAudio {
		t.Fatalf("expected MediaType=audio, got %q", c.MediaType)
	}
	if c.FrameInterval != 3 {
		t.Fatalf("expected FrameInterval=3, got %d", c.FrameInterval)
	}
	if c.AudioMaxSize != 2048 {
		t.Fatalf("expected AudioMaxSize=2048, got %d", c.AudioMaxSize)
	}
	if c.AudioProcessingInterval != 500 {
		t.Fatalf("expected AudioProcessingInterval=500, got %d", c.AudioProcessingInterval)
	}
	if c.AudioChunkSize != 4096 {
		t.Fatalf("expected AudioChunkSize=4096, got %d", c.AudioChunkSize)
	}
	if c.MaxRetries != 5 {
		t.Fatalf("expected MaxRetries=5, got %d", c.MaxRetries)
	}
	if c.RetryBackoff != 2000 {
		t.Fatalf("expected RetryBackoff=2000, got %d", c.RetryBackoff)
	}
	if c.JPEGQuality != 70 {
		t.Fatalf("expected JPEGQuality=70, got %d", c.JPEGQuality)
	}
}

// ---- RTSPVideoEvent --------------------------------------------------------

func TestNewRTSPVideoEvent(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	frameNum := int64(7)
	url := "rtsp://cam/live"
	ctx := context.Background()
	e := NewRTSPVideoEvent(ctx, jpegData, frameNum, url)
	if !bytes.Equal(e.GetBody(), jpegData) {
		t.Fatalf("body mismatch")
	}
	if e.GetContentType() != "image/jpeg" {
		t.Fatalf("expected content-type image/jpeg, got %q", e.GetContentType())
	}
	if e.GetURL() != url {
		t.Fatalf("expected URL %q, got %q", url, e.GetURL())
	}
	if e.GetContext() != ctx {
		t.Fatalf("context mismatch")
	}
	if e.GetTimestamp().IsZero() {
		t.Fatalf("expected non-zero timestamp")
	}
}

func TestRTSPVideoEvent_Headers(t *testing.T) {
	e := NewRTSPVideoEvent(context.Background(), []byte{0xFF}, 42, "rtsp://cam/live")
	headers := e.GetHeaders()
	if headers["Content-Type"] != "image/jpeg" {
		t.Fatalf("expected Content-Type image/jpeg, got %q", headers["Content-Type"])
	}
	if headers["X-Frame-Number"] != "42" {
		t.Fatalf("expected X-Frame-Number=42, got %q", headers["X-Frame-Number"])
	}
	if headers["X-Source-URL"] != "rtsp://cam/live" {
		t.Fatalf("expected X-Source-URL header, got %q", headers["X-Source-URL"])
	}
	if e.GetHeader("Content-Type") != "image/jpeg" {
		t.Fatalf("GetHeader(Content-Type) mismatch")
	}
}

// ---- RTSPAudioEvent --------------------------------------------------------

func TestNewRTSPAudioEvent(t *testing.T) {
	audioData := []byte{0x00, 0x01, 0x02, 0x03}
	url := "rtsp://cam/live"
	ctx := context.Background()
	e := NewRTSPAudioEvent(ctx, audioData, url, "lpcm", 44100, 16, 2)
	if !bytes.Equal(e.GetBody(), audioData) {
		t.Fatalf("body mismatch")
	}
	if e.GetContentType() != "audio/L16" {
		t.Fatalf("expected content-type audio/L16, got %q", e.GetContentType())
	}
	if e.GetURL() != url {
		t.Fatalf("expected URL %q, got %q", url, e.GetURL())
	}
	if e.GetContext() != ctx {
		t.Fatalf("context mismatch")
	}
	if e.GetTimestamp().IsZero() {
		t.Fatalf("expected non-zero timestamp")
	}
}

func TestNewRTSPAudioEvent_DefensiveCopy(t *testing.T) {
	audioData := []byte{0x10, 0x20, 0x30}
	e := NewRTSPAudioEvent(context.Background(), audioData, "rtsp://cam/live", "lpcm", 16000, 16, 1)
	audioData[0] = 0xFF
	if e.GetBody()[0] == 0xFF {
		t.Fatalf("RTSPAudioEvent should store a defensive copy of audio data")
	}
}

func TestRTSPAudioEvent_Headers(t *testing.T) {
	e := NewRTSPAudioEvent(context.Background(), []byte{}, "rtsp://cam/live", "g711-ulaw", 48000, 16, 2)
	headers := e.GetHeaders()
	if headers["Content-Type"] != "audio/L16" {
		t.Fatalf("expected Content-Type=audio/L16, got %q", headers["Content-Type"])
	}
	if headers["X-Source-URL"] != "rtsp://cam/live" {
		t.Fatalf("expected X-Source-URL header, got %q", headers["X-Source-URL"])
	}
	if headers["X-Audio-Codec"] != "g711-ulaw" {
		t.Fatalf("expected X-Audio-Codec=g711-ulaw, got %q", headers["X-Audio-Codec"])
	}
	if headers["X-Audio-Sample-Rate"] != "48000" {
		t.Fatalf("expected X-Audio-Sample-Rate=48000, got %q", headers["X-Audio-Sample-Rate"])
	}
	if headers["X-Audio-Bit-Depth"] != "16" {
		t.Fatalf("expected X-Audio-Bit-Depth=16, got %q", headers["X-Audio-Bit-Depth"])
	}
	if headers["X-Audio-Channels"] != "2" {
		t.Fatalf("expected X-Audio-Channels=2, got %q", headers["X-Audio-Channels"])
	}
	if e.GetHeader("Content-Type") != "audio/L16" {
		t.Fatalf("GetHeader(Content-Type) mismatch")
	}
}

// ---- audioBuffer -----------------------------------------------------------

func TestAudioBuffer_AppendAndSnapshot(t *testing.T) {
	buf := newAudioBuffer(100)
	buf.append([]byte{1, 2, 3})
	buf.append([]byte{4, 5})
	snap := buf.snapshot()
	if !bytes.Equal(snap, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("unexpected snapshot: %v", snap)
	}
}

func TestAudioBuffer_SnapshotEmptyReturnsNil(t *testing.T) {
	buf := newAudioBuffer(100)
	if buf.snapshot() != nil {
		t.Fatalf("expected nil snapshot for empty buffer")
	}
}

func TestAudioBuffer_SnapshotIsACopy(t *testing.T) {
	buf := newAudioBuffer(100)
	buf.append([]byte{1, 2, 3})
	snap := buf.snapshot()
	snap[0] = 0xFF
	snap2 := buf.snapshot()
	if snap2[0] == 0xFF {
		t.Fatalf("snapshot should be an independent copy")
	}
}

func TestAudioBuffer_OverflowTrimsOldest(t *testing.T) {
	buf := newAudioBuffer(4)
	buf.append([]byte{1, 2, 3, 4, 5, 6})
	snap := buf.snapshot()
	if len(snap) != 4 {
		t.Fatalf("expected buffer size 4, got %d", len(snap))
	}
	if !bytes.Equal(snap, []byte{3, 4, 5, 6}) {
		t.Fatalf("expected oldest bytes trimmed, got %v", snap)
	}
}

func TestAudioBuffer_IncrementalOverflow(t *testing.T) {
	buf := newAudioBuffer(4)
	buf.append([]byte{1, 2})
	buf.append([]byte{3, 4})
	buf.append([]byte{5, 6})
	snap := buf.snapshot()
	if !bytes.Equal(snap, []byte{3, 4, 5, 6}) {
		t.Fatalf("expected {3,4,5,6}, got %v", snap)
	}
}

func TestAudioBuffer_ConcurrentAppendSnapshot(t *testing.T) {
	buf := newAudioBuffer(1024)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			buf.append([]byte{byte(i)})
		}(i)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf.snapshot()
		}()
	}
	wg.Wait()
	snap := buf.snapshot()
	if len(snap) > 50 {
		t.Fatalf("unexpected snapshot length %d", len(snap))
	}
}

// ---- audioBuffer latestChunk -----------------------------------------------

func TestAudioBuffer_LatestChunk_Basic(t *testing.T) {
	buf := newAudioBuffer(1024)
	buf.append([]byte{1, 2, 3, 4, 5, 6})
	chunk := buf.latestChunk(3)
	if !bytes.Equal(chunk, []byte{4, 5, 6}) {
		t.Fatalf("expected latest 3 bytes {4,5,6}, got %v", chunk)
	}
}

func TestAudioBuffer_LatestChunk_ExactSize(t *testing.T) {
	buf := newAudioBuffer(1024)
	buf.append([]byte{1, 2, 3})
	chunk := buf.latestChunk(3)
	if !bytes.Equal(chunk, []byte{1, 2, 3}) {
		t.Fatalf("expected {1,2,3}, got %v", chunk)
	}
}

func TestAudioBuffer_LatestChunk_InsufficientData(t *testing.T) {
	buf := newAudioBuffer(1024)
	buf.append([]byte{1, 2})
	if buf.latestChunk(3) != nil {
		t.Fatalf("expected nil when buffer has fewer bytes than chunk size")
	}
}

func TestAudioBuffer_LatestChunk_EmptyBuffer(t *testing.T) {
	buf := newAudioBuffer(1024)
	if buf.latestChunk(4) != nil {
		t.Fatalf("expected nil for empty buffer")
	}
}

func TestAudioBuffer_LatestChunk_IsACopy(t *testing.T) {
	buf := newAudioBuffer(1024)
	buf.append([]byte{1, 2, 3, 4})
	chunk := buf.latestChunk(4)
	chunk[0] = 0xFF
	snap := buf.snapshot()
	if snap[0] == 0xFF {
		t.Fatalf("latestChunk should return a copy, not a slice of the internal buffer")
	}
}

func TestAudioBuffer_LatestChunk_AlwaysLatest(t *testing.T) {
	// Each call returns the tail of the current buffer, not an advancing cursor.
	buf := newAudioBuffer(1024)
	buf.append([]byte{1, 2, 3, 4})
	chunk1 := buf.latestChunk(2)
	if !bytes.Equal(chunk1, []byte{3, 4}) {
		t.Fatalf("expected {3,4}, got %v", chunk1)
	}
	// Second call without new data should return the same tail.
	chunk2 := buf.latestChunk(2)
	if !bytes.Equal(chunk2, []byte{3, 4}) {
		t.Fatalf("expected {3,4} again, got %v", chunk2)
	}
	// After appending, the tail updates.
	buf.append([]byte{5, 6})
	chunk3 := buf.latestChunk(2)
	if !bytes.Equal(chunk3, []byte{5, 6}) {
		t.Fatalf("expected {5,6} after append, got %v", chunk3)
	}
}

func TestAudioBuffer_LatestChunk_AfterOverflow(t *testing.T) {
	// Buffer capped at 4 bytes. The latest 2 bytes are always the tail.
	buf := newAudioBuffer(4)
	buf.append([]byte{1, 2, 3, 4, 5, 6})
	// Buffer holds [3,4,5,6].
	chunk := buf.latestChunk(2)
	if !bytes.Equal(chunk, []byte{5, 6}) {
		t.Fatalf("expected {5,6} after overflow trim, got %v", chunk)
	}
}

// ---- Validate --------------------------------------------------------------

func TestRTSPSourceProcessor_Validate(t *testing.T) {
	p := &RTSPSourceProcessor{}
	tests := []struct {
		name    string
		spec    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid video",
			spec:    map[string]interface{}{"url": "rtsp://cam/live", "media_type": "video"},
			wantErr: false,
		},
		{
			name:    "valid audio",
			spec:    map[string]interface{}{"url": "rtsp://cam/live", "media_type": "audio"},
			wantErr: false,
		},
		{
			name: "valid video with extra fields",
			spec: map[string]interface{}{
				"url":              "rtsp://cam/live",
				"media_type":       "video",
				"frame_interval":   2,
				"max_retries":      3,
				"retry_backoff_ms": 500,
			},
			wantErr: false,
		},
		{
			name: "valid audio with extra fields",
			spec: map[string]interface{}{
				"url":                       "rtsp://cam/live",
				"media_type":                "audio",
				"audio_max_size":            4096,
				"audio_processing_interval": 500,
				"audio_chunk_size":          1024,
			},
			wantErr: false,
		},
		{
			name:    "missing url",
			spec:    map[string]interface{}{"media_type": "video"},
			wantErr: true,
		},
		{
			name:    "missing media_type",
			spec:    map[string]interface{}{"url": "rtsp://cam/live"},
			wantErr: true,
		},
		{
			name:    "invalid media_type",
			spec:    map[string]interface{}{"url": "rtsp://cam/live", "media_type": "both"},
			wantErr: true,
		},
		{
			name:    "negative frame interval",
			spec:    map[string]interface{}{"url": "rtsp://cam/live", "media_type": "video", "frame_interval": -1},
			wantErr: true,
		},
		{
			name:    "negative audio buffer size",
			spec:    map[string]interface{}{"url": "rtsp://cam/live", "media_type": "audio", "audio_max_size": -1},
			wantErr: true,
		},
		{
			name:    "negative audio flush interval",
			spec:    map[string]interface{}{"url": "rtsp://cam/live", "media_type": "audio", "audio_processing_interval": -1},
			wantErr: true,
		},
		{
			name:    "negative audio chunk size",
			spec:    map[string]interface{}{"url": "rtsp://cam/live", "media_type": "audio", "audio_chunk_size": -1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Validate(model.InputSpec{Spec: tt.spec})
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// ---- Convert ---------------------------------------------------------------

func TestRTSPSourceProcessor_Convert_Video(t *testing.T) {
	p := &RTSPSourceProcessor{}
	spec := map[string]interface{}{
		"url":            "rtsp://cam/live",
		"media_type":     "video",
		"frame_interval": 3,
	}
	src, err := p.Convert(model.InputSpec{Spec: spec})
	if err != nil {
		t.Fatalf("Convert() unexpected error: %v", err)
	}
	rtspSrc, ok := src.(*RTSPSource)
	if !ok {
		t.Fatalf("Convert() did not return *RTSPSource")
	}
	if rtspSrc.Conf.URL != "rtsp://cam/live" {
		t.Fatalf("expected URL rtsp://cam/live, got %q", rtspSrc.Conf.URL)
	}
	if rtspSrc.Conf.MediaType != MediaTypeVideo {
		t.Fatalf("expected MediaType=video, got %q", rtspSrc.Conf.MediaType)
	}
	if rtspSrc.Conf.FrameInterval != 3 {
		t.Fatalf("expected FrameInterval=3, got %d", rtspSrc.Conf.FrameInterval)
	}
	if rtspSrc.Conf.RetryBackoff != defaultRetryBackoff {
		t.Fatalf("expected default RetryBackoff, got %d", rtspSrc.Conf.RetryBackoff)
	}
}

func TestRTSPSourceProcessor_Convert_Audio(t *testing.T) {
	p := &RTSPSourceProcessor{}
	spec := map[string]interface{}{
		"url":                       "rtsp://cam/live",
		"media_type":                "audio",
		"audio_max_size":            2048,
		"audio_processing_interval": 200,
		"audio_chunk_size":          512,
	}
	src, err := p.Convert(model.InputSpec{Spec: spec})
	if err != nil {
		t.Fatalf("Convert() unexpected error: %v", err)
	}
	rtspSrc, ok := src.(*RTSPSource)
	if !ok {
		t.Fatalf("Convert() did not return *RTSPSource")
	}
	if rtspSrc.Conf.MediaType != MediaTypeAudio {
		t.Fatalf("expected MediaType=audio, got %q", rtspSrc.Conf.MediaType)
	}
	if rtspSrc.Conf.AudioMaxSize != 2048 {
		t.Fatalf("expected AudioMaxSize=2048, got %d", rtspSrc.Conf.AudioMaxSize)
	}
	if rtspSrc.Conf.AudioProcessingInterval != 200 {
		t.Fatalf("expected AudioProcessingInterval=200, got %d", rtspSrc.Conf.AudioProcessingInterval)
	}
	if rtspSrc.Conf.AudioChunkSize != 512 {
		t.Fatalf("expected AudioChunkSize=512, got %d", rtspSrc.Conf.AudioChunkSize)
	}
}

func TestRTSPSourceProcessor_Convert_InvalidSpec(t *testing.T) {
	p := &RTSPSourceProcessor{}
	// Passing a slice for the url field causes JSON unmarshal to fail.
	_, err := p.Convert(model.InputSpec{Spec: map[string]interface{}{
		"url": []string{"not", "a", "string"},
	}})
	if err == nil {
		t.Fatalf("expected error for invalid spec, got nil")
	}
}
