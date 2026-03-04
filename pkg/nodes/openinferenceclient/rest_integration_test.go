// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package openinferenceclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

// Test REST end-to-end flow using httptest server
func TestREST_EndToEnd(t *testing.T) {
	// Prepare a mock HTTP server that emulates the REST OpenInference endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Return REST-style JSON response per Open Inference REST spec
		resp := map[string]any{
			"model_name":    "image-classifier",
			"model_version": "1",
			"id":            "rest-test-1",
			"outputs": []any{
				map[string]any{
					"name":     "probabilities",
					"datatype": "FP32",
					"shape":    []int64{1, 1000},
					// data: an array of zeros
					"data": make([]float64, 1000),
				},
			},
		}
		b, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal response: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	// Create configuration for REST client
	conf := Configuration{
		Address:      srv.URL, // includes scheme and host
		ModelName:    "image-classifier",
		ModelVersion: "1",
		NumInstances: 1,
		Timeout:      5,
		Protocol:     "rest",
		InputTensorSpec: []TensorSpec{
			{Name: "image", DataType: "FP32", Shape: []int64{1, 3, 224, 224}},
		},
		OutputTensorSpec: []TensorSpec{{Name: "probabilities", DataType: "FP32", Shape: []int64{1, 1000}}},
	}

	oic, err := NewOpenInferenceClient(conf)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer oic.Close()

	// prepare simple input body
	input := []byte("dummy")

	// send input
	go func() {
		oic.In() <- input
		close(oic.In())
	}()

	// wait for output
	select {
	case res := <-oic.Out():
		event, ok := res.(streams.Event)
		if !ok {
			t.Fatalf("expected Event, got %T", res)
		}
		if event.GetStatus() != 200 {
			t.Fatalf("expected status 200, got %d", event.GetStatus())
		}
		// validate JSON body
		var parsed map[string]any
		if err := json.Unmarshal(event.GetBody(), &parsed); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}

	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for rest response")
	}
}
