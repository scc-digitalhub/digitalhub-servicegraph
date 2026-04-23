// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package app

import "expvar"

// Package-level expvar counters exposed via the health server's /debug/vars
// endpoint. The variables are registered at init time so they appear in the
// expvar map even before the first event is processed.
var (
	// metricEventsIn counts every event accepted from the source.
	metricEventsIn = expvar.NewInt("servicegraph_events_in_total")
	// metricEventsErr counts events routed to the error sink.
	metricEventsErr = expvar.NewInt("servicegraph_events_err_total")
	// metricMultiSinkDrops counts events dropped by multiSink due to a
	// downstream sink's buffer being full.
	metricMultiSinkDrops = expvar.NewInt("servicegraph_multisink_drops_total")
)
