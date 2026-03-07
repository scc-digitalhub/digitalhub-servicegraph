// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

// Package rtsp provides an RTSP source that connects to an RTSP server and
// emits events into the service-graph processing pipeline.
//
// Set media_type to "video" to receive MJPEG frames as RTSPVideoEvents.
// Set media_type to "audio" to receive rolling LPCM snapshots as RTSPAudioEvents.
// The codec is auto-detected from the stream; an error is returned when no
// supported format is found.
package rtsp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	gortsplib "github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

// RTSPSource connects to an RTSP server and feeds video or audio events into the
// service-graph processing pipeline.
type RTSPSource struct {
	sources.Source
	Conf    *Configuration
	factory sources.FlowFactory
	logger  *slog.Logger
	input   chan any
}

// NewRTSPSource creates a new RTSPSource from the supplied Configuration.
func NewRTSPSource(conf *Configuration) *RTSPSource {
	s := &RTSPSource{Conf: conf}
	s.logger = slog.Default().With(
		slog.Group("source",
			slog.String("name", "rtsp"),
			slog.String("type", "source"),
		),
	)
	return s
}

// StartAsync begins reading the RTSP stream and forwarding events to the
// processing flow. It reconnects automatically on transient failures using
// exponential back-off. The method blocks until the process receives SIGTERM/SIGINT.
func (s *RTSPSource) StartAsync(factory sources.FlowFactory, sink streams.Sink) error {
	s.factory = factory
	s.input = make(chan any)

	go func() {
		factory.GenerateFlow(extension.NewChanSource(s.input), sink)
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(s.input)
		defer cancel()

		initialBackoff := time.Duration(s.Conf.RetryBackoff) * time.Millisecond
		if initialBackoff <= 0 {
			initialBackoff = time.Second
		}
		backoff := initialBackoff
		maxRetries := s.Conf.MaxRetries
		attempt := 0

		for {
			if err := s.streamMedia(ctx, s.input); err != nil {
				if errors.Is(err, context.Canceled) {
					s.logger.Info("RTSP stream cancelled, shutting down")
					return
				}
				if maxRetries > 0 && attempt >= maxRetries {
					s.logger.Error("Max RTSP reconnect attempts exceeded",
						slog.Int("attempts", attempt),
						slog.Any("error", err))
					return
				}
				attempt++
				s.logger.Warn("RTSP stream error, reconnecting",
					slog.Any("error", err),
					slog.Int("attempt", attempt),
					slog.Duration("backoff", backoff))
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return
				}
				backoff = min(backoff*2, 30*time.Second)
			} else {
				s.logger.Info("RTSP stream ended cleanly")
				return
			}
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown
	cancel()
	return nil
}

// streamMedia establishes an RTSP session, sets up the configured track, and
// streams data until the context is cancelled or the server closes the session.
func (s *RTSPSource) streamMedia(ctx context.Context, output chan any) error {
	s.logger.Info("Connecting to RTSP stream", slog.String("url", s.Conf.URL))

	u, err := base.ParseURL(s.Conf.URL)
	if err != nil {
		return fmt.Errorf("parsing RTSP URL: %w", err)
	}

	c := &gortsplib.Client{}
	if err := c.Start(u.Scheme, u.Host); err != nil {
		return fmt.Errorf("starting RTSP client: %w", err)
	}
	defer c.Close()

	desc, _, err := c.Describe(u)
	if err != nil {
		return fmt.Errorf("describing RTSP stream: %w", err)
	}

	switch s.Conf.MediaType {
	case MediaTypeVideo:
		if err := setupVideoTrack(ctx, s.Conf, c, desc, output, s.logger); err != nil {
			return err
		}
	case MediaTypeAudio:
		buf := newAudioBuffer(s.Conf.AudioMaxSize)
		if err := setupAudioTrack(s.Conf, c, desc, buf, s.logger); err != nil {
			return err
		}
		if _, err := c.Play(nil); err != nil {
			return fmt.Errorf("starting RTSP play: %w", err)
		}
		s.logger.Info("RTSP audio play started")
		startAudioFlusher(ctx, s.Conf, buf, output)
		return s.waitForEnd(ctx, c)
	default:
		return fmt.Errorf("unknown media_type %q", s.Conf.MediaType)
	}

	if _, err := c.Play(nil); err != nil {
		return fmt.Errorf("starting RTSP play: %w", err)
	}
	s.logger.Info("RTSP video play started")
	return s.waitForEnd(ctx, c)
}

// waitForEnd blocks until ctx is cancelled or c.Wait() returns.
func (s *RTSPSource) waitForEnd(ctx context.Context, c *gortsplib.Client) error {
	waitErr := make(chan error, 1)
	go func() { waitErr <- c.Wait() }()
	select {
	case err := <-waitErr:
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case <-ctx.Done():
		return nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

func init() {
	sources.RegistrySingleton.Register("rtsp", &RTSPSourceProcessor{})
}

// RTSPSourceProcessor implements sources.Converter and sources.Validator.
type RTSPSourceProcessor struct {
	sources.Converter
	sources.Validator
}

// Convert constructs an RTSPSource from the provided InputSpec.
func (p *RTSPSourceProcessor) Convert(input model.InputSpec) (sources.Source, error) {
	conf := &Configuration{}
	if err := util.Convert(input.Spec, conf); err != nil {
		return nil, err
	}
	conf = NewConfiguration(
		conf.URL,
		conf.MediaType,
		conf.FrameInterval,
		conf.AudioMaxSize,
		conf.AudioProcessingInterval,
		conf.AudioChunkSize,
		conf.MaxRetries,
		conf.RetryBackoff,
	)
	return NewRTSPSource(conf), nil
}

// Validate checks that the Configuration is coherent.
func (p *RTSPSourceProcessor) Validate(input model.InputSpec) error {
	conf := &Configuration{}
	if err := util.Convert(input.Spec, conf); err != nil {
		return err
	}
	if conf.URL == "" {
		return fmt.Errorf("rtsp source: 'url' is required")
	}
	if conf.MediaType != MediaTypeVideo && conf.MediaType != MediaTypeAudio {
		return fmt.Errorf("rtsp source: 'media_type' must be %q or %q, got %q",
			MediaTypeVideo, MediaTypeAudio, conf.MediaType)
	}
	if conf.FrameInterval < 0 {
		return fmt.Errorf("rtsp source: frame_interval must be >= 0")
	}
	if conf.AudioMaxSize < 0 {
		return fmt.Errorf("rtsp source: audio_max_size must be >= 0")
	}
	if conf.AudioProcessingInterval < 0 {
		return fmt.Errorf("rtsp source: audio_processing_interval must be >= 0")
	}
	if conf.AudioChunkSize < 0 {
		return fmt.Errorf("rtsp source: audio_chunk_size must be >= 0")
	}
	return nil
}
