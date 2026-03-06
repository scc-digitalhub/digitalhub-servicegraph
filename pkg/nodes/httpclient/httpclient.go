// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

// Http client takes one element calls the http api and produces.
//
// in  -- 1 -- 2 ---- 3 -- 4 ------ 5 --
//
// [ ---------- HTTP CALL ---------- ]
//
// out  -- 1 -- 2 ---- 3 -- 4 ------ 5 --
type HttpClient struct {
	conf       Configuration
	in         chan any
	out        chan any
	httpClient http.Client
	logger     *slog.Logger
}

// Verify HttpClient satisfies the Flow interface.
var _ streams.Flow = (*HttpClient)(nil)

// NewHttpClient returns a new HttpClient operator.
// T specifies the incoming element type, and the outgoing element type is R.
//
// mapFunction is the Map transformation function.
// parallelism specifies the number of goroutines to use for parallel processing. If
// the order of elements in the output stream must be preserved, set parallelism to 1.
//
// NewMap will panic if parallelism is less than 1.
func NewHttpClient(conf Configuration) *HttpClient {
	hcFlow := &HttpClient{
		conf:       conf,
		in:         make(chan any),
		out:        make(chan any),
		httpClient: *createClient(),
		logger: slog.Default().With(
			slog.Group("node",
				slog.String("type", "http"),
				slog.String("url", conf.URL),
			)),
	}

	// start processing stream elements
	go hcFlow.stream()

	return hcFlow
}

// Via asynchronously streams data to the given Flow and returns it.
func (hc *HttpClient) Via(flow streams.Flow) streams.Flow {
	go hc.transmit(flow)
	return flow
}

// To streams data to the given Sink and blocks until the Sink has completed
// processing all data.
func (hc *HttpClient) To(sink streams.Sink) {
	hc.transmit(sink)
	sink.AwaitCompletion()
}

// Out returns the output channel of the HttpClient operator.
func (m *HttpClient) Out() <-chan any {
	return m.out
}

// In returns the input channel of the HttpClient operator.
func (hc *HttpClient) In() chan<- any {
	return hc.in
}

func (hc *HttpClient) transmit(inlet streams.Inlet) {
	for element := range hc.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

// stream reads elements from the input channel, applies the call
// to each element, and sends the resulting element to the output channel.
// It uses a pool of goroutines to process elements in parallel.
func (hc *HttpClient) stream() {
	var wg sync.WaitGroup
	// create a pool of worker goroutines
	for i := 0; i < hc.conf.NumInstances; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					hc.logger.Error("Worker panic recovered", slog.Any("panic", r))
				}
			}()
			for element := range hc.in {
				// Pass error events through without making an HTTP call.
				if ev, ok := element.(streams.Event); ok && ev.GetStatus() >= 400 {
					hc.logger.Debug("Passing through error event", slog.Int("status", ev.GetStatus()))
					hc.out <- element
					continue
				}
				ctx := streams.ExtractContext(element)
				hc.out <- hc.call(streams.NewEventFrom(ctx, element))
			}
		}()
	}

	// wait for worker goroutines to finish processing inbound elements
	wg.Wait()
	// close the output channel
	close(hc.out)
}

func (hc *HttpClient) call(msg streams.Event) streams.Event {
	rootContext := msg.GetContext()

	// create span for http client call
	tr := otel.Tracer("example/client")
	childCtx, span := tr.Start(rootContext, "HTTP Client Node")
	defer span.End()

	// enrich message via grounding function
	req, err := hc.conf.Ground(msg)
	if err != nil {
		return streams.NewErrorEvent(rootContext, err, 500)
	}

	// process request body template if specified
	var bodyReader *bytes.Reader = nil
	if req.GetBody() != nil {
		body, err := util.ConvertBody(req.GetBody(), hc.conf.inTemplateObj)
		if err != nil {
			return streams.NewErrorEvent(rootContext, err, 500)
		}
		bodyReader = bytes.NewReader(body)
		// add request body to span
		util.AddRequestToSpan(childCtx, body, req.GetContentType())
	}

	// Determine which status codes should be retried.
	retryOn := hc.conf.RetryOn
	if retryOn == nil {
		retryOn = []int{500, 502, 503, 504}
	}
	backoff := time.Duration(hc.conf.RetryBackoffMs) * time.Millisecond
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}

	var resp *http.Response
	var lastErr error
	for attempt := 1; attempt <= hc.conf.MaxRetries+1; attempt++ {
		if bodyReader != nil {
			// Reset body position so each attempt reads the full body.
			_, _ = bodyReader.Seek(0, io.SeekStart)
		}
		// Create a fresh request for each attempt (bodies cannot be replayed).
		httpReq2, err2 := http.NewRequestWithContext(rootContext, strings.ToUpper(req.GetMethod()), req.GetURL(), bodyReader)
		if err2 != nil {
			return streams.NewErrorEvent(rootContext, err2, 500)
		}
		if req.GetHeaders() != nil {
			for header, value := range req.GetHeaders() {
				httpReq2.Header.Set(header, value)
			}
		}
		resp, lastErr = hc.httpClient.Do(httpReq2)
		if lastErr != nil {
			if resp != nil {
				resp.Body.Close()
				resp = nil
			}
		} else if slices.Contains(retryOn, resp.StatusCode) {
			// Consume and close the body before retry.
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("retryable status code: %d", resp.StatusCode)
			resp = nil
		} else {
			break
		}
		if attempt <= hc.conf.MaxRetries {
			hc.logger.Warn("HTTP call failed, retrying",
				slog.String("url", req.GetURL()),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", backoff),
				slog.Any("error", lastErr))
			time.Sleep(backoff)
			backoff = min(backoff*2, 10*time.Second)
		}
	}
	if lastErr != nil {
		return streams.NewErrorEvent(rootContext, lastErr, 500)
	}
	defer resp.Body.Close()

	// read response body
	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return streams.NewErrorEvent(rootContext, err, 500)
	}
	// process response body template if specified
	body, err := util.ConvertBody(resBody, hc.conf.outTemplateObj)
	if err != nil {
		return streams.NewErrorEvent(rootContext, err, 500)
	}

	headers := make(map[string]string)
	for key, values := range resp.Header {
		headers[key] = strings.Join(values, ", ")
	}

	result, err := streams.NewGenericEventBuilder(rootContext).
		WithBody(body).
		WithURL(req.GetURL()).
		WithMethod(req.GetMethod()).
		WithHeaders(headers).
		WithStatus(resp.StatusCode).
		Build()
	if err != nil {
		return streams.NewErrorEvent(rootContext, err, 500)
	}

	util.AddResponseToSpan(childCtx, result.GetBody(), result.GetContentType())

	return result
}

func createClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,              // Maximum idle connections
		MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
		IdleConnTimeout:     90 * time.Second, // Idle connection timeout
		DisableCompression:  false,            // Enable compression
		DisableKeepAlives:   false,            // Enable keep-alives
	}

	return &http.Client{
		Transport: otelhttp.NewTransport(transport),
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Custom redirect handling
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
}

func init() {
	nodes.RegistrySingleton.Register("http", &HTTPProcessor{})
}

type HTTPProcessor struct {
	nodes.Converter
	nodes.Validator
}

func (c *HTTPProcessor) Convert(spec model.NodeConfig) (streams.Flow, error) {
	// marshal to json, unmarshal to config
	var newConf *Configuration
	if cached := spec.ConfigCache(); cached != nil {
		newConf = (*cached).(*Configuration)
	} else {
		conf := &Configuration{}
		err := util.Convert(spec.Spec, conf)
		if err != nil {
			return nil, err
		}
		conf.setInputTemplate(conf.InputTemplate)
		conf.setOutputTemplate(conf.OutputTemplate)
		newConf = NewConfiguration(conf.URL, conf.Method, conf.Params, conf.Headers, conf.NumInstances)
		newConf.inTemplateObj = conf.inTemplateObj
		newConf.outTemplateObj = conf.outTemplateObj
		spec.SetConfigCache(newConf)
	}
	src := NewHttpClient(*newConf)

	return src, nil
}

func (c *HTTPProcessor) Validate(spec model.NodeConfig) error {
	// marshal to json, unmarshal to config
	conf := &Configuration{}
	err := util.Convert(spec.Spec, conf)

	if err != nil {
		return err
	}
	if conf.InputTemplate != "" {
		_, err := util.BuildTemplate(conf.InputTemplate, "inputTemplate")
		if err != nil {
			return errors.New("httpclient node has invalid input_template: " + err.Error())
		}
	}
	if conf.OutputTemplate != "" {
		_, err := util.BuildTemplate(conf.OutputTemplate, "outputTemplate")
		if err != nil {
			return errors.New("httpclient node has invalid output_template: " + err.Error())
		}
	}
	if conf.URL == "" {
		return errors.New("httpclient node requires a valid url")
	}

	allowedMethods := []string{http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace}
	if conf.Method != "" && !slices.Contains(allowedMethods, strings.ToUpper(conf.Method)) {
		return errors.New("httpclient node requires a valid method")
	}

	if conf.NumInstances < 0 {
		return errors.New("httpclient node requires a valid number of instances")
	}

	return nil
}
