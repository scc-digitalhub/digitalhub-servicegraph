// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sources

import (
	"fmt"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/registry"
)

type Registry struct {
	registry.Registry
}

// RegistrySingleton is a trigger global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("source"),
}

type Converter interface {
	Convert(spec model.InputSpec) (Source, error)
}

type Validator interface {
	Validate(spec model.InputSpec) error
}

// processor is the union of capabilities every source plugin must implement.
type processor interface {
	Converter
	Validator
}

// Register stores a source plugin under the given kind. The plugin must
// implement both Converter and Validator; registering anything else panics
// (this is an init-time programming error).
func (r *Registry) Register(kind string, p interface{}) {
	if _, ok := p.(processor); !ok {
		panic(fmt.Sprintf("sources.Registry: %q must implement both Converter and Validator", kind))
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
		return nil, fmt.Errorf("source %q does not implement Converter", kind)
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
		return nil, fmt.Errorf("source %q does not implement Validator", kind)
	}
	return val, nil
}
