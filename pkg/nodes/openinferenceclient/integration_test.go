// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package openinferenceclient

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"net"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	pb "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/proto/inference/v2"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// Integration tests for OpenInferenceClient

// TestIntegration_EndToEnd tests the complete flow from input to output
func TestIntegration_EndToEnd(t *testing.T) {
	// Setup mock server
	mockServer := &mockInferenceServer{
		modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
			// Validate request
			if req.ModelName != "image-classifier" {
				t.Errorf("Expected model_name 'image-classifier', got: %s", req.ModelName)
			}
			if len(req.Inputs) != 1 {
				t.Errorf("Expected 1 input, got: %d", len(req.Inputs))
			}

			// Return classification results
			return &pb.ModelInferResponse{
				ModelName:    req.ModelName,
				ModelVersion: "1",
				Id:           "test-req-1",
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name:     "probabilities",
						Datatype: "FP32",
						Shape:    []int64{1, 1000},
						Contents: &pb.InferTensorContents{
							Fp32Contents: generateFloats(1000, 0.001),
						},
					},
				},
			}, nil
		},
	}

	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	// Create client configuration
	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "image-classifier",
		ModelVersion: "1",
		NumInstances: 1,
		Timeout:      10,
		InputTensorSpec: []TensorSpec{
			{Name: "image", DataType: "FP32", Shape: []int64{1, 3, 224, 224}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "probabilities", DataType: "FP32", Shape: []int64{1, 1000}},
		},
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	// Prepare input - send binary data directly (no template)
	// 1*3*224*224 FP32 values = 602112 bytes
	inputData := make([]byte, 1*3*224*224*4) // 4 bytes per FP32
	for i := 0; i < len(inputData); i++ {
		inputData[i] = byte(i % 256)
	}

	// Send input
	oic.In() <- inputData
	close(oic.In())

	// Receive output
	select {
	case result := <-oic.Out():
		event, ok := result.(streams.Event)
		if !ok {
			t.Fatal("Expected Event from output")
		}

		if event.GetStatus() != 200 {
			t.Errorf("Expected status 200, got: %d", event.GetStatus())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(event.GetBody(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response["model_name"] != "image-classifier" {
			t.Errorf("Expected model_name 'image-classifier', got: %v", response["model_name"])
		}

		outputs := response["outputs"].([]interface{})
		if len(outputs) != 1 {
			t.Errorf("Expected 1 output, got: %d", len(outputs))
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for response")
	}
}

// TestIntegration_MultipleRequests tests handling multiple concurrent requests
func TestIntegration_MultipleRequests(t *testing.T) {
	var requestCount atomic.Int32
	mockServer := &mockInferenceServer{
		modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
			requestCount.Add(1)
			return &pb.ModelInferResponse{
				ModelName:    req.ModelName,
				ModelVersion: "1",
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name:     "output",
						Datatype: "FP32",
						Shape:    []int64{1, 1},
						Contents: &pb.InferTensorContents{
							Fp32Contents: []float32{1.0},
						},
					},
				},
			}, nil
		},
	}

	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "test-model",
		NumInstances: 3, // Multiple workers
		Timeout:      10,
		InputTensorSpec: []TensorSpec{
			{Name: "input", DataType: "FP32", Shape: []int64{1, 1}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "output", DataType: "FP32", Shape: []int64{1, 1}},
		},
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	// Send multiple requests - binary data for FP32 tensor [1,1]
	numRequests := 10

	// Send requests concurrently to avoid blocking
	go func() {
		for i := 0; i < numRequests; i++ {
			// 1*1 FP32 values = 4 bytes
			inputData := []byte{0x00, 0x00, 0x80, 0x3f} // 1.0 in little-endian FP32
			oic.In() <- inputData
		}
		close(oic.In())
	}()

	// Collect all responses
	responseCount := 0
	for range oic.Out() {
		responseCount++
	}

	if responseCount != numRequests {
		t.Errorf("Expected %d responses, got: %d", numRequests, responseCount)
	}
}

// TestIntegration_WithInputTemplate tests input transformation using templates
func TestIntegration_WithInputTemplate(t *testing.T) {
	mockServer := &mockInferenceServer{
		modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
			// Verify the transformed input
			if len(req.Inputs) != 1 {
				t.Errorf("Expected 1 input, got: %d", len(req.Inputs))
			}
			if req.Inputs[0].Name != "processed_input" {
				t.Errorf("Expected input name 'processed_input', got: %s", req.Inputs[0].Name)
			}

			return &pb.ModelInferResponse{
				ModelName: req.ModelName,
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name:     "output",
						Datatype: "FP32",
						Shape:    []int64{1, 1},
						Contents: &pb.InferTensorContents{
							Fp32Contents: []float32{0.9},
						},
					},
				},
			}, nil
		},
	}

	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "test-model",
		NumInstances: 1,
		Timeout:      10,
		InputTensorSpec: []TensorSpec{
			{Name: "processed_input", DataType: "FP32", Shape: []int64{1, 1}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "output", DataType: "FP32", Shape: []int64{1, 1}},
		},
		InputTemplates: []InputTemplateSpec{
			{
				Name:     "processed_input",
				Template: `{{.}}`, // Pass through JSON array
			},
		},
	}

	// Compile templates
	if err := conf.setInputTemplates(conf.InputTemplates); err != nil {
		t.Fatalf("Failed to set input templates: %v", err)
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	// Send JSON data which will be converted to binary via template
	inputData := []byte(`[2.0]`) // JSON array that will be converted to FP32 binary
	oic.In() <- inputData
	close(oic.In())

	// Verify response
	select {
	case result := <-oic.Out():
		event := result.(streams.Event)
		if event.GetStatus() != 200 {
			t.Errorf("Expected status 200, got: %d", event.GetStatus())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for response")
	}
}

// TestIntegration_WithOutputTemplate tests output transformation using templates
func TestIntegration_WithOutputTemplate(t *testing.T) {
	mockServer := &mockInferenceServer{
		modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
			return &pb.ModelInferResponse{
				ModelName:    "classifier",
				ModelVersion: "2",
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name:     "probabilities",
						Datatype: "FP32",
						Shape:    []int64{1, 3},
						Contents: &pb.InferTensorContents{
							Fp32Contents: []float32{0.1, 0.8, 0.1},
						},
					},
				},
			}, nil
		},
	}

	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "classifier",
		NumInstances: 1,
		Timeout:      10,
		InputTensorSpec: []TensorSpec{
			{Name: "input", DataType: "FP32", Shape: []int64{1, 3}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "probabilities", DataType: "FP32", Shape: []int64{1, 3}},
		},
		OutputTemplate: `{"prediction": {{. | jp "$.outputs[0].data[1]"}}, "model": "{{. | jp "$.model_name"}}"}`,
	}

	// Compile template
	if err := conf.setOutputTemplate(conf.OutputTemplate); err != nil {
		t.Fatalf("Failed to set output template: %v", err)
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	// Send binary data for FP32 [1,3] tensor (12 bytes = 3 floats)
	inputData := make([]byte, 12)
	// 1.0, 2.0, 3.0 in little-endian FP32
	*(*float32)(unsafe.Pointer(&inputData[0])) = 1.0
	*(*float32)(unsafe.Pointer(&inputData[4])) = 2.0
	*(*float32)(unsafe.Pointer(&inputData[8])) = 3.0

	oic.In() <- inputData
	close(oic.In())

	// Verify transformed output
	select {
	case result := <-oic.Out():
		event := result.(streams.Event)
		if event.GetStatus() != 200 {
			t.Errorf("Expected status 200, got: %d", event.GetStatus())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(event.GetBody(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Check transformed output
		if response["model"] != "classifier" {
			t.Errorf("Expected model 'classifier', got: %v", response["model"])
		}

		// The prediction should be the second value (0.8)
		prediction, ok := response["prediction"].(float64)
		if !ok {
			t.Errorf("Expected prediction to be float64, got: %T", response["prediction"])
		} else if prediction != 0.8 {
			t.Errorf("Expected prediction 0.8, got: %v", prediction)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for response")
	}
}

// TestIntegration_ErrorHandling tests various error scenarios
func TestIntegration_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		serverFunc     func(context.Context, *pb.ModelInferRequest) (*pb.ModelInferResponse, error)
		input          []byte
		expectError    bool
		expectedStatus int
	}{
		{
			name: "server error",
			serverFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
				return nil, status.Error(codes.Internal, "internal server error")
			},
			input:          []byte{0x00, 0x00, 0x80, 0x3f}, // Valid FP32 data
			expectError:    true,
			expectedStatus: 500,
		},
		{
			name: "invalid input data",
			serverFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
				// Return valid response - client doesn't validate binary data size
				return &pb.ModelInferResponse{
					ModelName:         req.ModelName,
					RawOutputContents: [][]byte{{0x00, 0x00, 0x80, 0x3f}},
					Outputs: []*pb.ModelInferResponse_InferOutputTensor{
						{Name: "output", Datatype: "FP32", Shape: []int64{1, 1}},
					},
				}, nil
			},
			input:          []byte{0x01, 0x02}, // Client sends any binary data without validation
			expectError:    false,              // No client-side validation
			expectedStatus: 200,
		},
		{
			name: "timeout",
			serverFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
				time.Sleep(2 * time.Second)
				return &pb.ModelInferResponse{ModelName: req.ModelName}, nil
			},
			input:          []byte{0x00, 0x00, 0x80, 0x3f}, // Valid FP32 data
			expectError:    true,
			expectedStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := &mockInferenceServer{
				modelInferFunc: tt.serverFunc,
			}

			srv, lis := setupMockServer(t, mockServer)
			defer srv.Stop()

			conf := Configuration{
				Address:      "bufnet",
				ModelName:    "test-model",
				NumInstances: 1,
				Timeout:      1, // Short timeout for timeout test
				InputTensorSpec: []TensorSpec{
					{Name: "input", DataType: "FP32", Shape: []int64{1, 1}},
				},
				OutputTensorSpec: []TensorSpec{
					{Name: "output", DataType: "FP32", Shape: []int64{1, 1}},
				},
			}

			oic := createMockClient(t, lis, conf)
			defer oic.Close()

			// Send input concurrently to avoid blocking
			go func() {
				oic.In() <- tt.input
				close(oic.In())
			}()

			select {
			case result := <-oic.Out():
				event := result.(streams.Event)

				if tt.expectError && event.GetStatus() == 200 {
					t.Error("Expected error status, got 200")
				}
				if !tt.expectError && event.GetStatus() != 200 {
					t.Errorf("Expected status 200, got: %d", event.GetStatus())
				}
				if tt.expectedStatus > 0 && event.GetStatus() != tt.expectedStatus {
					t.Errorf("Expected status %d, got: %d", tt.expectedStatus, event.GetStatus())
				}

			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for response")
			}
		})
	}
}

// TestIntegration_AllDataTypes tests all supported tensor data types
func TestIntegration_AllDataTypes(t *testing.T) {
	tests := []struct {
		name      string
		datatype  string
		inputData []byte
		count     int
		validate  func(*testing.T, []interface{})
	}{
		{
			name:      "BOOL",
			datatype:  "BOOL",
			inputData: []byte{0x01, 0x00, 0x01}, // true, false, true
			count:     3,
			validate: func(t *testing.T, data []interface{}) {
				if len(data) != 3 {
					t.Errorf("Expected 3 values, got: %d", len(data))
				}
			},
		},
		{
			name:     "INT32",
			datatype: "INT32",
			inputData: func() []byte {
				buf := make([]byte, 12) // 3 * 4 bytes
				binary.LittleEndian.PutUint32(buf[0:4], 1)
				binary.LittleEndian.PutUint32(buf[4:8], 2)
				binary.LittleEndian.PutUint32(buf[8:12], 3)
				return buf
			}(),
			count: 3,
			validate: func(t *testing.T, data []interface{}) {
				if len(data) != 3 {
					t.Errorf("Expected 3 values, got: %d", len(data))
				}
			},
		},
		{
			name:     "INT64",
			datatype: "INT64",
			inputData: func() []byte {
				buf := make([]byte, 16) // 2 * 8 bytes
				binary.LittleEndian.PutUint64(buf[0:8], 100)
				binary.LittleEndian.PutUint64(buf[8:16], 200)
				return buf
			}(),
			count: 2,
			validate: func(t *testing.T, data []interface{}) {
				if len(data) != 2 {
					t.Errorf("Expected 2 values, got: %d", len(data))
				}
			},
		},
		{
			name:     "FP32",
			datatype: "FP32",
			inputData: func() []byte {
				buf := make([]byte, 8) // 2 * 4 bytes
				binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(1.5))
				binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(2.5))
				return buf
			}(),
			count: 2,
			validate: func(t *testing.T, data []interface{}) {
				if len(data) != 2 {
					t.Errorf("Expected 2 values, got: %d", len(data))
				}
			},
		},
		{
			name:     "FP64",
			datatype: "FP64",
			inputData: func() []byte {
				buf := make([]byte, 16) // 2 * 8 bytes
				binary.LittleEndian.PutUint64(buf[0:8], math.Float64bits(3.14))
				binary.LittleEndian.PutUint64(buf[8:16], math.Float64bits(2.71))
				return buf
			}(),
			count: 2,
			validate: func(t *testing.T, data []interface{}) {
				if len(data) != 2 {
					t.Errorf("Expected 2 values, got: %d", len(data))
				}
			},
		},
		{
			name:      "BYTES",
			datatype:  "BYTES",
			inputData: []byte("helloworld"), // 10 bytes
			count:     10,
			validate: func(t *testing.T, data []interface{}) {
				// BYTES returns [][]byte, which gets converted to []interface{}
				// Each element is a byte array
				if len(data) != 1 {
					t.Errorf("Expected 1 byte array, got: %d", len(data))
					return
				}
				// First element should be a string (base64 encoded or the bytes themselves)
				// In JSON, byte arrays are typically represented as base64 strings or arrays
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := &mockInferenceServer{
				modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
					// Echo back the input using RawOutputContents
					output := &pb.ModelInferResponse_InferOutputTensor{
						Name:     "output",
						Datatype: req.Inputs[0].Datatype,
						Shape:    req.Inputs[0].Shape,
					}

					// Echo back RawInputContents as RawOutputContents
					rawOutputContents := req.RawInputContents

					return &pb.ModelInferResponse{
						ModelName:         req.ModelName,
						Outputs:           []*pb.ModelInferResponse_InferOutputTensor{output},
						RawOutputContents: rawOutputContents,
					}, nil
				},
			}

			srv, lis := setupMockServer(t, mockServer)
			defer srv.Stop()

			conf := Configuration{
				Address:      "bufnet",
				ModelName:    "test-model",
				NumInstances: 1,
				Timeout:      10,
				InputTensorSpec: []TensorSpec{
					{
						Name:     "test_input",
						DataType: tt.datatype,
						Shape:    []int64{1, int64(tt.count)},
					},
				},
				OutputTensorSpec: []TensorSpec{
					{
						Name:     "output",
						DataType: tt.datatype,
					},
				},
			}

			oic := createMockClient(t, lis, conf)
			defer oic.Close()

			// Send binary data in goroutine to avoid blocking on unbuffered channel
			go func() {
				oic.In() <- tt.inputData
				close(oic.In())
			}()

			select {
			case result := <-oic.Out():
				event := result.(streams.Event)
				if event.GetStatus() != 200 {
					t.Errorf("Expected status 200, got: %d", event.GetStatus())
				}

				var response map[string]interface{}
				if err := json.Unmarshal(event.GetBody(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				outputs := response["outputs"].([]interface{})
				if len(outputs) != 1 {
					t.Fatalf("Expected 1 output, got: %d", len(outputs))
				}

				outputMap := outputs[0].(map[string]interface{})
				// With RawOutputContents, data is in data field as base64 string
				if rawData, ok := outputMap["data"]; ok && rawData != nil {
					// Successfully received raw output data
					// For this test, we just verify we got the data back
				} else {
					t.Errorf("Expected data field in output")
				}

			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for response")
			}
		})
	}
}

// Helper functions

func generateFloats(count int, value float32) []float32 {
	result := make([]float32, count)
	for i := 0; i < count; i++ {
		result[i] = value
	}
	return result
}

func generateFloatsJSON(count int, value float64) string {
	result := "["
	for i := 0; i < count; i++ {
		if i > 0 {
			result += ","
		}
		result += "0.5"
	}
	result += "]"
	return result
}

func generateShapeJSON(size int) string {
	return "1, " + string(rune('0'+size))
}

// mockGRPCServer provides a complete mock inference server for integration tests
type mockGRPCServer struct {
	server   *grpc.Server
	listener *bufconn.Listener
	address  string
}

func newMockGRPCServer(t *testing.T) *mockGRPCServer {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()

	mockImpl := &mockInferenceServer{
		modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
			return &pb.ModelInferResponse{
				ModelName:    req.ModelName,
				ModelVersion: "1",
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name:     "output",
						Datatype: "FP32",
						Shape:    []int64{1, 1},
						Contents: &pb.InferTensorContents{
							Fp32Contents: []float32{1.0},
						},
					},
				},
			}, nil
		},
	}

	pb.RegisterGRPCInferenceServiceServer(srv, mockImpl)

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Mock server exited: %v", err)
		}
	}()

	return &mockGRPCServer{
		server:   srv,
		listener: lis,
		address:  "bufnet",
	}
}

func (m *mockGRPCServer) dialContext(ctx context.Context, address string) (net.Conn, error) {
	return m.listener.Dial()
}

func (m *mockGRPCServer) stop() {
	m.server.Stop()
}
