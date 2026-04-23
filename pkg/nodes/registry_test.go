// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package nodes_test

import (
	"errors"
	"testing"

	// ensure subpackages register their processors
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/httpclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/wsclient"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/registry"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// fakeNodeProc satisfies both nodes.Converter and nodes.Validator. The Flow
// returned by Convert is left nil so tests need not exercise it.
type fakeNodeProc struct {
	convertErr  error
	validateErr error
}

func (p *fakeNodeProc) Convert(_ *model.NodeConfig) (streams.Flow, error) {
	return nil, p.convertErr
}
func (p *fakeNodeProc) Validate(_ *model.NodeConfig) error { return p.validateErr }

// converterOnlyNode satisfies only Converter — used for the panic path.
type converterOnlyNode struct{}

func (converterOnlyNode) Convert(_ *model.NodeConfig) (streams.Flow, error) { return nil, nil }

func TestRegistrySingletonHasEntries(t *testing.T) {
	if nodes.RegistrySingleton.Registered == nil {
		t.Fatalf("expected RegistrySingleton.Registered to be non-nil")
	}
	if len(nodes.RegistrySingleton.Registered) == 0 {
		t.Fatalf("expected some nodes to be registered")
	}
}

func TestNodesRegistry_RegisterAndGet(t *testing.T) {
	r := nodes.Registry{Registry: *registry.NewRegistry("test-node")}
	r.Register("fake", &fakeNodeProc{})

	conv, err := r.GetConverter("fake")
	if err != nil || conv == nil {
		t.Fatalf("GetConverter failed: %v", err)
	}
	val, err := r.GetValidator("fake")
	if err != nil || val == nil {
		t.Fatalf("GetValidator failed: %v", err)
	}

	if _, err := r.GetConverter("missing"); err == nil {
		t.Errorf("expected error for missing kind from GetConverter")
	}
	if _, err := r.GetValidator("missing"); err == nil {
		t.Errorf("expected error for missing kind from GetValidator")
	}
}

func TestNodesRegistry_RegisterPanicsOnIncomplete(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when registering converter-only processor")
		}
	}()
	r := nodes.Registry{Registry: *registry.NewRegistry("test-node-bad")}
	r.Register("bad", converterOnlyNode{})
}

func TestFakeNodeProc_PropagatesErrors(t *testing.T) {
	want := errors.New("boom")
	p := &fakeNodeProc{validateErr: want, convertErr: want}
	if err := p.Validate(&model.NodeConfig{}); !errors.Is(err, want) {
		t.Errorf("validate err = %v, want %v", err, want)
	}
	if _, err := p.Convert(&model.NodeConfig{}); !errors.Is(err, want) {
		t.Errorf("convert err = %v, want %v", err, want)
	}
}
