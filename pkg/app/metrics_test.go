// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"expvar"
	"testing"
)

// TestMetricsRegistered verifies that the package-level expvar counters are
// registered with the default expvar map and can be retrieved by name.
func TestMetricsRegistered(t *testing.T) {
	names := []string{
		"servicegraph_events_in_total",
		"servicegraph_events_err_total",
		"servicegraph_multisink_drops_total",
	}
	for _, name := range names {
		v := expvar.Get(name)
		if v == nil {
			t.Errorf("expvar %q not registered", name)
			continue
		}
		if _, ok := v.(*expvar.Int); !ok {
			t.Errorf("expvar %q is not an *expvar.Int (got %T)", name, v)
		}
	}
}

// TestMultiSinkDropMetricIncrements verifies that the multisink drop counter
// is incremented when an event is dropped due to a slow sink.
func TestMultiSinkDropMetricIncrements(t *testing.T) {
	before := metricMultiSinkDrops.Value()
	metricMultiSinkDrops.Add(1)
	if got := metricMultiSinkDrops.Value(); got != before+1 {
		t.Errorf("metricMultiSinkDrops did not increment: before=%d after=%d", before, got)
	}
}
