package extension_test

import (
	"testing"

	ext "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
)

func TestChanSourceAndSink_BasicFlow(t *testing.T) {
	in := make(chan any, 2)
	out := make(chan any, 2)
	in <- "a"
	in <- "b"
	close(in)

	src := ext.NewChanSource(in)
	sink := ext.NewChanSink(out)

	// connect source via pass-through to sink
	go func() {
		src.Via(flow.NewPassThrough()).To(sink)
	}()

	var got []string
	for v := range out {
		got = append(got, v.(string))
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %d", len(got))
	}
}

func TestChanSink_AwaitCompletionNoop(t *testing.T) {
	out := make(chan any, 1)
	sink := ext.NewChanSink(out)
	// AwaitCompletion is no-op; just ensure it exists and doesn't block
	sink.AwaitCompletion()
}
