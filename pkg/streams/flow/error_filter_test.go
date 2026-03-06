// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"sort"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
)

// drainBoth reads all events from both channels concurrently and returns them
// separately. It blocks until both channels are closed.
func drainBoth(ef *flow.ErrorFilter) (ok []any, errs []any) {
	okCh := make(chan []any, 1)
	errCh := make(chan []any, 1)

	go func() {
		var s []any
		for e := range ef.Out() {
			s = append(s, e)
		}
		okCh <- s
	}()
	go func() {
		var s []any
		for e := range ef.ErrOut() {
			s = append(s, e)
		}
		errCh <- s
	}()

	return <-okCh, <-errCh
}

func makeEvent(t *testing.T, status int) streams.Event {
	t.Helper()
	ev, err := streams.NewGenericEvent(context.Background(), []byte("body"), "http://example.com/", "GET", nil, nil, status)
	if err != nil {
		t.Fatalf("NewGenericEvent: %v", err)
	}
	return ev
}

func statusOf(e any) int {
	if ev, ok := e.(streams.Event); ok {
		return ev.GetStatus()
	}
	return -1
}

// TestErrorFilter_HappyPath verifies that non-error events reach Out and
// ErrOut remains empty.
func TestErrorFilter_HappyPath(t *testing.T) {
	ef := flow.NewErrorFilter(1)

	go func() {
		for _, sc := range []int{200, 201, 204, 301, 399} {
			ef.In() <- makeEvent(t, sc)
		}
		close(ef.In())
	}()

	ok, errs := drainBoth(ef)
	if len(errs) != 0 {
		t.Errorf("expected 0 error events, got %d", len(errs))
	}
	if len(ok) != 5 {
		t.Errorf("expected 5 ok events, got %d", len(ok))
	}
}

// TestErrorFilter_AllErrors verifies that error events reach ErrOut and Out
// remains empty.
func TestErrorFilter_AllErrors(t *testing.T) {
	ef := flow.NewErrorFilter(1)

	go func() {
		for _, sc := range []int{400, 404, 500, 502, 503} {
			ef.In() <- makeEvent(t, sc)
		}
		close(ef.In())
	}()

	ok, errs := drainBoth(ef)
	if len(ok) != 0 {
		t.Errorf("expected 0 ok events, got %d", len(ok))
	}
	if len(errs) != 5 {
		t.Errorf("expected 5 error events, got %d", len(errs))
	}
}

// TestErrorFilter_Mixed verifies correct routing when the stream contains a
// mixture of ok and error events.
func TestErrorFilter_Mixed(t *testing.T) {
	ef := flow.NewErrorFilter(1)

	input := []int{200, 500, 201, 404, 204, 200}
	go func() {
		for _, sc := range input {
			ef.In() <- makeEvent(t, sc)
		}
		close(ef.In())
	}()

	ok, errs := drainBoth(ef)
	if len(ok) != 4 {
		t.Errorf("expected 4 ok events, got %d: %v", len(ok), ok)
	}
	if len(errs) != 2 {
		t.Errorf("expected 2 error events, got %d: %v", len(errs), errs)
	}

	// status codes of error events must be the error ones
	got := make([]int, 0, len(errs))
	for _, e := range errs {
		got = append(got, statusOf(e))
	}
	sort.Ints(got)
	want := []int{404, 500}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("error event %d: want status %d, got %d", i, v, got[i])
		}
	}
}

// TestErrorFilter_NonEventPassthrough verifies that non-streams.Event values
// (plain strings, structs) are forwarded to the happy-path channel unchanged.
func TestErrorFilter_NonEventPassthrough(t *testing.T) {
	ef := flow.NewErrorFilter(1)

	go func() {
		ef.In() <- "hello"
		ef.In() <- struct{ X int }{X: 42}
		close(ef.In())
	}()

	ok, errs := drainBoth(ef)
	if len(errs) != 0 {
		t.Errorf("expected 0 error events for non-Event values, got %d", len(errs))
	}
	if len(ok) != 2 {
		t.Errorf("expected 2 ok events for non-Event values, got %d", len(ok))
	}
}

// TestErrorFilter_EmptyStream verifies that both channels are closed with no
// events when the input stream is empty.
func TestErrorFilter_EmptyStream(t *testing.T) {
	ef := flow.NewErrorFilter(1)
	close(ef.In())

	ok, errs := drainBoth(ef)
	if len(ok) != 0 || len(errs) != 0 {
		t.Errorf("expected empty channels, got ok=%d err=%d", len(ok), len(errs))
	}
}

// TestErrorFilter_Boundary verifies the exact boundary: status 399 is ok,
// status 400 is an error.
func TestErrorFilter_Boundary(t *testing.T) {
	ef := flow.NewErrorFilter(1)

	go func() {
		ef.In() <- makeEvent(t, 399)
		ef.In() <- makeEvent(t, 400)
		close(ef.In())
	}()

	ok, errs := drainBoth(ef)
	if len(ok) != 1 || statusOf(ok[0]) != 399 {
		t.Errorf("expected ok=[399], got %v", ok)
	}
	if len(errs) != 1 || statusOf(errs[0]) != 400 {
		t.Errorf("expected err=[400], got %v", errs)
	}
}

// TestErrorFilter_Parallelism verifies that the filter works correctly with
// parallelism > 1 (ordering may differ, but counts must be correct).
func TestErrorFilter_Parallelism(t *testing.T) {
	const n = 100
	ef := flow.NewErrorFilter(4)

	go func() {
		for i := 0; i < n; i++ {
			if i%2 == 0 {
				ef.In() <- makeEvent(t, 200)
			} else {
				ef.In() <- makeEvent(t, 500)
			}
		}
		close(ef.In())
	}()

	ok, errs := drainBoth(ef)
	if len(ok) != n/2 {
		t.Errorf("expected %d ok events, got %d", n/2, len(ok))
	}
	if len(errs) != n/2 {
		t.Errorf("expected %d error events, got %d", n/2, len(errs))
	}
}

// TestErrorFilter_ErrorEvent verifies that events created via NewErrorEvent
// (which always carry status >= 400) are routed to ErrOut.
func TestErrorFilter_ErrorEvent(t *testing.T) {
	ef := flow.NewErrorFilter(1)

	go func() {
		ef.In() <- streams.NewErrorEvent(context.Background(), nil, 503)
		ef.In() <- makeEvent(t, 200)
		close(ef.In())
	}()

	ok, errs := drainBoth(ef)
	if len(errs) != 1 || statusOf(errs[0]) != 503 {
		t.Errorf("expected err=[503], got %v", errs)
	}
	if len(ok) != 1 || statusOf(ok[0]) != 200 {
		t.Errorf("expected ok=[200], got %v", ok)
	}
}
