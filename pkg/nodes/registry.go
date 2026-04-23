// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"fmt"

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

// processor is the union of capabilities every node plugin must implement.
type processor interface {
	Converter
	Validator
}

// Register stores a node plugin under the given kind. The plugin must
// implement both Converter and Validator; registering anything else panics.
func (r *Registry) Register(kind string, p interface{}) {
	if _, ok := p.(processor); !ok {
		panic(fmt.Sprintf("nodes.Registry: %q must implement both Converter and Validator", kind))
	}
	r.Registry.Register(kind, p)
}

// GetConverter returns the Converter for the given kind.
func (r *Registry) GetConverter(kind string) (Converter, error) {
	v, err := r.Registry.Get(kind)
	if err != nil {
		return nil, err
	}
	c, ok := v.(Converter)
	if !ok {
		return nil, fmt.Errorf("node %q does not implement Converter", kind)
	}
	return c, nil
}

// GetValidator returns the Validator for the given kind.
func (r *Registry) GetValidator(kind string) (Validator, error) {
	v, err := r.Registry.Get(kind)
	if err != nil {
		return nil, err
	}
	val, ok := v.(Validator)
	if !ok {
		return nil, fmt.Errorf("node %q does not implement Validator", kind)
	}
	return val, nil
}
