// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sources_test

import (
	"testing"

	src "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
)

func TestRegistrySingletonExists(t *testing.T) {
	if src.RegistrySingleton.Registered == nil {
		t.Fatalf("RegistrySingleton not initialized")
	}
}
