//go:build !openh264
// +build !openh264

package h264

import "fmt"

// OpenH264Decoder stub when OpenH264 is not available at build time.
type OpenH264Decoder struct{}

func NewOpenH264Decoder() (*OpenH264Decoder, error) {
	return nil, fmt.Errorf("openh264 support not compiled in; build with -tags openh264 and install libopenh264")
}

func (d *OpenH264Decoder) Decode(nalus [][]byte) ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("openh264 not available")
}

func (d *OpenH264Decoder) Close() {}
