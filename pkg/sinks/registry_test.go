// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package sinks_test

import (
	"errors"
	"testing"

	// ensure subpackages register their converters
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/base"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/mjpeg"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/webhook"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/websocket"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/registry"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// fakeSink is a minimal streams.Sink used by registry tests.
type fakeSink struct{ in chan any }

func (s *fakeSink) In() chan<- any   { return s.in }
func (s *fakeSink) AwaitCompletion() {}

// fakeSinkProc implements sinks.Converter and sinks.Validator.
type fakeSinkProc struct {
	convertErr  error
	validateErr error
}

func (p *fakeSinkProc) Convert(_ model.OutputSpec) (streams.Sink, error) {
	if p.convertErr != nil {
		return nil, p.convertErr
	}
	return &fakeSink{in: make(chan any, 1)}, nil
}
func (p *fakeSinkProc) Validate(_ model.OutputSpec) error { return p.validateErr }

// validatorOnlySink satisfies only Validator — used for the panic path.
type validatorOnlySink struct{}

func (validatorOnlySink) Validate(_ model.OutputSpec) error { return nil }

func TestRegistrySingletonHasEntries(t *testing.T) {
	if sinks.RegistrySingleton.Registered == nil {
		t.Fatalf("expected RegistrySingleton.Registered to be non-nil")
	}
	if len(sinks.RegistrySingleton.Registered) == 0 {
		t.Fatalf("expected some sinks to be registered")
	}
}

func TestSinksRegistry_RegisterAndGet(t *testing.T) {
	r := sinks.Registry{Registry: *registry.NewRegistry("test-sink")}
	r.Register("fake", &fakeSinkProc{})

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

func TestSinksRegistry_RegisterPanicsOnIncomplete(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when registering validator-only processor")
		}
	}()
	r := sinks.Registry{Registry: *registry.NewRegistry("test-sink-bad")}
	r.Register("bad", validatorOnlySink{})
}

func TestFakeSinkProc_PropagatesErrors(t *testing.T) {
	want := errors.New("boom")
	p := &fakeSinkProc{validateErr: want, convertErr: want}
	if err := p.Validate(model.OutputSpec{}); !errors.Is(err, want) {
		t.Errorf("validate err = %v, want %v", err, want)
	}
	if _, err := p.Convert(model.OutputSpec{}); !errors.Is(err, want) {
		t.Errorf("convert err = %v, want %v", err, want)
	}
}
