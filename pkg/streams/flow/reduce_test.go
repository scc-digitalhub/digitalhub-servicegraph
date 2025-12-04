// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	ext "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/internal/assert"
)

func TestReduce(t *testing.T) {
	tests := []struct {
		name       string
		reduceFlow streams.Flow
		ptr        bool
	}{
		{
			name: "values",
			reduceFlow: flow.NewReduce(func(a int, b int) int {
				return a + b
			}),
			ptr: false,
		},
		{
			name: "pointers",
			reduceFlow: flow.NewReduce(func(a *int, b *int) *int {
				result := *a + *b
				return &result
			}),
			ptr: true,
		},
	}
	input := []int{1, 2, 3, 4, 5}
	expected := []int{1, 3, 6, 10, 15}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := make(chan any, 5)
			out := make(chan any, 5)

			source := ext.NewChanSource(in)
			sink := ext.NewChanSink(out)

			if tt.ptr {
				ingestSlice(ptrSlice(input), in)
				close(in)

				source.
					Via(tt.reduceFlow).
					To(sink)

				output := readSlicePtr[int](out)
				assert.Equal(t, ptrSlice(expected), output)
			} else {
				ingestSlice(input, in)
				close(in)

				source.
					Via(tt.reduceFlow).
					Via(flow.NewPassThrough()). // Via coverage
					To(sink)

				output := readSlice[int](out)
				assert.Equal(t, expected, output)
			}
		})
	}
}
