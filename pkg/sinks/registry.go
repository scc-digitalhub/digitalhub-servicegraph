// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sinks

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
	Registry: *registry.NewRegistry("sink"),
}

type Converter interface {
	Convert(spec model.OutputSpec) (streams.Sink, error)
}

type Validator interface {
	Validate(spec model.OutputSpec) error
}
