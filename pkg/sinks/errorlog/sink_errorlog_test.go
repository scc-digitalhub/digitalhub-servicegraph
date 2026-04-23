// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package errorlog

import (
	"context"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

func TestLogSink_AcceptsEventsAndCompletes(t *testing.T) {
	s := NewLogSink()
	ev, err := streams.NewGenericEvent(context.Background(), []byte("oops"), "", "", nil, nil, 500)
	if err != nil {
		t.Fatalf("failed to build event: %v", err)
	}
	s.In() <- ev
	s.In() <- "non-event element"
	close(s.In())
	s.AwaitCompletion()
}

func TestLogSinkProcessor_ConvertAndValidate(t *testing.T) {
	p := &LogSinkProcessor{}
	if err := p.Validate(model.OutputSpec{Kind: "errorlog"}); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	sink, err := p.Convert(model.OutputSpec{Kind: "errorlog"})
	if err != nil {
		t.Fatalf("Convert returned unexpected error: %v", err)
	}
	if sink == nil {
		t.Fatal("Convert returned nil sink")
	}
	close(sink.In())
	sink.AwaitCompletion()
}
