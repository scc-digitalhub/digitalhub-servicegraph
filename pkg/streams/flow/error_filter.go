// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"sync"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// ErrorFilter is a Flow element that splits the stream at the error boundary.
// Events whose status is >= 400 are routed to ErrOut; all other events pass
// through the happy-path Out channel unmodified.
//
//	in  -- OK -- ERR -- OK --- ERR -- OK --
//	                |              |
//	             [ErrOut]       [ErrOut]
//	out  -- OK -------- OK --------- OK --
//
// Both Out and ErrOut are closed when all input has been processed.
// The ErrOut channel must be consumed to prevent the pipeline from stalling;
// use the graph-level wiring in app.App.GenerateFlow for production use.
type ErrorFilter struct {
	in          chan any
	out         chan any
	errOut      chan any
	parallelism int
}

// Verify ErrorFilter satisfies the Flow interface.
var _ streams.Flow = (*ErrorFilter)(nil)

// NewErrorFilter returns a new ErrorFilter with the given parallelism.
// Set parallelism to 1 when output ordering must be preserved.
func NewErrorFilter(parallelism int) *ErrorFilter {
	if parallelism < 1 {
		parallelism = 1
	}
	ef := &ErrorFilter{
		in:          make(chan any),
		out:         make(chan any),
		errOut:      make(chan any),
		parallelism: parallelism,
	}
	go ef.stream()
	return ef
}

// ErrOut returns the dead-letter channel that receives all error events
// (GetStatus() >= 400). The caller is responsible for consuming this channel.
func (ef *ErrorFilter) ErrOut() <-chan any {
	return ef.errOut
}

// Via asynchronously streams happy-path data to the given Flow and returns it.
func (ef *ErrorFilter) Via(flow streams.Flow) streams.Flow {
	go ef.transmit(flow)
	return flow
}

// To streams happy-path data to the given Sink and blocks until it completes.
func (ef *ErrorFilter) To(sink streams.Sink) {
	ef.transmit(sink)
	sink.AwaitCompletion()
}

// Out returns the happy-path output channel (only events with status < 400).
func (ef *ErrorFilter) Out() <-chan any {
	return ef.out
}

// In returns the input channel of the ErrorFilter.
func (ef *ErrorFilter) In() chan<- any {
	return ef.in
}

func (ef *ErrorFilter) transmit(inlet streams.Inlet) {
	for element := range ef.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

// stream routes each incoming element to either the happy-path or dead-letter
// channel based on its status code.
func (ef *ErrorFilter) stream() {
	var wg sync.WaitGroup
	for i := 0; i < ef.parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for element := range ef.in {
				if ev, ok := element.(streams.Event); ok && ev.GetStatus() >= 400 {
					ef.errOut <- element
				} else {
					ef.out <- element
				}
			}
		}()
	}
	wg.Wait()
	close(ef.out)
	close(ef.errOut)
}
