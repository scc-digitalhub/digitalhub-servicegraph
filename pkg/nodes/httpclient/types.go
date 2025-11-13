package httpclient

import (
	"maps"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

const (
	defaultMethod = "POST"
)

type Configuration struct {
	URL          string            `json,yaml:"url"`
	Method       string            `json,yaml:"method"`
	Params       map[string]string `json,yaml:"params,omitempty"`
	Headers      map[string]string `json,yaml:"headers,omitempty"`
	numInstances int               `json,yaml:"num_instances,omitempty"`
}

func NewConfiguration(url, method string, params, headers map[string]string, numInstances int) *Configuration {
	conf := &Configuration{
		URL:     url,
		Method:  defaultMethod,
		Params:  make(map[string]string),
		Headers: make(map[string]string),
	}
	if numInstances > 0 {
		conf.numInstances = numInstances
	} else {
		conf.numInstances = 1
	}

	if method != "" {
		conf.Method = method
	}
	if params != nil {
		maps.Copy(conf.Params, params)
	}
	if headers != nil {
		maps.Copy(conf.Headers, headers)
	}

	return conf
}

func (c *Configuration) Ground(in streams.Event) (streams.Event, error) {
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

	url := c.URL
	method := c.Method

	event, err := streams.NewGenericEvent(in.GetBody(), url, method, headers, fields, 200)
	if err != nil {
		return nil, err
	}
	return event, nil
}
