// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sources

import (
	"context"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

type FlowFactory interface {
	GenerateFlow(source streams.Source, sink streams.Sink)
}

type Source interface {
	Start(generator FlowFactory) error
	StartAsync(generator FlowFactory, sink streams.Sink) error
}

// Stoppable is an optional capability that sources can implement to support
// graceful shutdown driven by an external context (e.g. App.RunWithContext).
// Stop should cause any blocking Start/StartAsync call to return promptly.
type Stoppable interface {
	Stop(ctx context.Context) error
}
