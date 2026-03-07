// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package rtsp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"time"

	gortsplib "github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpmjpeg"
	"github.com/pion/rtp"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/rtsp/h264"
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
func (e *RTSPVideoEvent) GetBody() []byte             { return e.jpegData }
func (e *RTSPVideoEvent) GetTimestamp() time.Time     { return e.timestamp }
func (e *RTSPVideoEvent) GetContext() context.Context { return e.ctx }
func (e *RTSPVideoEvent) GetURL() string              { return e.url }

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

	var medi *description.Media
	var mjpegFmt *format.MJPEG
	var h264Fmt *format.H264

	// Initialize H.264 decoders for video formats
	for _, media := range desc.Medias {
		for _, forma := range media.Formats {
			if f, ok := forma.(*format.MJPEG); ok {
				medi = media
				mjpegFmt = f
			} else if f, ok := forma.(*format.H264); ok {
				medi = media
				h264Fmt = f
			}
		}
	}

	if medi == nil {
		return fmt.Errorf("no supported video format found in RTSP stream (supported: mjpeg, h264)")
	}

	if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return fmt.Errorf("setting up media track: %w", err)
	}

	var frameNum int64

	if mjpegFmt != nil {
		mjpegDec, err := mjpegFmt.CreateDecoder()
		if err != nil {
			return err
		}

		c.OnPacketRTP(medi, mjpegFmt, func(pkt *rtp.Packet) {
			jpegData, err := mjpegDec.Decode(pkt)
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
	}
	if h264Fmt != nil {
		h264FirstIDR := make(map[uint8]bool) // track payload type -> whether first IDR frame has been seen
		h264Decoder, err := h264.NewOpenH264Decoder()
		if err != nil {
			return err
		}
		depacketizer, err := h264Fmt.CreateDecoder()
		if err != nil {
			return err
		}

		// feed SPS/PPS to decoder
		initNALUs := [][]byte{}
		if len(h264Fmt.SPS) > 0 {
			initNALUs = append(initNALUs, h264Fmt.SPS)
		}
		if len(h264Fmt.PPS) > 0 {
			initNALUs = append(initNALUs, h264Fmt.PPS)
		}
		if len(initNALUs) > 0 {
			h264Decoder.Decode(initNALUs)
		}
		c.OnPacketRTP(medi, h264Fmt, func(pkt *rtp.Packet) {

			au, err := depacketizer.Decode(pkt)
			if err != nil || au == nil {
				return
			}

			pt := h264Fmt.PayloadType()

			// wait first IDR
			if !h264FirstIDR[pt] {
				if !containsIDR(au) {
					return
				}
				h264FirstIDR[pt] = true
			}

			yuv, w, h, err := h264Decoder.Decode(au)
			if err != nil || yuv == nil {
				return
			}

			frameNum++
			if (frameNum-1)%int64(conf.FrameInterval) != 0 {
				logger.Debug("Dropping H.264 frame",
					slog.Int64("frame", frameNum),
					slog.Int("interval", conf.FrameInterval))
				return
			}
			jpeg, err := EncodeFrameToJPEG(yuv, w, h, 80)
			if err != nil {
				return
			}

			event := NewRTSPVideoEvent(ctx, jpeg, frameNum, conf.URL)
			select {
			case output <- event:
			case <-ctx.Done():
			}
		})
	}

	logger.Info("MJPEG video track set up")
	return nil
}

// EncodeFrameToJPEG converts a YUV frame to JPEG format with the specified quality
func EncodeFrameToJPEG(yuv []byte, width, height int, quality int) ([]byte, error) {

	img := image.NewYCbCr(
		image.Rect(0, 0, width, height),
		image.YCbCrSubsampleRatio420,
	)

	ySize := width * height
	uvSize := ySize / 4

	copy(img.Y, yuv[:ySize])
	copy(img.Cb, yuv[ySize:ySize+uvSize])
	copy(img.Cr, yuv[ySize+uvSize:])

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// containsIDR checks whether any of the supplied NALUs is an IDR frame
func containsIDR(nalus [][]byte) bool {
	for _, n := range nalus {
		if len(n) == 0 {
			continue
		}
		if (n[0] & 0x1F) == 5 {
			return true
		}
	}
	return false
}
