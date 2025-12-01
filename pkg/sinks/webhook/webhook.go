package webhook

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type WebHook struct {
	conf       Configuration
	in         chan any
	done       chan struct{}
	httpClient http.Client
}

// Verify WebHook satisfies the Output
//
//	interface.
var _ streams.Sink = (*WebHook)(nil)

// NewWebHook returns a new WebHook operator.
func NewWebHook(conf Configuration) *WebHook {
	wh := &WebHook{
		conf:       conf,
		in:         make(chan any),
		done:       make(chan struct{}),
		httpClient: *createClient(),
	}

	go wh.stream()

	return wh
}

// In returns the input channel of the WebHook connector.
func (wh *WebHook) In() chan<- any {
	return wh.in
}

// AwaitCompletion blocks until the WebHook has processed all received data.
func (wh *WebHook) AwaitCompletion() {
	<-wh.done
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

// stream reads elements from the input channel, applies the call
// to each element, and sends the resulting element to the output channel.
// It uses a pool of goroutines to process elements in parallel.
func (whc *WebHook) stream() {
	var wg sync.WaitGroup
	defer close(whc.done)
	// create a pool of worker goroutines
	for i := 0; i < whc.conf.Parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for element := range whc.in {
				whc.call(streams.NewEventFrom(element))
			}
		}()
	}

	// wait for worker goroutines to finish processing inbound elements
	wg.Wait()
}

func (whc *WebHook) call(msg streams.Event) streams.Event {
	req, err := whc.conf.Ground(msg)
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

	resp, err := whc.httpClient.Do(httpReq)
	if err != nil {
		return streams.NewErrorEvent(err, resp.StatusCode)
	}
	defer resp.Body.Close()

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return streams.NewErrorEvent(err, 500)
	}
	return streams.NewEventFrom(resBody)
}

func init() {
	sinks.RegistrySingleton.Register("webhook", &WebHookProcessor{})
}

type WebHookProcessor struct {
	sinks.Converter
	sinks.Validator
}

func (c *WebHookProcessor) Convert(output model.OutputSpec) (streams.Sink, error) {
	// marshal to json, unmarshal to config
	conf := &Configuration{}
	err := util.Convert(output.Spec, conf)
	if err != nil {
		return nil, err
	}
	conf = NewConfiguration(conf.URL, conf.Params, conf.Headers, conf.Parallelism)
	src := NewWebHook(*conf)
	return src, nil
}

func (c *WebHookProcessor) Validate(spec model.OutputSpec) error {
	conf := &Configuration{}
	err := util.Convert(spec.Spec, conf)
	if err != nil {
		return err
	}
	if conf.URL == "" {
		return fmt.Errorf("url is required for webhook sink")
	}
	return nil
}
