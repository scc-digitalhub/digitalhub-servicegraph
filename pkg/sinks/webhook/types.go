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
	url := c.URL

	headers := in.GetHeaders()
	if headers == nil {
		headers = make(map[string]string)
	}
	for k, v := range c.Headers {
		headers[k] = v
	}

	fields := in.GetFields()
	if fields == nil {
		fields = make(map[string]string)
	}
	for k, v := range c.Params {
		fields[k] = v
	}

	event, err := streams.NewGenericEvent(in.GetBody(), url, http.MethodPost, headers, fields, 200)
	if err != nil {
		return nil, err
	}
	return event, nil
}
