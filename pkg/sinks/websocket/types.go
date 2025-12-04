// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"maps"
)

type Configuration struct {
	URL     string            `json:"url"`
	MsgType int               `json:"msg_type"`
	Params  map[string]string `json:"params,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
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
