// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package nodes_test

import (
	"testing"

	// ensure subpackages register their processors
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/httpclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/wsclient"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
)

func TestRegistrySingletonHasEntries(t *testing.T) {
	if nodes.RegistrySingleton.Registered == nil {
		t.Fatalf("expected RegistrySingleton.Registered to be non-nil")
	}
	if len(nodes.RegistrySingleton.Registered) == 0 {
		t.Fatalf("expected some nodes to be registered")
	}
}
