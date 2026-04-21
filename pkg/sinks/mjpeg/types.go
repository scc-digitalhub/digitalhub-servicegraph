// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package mjpeg

const (
	defaultPort   = 8090
	defaultPath   = "/stream"
	mjpegBoundary = "mjpegboundary"
)

// Configuration holds the MJPEG sink configuration.
type Configuration struct {
	// Port is the TCP port the HTTP server listens on. Defaults to 8090.
	Port int `json:"port,omitempty"`
	// Path is the HTTP path that serves the MJPEG stream. Defaults to "/stream".
	Path string `json:"path,omitempty"`
}

// NewConfiguration returns a Configuration with defaults applied for zero values.
func NewConfiguration(port int, path string) *Configuration {
	conf := &Configuration{
		Port: port,
		Path: path,
	}
	if conf.Port == 0 {
		conf.Port = defaultPort
	}
	if conf.Path == "" {
		conf.Path = defaultPath
	}
	return conf
}
