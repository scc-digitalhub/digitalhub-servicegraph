// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sources

import (
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
