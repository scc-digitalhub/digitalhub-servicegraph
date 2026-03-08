// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package g711

import (
	"encoding/binary"
	"testing"
)

// ---- mu-law known-value tests -----------------------------------------------

func TestMulaw_Silence(t *testing.T) {
	for _, b := range []byte{0xFF, 0x7F} {
		if got := mulawTable[b]; got != 0 {
			t.Errorf("mulaw silence 0x%02X: expected 0, got %d", b, got)
		}
	}
}

func TestMulaw_MaxMagnitude(t *testing.T) {
	if got := mulawTable[0x80]; got != 32124 {
		t.Errorf("mulaw 0x80: expected +32124, got %d", got)
	}
	if got := mulawTable[0x00]; got != -32124 {
		t.Errorf("mulaw 0x00: expected -32124, got %d", got)
	}
}

func TestMulaw_Symmetry(t *testing.T) {
	for i := 0; i < 128; i++ {
		pos := mulawTable[byte(i|0x80)]
		neg := mulawTable[byte(i&0x7F)]
		if pos != -neg {
			t.Errorf("mulaw symmetry at %d: pos=%d neg=%d", i, pos, neg)
		}
	}
}

// TestMulaw_PositiveHalfRange verifies that positive-half codes (0x80–0xFF)
// decode to non-negative values and negative-half codes (0x00–0x7F) decode to
// non-positive values.
func TestMulaw_PositiveHalfRange(t *testing.T) {
	for i := 0x80; i <= 0xFF; i++ {
		if mulawTable[i] < 0 {
			t.Errorf("mulaw 0x%02X: expected >=0, got %d", i, mulawTable[i])
		}
	}
	for i := 0x00; i < 0x80; i++ {
		if mulawTable[i] > 0 {
			t.Errorf("mulaw 0x%02X: expected <=0, got %d", i, mulawTable[i])
		}
	}
}

func TestMulawToLPCM_Length(t *testing.T) {
	src := []byte{0x00, 0x7F, 0x80, 0xFF}
	if got := len(MulawToLPCM(src)); got != len(src)*2 {
		t.Fatalf("expected len %d, got %d", len(src)*2, got)
	}
}

// TestMulawToLPCM_BigEndian checks that 0x80 (+32124 = 0x7D7C) is stored
// big-endian.
func TestMulawToLPCM_BigEndian(t *testing.T) {
	dst := MulawToLPCM([]byte{0x80})
	got := int16(binary.BigEndian.Uint16(dst))
	if got != 32124 {
		t.Errorf("expected 32124, got %d", got)
	}
}

func TestMulawToLPCM_Empty(t *testing.T) {
	if dst := MulawToLPCM([]byte{}); len(dst) != 0 {
		t.Errorf("expected empty slice, got len %d", len(dst))
	}
}

// ---- A-law known-value tests ------------------------------------------------

// TestAlaw_NearSilence verifies that 0xD5 (smallest positive step) and 0x55
// (smallest negative step) are symmetric and non-zero.
func TestAlaw_NearSilence(t *testing.T) {
	pos := alawTable[0xD5]
	neg := alawTable[0x55]
	if pos <= 0 {
		t.Errorf("alaw 0xD5: expected >0, got %d", pos)
	}
	if neg >= 0 {
		t.Errorf("alaw 0x55: expected <0, got %d", neg)
	}
	if pos != -neg {
		t.Errorf("alaw near-silence not symmetric: +%d / %d", pos, neg)
	}
}

func TestAlaw_Symmetry(t *testing.T) {
	for i := 0; i < 128; i++ {
		pos := alawTable[byte(i|0x80)]
		neg := alawTable[byte(i&0x7F)]
		if pos != -neg {
			t.Errorf("alaw symmetry at %d: pos=%d neg=%d", i, pos, neg)
		}
	}
}

func TestAlaw_MaxMagnitude(t *testing.T) {
	max := int16(0)
	for i := 0; i < 256; i++ {
		v := alawTable[i]
		if v < 0 {
			v = -v
		}
		if v > max {
			max = v
		}
	}
	if max < 30000 {
		t.Errorf("alaw max magnitude too small: %d", max)
	}
}

// TestAlaw_PositiveHalfRange verifies that positive-half codes (0x80–0xFF)
// decode to non-negative values and negative-half codes (0x00–0x7F) to
// non-positive values.
func TestAlaw_PositiveHalfRange(t *testing.T) {
	for i := 0x80; i <= 0xFF; i++ {
		if alawTable[i] < 0 {
			t.Errorf("alaw 0x%02X: expected >=0, got %d", i, alawTable[i])
		}
	}
	for i := 0x00; i < 0x80; i++ {
		if alawTable[i] > 0 {
			t.Errorf("alaw 0x%02X: expected <=0, got %d", i, alawTable[i])
		}
	}
}

func TestAlawToLPCM_Length(t *testing.T) {
	src := []byte{0x00, 0x55, 0xD5, 0xFF}
	if got := len(AlawToLPCM(src)); got != len(src)*2 {
		t.Fatalf("expected len %d, got %d", len(src)*2, got)
	}
}

// TestAlawToLPCM_BigEndian verifies big-endian byte order for a known sample.
func TestAlawToLPCM_BigEndian(t *testing.T) {
	dst := AlawToLPCM([]byte{0xD5})
	got := int16(binary.BigEndian.Uint16(dst))
	if got != alawTable[0xD5] {
		t.Errorf("big-endian mismatch: expected %d, got %d", alawTable[0xD5], got)
	}
}

func TestAlawToLPCM_Empty(t *testing.T) {
	if dst := AlawToLPCM([]byte{}); len(dst) != 0 {
		t.Errorf("expected empty slice, got len %d", len(dst))
	}
}

// ---- Cross-codec invariants -------------------------------------------------

// TestAllSamplesLengthInvariant verifies both functions produce exactly 2 bytes
// for every possible single-byte input.
func TestAllSamplesLengthInvariant(t *testing.T) {
	for i := 0; i < 256; i++ {
		b := []byte{byte(i)}
		if len(MulawToLPCM(b)) != 2 {
			t.Errorf("mulaw: sample 0x%02X produces wrong length", i)
		}
		if len(AlawToLPCM(b)) != 2 {
			t.Errorf("alaw: sample 0x%02X produces wrong length", i)
		}
	}
}
