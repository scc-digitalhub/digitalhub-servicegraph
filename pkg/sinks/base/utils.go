// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package base

// drainChan discards all elements in the channel and returns when
// it is closed.
func drainChan(ch <-chan any) {
	if ch == nil {
		return
	}
	for {
		_, ok := <-ch
		if !ok {
			break
		}
	}
}
