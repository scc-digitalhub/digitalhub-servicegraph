package websocket

import (
	"maps"
)

type Configuration struct {
	URL     string            `json,yaml:"url"`
	MsgType int               `json,yaml:"msg_type"`
	Params  map[string]string `json,yaml:"params,omitempty"`
	Headers map[string]string `json,yaml:"headers,omitempty"`
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
