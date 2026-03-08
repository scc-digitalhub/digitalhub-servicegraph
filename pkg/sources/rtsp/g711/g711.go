// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

// Package g711 converts G.711 encoded audio to 16-bit big-endian linear PCM (audio/L16).
// Both codecs use pre-computed 256-entry lookup tables built at package init,
// so the hot path per sample is a single table lookup and two byte stores
// with no branching.
// Specification: ITU-T G.711 (1988), Annex A (A-law) and Annex B (µ-law).
package g711

import "encoding/binary"

// mulawTable maps each 8-bit µ-law encoded byte to its 16-bit linear value.
var mulawTable [256]int16

// alawTable maps each 8-bit A-law encoded byte to its 16-bit linear value.
var alawTable [256]int16

func init() {
	for i := 0; i < 256; i++ {
		// µ-law expansion (ITU-T G.711 Annex B)
		c := byte(^i)
		t := (int16(c&0x0F) << 3) + 0x84
		t <<= (c & 0x70) >> 4
		if c&0x80 != 0 {
			mulawTable[i] = 0x84 - t
		} else {
			mulawTable[i] = t - 0x84
		}

		// A-law expansion (ITU-T G.711 Annex A)
		a := byte(i) ^ 0x55
		s := int16(a & 0x0F)
		seg := (a & 0x70) >> 4
		if seg != 0 {
			s = (s | 0x10) << seg
		} else {
			s = s*2 + 1
		}
		s <<= 3
		if a&0x80 == 0 {
			s = -s
		}
		alawTable[i] = s
	}
}

// MulawToLPCM converts G.711 µ-law bytes to 16-bit big-endian linear PCM.
// The returned slice is always 2*len(src).
func MulawToLPCM(src []byte) []byte {
	dst := make([]byte, len(src)*2)
	for i, b := range src {
		binary.BigEndian.PutUint16(dst[i*2:], uint16(mulawTable[b]))
	}
	return dst
}

// AlawToLPCM converts G.711 A-law bytes to 16-bit big-endian linear PCM.
// The returned slice is always 2*len(src).
func AlawToLPCM(src []byte) []byte {
	dst := make([]byte, len(src)*2)
	for i, b := range src {
		binary.BigEndian.PutUint16(dst[i*2:], uint16(alawTable[b]))
	}
	return dst
}
