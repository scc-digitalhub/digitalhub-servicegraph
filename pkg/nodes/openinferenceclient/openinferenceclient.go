// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package openinferenceclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/proto/inference/v2"
)

// OpenInferenceClient is a gRPC client for Open Inference Protocol v2
type OpenInferenceClient struct {
	conf   Configuration
	in     chan any
	out    chan any
	client pb.GRPCInferenceServiceClient
	conn   *grpc.ClientConn
}

// Verify OpenInferenceClient satisfies the Flow interface
var _ streams.Flow = (*OpenInferenceClient)(nil)

// NewOpenInferenceClient creates a new OpenInferenceClient operator
func NewOpenInferenceClient(conf Configuration) (*OpenInferenceClient, error) {
	// Ensure protocol default
	if conf.Protocol == "" {
		conf.Protocol = "grpc"
	}

	proto := strings.ToLower(conf.Protocol)
	if proto == "grpc" {
		// Create gRPC connection
		conn, err := createGRPCConnection(conf)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
		}

		// Create gRPC client
		client := pb.NewGRPCInferenceServiceClient(conn)

		oic := &OpenInferenceClient{
			conf:   conf,
			in:     make(chan any),
			out:    make(chan any),
			client: client,
			conn:   conn,
		}

		// Start processing stream elements
		go oic.stream()

		return oic, nil
	}

	if proto == "rest" {
		oic := &OpenInferenceClient{
			conf: conf,
			in:   make(chan any),
			out:  make(chan any),
		}
		go oic.stream()
		return oic, nil
	}

	return nil, fmt.Errorf("unsupported protocol: %s", conf.Protocol)
}

// Via asynchronously streams data to the given Flow and returns it
func (oic *OpenInferenceClient) Via(flow streams.Flow) streams.Flow {
	go oic.transmit(flow)
	return flow
}

// To streams data to the given Sink and blocks until completion
func (oic *OpenInferenceClient) To(sink streams.Sink) {
	oic.transmit(sink)
	sink.AwaitCompletion()
}

// Out returns the output channel
func (oic *OpenInferenceClient) Out() <-chan any {
	return oic.out
}

// In returns the input channel
func (oic *OpenInferenceClient) In() chan<- any {
	return oic.in
}

// Close closes the gRPC connection
func (oic *OpenInferenceClient) Close() error {
	if oic.conn != nil {
		return oic.conn.Close()
	}
	return nil
}

func (oic *OpenInferenceClient) transmit(inlet streams.Inlet) {
	for element := range oic.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}

// stream processes elements from input channel
func (oic *OpenInferenceClient) stream() {
	var wg sync.WaitGroup
	// Create a pool of worker goroutines
	for i := 0; i < oic.conf.NumInstances; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for element := range oic.in {
				ctx := streams.ExtractContext(element)
				oic.out <- oic.infer(streams.NewEventFrom(ctx, element))
			}
		}()
	}

	// Wait for worker goroutines to finish
	wg.Wait()
	// Close the output channel
	close(oic.out)
	// Close gRPC connection
	_ = oic.Close()
}

// infer performs inference using the gRPC client
func (oic *OpenInferenceClient) infer(msg streams.Event) streams.Event {
	rootContext := msg.GetContext()

	tr := otel.Tracer("openinference/client")
	proto := strings.ToLower(oic.conf.Protocol)

	if proto == "rest" {
		_, span := tr.Start(rootContext, "OpenInference REST Client Node")
		defer span.End()

		// Build inference request from input
		request, err := oic.buildInferRequest(msg)
		if err != nil {
			return streams.NewErrorEvent(rootContext, fmt.Errorf("failed to build request: %w", err), 500)
		}

		// Build REST JSON request according to Open Inference REST spec
		restReq := buildRESTRequestFromProto(request)
		reqJSON, err := json.Marshal(restReq)
		if err != nil {
			return streams.NewErrorEvent(rootContext, fmt.Errorf("failed to marshal rest request: %w", err), 500)
		}

		// Prepare URL
		address := oic.conf.Address
		if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
			address = "http://" + address
		}
		url := strings.TrimRight(address, "/") + fmt.Sprintf("/v2/models/%s/infer", oic.conf.ModelName)

		// HTTP POST
		httpClient := &http.Client{Timeout: time.Duration(oic.conf.Timeout) * time.Second}
		resp, err := httpClient.Post(url, "application/json", bytes.NewReader(reqJSON))
		if err != nil {
			return streams.NewErrorEvent(rootContext, fmt.Errorf("rest inference failed: %w", err), 500)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return streams.NewErrorEvent(rootContext, fmt.Errorf("rest inference returned status %d: %s", resp.StatusCode, string(b)), 500)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return streams.NewErrorEvent(rootContext, fmt.Errorf("failed to read rest response: %w", err), 500)
		}

		// Unmarshal into REST response model
		var restResp RESTInferResponse
		if err := json.Unmarshal(body, &restResp); err != nil {
			return streams.NewErrorEvent(rootContext, fmt.Errorf("failed to unmarshal rest response: %w", err), 500)
		}

		// Convert REST response to protobuf ModelInferResponse
		response := convertRESTResponseToProto(&restResp)

		// Process response
		result, err := oic.processResponse(rootContext, response)
		if err != nil {
			return streams.NewErrorEvent(rootContext, fmt.Errorf("failed to process response: %w", err), 500)
		}
		return result
	}

	// Default: gRPC
	_, span := tr.Start(rootContext, "OpenInference gRPC Client Node")
	defer span.End()

	// Set timeout context
	timeout := time.Duration(oic.conf.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build inference request from input
	request, err := oic.buildInferRequest(msg)
	if err != nil {
		return streams.NewErrorEvent(rootContext, fmt.Errorf("failed to build request: %w", err), 500)
	}

	// Perform inference
	response, err := oic.client.ModelInfer(ctx, request)
	if err != nil {
		return streams.NewErrorEvent(rootContext, fmt.Errorf("inference failed: %w", err), 500)
	}

	// Process response
	result, err := oic.processResponse(rootContext, response)
	if err != nil {
		return streams.NewErrorEvent(rootContext, fmt.Errorf("failed to process response: %w", err), 500)
	}

	return result
}

// buildInferRequest builds a ModelInferRequest from the input event
func (oic *OpenInferenceClient) buildInferRequest(msg streams.Event) (*pb.ModelInferRequest, error) {
	// Process input through template if specified
	mapData := make(map[string][]byte)
	mapLen := make(map[string]int64)
	// convert each single tensor
	if len(oic.conf.inputTemplateObjMap) > 0 {
		for _, tensorSpec := range oic.conf.InputTensorSpec {
			if oic.conf.inputTemplateObjMap[tensorSpec.Name] != nil {
				// data is a json representation of the input tensors, generated by applying the template to the input event
				data, err := util.ConvertBody(msg.GetBody(), oic.conf.inputTemplateObjMap[tensorSpec.Name])
				if err != nil {
					return nil, fmt.Errorf("failed to process input template '%s': %w", tensorSpec.Name, err)
				}
				data, length, err := util.ConvertTensorData(data, tensorSpec.DataType)
				if err != nil {
					return nil, fmt.Errorf("failed to convert tensor data for '%s': %w", tensorSpec.Name, err)
				}
				mapData[tensorSpec.Name] = data
				mapLen[tensorSpec.Name] = length
			}
		}
		// no templates: consider a single tensor using raw body as input
	} else {
		// No templates: use raw body. Determine element count based on datatype
		raw := msg.GetBody()
		name := oic.conf.InputTensorSpec[0].Name
		mapData[name] = raw
		// compute element count
		elemCount := computeElementCount(oic.conf.InputTensorSpec[0].DataType, len(raw))
		mapLen[name] = elemCount
	}

	// Build the inference request
	request := &pb.ModelInferRequest{
		ModelName:    oic.conf.ModelName,
		ModelVersion: oic.conf.ModelVersion,
	}

	// Add parameters if specified
	if oic.conf.Params != nil {
		request.Parameters = make(map[string]*pb.InferParameter)
		for key, value := range oic.conf.Params {
			param := &pb.InferParameter{
				ParameterChoice: &pb.InferParameter_StringParam{
					StringParam: value,
				},
			}
			request.Parameters[key] = param
		}
	}

	// Extract inputs from the tensor request
	for _, inputSpec := range oic.conf.InputTensorSpec {
		tensorInput, err := oic.buildInferInputTensor(mapLen[inputSpec.Name], inputSpec)
		if err != nil {
			return nil, err
		}
		request.Inputs = append(request.Inputs, tensorInput)
	}
	rawData := make([][]byte, len(oic.conf.InputTensorSpec))
	for i, inputSpec := range oic.conf.InputTensorSpec {
		rawData[i] = mapData[inputSpec.Name]
	}
	request.RawInputContents = rawData

	return request, nil
}

// buildInferInputTensor builds an InferInputTensor from a map
func (oic *OpenInferenceClient) buildInferInputTensor(length int64, inputSpec TensorSpec) (*pb.ModelInferRequest_InferInputTensor, error) {
	tensor := &pb.ModelInferRequest_InferInputTensor{
		Name:     inputSpec.Name,
		Datatype: inputSpec.DataType,
		Shape:    []int64{1, length},
	}

	// tensorContents := &pb.InferTensorContents{}
	// contents, err := util.BytesToTensor(tensor.Datatype, data)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to convert bytes to tensor: %w", err)
	// }

	// var length = 0

	// // Determine content type based on datatype
	// switch tensor.Datatype {
	// case "BOOL":
	// 	tensorContents.BoolContents = contents.([]bool)
	// 	length = len(tensorContents.BoolContents)
	// case "INT8":
	// 	for _, v := range contents.([]int) {
	// 		tensorContents.IntContents = append(tensorContents.IntContents, int32(v))
	// 	}
	// 	length = len(tensorContents.IntContents)
	// case "INT16":
	// 	for _, v := range contents.([]int16) {
	// 		tensorContents.IntContents = append(tensorContents.IntContents, int32(v))
	// 	}
	// 	length = len(tensorContents.IntContents)
	// case "INT32":
	// 	for _, v := range contents.([]int32) {
	// 		tensorContents.IntContents = append(tensorContents.IntContents, int32(v))
	// 	}
	// 	length = len(tensorContents.IntContents)
	// case "INT64":
	// 	for _, v := range contents.([]int64) {
	// 		tensorContents.Int64Contents = append(tensorContents.Int64Contents, v)
	// 	}
	// 	length = len(tensorContents.Int64Contents)
	// case "UINT8":
	// 	for _, v := range contents.([]int) {
	// 		tensorContents.UintContents = append(tensorContents.UintContents, uint32(v))
	// 	}
	// 	length = len(tensorContents.UintContents)
	// case "UINT16":
	// 	for _, v := range contents.([]uint16) {
	// 		tensorContents.UintContents = append(tensorContents.UintContents, uint32(v))
	// 	}
	// 	length = len(tensorContents.UintContents)
	// case "UINT32":
	// 	for _, v := range contents.([]uint32) {
	// 		tensorContents.UintContents = append(tensorContents.UintContents, uint32(v))
	// 	}
	// 	length = len(tensorContents.UintContents)
	// case "UINT64":
	// 	for _, v := range contents.([]uint64) {
	// 		tensorContents.Uint64Contents = append(tensorContents.Uint64Contents, v)
	// 	}
	// 	length = len(tensorContents.Uint64Contents)
	// case "FP16", "FP32":
	// 	for _, v := range contents.([]float32) {
	// 		tensorContents.Fp32Contents = append(tensorContents.Fp32Contents, v)
	// 	}
	// 	length = len(tensorContents.Fp32Contents)
	// case "FP64":
	// 	for _, v := range contents.([]float64) {
	// 		tensorContents.Fp64Contents = append(tensorContents.Fp64Contents, v)
	// 	}
	// 	length = len(tensorContents.Fp64Contents)
	// case "BYTES":
	// 	cont := make([]byte, 0)
	// 	for _, v := range contents.([]int) {
	// 		cont = append(cont, byte(v))
	// 	}
	// 	tensorContents.BytesContents = [][]byte{cont}
	// 	length = len(cont)
	// }

	// tensor.Shape = []int64{1, int64(length)}

	// tensor.Contents = tensorContents
	return tensor, nil
}

// computeElementCount returns number of elements for the given dataType and raw byte length.
func computeElementCount(dataType string, byteLen int) int64 {
	switch strings.ToUpper(dataType) {
	case "BOOL", "INT8", "UINT8":
		return int64(byteLen)
	case "INT16", "UINT16", "FP16":
		if byteLen%2 != 0 {
			return int64(byteLen) // fallback to bytes
		}
		return int64(byteLen / 2)
	case "INT32", "UINT32", "FP32":
		if byteLen%4 != 0 {
			return int64(byteLen)
		}
		return int64(byteLen / 4)
	case "INT64", "UINT64", "FP64":
		if byteLen%8 != 0 {
			return int64(byteLen)
		}
		return int64(byteLen / 8)
	case "BYTES":
		// BYTES is treated as raw bytes length
		return int64(byteLen)
	default:
		return int64(byteLen)
	}
}

// REST request/response models (simplified per Open Inference REST spec)
type RESTInferInput struct {
	Name     string  `json:"name"`
	Datatype string  `json:"datatype"`
	Shape    []int64 `json:"shape,omitempty"`
	// Data contains the typed values for the input (arrays or base64 strings for BYTES)
	Data []interface{} `json:"data,omitempty"`
}

type RESTInferRequest struct {
	ModelName    string            `json:"model_name"`
	ModelVersion string            `json:"model_version,omitempty"`
	Parameters   map[string]string `json:"parameters,omitempty"`
	Inputs       []RESTInferInput  `json:"inputs,omitempty"`
}

type RESTInferOutput struct {
	Name              string        `json:"name"`
	Datatype          string        `json:"datatype,omitempty"`
	Shape             []int64       `json:"shape,omitempty"`
	Data              []interface{} `json:"data,omitempty"`
	RawOutputContents []string      `json:"raw_output_contents,omitempty"`
}

type RESTInferResponse struct {
	ModelName    string            `json:"model_name,omitempty"`
	ModelVersion string            `json:"model_version,omitempty"`
	Id           string            `json:"id,omitempty"`
	Outputs      []RESTInferOutput `json:"outputs,omitempty"`
}

// buildRESTRequestFromProto builds a REST request using the proto request as a source.
func buildRESTRequestFromProto(req *pb.ModelInferRequest) *RESTInferRequest {
	r := &RESTInferRequest{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Parameters:   make(map[string]string),
	}
	for k, v := range req.Parameters {
		if v != nil {
			if s := v.GetStringParam(); s != "" {
				r.Parameters[k] = s
			}
		}
	}

	// Inputs: include name/datatype/shape and populate Data from RawInputContents when available
	for i, in := range req.Inputs {
		input := RESTInferInput{
			Name:     in.Name,
			Datatype: in.Datatype,
			Shape:    in.Shape,
		}

		// If raw input bytes were provided, try to convert them to typed arrays
		if i < len(req.RawInputContents) {
			raw := req.RawInputContents[i]
			// For BYTES, send base64 string in data array
			if strings.ToUpper(in.Datatype) == "BYTES" {
				input.Data = []interface{}{base64.StdEncoding.EncodeToString(raw)}
			} else {
				// Convert bytes to typed tensor values
				if v, err := util.BytesToTensor(in.Datatype, raw); err == nil {
					input.Data = convertToInterfaceSlice(v)
				} else {
					// fallback: send base64
					input.Data = []interface{}{base64.StdEncoding.EncodeToString(raw)}
				}
			}
		}

		r.Inputs = append(r.Inputs, input)
	}

	return r
}

// convertRESTResponseToProto converts REST response into a protobuf ModelInferResponse
func convertRESTResponseToProto(rest *RESTInferResponse) *pb.ModelInferResponse {
	resp := &pb.ModelInferResponse{
		ModelName:    rest.ModelName,
		ModelVersion: rest.ModelVersion,
		Id:           rest.Id,
	}

	for _, out := range rest.Outputs {
		o := &pb.ModelInferResponse_InferOutputTensor{
			Name:     out.Name,
			Datatype: out.Datatype,
			Shape:    out.Shape,
		}

		// Prefer typed Data if present
		if len(out.Data) > 0 {
			// Convert interface slice to appropriate typed contents based on datatype
			contents := &pb.InferTensorContents{}
			switch strings.ToUpper(o.Datatype) {
			case "FP32":
				fp := make([]float32, len(out.Data))
				for i, v := range out.Data {
					// JSON numbers are float64 by default
					if num, ok := v.(float64); ok {
						fp[i] = float32(num)
					}
				}
				contents.Fp32Contents = fp
			case "FP64":
				fp := make([]float64, len(out.Data))
				for i, v := range out.Data {
					if num, ok := v.(float64); ok {
						fp[i] = num
					}
				}
				contents.Fp64Contents = fp
			case "INT32", "INT16", "INT8":
				ints := make([]int32, len(out.Data))
				for i, v := range out.Data {
					if num, ok := v.(float64); ok {
						ints[i] = int32(num)
					}
				}
				contents.IntContents = ints
			case "INT64":
				l := make([]int64, len(out.Data))
				for i, v := range out.Data {
					if num, ok := v.(float64); ok {
						l[i] = int64(num)
					}
				}
				contents.Int64Contents = l
			case "BYTES":
				b := make([][]byte, len(out.Data))
				for i, v := range out.Data {
					if s, ok := v.(string); ok {
						b[i] = []byte(s)
					}
				}
				contents.BytesContents = b
			default:
				// fallback: try to decode as floats
				fp := make([]float32, len(out.Data))
				for i, v := range out.Data {
					if num, ok := v.(float64); ok {
						fp[i] = float32(num)
					}
				}
				contents.Fp32Contents = fp
			}
			o.Contents = contents
		} else if len(out.RawOutputContents) > 0 {
			// Decode base64 raw outputs
			raw := make([][]byte, len(out.RawOutputContents))
			for i, s := range out.RawOutputContents {
				if b, err := base64.StdEncoding.DecodeString(s); err == nil {
					raw[i] = b
				}
			}
			resp.RawOutputContents = append(resp.RawOutputContents, raw...)
		}

		resp.Outputs = append(resp.Outputs, o)
	}

	return resp
}

// convertToInterfaceSlice converts supported slice types to []interface{} for JSON marshalling
func convertToInterfaceSlice(v any) []interface{} {
	switch s := v.(type) {
	case []bool:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []int:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []int16:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []int32:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []int64:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []uint16:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []uint32:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []uint64:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []float32:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = float64(v)
		}
		return out
	case []float64:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	case []string:
		out := make([]interface{}, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	default:
		return nil
	}
}

// processResponse processes the inference response
func (oic *OpenInferenceClient) processResponse(ctx context.Context, response *pb.ModelInferResponse) (streams.Event, error) {
	// Convert response to JSON
	responseData := map[string]any{
		"model_name":    response.ModelName,
		"model_version": response.ModelVersion,
		"id":            response.Id,
		"outputs":       make([]map[string]any, 0),
	}

	// Process output tensors
	for i, output := range response.Outputs {
		outputMap := map[string]any{
			"name":     output.Name,
			"datatype": output.Datatype,
			"shape":    output.Shape,
		}

		// Extract data from contents or raw_output_contents
		if output.Contents != nil {
			outputMap["data"] = oic.extractTensorData(output.Datatype, output.Contents)
		} else if i < len(response.RawOutputContents) {
			// Use raw output contents
			outputMap["data"] = response.RawOutputContents[i]
			d, err := util.BytesToTensor(output.Datatype, response.RawOutputContents[i])
			if err != nil {
				return nil, fmt.Errorf("failed to convert raw output contents: %w", err)
			}
			outputMap["data"] = d
		}

		responseData["outputs"] = append(responseData["outputs"].([]map[string]any), outputMap)
	}

	// Marshal to JSON
	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	// Apply output template if specified
	var finalData []byte
	if oic.conf.outputTemplateObj != nil {
		finalData, err = util.ConvertBody(responseJSON, oic.conf.outputTemplateObj)
		if err != nil {
			return nil, fmt.Errorf("failed to process output template: %w", err)
		}
	} else {
		finalData = responseJSON
	}

	// Create result event
	result, err := streams.NewGenericEventBuilder(ctx).
		WithBody(finalData).
		WithStatus(200).
		Build()
	if err != nil {
		return nil, err
	}

	return result, nil
}

// extractTensorData extracts data from InferTensorContents based on datatype
func (oic *OpenInferenceClient) extractTensorData(datatype string, contents *pb.InferTensorContents) any {
	switch datatype {
	case "BOOL":
		return contents.BoolContents
	case "INT8", "INT16", "INT32":
		return contents.IntContents
	case "INT64":
		return contents.Int64Contents
	case "UINT8", "UINT16", "UINT32":
		return contents.UintContents
	case "UINT64":
		return contents.Uint64Contents
	case "FP32":
		return contents.Fp32Contents
	case "FP64":
		return contents.Fp64Contents
	case "BYTES":
		// convert [][]byte to []string
		strContents := make([]string, len(contents.BytesContents))
		for i, b := range contents.BytesContents {
			strContents[i] = string(b)
		}
		return strContents
	default:
		return nil
	}
}

// createGRPCConnection creates a gRPC connection with the specified configuration
func createGRPCConnection(conf Configuration) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	// Add other options
	opts = append(opts, grpc.WithBlock())

	// Create connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, conf.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	return conn, nil
}

// init registers the OpenInference node processor
func init() {
	nodes.RegistrySingleton.Register("openinference", &OpenInferenceProcessor{})
}

// OpenInferenceProcessor handles node conversion and validation
type OpenInferenceProcessor struct {
	nodes.Converter
	nodes.Validator
}

// Convert creates an OpenInferenceClient from the node configuration
func (p *OpenInferenceProcessor) Convert(spec model.NodeConfig) (streams.Flow, error) {
	// Check cache first
	if cached := spec.ConfigCache(); cached != nil {
		conf := (*cached).(*Configuration)
		client, err := NewOpenInferenceClient(*conf)
		return client, err
	}

	// Parse configuration
	conf := &Configuration{}
	err := util.Convert(spec.Spec, conf)
	if err != nil {
		return nil, err
	}

	// Set templates
	if err := conf.setInputTemplates(conf.InputTemplates); err != nil {
		return nil, err
	}
	if err := conf.setOutputTemplate(conf.OutputTemplate); err != nil {
		return nil, err
	}

	// Cache configuration
	spec.SetConfigCache(conf)

	// Create client
	client, err := NewOpenInferenceClient(*conf)
	return client, err
}

// Validate validates the node configuration
func (p *OpenInferenceProcessor) Validate(spec model.NodeConfig) error {
	conf := &Configuration{}
	err := util.Convert(spec.Spec, conf)
	if err != nil {
		return err
	}

	if conf.Address == "" {
		return errors.New("openinference node requires an address")
	}

	if conf.ModelName == "" {
		return errors.New("openinference node requires a model_name")
	}

	if conf.NumInstances < 0 {
		return errors.New("openinference node requires a non-negative num_instances")
	}

	if conf.Timeout < 0 {
		return errors.New("openinference node requires a non-negative timeout")
	}

	// Validate input tensors
	if conf.InputTensorSpec != nil {
		for _, tensorSpec := range conf.InputTensorSpec {
			if tensorSpec.DataType == "" {
				return errors.New("input tensor spec requires a datatype")
			}
			if len(tensorSpec.Shape) == 0 {
				return errors.New("input tensor spec requires a shape")
			}
		}
	} else {
		return errors.New("input tensor cannot be empty")
	}

	// Validate output tensors
	if conf.OutputTensorSpec != nil {
		for _, tensorSpec := range conf.OutputTensorSpec {
			if tensorSpec.DataType == "" {
				return errors.New("output tensor spec requires a datatype")
			}
			if len(tensorSpec.Shape) == 0 {
				return errors.New("output tensor spec requires a shape")
			}
		}
	} else {
		return errors.New("output tensor cannot be empty")
	}

	// Validate input templates
	if conf.InputTemplates != nil {
		for _, templateSpec := range conf.InputTemplates {
			if templateSpec.Name == "" {
				return errors.New("input template spec requires a name")
			}
			if templateSpec.Template == "" {
				return errors.New("input template spec requires a template")
			}
		}
	}

	return nil
}
