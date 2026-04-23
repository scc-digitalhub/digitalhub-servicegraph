// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sources_test

import (
	"errors"
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/registry"
	src "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// fakeSource is a minimal Source implementation used by registry tests.
type fakeSource struct{}

func (fakeSource) Start(_ src.FlowFactory) error                      { return nil }
func (fakeSource) StartAsync(_ src.FlowFactory, _ streams.Sink) error { return nil }

// fakeProcessor implements both Converter and Validator.
type fakeProcessor struct {
	validateErr error
	convertErr  error
}

func (p *fakeProcessor) Convert(_ model.InputSpec) (src.Source, error) {
	if p.convertErr != nil {
		return nil, p.convertErr
	}
	return fakeSource{}, nil
}
func (p *fakeProcessor) Validate(_ model.InputSpec) error { return p.validateErr }

// converterOnly satisfies only Converter — used to exercise the panic path.
type converterOnly struct{}

func (converterOnly) Convert(_ model.InputSpec) (src.Source, error) { return nil, nil }

func TestRegistrySingletonExists(t *testing.T) {
	if src.RegistrySingleton.Registered == nil {
		t.Fatalf("RegistrySingleton not initialized")
	}
}

func TestSourcesRegistry_RegisterAndGet(t *testing.T) {
	r := src.Registry{Registry: *registry.NewRegistry("test-src")}
	p := &fakeProcessor{}
	r.Register("fake", p)

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

func TestSourcesRegistry_RegisterPanicsOnIncomplete(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when registering converter-only processor")
		}
	}()
	r := src.Registry{Registry: *registry.NewRegistry("test-src-bad")}
	r.Register("bad", converterOnly{})
}

func TestFakeProcessor_PropagatesErrors(t *testing.T) {
	want := errors.New("boom")
	p := &fakeProcessor{validateErr: want, convertErr: want}
	if err := p.Validate(model.InputSpec{}); !errors.Is(err, want) {
		t.Errorf("validate err = %v, want %v", err, want)
	}
	if _, err := p.Convert(model.InputSpec{}); !errors.Is(err, want) {
		t.Errorf("convert err = %v, want %v", err, want)
	}
}
