package flow_test

import (
	"strings"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	ext "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/internal/assert"
)

func TestMap(t *testing.T) {
	tests := []struct {
		name    string
		mapFlow streams.Flow
		inPtr   bool
		outPtr  bool
	}{
		{
			name:    "val-val",
			inPtr:   false,
			mapFlow: flow.NewMap(strings.ToUpper, 1),
			outPtr:  false,
		},
		{
			name:  "ptr-val",
			inPtr: true,
			mapFlow: flow.NewMap(func(in *string) string {
				return strings.ToUpper(*in)
			}, 1),
			outPtr: false,
		},
		{
			name:  "ptr-ptr",
			inPtr: true,
			mapFlow: flow.NewMap(func(in *string) *string {
				result := strings.ToUpper(*in)
				return &result
			}, 1),
			outPtr: true,
		},
		{
			name:  "val-ptr",
			inPtr: false,
			mapFlow: flow.NewMap(func(in string) *string {
				result := strings.ToUpper(in)
				return &result
			}, 1),
			outPtr: true,
		},
	}
	input := []string{"a", "b", "c"}
	expected := []string{"A", "B", "C"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := make(chan any, 3)
			out := make(chan any, 3)

			source := ext.NewChanSource(in)
			sink := ext.NewChanSink(out)

			if tt.inPtr {
				ingestSlice(ptrSlice(input), in)
			} else {
				ingestSlice(input, in)
			}
			close(in)

			source.
				Via(tt.mapFlow).
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

func TestMap_NonPositiveParallelism(t *testing.T) {
	assert.Panics(t, func() {
		flow.NewMap(strings.ToUpper, 0)
	})
	assert.Panics(t, func() {
		flow.NewMap(strings.ToUpper, -1)
	})
}
