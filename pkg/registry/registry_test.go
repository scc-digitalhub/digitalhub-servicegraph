// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"sync"
	"testing"
)

type dummy struct{ Name string }

func TestRegisterGetAndKinds(t *testing.T) {
	r := NewRegistry("test")

	r.Register("one", &dummy{Name: "d1"})
	r.Register("two", &dummy{Name: "d2"})

	v, err := r.Get("one")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d, ok := v.(*dummy); !ok || d.Name != "d1" {
		t.Fatalf("unexpected value from registry: %#v", v)
	}

	kinds := r.GetKinds()
	if len(kinds) != 2 {
		t.Fatalf("expected 2 kinds, got %d", len(kinds))
	}
}

func TestGetMissingReturnsError(t *testing.T) {
	r := NewRegistry("missing")
	_, err := r.Get("nope")
	if err == nil {
		t.Fatalf("expected error for missing kind")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on duplicate register")
		}
	}()

	r := NewRegistry("dup")
	r.Register("k", &dummy{Name: "first"})
	// second register should panic
	r.Register("k", &dummy{Name: "second"})
}

func TestConcurrentRegisterAndGet(t *testing.T) {
	r := NewRegistry("concurrent")
	var wg sync.WaitGroup

	// start concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// repeatedly call Get; initially missing, then present
			for j := 0; j < 50; j++ {
				r.Get("x")
			}
		}()
	}

	// concurrently register after a few iterations
	r.Register("x", &dummy{Name: "con"})

	wg.Wait()
}
