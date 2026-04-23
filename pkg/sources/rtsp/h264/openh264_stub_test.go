// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

//go:build !openh264
// +build !openh264

package h264

import (
	"strings"
	"testing"
)

// TestOpenH264Stub_NewReturnsError verifies that the stub returned when the
// project is built without the `openh264` build tag fails fast with a clear
// error rather than silently returning a nil decoder.
func TestOpenH264Stub_NewReturnsError(t *testing.T) {
	dec, err := NewOpenH264Decoder()
	if err == nil {
		t.Fatal("expected error from stub NewOpenH264Decoder, got nil")
	}
	if dec != nil {
		t.Errorf("expected nil decoder, got %v", dec)
	}
	if !strings.Contains(err.Error(), "openh264") {
		t.Errorf("error message should mention openh264, got %q", err.Error())
	}
}

// TestOpenH264Stub_DecodeReturnsError verifies the stub Decode method also
// returns an error.
func TestOpenH264Stub_DecodeReturnsError(t *testing.T) {
	d := &OpenH264Decoder{}
	_, _, _, err := d.Decode(nil)
	if err == nil {
		t.Fatal("expected error from stub Decode, got nil")
	}
	// Close should be a no-op and not panic.
	d.Close()
}
