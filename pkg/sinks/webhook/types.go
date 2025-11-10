package webhook

import (
	"maps"
	"net/http"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

type Configuration struct {
	URL         string            `json,yaml:"url"`
	Params      map[string]string `json,yaml:"params,omitempty"`
	Headers     map[string]string `json,yaml:"headers,omitempty"`
	Parallelism int               `json,yaml:"parallelism,omitempty"`
}

func NewConfiguration(url string, params, headers map[string]string, parallelism int) *Configuration {
	conf := &Configuration{
		URL:         url,
		Params:      make(map[string]string),
		Headers:     make(map[string]string),
		Parallelism: parallelism,
	}
	if params != nil {
		maps.Copy(conf.Params, params)
	}
	if headers != nil {
		maps.Copy(conf.Headers, headers)
	}
	if parallelism > 0 {
		conf.Parallelism = parallelism
	}

	return conf
}

func (c *Configuration) Ground(in streams.Event) (streams.Event, error) {
	// TODO ground jsonpath expressions in headers and fields to produce new URL
	url := c.URL

	event, err := streams.NewGenericEvent(in.GetBody(), url, http.MethodPost, in.GetHeaders(), in.GetFields(), 200)
	if err != nil {
		return nil, err
	}
	return event, nil
}
