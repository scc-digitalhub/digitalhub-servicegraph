// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package mjpeg

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	ext "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
)

type testFactory struct{}

func (f *testFactory) GenerateFlow(source streams.Source, sink streams.Sink) {
	go func() {
		for v := range source.Out() {
			sink.In() <- v
		}
		if ch, ok := sink.(*ext.ChanSink); ok {
			close(ch.Out)
		}
	}()
}

func TestNewConfigurationDefaults(t *testing.T) {
	c := NewConfiguration("http://example.com/stream", 0, 0)
	if c.URL != "http://example.com/stream" {
		t.Fatalf("expected URL to be set")
	}
	if c.FrameInterval == 0 {
		t.Fatalf("expected default frame interval to be set")
	}
	if c.ReadTimeout == 0 {
		t.Fatalf("expected default read timeout to be set")
	}
}

func TestNewMJPEGEvent(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG header
	frameNum := int64(42)
	url := "http://example.com/stream"

	event := NewMJPEGEvent(context.Background(), jpegData, frameNum, url)

	if !bytes.Equal(event.GetBody(), jpegData) {
		t.Fatalf("expected body to match jpeg data")
	}
	if event.GetContentType() != "image/jpeg" {
		t.Fatalf("expected content type to be image/jpeg, got %s", event.GetContentType())
	}
	if event.GetFrameNumber() != frameNum {
		t.Fatalf("expected frame number %d, got %d", frameNum, event.GetFrameNumber())
	}
	if event.GetURL() != url {
		t.Fatalf("expected URL %s, got %s", url, event.GetURL())
	}
}

func TestMJPEGSource_ProcessMultipartStream(t *testing.T) {
	// Create a test MJPEG stream
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Write first frame
	part1, _ := mw.CreatePart(map[string][]string{
		"Content-Type": {"image/jpeg"},
	})
	jpegData1 := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10} // Fake JPEG
	part1.Write(jpegData1)

	// Write second frame
	part2, _ := mw.CreatePart(map[string][]string{
		"Content-Type": {"image/jpeg"},
	})
	jpegData2 := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x20} // Fake JPEG
	part2.Write(jpegData2)

	mw.Close()

	// Create source
	conf := NewConfiguration("http://example.com/stream", 1, 5)
	src := NewMJPEGSource(conf)

	// Create output channel
	output := make(chan any, 10)

	// Process stream with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		src.processMultipartStream(ctx, &buf, mw.Boundary(), output)
		close(output)
	}()

	// Read frames
	frameCount := 0
	for event := range output {
		if mjpegEvent, ok := event.(*MJPEGEvent); ok {
			frameCount++
			if len(mjpegEvent.GetBody()) == 0 {
				t.Fatalf("expected non-empty JPEG data")
			}
		}
	}

	if frameCount != 2 {
		t.Fatalf("expected 2 frames, got %d", frameCount)
	}
}

func TestMJPEGSource_FrameInterval(t *testing.T) {
	// Create a test MJPEG stream with 10 frames
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	for i := 0; i < 10; i++ {
		part, _ := mw.CreatePart(map[string][]string{
			"Content-Type": {"image/jpeg"},
		})
		jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, byte(i)} // Fake JPEG with frame marker
		part.Write(jpegData)
	}
	mw.Close()

	// Create source with frame interval of 3 (process every 3rd frame)
	conf := NewConfiguration("http://example.com/stream", 3, 5)
	src := NewMJPEGSource(conf)

	// Create output channel
	output := make(chan any, 10)

	// Process stream
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		src.processMultipartStream(ctx, &buf, mw.Boundary(), output)
		close(output)
	}()

	// Read frames
	frameCount := 0
	for range output {
		frameCount++
	}

	// With 10 frames and interval of 3, we should get frames 1, 4, 7, 10 = 4 frames
	// (frames are 1-indexed, interval check is (frameNum-1) % interval == 0)
	expectedFrames := 4
	if frameCount != expectedFrames {
		t.Fatalf("expected %d frames with interval 3, got %d", expectedFrames, frameCount)
	}
}

func TestMJPEGSourceProcessor_Validate(t *testing.T) {
	processor := &MJPEGSourceProcessor{}

	tests := []struct {
		name    string
		spec    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid configuration",
			spec: map[string]interface{}{
				"url":            "http://example.com/stream",
				"frame_interval": 1,
				"read_timeout":   10,
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			spec: map[string]interface{}{
				"frame_interval": 1,
			},
			wantErr: true,
		},
		{
			name: "invalid frame interval",
			spec: map[string]interface{}{
				"url":            "http://example.com/stream",
				"frame_interval": 0,
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			spec: map[string]interface{}{
				"url": "http://example.com/stream",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.Validate(model.InputSpec{Spec: tt.spec})

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func createTestMJPEGServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := multipart.NewWriter(w)
		w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", mw.Boundary()))

		// Send a few frames
		for i := 0; i < 5; i++ {
			part, _ := mw.CreatePart(map[string][]string{
				"Content-Type": {"image/jpeg"},
			})
			jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, byte(i)}
			part.Write(jpegData)
			time.Sleep(10 * time.Millisecond)
		}
		mw.Close()
	}))
}
