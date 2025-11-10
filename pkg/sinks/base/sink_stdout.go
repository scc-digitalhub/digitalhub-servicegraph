package base

import (
	"fmt"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// StdoutSink represents a simple outbound connector that writes
// streaming data to standard output.
type StdoutSink struct {
	in   chan any
	done chan struct{}
}

var _ streams.Sink = (*StdoutSink)(nil)

// NewStdoutSink returns a new StdoutSink connector.
func NewStdoutSink() *StdoutSink {
	stdoutSink := &StdoutSink{
		in:   make(chan any),
		done: make(chan struct{}),
	}

	// asynchronously process stream data
	go stdoutSink.process()

	return stdoutSink
}

func (stdout *StdoutSink) process() {
	defer close(stdout.done)
	for elem := range stdout.in {
		switch v := elem.(type) {
		case error:
			fmt.Printf("Error: %v\n", v)
		case streams.Event:
			fmt.Printf("Event Status: %v\n", v.GetStatus())
			fmt.Printf("Event Body: %s\n", string(v.GetBody()))
		case string:
			fmt.Println(v)
		case []byte:
			fmt.Println(string(v))
		default:
			fmt.Println(v)
		}
	}
}

// In returns the input channel of the StdoutSink connector.
func (stdout *StdoutSink) In() chan<- any {
	return stdout.in
}

// AwaitCompletion blocks until the StdoutSink has processed all received data.
func (stdout *StdoutSink) AwaitCompletion() {
	<-stdout.done
}

func init() {
	sinks.RegistrySingleton.Register("stdout", &StdOutConverter{})
}

type StdOutConverter struct {
	sinks.Converter
}

func (c *StdOutConverter) Convert(input model.OutputSpec) (streams.Sink, error) {
	src := NewStdoutSink()
	return src, nil
}
