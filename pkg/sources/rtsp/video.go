// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package rtsp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	gortsplib "github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpmjpeg"
	"github.com/pion/rtp"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// RTSPVideoEvent carries a JPEG-encoded video frame from an RTSP stream.
type RTSPVideoEvent struct {
	streams.GenericEvent
	jpegData  []byte
	frameNum  int64
	timestamp time.Time
	ctx       context.Context
	url       string
}

// NewRTSPVideoEvent creates a new RTSPVideoEvent.
func NewRTSPVideoEvent(ctx context.Context, jpegData []byte, frameNum int64, url string) *RTSPVideoEvent {
	return &RTSPVideoEvent{
		jpegData:  jpegData,
		frameNum:  frameNum,
		timestamp: time.Now(),
		ctx:       ctx,
		url:       url,
	}
}

func (e *RTSPVideoEvent) GetContentType() string      { return "image/jpeg" }
func (e *RTSPVideoEvent) GetBody() []byte              { return e.jpegData }
func (e *RTSPVideoEvent) GetTimestamp() time.Time      { return e.timestamp }
func (e *RTSPVideoEvent) GetContext() context.Context  { return e.ctx }
func (e *RTSPVideoEvent) GetURL() string               { return e.url }

func (e *RTSPVideoEvent) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "image/jpeg",
		"X-Frame-Number": fmt.Sprintf("%d", e.frameNum),
		"X-Source-URL":   e.url,
	}
}

func (e *RTSPVideoEvent) GetHeader(key string) string { return e.GetHeaders()[key] }

// setupVideoTrack finds the MJPEG track in the RTSP session, registers the RTP
// callback that emits RTSPVideoEvents, and calls c.Setup on the media.
// It returns an error if no supported video format is present in the stream.
func setupVideoTrack(ctx context.Context, conf *Configuration, c *gortsplib.Client,
	desc *description.Session, output chan<- any, logger *slog.Logger) error {

	var mjpegFmt *format.MJPEG
	medi := desc.FindFormat(&mjpegFmt)
	if medi == nil {
		return fmt.Errorf("no supported video format found in RTSP stream (supported: mjpeg)")
	}

	rtpDec, err := mjpegFmt.CreateDecoder()
	if err != nil {
		return fmt.Errorf("creating MJPEG RTP decoder: %w", err)
	}

	if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return fmt.Errorf("setting up MJPEG track: %w", err)
	}

	var frameNum int64
	c.OnPacketRTP(medi, mjpegFmt, func(pkt *rtp.Packet) {
		jpegData, err := rtpDec.Decode(pkt)
		if err != nil {
			if !errors.Is(err, rtpmjpeg.ErrMorePacketsNeeded) &&
				!errors.Is(err, rtpmjpeg.ErrNonStartingPacketAndNoPrevious) {
				logger.Warn("MJPEG decode error", slog.Any("error", err))
			}
			return
		}
		if len(jpegData) == 0 {
			return
		}
		frameNum++
		if (frameNum-1)%int64(conf.FrameInterval) != 0 {
			logger.Debug("Dropping MJPEG frame",
				slog.Int64("frame", frameNum),
				slog.Int("interval", conf.FrameInterval))
			return
		}
		event := NewRTSPVideoEvent(ctx, jpegData, frameNum, conf.URL)
		select {
		case output <- event:
		case <-ctx.Done():
		}
	})

	logger.Info("MJPEG video track set up")
	return nil
}
