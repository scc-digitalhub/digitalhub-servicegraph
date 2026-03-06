// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package wsclient

import (
	"maps"
	"text/template"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type Configuration struct {
	URL            string
	MsgType        int
	Params         map[string]string
	Headers        map[string]string // check
	InputTemplate  string            `json:"input_template,omitempty"`
	OutputTemplate string            `json:"output_template,omitempty"`
	// MaxRetries is the number of reconnection attempts after a connection
	// failure. 0 = no reconnect (default), -1 = unlimited reconnects.
	MaxRetries int `json:"max_retries,omitempty"`
	// RetryBackoffMs is the initial backoff in milliseconds between reconnection
	// attempts. Doubles each attempt, capped at 30 s. Defaults to 1000 ms.
	RetryBackoffMs int `json:"retry_backoff_ms,omitempty"`
	inTemplateObj  *template.Template
	outTemplateObj *template.Template
}

func NewConfiguration(url string, params, headers map[string]string, msgType int) *Configuration {
	conf := &Configuration{
		URL:     url,
		Params:  make(map[string]string),
		Headers: make(map[string]string),
		MsgType: msgType,
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
		conf.inTemplateObj, _ = util.BuildTemplate(templateStr, "inputTemplate")
	}
}

func (conf *Configuration) setOutputTemplate(templateStr string) {
	conf.OutputTemplate = templateStr
	if templateStr != "" {
		conf.outTemplateObj, _ = util.BuildTemplate(templateStr, "outputTemplate")
	}
}
