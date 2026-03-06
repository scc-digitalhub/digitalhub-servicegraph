// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/registry"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

type Registry struct {
	registry.Registry
}

// RegistrySingleton is a trigger global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("services"),
}

type Converter interface {
	// Convert constructs a Flow from the node's configuration.
	// The pointer allows Convert to cache parsed config on the NodeConfig
	// so subsequent calls skip re-parsing (safe for concurrent use).
	Convert(spec *model.NodeConfig) (streams.Flow, error)
}

type Validator interface {
	// Validate checks the node configuration for correctness without building
	// the full Flow.
	Validate(spec *model.NodeConfig) error
}
