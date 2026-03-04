// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package mjpeg

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

// MJPEGSource represents an MJPEG stream source
type MJPEGSource struct {
	sources.Source
	Conf    *Configuration
	factory sources.FlowFactory
	logger  *slog.Logger
	input   chan any
}

// NewMJPEGSource creates a new MJPEG source with the given configuration
func NewMJPEGSource(conf *Configuration) *MJPEGSource {
	source := &MJPEGSource{}
	source.Conf = conf
	source.logger = slog.Default()
	source.logger = source.logger.With(slog.Group("source",
		slog.String("name", "mjpeg"),
		slog.String("type", "source")))

	return source
}

// StartAsync begins processing the MJPEG stream asynchronously
func (s *MJPEGSource) StartAsync(factory sources.FlowFactory, sink streams.Sink) error {
	s.factory = factory
	s.input = make(chan any)

	// Start the flow
	go func() {
		factory.GenerateFlow(
			extension.NewChanSource(s.input),
			sink,
		)
	}()

	// Start streaming
	ctx := context.Background()
	go func() {
		if err := s.streamFrames(ctx, s.input); err != nil {
			s.logger.Error("Error streaming MJPEG", slog.Any("error", err))
		}
	}()

	// Keep running until interrupted
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown

	close(s.input)
	return nil
}

// streamFrames connects to the MJPEG stream and processes frames
func (s *MJPEGSource) streamFrames(ctx context.Context, output chan any) error {
	s.logger.Info("Connecting to MJPEG stream", slog.String("url", s.Conf.URL))

	// Create HTTP client with timeout
	client := &http.Client{}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", s.Conf.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to MJPEG stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the boundary from Content-Type header
	contentType := resp.Header.Get("Content-Type")
	boundary := s.extractBoundary(contentType)
	if boundary == "" {
		s.logger.Warn("Could not extract boundary from Content-Type, using default")
		boundary = "--myboundary"
	} else {
		boundary = "--" + boundary
	}

	s.logger.Info("Connected to MJPEG stream",
		slog.String("content-type", contentType),
		slog.String("boundary", boundary))

	// Process multipart stream
	return s.processMultipartStream(ctx, resp.Body, boundary, output)
}

func (s *MJPEGSource) extractBoundary(contentType string) string {
	// Look for "boundary=" and return the token after it
	// Handle formats like: multipart/x-mixed-replace; boundary=--myboundary
	parts := strings.Split(contentType, "boundary=")
	if len(parts) < 2 {
		return ""
	}
	b := strings.TrimSpace(parts[1])
	// Trim possible surrounding quotes
	b = strings.Trim(b, "\"'")
	// If boundary starts with two dashes, strip them for multipart.Reader
	b = strings.TrimPrefix(b, "--")
	return b
}

// processMultipartStream reads and processes the multipart MJPEG stream
func (s *MJPEGSource) processMultipartStream(ctx context.Context, body io.Reader, boundary string, output chan any) error {
	// multipart.NewReader expects the boundary without the leading "--"
	boundary = strings.TrimPrefix(boundary, "--")
	mr := multipart.NewReader(body, boundary)
	frameNum := int64(0)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, stopping stream")
			return ctx.Err()
		default:
		}

		part, err := mr.NextPart()
		if err == io.EOF {
			// End of stream
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read next part: %w", err)
		}

		// Read full part content
		frameData, err := io.ReadAll(part)
		if err != nil {
			return fmt.Errorf("failed to read part data: %w", err)
		}

		frameNum++

		// Apply processing factor: process frames where (frameNum-1) % FrameInterval == 0
		if (frameNum-1)%int64(s.Conf.FrameInterval) == 0 {
			event := NewMJPEGEvent(ctx, frameData, frameNum, s.Conf.URL)
			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			s.logger.Debug("Skipping frame",
				slog.Int64("frame", frameNum),
				slog.Int("factor", s.Conf.FrameInterval))
		}
	}
}

// init registers the MJPEG source in the sources registry
func init() {
	sources.RegistrySingleton.Register("mjpeg", &MJPEGSourceProcessor{})
}

// MJPEGSourceProcessor implements the Converter and Validator interfaces
type MJPEGSourceProcessor struct {
	sources.Converter
	sources.Validator
}

// Convert converts a model.InputSpec to an MJPEG source
func (c *MJPEGSourceProcessor) Convert(input model.InputSpec) (sources.Source, error) {
	conf := &Configuration{}
	err := util.Convert(input.Spec, conf)
	if err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.URL, conf.FrameInterval, conf.ReadTimeout)
	src := NewMJPEGSource(conf)
	return src, nil
}

// Validate validates the MJPEG source configuration
func (c *MJPEGSourceProcessor) Validate(input model.InputSpec) error {
	conf := &Configuration{}
	err := util.Convert(input.Spec, conf)
	if err != nil {
		return err
	}
	if conf.URL == "" {
		return fmt.Errorf("URL is required")
	}
	if conf.FrameInterval < 1 {
		return fmt.Errorf("frame interval must be at least 1")
	}
	if conf.ReadTimeout < 0 {
		return fmt.Errorf("read timeout must be non-negative")
	}
	return nil
}
