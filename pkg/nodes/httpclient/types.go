// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"maps"
	"text/template"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

const (
	defaultMethod = "POST"
)

type Configuration struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Params         map[string]string `json:"params,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	NumInstances   int               `json:"num_instances,omitempty"`
	InputTemplate  string            `json:"input_template,omitempty"`
	OutputTemplate string            `json:"output_template,omitempty"`
	inTemplateObj  *template.Template
	outTemplateObj *template.Template
}

func NewConfiguration(url, method string, params, headers map[string]string, numInstances int) *Configuration {
	conf := &Configuration{
		URL:     url,
		Method:  defaultMethod,
		Params:  make(map[string]string),
		Headers: make(map[string]string),
	}
	if numInstances > 0 {
		conf.NumInstances = numInstances
	} else {
		conf.NumInstances = 1
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
func (conf *Configuration) setInputTemplate(templateStr string) {
	conf.InputTemplate = templateStr
	if templateStr != "" {
		conf.inTemplateObj, _ = template.New("inputTemplate").Parse(templateStr)
	}
}

func (conf *Configuration) setOutputTemplate(templateStr string) {
	conf.OutputTemplate = templateStr
	if templateStr != "" {
		conf.outTemplateObj, _ = template.New("outputTemplate").Parse(templateStr)
	}
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
