// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package mjpeg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	// Parse Content-Type header to extract boundary
	// Expected format: multipart/x-mixed-replace;boundary=myboundary
	// or with spaces: multipart/x-mixed-replace; boundary = myboundary

	// First, find the "boundary" keyword
	idx := bytes.Index([]byte(contentType), []byte("boundary"))
	if idx == -1 {
		return ""
	}

	// Get the substring starting from "boundary"
	remaining := contentType[idx+len("boundary"):]

	// Find the "=" sign
	eqIdx := bytes.IndexByte([]byte(remaining), '=')
	if eqIdx == -1 {
		return ""
	}

	// Get everything after the "=" and trim spaces
	boundary := bytes.TrimSpace([]byte(remaining[eqIdx+1:]))
	return string(boundary)
}

// processMultipartStream reads and processes the multipart MJPEG stream
func (s *MJPEGSource) processMultipartStream(ctx context.Context, body io.Reader, boundary string, output chan any) error {
	reader := bufio.NewReader(body)
	boundaryBytes := []byte(boundary)
	frameNum := int64(0)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, stopping stream")
			return ctx.Err()
		default:
		}

		// Read until boundary
		_, err := s.readUntil(reader, boundaryBytes)
		if err != nil {
			return fmt.Errorf("failed to read boundary: %w", err)
		}

		// Read headers
		headers, err := s.readHeaders(reader)
		if err != nil {
			return fmt.Errorf("failed to read headers: %w", err)
		}

		// Get content length
		contentLength := s.getContentLength(headers)
		if contentLength <= 0 {
			s.logger.Warn("Invalid or missing Content-Length header")
			continue
		}

		// Read frame data
		frameData := make([]byte, contentLength)
		_, err = io.ReadFull(reader, frameData)
		if err != nil {
			return fmt.Errorf("failed to read frame data: %w", err)
		}

		frameNum++

		// Apply processing factor (skip frames if needed)
		if frameNum%int64(s.Conf.FrameInterval) == 0 {
			// Create event and send to output
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

func (m *MJPEGSource) readUntil(reader *bufio.Reader, delimiter []byte) ([]byte, error) {
	var result []byte
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		result = append(result, line...)
		if bytes.Contains(line, delimiter) {
			return result, nil
		}
	}
}

func (m *MJPEGSource) readHeaders(reader *bufio.Reader) (map[string]string, error) {
	headers := make(map[string]string)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		// Empty line marks end of headers
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			break
		}

		// Parse header
		parts := bytes.SplitN(trimmed, []byte(":"), 2)
		if len(parts) == 2 {
			key := string(bytes.TrimSpace(parts[0]))
			value := string(bytes.TrimSpace(parts[1]))
			headers[key] = value
		}
	}
	return headers, nil
}

func (m *MJPEGSource) getContentLength(headers map[string]string) int {
	// Try different case variations
	for _, key := range []string{"Content-Length", "content-length", "Content-length"} {
		if val, ok := headers[key]; ok {
			length, err := strconv.Atoi(val)
			if err == nil && length > 0 {
				return length
			}
		}
	}
	return 0
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
