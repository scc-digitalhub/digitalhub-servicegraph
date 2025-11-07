package flow_test

import (
	"strings"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	ext "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/internal/assert"
)

func TestFlatMap(t *testing.T) {
	tests := []struct {
		name        string
		flatMapFlow streams.Flow
		inPtr       bool
		outPtr      bool
	}{
		{
			name:  "val-val",
			inPtr: false,
			flatMapFlow: flow.NewFlatMap(func(in string) []string {
				return []string{in, strings.ToUpper(in)}
			}, 1),
			outPtr: false,
		},
		{
			name:  "ptr-val",
			inPtr: true,
			flatMapFlow: flow.NewFlatMap(func(in *string) []string {
				return []string{*in, strings.ToUpper(*in)}
			}, 1),
			outPtr: false,
		},
		{
			name:  "ptr-ptr",
			inPtr: true,
			flatMapFlow: flow.NewFlatMap(func(in *string) []*string {
				upper := strings.ToUpper(*in)
				return []*string{in, &upper}
			}, 1),
			outPtr: true,
		},
		{
			name:  "val-ptr",
			inPtr: false,
			flatMapFlow: flow.NewFlatMap(func(in string) []*string {
				upper := strings.ToUpper(in)
				return []*string{&in, &upper}
			}, 1),
			outPtr: true,
		},
	}
	input := []string{"a", "b", "c"}
	expected := []string{"a", "A", "b", "B", "c", "C"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := make(chan any, 3)
			out := make(chan any, 6)

			source := ext.NewChanSource(in)
			sink := ext.NewChanSink(out)

			if tt.inPtr {
				ingestSlice(ptrSlice(input), in)
			} else {
				ingestSlice(input, in)
			}
			close(in)

			source.
				Via(tt.flatMapFlow).
				To(sink)

			if tt.outPtr {
				output := readSlicePtr[string](out)
				assert.Equal(t, ptrSlice(expected), output)
			} else {
				output := readSlice[string](out)
				assert.Equal(t, expected, output)
			}
		})
	}
}

func TestFlatMap_NonPositiveParallelism(t *testing.T) {
	assert.Panics(t, func() {
		flow.NewFlatMap(addAsterisk, 0)
	})
	assert.Panics(t, func() {
		flow.NewFlatMap(addAsterisk, -1)
	})
}
