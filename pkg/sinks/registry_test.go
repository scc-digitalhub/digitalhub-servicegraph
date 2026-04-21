// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sinks_test

import (
	"testing"

	// ensure subpackages register their converters
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/base"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/mjpeg"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/webhook"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/websocket"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
)

func TestRegistrySingletonHasEntries(t *testing.T) {
	// Ensure the registry singleton is initialized and has at least the
	// standard converters registered by init() functions.
	if sinks.RegistrySingleton.Registered == nil {
		t.Fatalf("expected RegistrySingleton.Registered to be non-nil")
	}
	if len(sinks.RegistrySingleton.Registered) == 0 {
		t.Fatalf("expected some sinks to be registered")
	}
}
