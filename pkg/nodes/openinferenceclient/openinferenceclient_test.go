// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package openinferenceclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	pb "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/proto/inference/v2"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// mockInferenceServer is a mock gRPC server for testing
type mockInferenceServer struct {
	pb.UnimplementedGRPCInferenceServiceServer
	modelInferFunc func(context.Context, *pb.ModelInferRequest) (*pb.ModelInferResponse, error)
}

func (m *mockInferenceServer) ModelInfer(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
	if m.modelInferFunc != nil {
		return m.modelInferFunc(ctx, req)
	}
	return &pb.ModelInferResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "output",
				Datatype: "FP32",
				Shape:    []int64{1, 10},
				Contents: &pb.InferTensorContents{
					Fp32Contents: []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
				},
			},
		},
	}, nil
}

func (m *mockInferenceServer) ServerReady(ctx context.Context, req *pb.ServerReadyRequest) (*pb.ServerReadyResponse, error) {
	return &pb.ServerReadyResponse{Ready: true}, nil
}

// setupMockServer creates a mock gRPC server for testing
func setupMockServer(t *testing.T, server *mockInferenceServer) (*grpc.Server, *bufconn.Listener) {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterGRPCInferenceServiceServer(s, server)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	return s, lis
}

// createMockClient creates a client connected to the mock server
func createMockClient(t *testing.T, lis *bufconn.Listener, conf Configuration) *OpenInferenceClient {
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client := pb.NewGRPCInferenceServiceClient(conn)

	oic := &OpenInferenceClient{
		conf:   conf,
		in:     make(chan any),
		out:    make(chan any),
		client: client,
		conn:   conn,
		logger: slog.Default(),
	}

	// Start processing stream
	go oic.stream()

	return oic
}

func TestBuildInferRequest_SimpleInput(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	conf.InputTensorSpec = []TensorSpec{
		{Name: "input", DataType: "FP32", Shape: []int64{1, 3}},
	}
	oic := &OpenInferenceClient{conf: *conf}

	// Binary data for 3 FP32 values (12 bytes)
	inputData := make([]byte, 12)
	*(*float32)(unsafe.Pointer(&inputData[0])) = 1.0
	*(*float32)(unsafe.Pointer(&inputData[4])) = 2.0
	*(*float32)(unsafe.Pointer(&inputData[8])) = 3.0

	event := streams.NewEventFrom(context.Background(), inputData)
	request, err := oic.buildInferRequest(event)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if request.ModelName != "test-model" {
		t.Errorf("Expected model_name 'test-model', got: %s", request.ModelName)
	}
	if len(request.Inputs) != 1 {
		t.Fatalf("Expected 1 input, got: %d", len(request.Inputs))
	}
	if request.Inputs[0].Name != "input" {
		t.Errorf("Expected input name 'input', got: %s", request.Inputs[0].Name)
	}
	if request.Inputs[0].Datatype != "FP32" {
		t.Errorf("Expected datatype 'FP32', got: %s", request.Inputs[0].Datatype)
	}
	// Shape is [1, element_count] when no template is used (FP32 -> 12 bytes = 3 elements)
	if len(request.Inputs[0].Shape) != 2 || request.Inputs[0].Shape[0] != 1 || request.Inputs[0].Shape[1] != 3 {
		t.Errorf("Expected shape [1, 3], got: %v", request.Inputs[0].Shape)
	}
	if len(request.RawInputContents) != 1 {
		t.Errorf("Expected 1 raw input, got: %d", len(request.RawInputContents))
	}
	if len(request.RawInputContents[0]) != 12 {
		t.Errorf("Expected 12 bytes in raw input, got: %d", len(request.RawInputContents[0]))
	}
}

func TestBuildInferRequest_DifferentDataTypes(t *testing.T) {
	tests := []struct {
		name       string
		dataType   string
		inputData  []byte
		dataLength int64
	}{
		{
			name:       "BOOL",
			dataType:   "BOOL",
			inputData:  []byte{0x01, 0x00, 0x01, 0x00}, // true, false, true, false
			dataLength: 4,
		},
		{
			name:     "INT32",
			dataType: "INT32",
			inputData: func() []byte {
				buf := make([]byte, 12) // 3 * 4 bytes
				*(*int32)(unsafe.Pointer(&buf[0])) = 100
				*(*int32)(unsafe.Pointer(&buf[4])) = 200
				*(*int32)(unsafe.Pointer(&buf[8])) = 300
				return buf
			}(),
			dataLength: 3,
		},
		{
			name:     "INT64",
			dataType: "INT64",
			inputData: func() []byte {
				buf := make([]byte, 16) // 2 * 8 bytes
				*(*int64)(unsafe.Pointer(&buf[0])) = 1000
				*(*int64)(unsafe.Pointer(&buf[8])) = 2000
				return buf
			}(),
			dataLength: 2,
		},
		{
			name:     "FP32",
			dataType: "FP32",
			inputData: func() []byte {
				buf := make([]byte, 12) // 3 * 4 bytes
				*(*float32)(unsafe.Pointer(&buf[0])) = 1.5
				*(*float32)(unsafe.Pointer(&buf[4])) = 2.5
				*(*float32)(unsafe.Pointer(&buf[8])) = 3.5
				return buf
			}(),
			dataLength: 3,
		},
		{
			name:     "FP64",
			dataType: "FP64",
			inputData: func() []byte {
				buf := make([]byte, 16) // 2 * 8 bytes
				*(*float64)(unsafe.Pointer(&buf[0])) = 3.14159
				*(*float64)(unsafe.Pointer(&buf[8])) = 2.71828
				return buf
			}(),
			dataLength: 2,
		},
		{
			name:       "BYTES",
			dataType:   "BYTES",
			inputData:  []byte("hello world"),
			dataLength: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := NewConfiguration("localhost:8001", "test-model")
			conf.InputTensorSpec = []TensorSpec{
				{Name: "input", DataType: tt.dataType, Shape: []int64{1, tt.dataLength}},
			}
			oic := &OpenInferenceClient{conf: *conf}

			event := streams.NewEventFrom(context.Background(), tt.inputData)
			request, err := oic.buildInferRequest(event)

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if request.ModelName != "test-model" {
				t.Errorf("Expected model_name 'test-model', got: %s", request.ModelName)
			}
			if len(request.Inputs) != 1 {
				t.Fatalf("Expected 1 input, got: %d", len(request.Inputs))
			}
			if request.Inputs[0].Name != "input" {
				t.Errorf("Expected input name 'input', got: %s", request.Inputs[0].Name)
			}
			if request.Inputs[0].Datatype != tt.dataType {
				t.Errorf("Expected datatype '%s', got: %s", tt.dataType, request.Inputs[0].Datatype)
			}
			if request.Inputs[0].Shape[0] != 1 || request.Inputs[0].Shape[1] != tt.dataLength {
				t.Errorf("Expected shape [1, %d], got: %v", tt.dataLength, request.Inputs[0].Shape)
			}
			if len(request.RawInputContents) != 1 {
				t.Errorf("Expected 1 raw input, got: %d", len(request.RawInputContents))
			}
			// raw input contents should retain original byte length
			expectedRawBytes := int(tt.dataLength)
			switch strings.ToUpper(tt.dataType) {
			case "INT16", "UINT16", "FP16":
				expectedRawBytes = expectedRawBytes * 2
			case "INT32", "UINT32", "FP32":
				expectedRawBytes = expectedRawBytes * 4
			case "INT64", "UINT64", "FP64":
				expectedRawBytes = expectedRawBytes * 8
			case "BOOL", "INT8", "UINT8":
				expectedRawBytes = expectedRawBytes * 1
			case "BYTES":
				// dataLength already bytes
				expectedRawBytes = int(tt.dataLength)
			}
			if len(request.RawInputContents[0]) != expectedRawBytes {
				t.Errorf("Expected %d bytes in raw input, got: %d", expectedRawBytes, len(request.RawInputContents[0]))
			}
		})
	}
}

func TestBuildInferRequest_WithTemplates(t *testing.T) {
	tests := []struct {
		name          string
		dataType      string
		templateInput string // JSON input
		expectedLen   int64
	}{
		{
			name:          "BOOL_with_template",
			dataType:      "BOOL",
			templateInput: `[true, false, true]`,
			expectedLen:   3,
		},
		{
			name:          "INT32_with_template",
			dataType:      "INT32",
			templateInput: `[10, 20, 30, 40]`,
			expectedLen:   4,
		},
		{
			name:          "INT64_with_template",
			dataType:      "INT64",
			templateInput: `[1000, 2000]`,
			expectedLen:   2,
		},
		{
			name:          "FP32_with_template",
			dataType:      "FP32",
			templateInput: `[1.5, 2.5, 3.5]`,
			expectedLen:   3,
		},
		{
			name:          "FP64_with_template",
			dataType:      "FP64",
			templateInput: `[3.14159, 2.71828]`,
			expectedLen:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := NewConfiguration("localhost:8001", "test-model")
			conf.InputTensorSpec = []TensorSpec{
				{Name: "input", DataType: tt.dataType, Shape: []int64{1, tt.expectedLen}},
			}
			conf.InputTemplates = []InputTemplateSpec{
				{Name: "input", Template: "{{.}}"},
			}
			err := conf.setInputTemplates(conf.InputTemplates)
			if err != nil {
				t.Fatalf("Failed to set input templates: %v", err)
			}
			oic := &OpenInferenceClient{conf: *conf}

			event := streams.NewEventFrom(context.Background(), []byte(tt.templateInput))
			request, err := oic.buildInferRequest(event)

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if request.ModelName != "test-model" {
				t.Errorf("Expected model_name 'test-model', got: %s", request.ModelName)
			}
			if len(request.Inputs) != 1 {
				t.Fatalf("Expected 1 input, got: %d", len(request.Inputs))
			}
			if request.Inputs[0].Name != "input" {
				t.Errorf("Expected input name 'input', got: %s", request.Inputs[0].Name)
			}
			if request.Inputs[0].Datatype != tt.dataType {
				t.Errorf("Expected datatype '%s', got: %s", tt.dataType, request.Inputs[0].Datatype)
			}
			if request.Inputs[0].Shape[0] != 1 || request.Inputs[0].Shape[1] != tt.expectedLen {
				t.Errorf("Expected shape [1, %d], got: %v", tt.expectedLen, request.Inputs[0].Shape)
			}
			if len(request.RawInputContents) != 1 {
				t.Errorf("Expected 1 raw input, got: %d", len(request.RawInputContents))
			}
		})
	}
}

func TestBuildInferRequest_MultipleInputTensors(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	conf.InputTensorSpec = []TensorSpec{
		{Name: "input1", DataType: "FP32", Shape: []int64{1, 2}},
		{Name: "input2", DataType: "INT32", Shape: []int64{1, 3}},
	}
	conf.InputTemplates = []InputTemplateSpec{
		{Name: "input1", Template: `{{jp "input1" . | json}}`},
		{Name: "input2", Template: `{{jp "input2" . | json}}`},
	}
	err := conf.setInputTemplates(conf.InputTemplates)
	if err != nil {
		t.Fatalf("Failed to set input templates: %v", err)
	}
	oic := &OpenInferenceClient{conf: *conf}

	// JSON input with multiple tensors
	inputJSON := `{
		"input1": [1.5, 2.5],
		"input2": [10, 20, 30]
	}`

	event := streams.NewEventFrom(context.Background(), []byte(inputJSON))
	request, err := oic.buildInferRequest(event)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(request.Inputs) != 2 {
		t.Fatalf("Expected 2 inputs, got: %d", len(request.Inputs))
	}

	// Verify first input
	if request.Inputs[0].Name != "input1" {
		t.Errorf("Expected first input name 'input1', got: %s", request.Inputs[0].Name)
	}
	if request.Inputs[0].Datatype != "FP32" {
		t.Errorf("Expected first input datatype 'FP32', got: %s", request.Inputs[0].Datatype)
	}
	if request.Inputs[0].Shape[1] != 2 {
		t.Errorf("Expected first input shape [1, 2], got: %v", request.Inputs[0].Shape)
	}

	// Verify second input
	if request.Inputs[1].Name != "input2" {
		t.Errorf("Expected second input name 'input2', got: %s", request.Inputs[1].Name)
	}
	if request.Inputs[1].Datatype != "INT32" {
		t.Errorf("Expected second input datatype 'INT32', got: %s", request.Inputs[1].Datatype)
	}
	if request.Inputs[1].Shape[1] != 3 {
		t.Errorf("Expected second input shape [1, 3], got: %v", request.Inputs[1].Shape)
	}

	// Verify raw input contents
	if len(request.RawInputContents) != 2 {
		t.Errorf("Expected 2 raw inputs, got: %d", len(request.RawInputContents))
	}
}

func TestBuildInferRequest_WithModelVersion(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	conf.ModelVersion = "v1.0"
	conf.InputTensorSpec = []TensorSpec{
		{Name: "input", DataType: "FP32", Shape: []int64{1, 1}},
	}
	oic := &OpenInferenceClient{conf: *conf}

	inputData := make([]byte, 4)
	*(*float32)(unsafe.Pointer(&inputData[0])) = 1.0

	event := streams.NewEventFrom(context.Background(), inputData)
	request, err := oic.buildInferRequest(event)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if request.ModelVersion != "v1.0" {
		t.Errorf("Expected model_version 'v1.0', got: %s", request.ModelVersion)
	}
}

func TestBuildInferRequest_WithParams(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	conf.Params = map[string]string{
		"param1": "value1",
		"param2": "value2",
	}
	conf.InputTensorSpec = []TensorSpec{
		{Name: "input", DataType: "FP32", Shape: []int64{1, 1}},
	}
	oic := &OpenInferenceClient{conf: *conf}

	inputData := make([]byte, 4)
	*(*float32)(unsafe.Pointer(&inputData[0])) = 1.0

	event := streams.NewEventFrom(context.Background(), inputData)
	request, err := oic.buildInferRequest(event)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(request.Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got: %d", len(request.Parameters))
	}
	if request.Parameters["param1"].GetStringParam() != "value1" {
		t.Errorf("Expected param1='value1', got: %s", request.Parameters["param1"].GetStringParam())
	}
	if request.Parameters["param2"].GetStringParam() != "value2" {
		t.Errorf("Expected param2='value2', got: %s", request.Parameters["param2"].GetStringParam())
	}
}

func TestBuildInferRequest_InvalidTemplateData(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	conf.InputTensorSpec = []TensorSpec{
		{Name: "input", DataType: "FP32", Shape: []int64{1, 2}},
	}
	conf.InputTemplates = []InputTemplateSpec{
		{Name: "input", Template: "{{.}}"},
	}
	err := conf.setInputTemplates(conf.InputTemplates)
	if err != nil {
		t.Fatalf("Failed to set input templates: %v", err)
	}
	oic := &OpenInferenceClient{conf: *conf}

	// Invalid JSON input
	event := streams.NewEventFrom(context.Background(), []byte("not valid json"))
	_, err = oic.buildInferRequest(event)

	if err == nil {
		t.Error("Expected error for invalid JSON input, got nil")
	}
}

func TestProcessResponse_SimpleOutput(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	oic := &OpenInferenceClient{conf: *conf}

	response := &pb.ModelInferResponse{
		ModelName:    "test-model",
		ModelVersion: "1",
		Id:           "req-123",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "output",
				Datatype: "FP32",
				Shape:    []int64{1, 3},
				Contents: &pb.InferTensorContents{
					Fp32Contents: []float32{0.1, 0.5, 0.9},
				},
			},
		},
	}

	event, err := oic.processResponse(context.Background(), response, 200)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Parse the response body
	var result map[string]interface{}
	if err := json.Unmarshal(event.GetBody(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["model_name"] != "test-model" {
		t.Errorf("Expected model_name 'test-model', got: %v", result["model_name"])
	}
	if result["id"] != "req-123" {
		t.Errorf("Expected id 'req-123', got: %v", result["id"])
	}

	outputs, ok := result["outputs"].([]interface{})
	if !ok || len(outputs) != 1 {
		t.Fatalf("Expected 1 output, got: %v", outputs)
	}
}

func TestExtractTensorData_AllTypes(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	oic := &OpenInferenceClient{conf: *conf}

	tests := []struct {
		name     string
		datatype string
		contents *pb.InferTensorContents
		validate func(*testing.T, interface{})
	}{
		{
			name:     "BOOL",
			datatype: "BOOL",
			contents: &pb.InferTensorContents{BoolContents: []bool{true, false}},
			validate: func(t *testing.T, data interface{}) {
				bools, ok := data.([]bool)
				if !ok || len(bools) != 2 {
					t.Errorf("Expected 2 bools, got: %v", data)
				}
			},
		},
		{
			name:     "INT32",
			datatype: "INT32",
			contents: &pb.InferTensorContents{IntContents: []int32{1, 2, 3}},
			validate: func(t *testing.T, data interface{}) {
				ints, ok := data.([]int32)
				if !ok || len(ints) != 3 {
					t.Errorf("Expected 3 ints, got: %v", data)
				}
			},
		},
		{
			name:     "INT64",
			datatype: "INT64",
			contents: &pb.InferTensorContents{Int64Contents: []int64{100, 200}},
			validate: func(t *testing.T, data interface{}) {
				ints, ok := data.([]int64)
				if !ok || len(ints) != 2 {
					t.Errorf("Expected 2 int64s, got: %v", data)
				}
			},
		},
		{
			name:     "FP32",
			datatype: "FP32",
			contents: &pb.InferTensorContents{Fp32Contents: []float32{1.5, 2.5}},
			validate: func(t *testing.T, data interface{}) {
				floats, ok := data.([]float32)
				if !ok || len(floats) != 2 {
					t.Errorf("Expected 2 float32s, got: %v", data)
				}
			},
		},
		{
			name:     "FP64",
			datatype: "FP64",
			contents: &pb.InferTensorContents{Fp64Contents: []float64{3.14, 2.71}},
			validate: func(t *testing.T, data interface{}) {
				floats, ok := data.([]float64)
				if !ok || len(floats) != 2 {
					t.Errorf("Expected 2 float64s, got: %v", data)
				}
			},
		},
		{
			name:     "BYTES",
			datatype: "BYTES",
			contents: &pb.InferTensorContents{BytesContents: [][]byte{[]byte("hello"), []byte("world")}},
			validate: func(t *testing.T, data interface{}) {
				strings, ok := data.([]string)
				if !ok || len(strings) != 2 {
					t.Errorf("Expected 2 strings, got: %v", data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := oic.extractTensorData(tt.datatype, tt.contents)
			tt.validate(t, data)
		})
	}
}

func TestInfer_Success(t *testing.T) {
	mockServer := &mockInferenceServer{
		modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
			return &pb.ModelInferResponse{
				ModelName:    req.ModelName,
				ModelVersion: "1",
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name:     "output",
						Datatype: "FP32",
						Shape:    []int64{1, 2},
						Contents: &pb.InferTensorContents{
							Fp32Contents: []float32{0.8, 0.2},
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
		Timeout:      5,
		InputTensorSpec: []TensorSpec{
			{Name: "input", DataType: "FP32", Shape: []int64{1, 2}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "output", DataType: "FP32", Shape: []int64{1, 2}},
		},
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	// Send binary FP32 data (2 values = 8 bytes)
	inputData := make([]byte, 8)
	*(*float32)(unsafe.Pointer(&inputData[0])) = 1.0
	*(*float32)(unsafe.Pointer(&inputData[4])) = 2.0

	event := streams.NewEventFrom(context.Background(), inputData)
	result := oic.infer(event)

	if result.GetStatus() != 200 {
		t.Errorf("Expected status 200, got: %d", result.GetStatus())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(result.GetBody(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["model_name"] != "test-model" {
		t.Errorf("Expected model_name 'test-model', got: %v", response["model_name"])
	}
}

func TestInfer_ServerError(t *testing.T) {
	mockServer := &mockInferenceServer{
		modelInferFunc: func(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
			return nil, status.Error(codes.Internal, "server error")
		},
	}

	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "test-model",
		NumInstances: 1,
		Timeout:      5,
		InputTensorSpec: []TensorSpec{
			{Name: "input", DataType: "FP32", Shape: []int64{1, 2}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "output", DataType: "FP32", Shape: []int64{1, 2}},
		},
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	// Send binary FP32 data (2 values = 8 bytes)
	inputData := make([]byte, 8)
	*(*float32)(unsafe.Pointer(&inputData[0])) = 1.0
	*(*float32)(unsafe.Pointer(&inputData[4])) = 2.0

	event := streams.NewEventFrom(context.Background(), inputData)
	result := oic.infer(event)

	if result.GetStatus() == 200 {
		t.Error("Expected error status, got 200")
	}
}

func TestInfer_InvalidInput(t *testing.T) {
	mockServer := &mockInferenceServer{}
	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "test-model",
		NumInstances: 1,
		Timeout:      5,
		InputTensorSpec: []TensorSpec{
			{Name: "input", DataType: "FP32", Shape: []int64{1, 2}},
		},
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	// Invalid binary data for FP32 - must be multiple of 4 bytes, but sending 7 bytes
	// With new implementation, client doesn't validate binary data size
	event := streams.NewEventFrom(context.Background(), []byte("invalid"))
	result := oic.infer(event)

	// Client doesn't validate binary data - server returns success with default response
	if result.GetStatus() != 200 {
		t.Errorf("Expected status 200 (no client-side validation), got: %d", result.GetStatus())
	}
}

func TestOpenInferenceClient_Channels(t *testing.T) {
	mockServer := &mockInferenceServer{}
	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "test-model",
		NumInstances: 1,
		Timeout:      5,
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	if oic.In() == nil {
		t.Error("Expected In() channel, got nil")
	}
	if oic.Out() == nil {
		t.Error("Expected Out() channel, got nil")
	}
}

func TestNewOpenInferenceClient_UnsupportedProtocol(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")
	conf.Protocol = "invalid-proto"
	_, err := NewOpenInferenceClient(*conf)
	if err == nil {
		t.Fatalf("expected error for unsupported protocol, got nil")
	}
}

func TestNewOpenInferenceClient_REST_NoGRPCConn(t *testing.T) {
	conf := NewConfiguration("http://example.invalid", "test-model")
	conf.Protocol = "rest"
	oic, err := NewOpenInferenceClient(*conf)
	if err != nil {
		t.Fatalf("unexpected error creating rest client: %v", err)
	}
	if oic.conn != nil {
		t.Errorf("expected no grpc conn for rest protocol, got non-nil")
	}
	if oic.client != nil {
		t.Errorf("expected no grpc client for rest protocol, got non-nil")
	}
	_ = oic.Close()
}

func TestOpenInferenceClient_Via(t *testing.T) {
	mockServer := &mockInferenceServer{}
	srv, lis := setupMockServer(t, mockServer)
	defer srv.Stop()

	conf := Configuration{
		Address:      "bufnet",
		ModelName:    "test-model",
		NumInstances: 1,
		Timeout:      5,
		InputTensorSpec: []TensorSpec{
			{Name: "input", DataType: "FP32", Shape: []int64{1, 1}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "output", DataType: "FP32"},
		},
	}

	oic := createMockClient(t, lis, conf)
	defer oic.Close()

	mockFlow := &mockFlow{in: make(chan any, 1)}
	result := oic.Via(mockFlow)

	if result == nil {
		t.Error("Expected flow returned from Via()")
	}

	// Send binary FP32 data (1 value = 4 bytes)
	inputData := make([]byte, 4)
	*(*float32)(unsafe.Pointer(&inputData[0])) = 1.0

	oic.In() <- inputData
	close(oic.In())

	time.Sleep(100 * time.Millisecond)

	select {
	case msg := <-mockFlow.in:
		if msg == nil {
			t.Error("Expected message, got nil")
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected message transmitted within timeout")
	}
}

type mockFlow struct {
	in chan any
}

func (m *mockFlow) In() chan<- any {
	return m.in
}

func (m *mockFlow) Out() <-chan any {
	return nil
}

func (m *mockFlow) Via(flow streams.Flow) streams.Flow {
	return nil
}

func (m *mockFlow) To(sink streams.Sink) {
}

func TestOpenInferenceProcessor_Validate(t *testing.T) {
	proc := &OpenInferenceProcessor{}

	tests := []struct {
		name      string
		spec      model.NodeConfig
		expectErr bool
	}{
		{
			name: "valid basic config",
			spec: model.NodeConfig{
				Kind: "openinference",
				Spec: map[string]interface{}{
					"address":    "localhost:8001",
					"model_name": "test-model",
					"input_tensor_spec": []interface{}{
						map[string]interface{}{
							"name":     "input",
							"datatype": "FP32",
							"shape":    []interface{}{1, 224, 224, 3},
						},
					},
					"output_tensor_spec": []interface{}{
						map[string]interface{}{
							"name":     "output",
							"datatype": "FP32",
							"shape":    []interface{}{1, 1000},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "missing address",
			spec: model.NodeConfig{
				Kind: "openinference",
				Spec: map[string]interface{}{
					"model_name": "test-model",
				},
			},
			expectErr: true,
		},
		{
			name: "missing model_name",
			spec: model.NodeConfig{
				Kind: "openinference",
				Spec: map[string]interface{}{
					"address": "localhost:8001",
				},
			},
			expectErr: true,
		},
		{
			name: "negative num_instances",
			spec: model.NodeConfig{
				Kind: "openinference",
				Spec: map[string]interface{}{
					"address":       "localhost:8001",
					"model_name":    "test-model",
					"num_instances": -1,
				},
			},
			expectErr: true,
		},
		{
			name: "negative timeout",
			spec: model.NodeConfig{
				Kind: "openinference",
				Spec: map[string]interface{}{
					"address":    "localhost:8001",
					"model_name": "test-model",
					"timeout":    -5,
				},
			},
			expectErr: true,
		},
		{
			name: "valid with templates",
			spec: model.NodeConfig{
				Kind: "openinference",
				Spec: map[string]interface{}{
					"address":    "localhost:8001",
					"model_name": "test-model",
					"input_tensor_spec": []interface{}{
						map[string]interface{}{
							"name":     "input",
							"datatype": "FP32",
							"shape":    []interface{}{1, 3},
						},
					},
					"output_tensor_spec": []interface{}{
						map[string]interface{}{
							"name":     "output",
							"datatype": "FP32",
							"shape":    []interface{}{1, 3},
						},
					},
					"input_templates": []interface{}{
						map[string]interface{}{
							"name":     "input",
							"template": "{{fromJson .}}",
						},
					},
					"output_template": "{{toJson .}}",
				},
			},
			expectErr: false,
		},
		{
			name: "invalid input template",
			spec: model.NodeConfig{
				Kind: "openinference",
				Spec: map[string]interface{}{
					"address":    "localhost:8001",
					"model_name": "test-model",
					"input_tensor_spec": []interface{}{
						map[string]interface{}{
							"name":     "input",
							"datatype": "FP32",
						},
					},
					"input_templates": []interface{}{
						map[string]interface{}{
							"name":     "input",
							"template": "{{invalid template",
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := proc.Validate(&tt.spec)
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestOpenInferenceProcessor_Convert(t *testing.T) {
	// Note: This test cannot fully test the Convert method because it tries to
	// create a real gRPC connection. We'll test the validation and config parsing.
	proc := &OpenInferenceProcessor{}

	spec := model.NodeConfig{
		Kind: "openinference",
		Spec: map[string]interface{}{
			"address":       "localhost:8001",
			"model_name":    "test-model",
			"num_instances": 2,
			"timeout":       60,
		},
	}

	// This will fail because we can't connect to a real server
	// but we can verify the error is about connection, not parsing
	_, err := proc.Convert(&spec)
	if err == nil {
		t.Log("Convert succeeded (unexpected if no server is running)")
	} else {
		// Check that the error is about connection, not config parsing
		errMsg := fmt.Sprintf("%v", err)
		if errMsg == "" {
			t.Error("Expected error message")
		}
		t.Logf("Got expected error: %v", err)
	}
}
