package httpclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
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
	for i := 0; i < hc.conf.numInstances; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for element := range hc.in {
				hc.out <- hc.call(streams.NewEventFrom(element))
			}
		}()
	}

	// wait for worker goroutines to finish processing inbound elements
	wg.Wait()
	// close the output channel
	close(hc.out)
}

func (hc *HttpClient) call(msg streams.Event) streams.Event {
	req, err := hc.conf.Ground(msg)
	if err != nil {
		return streams.NewErrorEvent(err, 500)
	}

	var bodyReader *bytes.Reader = nil
	if req.GetBody() != nil {
		bodyReader = bytes.NewReader(req.GetBody())
	}

	httpReq, err := http.NewRequest(strings.ToUpper(req.GetMethod()), req.GetURL(), bodyReader)
	if err != nil {
		return streams.NewErrorEvent(err, 500)
	}
	if req.GetHeaders() != nil {
		for header, value := range req.GetHeaders() {
			httpReq.Header.Set(header, value)
		}
	}

	resp, err := hc.httpClient.Do(httpReq)
	if err != nil && resp != nil {
		return streams.NewErrorEvent(err, resp.StatusCode)
	} else if err != nil {
		return streams.NewErrorEvent(err, 500)
	}
	defer resp.Body.Close()

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return streams.NewErrorEvent(err, 500)
	}
	return streams.NewEventFrom(resBody)
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
		Transport: transport,
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
	data, err := json.Marshal(spec.Spec)
	if err != nil {
		return nil, err
	}
	conf := &Configuration{}
	err = json.Unmarshal(data, conf)
	if err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.URL, conf.Method, conf.Params, conf.Headers, conf.numInstances)
	src := NewHttpClient(*conf)
	return src, nil
}

func (c *HTTPProcessor) Validate(spec model.NodeConfig) error {
	// marshal to json, unmarshal to config
	conf := &Configuration{}
	err := util.Convert(spec.Spec, conf)

	if err != nil {
		return err
	}
	if conf.URL == "" {
		return errors.New("httpclient node requires a valid url")
	}

	allowedMethods := []string{http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace}
	if conf.Method != "" && !slices.Contains(allowedMethods, strings.ToUpper(conf.Method)) {
		return errors.New("httpclient node requires a valid method")
	}

	if conf.numInstances < 0 {
		return errors.New("httpclient node requires a valid number of instances")
	}

	return nil
}
