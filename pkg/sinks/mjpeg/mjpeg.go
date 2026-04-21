// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package mjpeg

import (
	"fmt"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"sync"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

// MJPEGSink receives JPEG byte arrays and broadcasts them as an MJPEG HTTP stream.
// It starts an HTTP server on the configured port and path; any number of clients
// can connect concurrently and receive the live stream.
type MJPEGSink struct {
	conf     Configuration
	in       chan any
	done     chan struct{}
	mu       sync.RWMutex
	clients  map[chan []byte]struct{}
	server   *http.Server
	listener net.Listener
	logger   *slog.Logger
}

var _ streams.Sink = (*MJPEGSink)(nil)

// NewMJPEGSink creates and starts an MJPEGSink.
// The HTTP server begins listening immediately; call In() to push frames.
func NewMJPEGSink(conf Configuration) (*MJPEGSink, error) {
	logger := slog.Default().With(slog.Group("connector",
		slog.String("name", "mjpeg"),
		slog.String("type", "sink"),
		slog.Int("port", conf.Port),
		slog.String("path", conf.Path),
	))

	s := &MJPEGSink{
		conf:    conf,
		in:      make(chan any),
		done:    make(chan struct{}),
		clients: make(map[chan []byte]struct{}),
		logger:  logger,
	}

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(conf.Port))
	if err != nil {
		return nil, fmt.Errorf("mjpeg sink: failed to listen on port %d: %w", conf.Port, err)
	}
	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc(conf.Path, s.handleStream)
	s.server = &http.Server{Handler: mux}

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("MJPEG HTTP server error", slog.Any("error", err))
		}
	}()

	go s.process()

	logger.Info("MJPEG sink started")
	return s, nil
}

// process reads frames from the input channel and fans them out to all connected clients.
func (s *MJPEGSink) process() {
	defer func() {
		// Shut down the HTTP server and close all client channels.
		if err := s.server.Close(); err != nil {
			s.logger.Warn("Error closing MJPEG HTTP server", slog.Any("error", err))
		}
		s.mu.Lock()
		for ch := range s.clients {
			close(ch)
		}
		s.clients = make(map[chan []byte]struct{})
		s.mu.Unlock()

		close(s.done)
		s.logger.Info("MJPEG sink closed")
	}()

	for msg := range s.in {
		frame, ok := s.toJPEGBytes(msg)
		if !ok {
			continue
		}
		s.broadcast(frame)
	}
}

// toJPEGBytes extracts a JPEG byte slice from the supported message types.
func (s *MJPEGSink) toJPEGBytes(msg any) ([]byte, bool) {
	switch v := msg.(type) {
	case streams.Event:
		return v.GetBody(), true
	case []byte:
		return v, true
	default:
		s.logger.Error("Unsupported message type; expected []byte or streams.Event",
			slog.String("type", fmt.Sprintf("%T", msg)))
		return nil, false
	}
}

// broadcast sends a copy of the frame to every connected client channel.
func (s *MJPEGSink) broadcast(frame []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.clients {
		// Non-blocking send: drop the frame for slow clients rather than blocking
		// the broadcast loop.
		select {
		case ch <- frame:
		default:
			s.logger.Warn("Dropping frame for slow client")
		}
	}
}

// subscribe registers a new client channel and returns it.
func (s *MJPEGSink) subscribe() chan []byte {
	ch := make(chan []byte, 8)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// unsubscribe removes a client channel from the broadcast set.
func (s *MJPEGSink) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

// handleStream is the HTTP handler that streams MJPEG content to a client.
func (s *MJPEGSink) handleStream(w http.ResponseWriter, r *http.Request) {
	ch := s.subscribe()
	defer s.unsubscribe(ch)

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+mjpegBoundary)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Pragma", "no-cache")

	mw := multipart.NewWriter(w)
	if err := mw.SetBoundary(mjpegBoundary); err != nil {
		s.logger.Error("Failed to set MJPEG boundary", slog.Any("error", err))
		return
	}

	flusher, canFlush := w.(http.Flusher)

	s.logger.Info("MJPEG client connected", slog.String("remote", r.RemoteAddr))
	defer s.logger.Info("MJPEG client disconnected", slog.String("remote", r.RemoteAddr))

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-ch:
			if !ok {
				// Sink closed; end the stream.
				return
			}
			partHeader := textproto.MIMEHeader{}
			partHeader.Set("Content-Type", "image/jpeg")
			partHeader.Set("Content-Length", strconv.Itoa(len(frame)))

			pw, err := mw.CreatePart(partHeader)
			if err != nil {
				s.logger.Error("Failed to create MJPEG part", slog.Any("error", err))
				return
			}
			if _, err = pw.Write(frame); err != nil {
				s.logger.Error("Failed to write MJPEG frame", slog.Any("error", err))
				return
			}
			// Write the blank line that separates the part body from the next boundary.
			if _, err = fmt.Fprintf(w, "\r\n"); err != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
	}
}

// In returns the write-only input channel for the sink.
func (s *MJPEGSink) In() chan<- any {
	return s.in
}

// AwaitCompletion blocks until the sink has finished processing all frames
// and the HTTP server has shut down.
func (s *MJPEGSink) AwaitCompletion() {
	<-s.done
}

// ListenAddr returns the address the HTTP server is listening on,
// which is useful when Port 0 is configured (OS-assigned port).
func (s *MJPEGSink) ListenAddr() string {
	return s.listener.Addr().String()
}

func init() {
	sinks.RegistrySingleton.Register("mjpeg", &MJPEGProcessor{})
}

// MJPEGProcessor converts an OutputSpec into an MJPEGSink.
type MJPEGProcessor struct {
	sinks.Converter
	sinks.Validator
}

func (p *MJPEGProcessor) Convert(output model.OutputSpec) (streams.Sink, error) {
	conf := &Configuration{}
	if err := util.Convert(output.Spec, conf); err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.Port, conf.Path)
	return NewMJPEGSink(*conf)
}

func (p *MJPEGProcessor) Validate(output model.OutputSpec) error {
	conf := &Configuration{}
	if err := util.Convert(output.Spec, conf); err != nil {
		return err
	}
	if conf.Port < 0 || conf.Port > 65535 {
		return fmt.Errorf("mjpeg sink: invalid port %d", conf.Port)
	}
	// validate that path begins with /
	if conf.Path != "" && conf.Path[0] != '/' {
		return fmt.Errorf("mjpeg sink: path must begin with '/': %q", conf.Path)
	}
	return nil
}
