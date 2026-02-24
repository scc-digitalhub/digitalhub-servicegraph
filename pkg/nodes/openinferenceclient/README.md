# Open Inference Client Node

This package implements a gRPC client node for the [Open Inference Protocol v2](https://github.com/kserve/open-inference-protocol), enabling integration with inference servers that support this protocol (e.g., Triton Inference Server, KServe).

## Features

- **gRPC Communication**: Efficient binary protocol for inference requests
- **Tensor Conversion**: Automatic conversion between binary data and inference protocol tensors
- **Template Processing**: Flexible input/output transformation using Go templates
- **Parallel Processing**: Configurable worker pool for concurrent inference requests
- **OpenTelemetry Integration**: Built-in tracing for observability

## Prerequisites

### Proto Code Generation

The client requires generated Go code from the Open Inference Protocol v2 proto files. To generate this code, you need:

1. **Protocol Buffers Compiler (protoc)** version 3.0 or later
   ```bash
   # macOS
   brew install protobuf
   
   # Linux
   apt-get install protobuf-compiler
   
   # Or download from: https://github.com/protocolbuffers/protobuf/releases
   ```

2. **Go plugins for protoc**
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

3. **Generate the code**
   ```bash
   # From repository root
   ./scripts/generate-proto.sh
   ```

**Note**: If you encounter `Unrecognized syntax identifier "proto3"`, your protoc version is too old (< 3.0). Please upgrade to protoc 3.0 or later.

## Configuration

### Basic Configuration

```yaml
nodes:
  - type: "service"
    name: "triton-inference"
    config:
      kind: "openinference"
      spec:
        address: "localhost:8001"           # gRPC server address
        model_name: "my_model"              # Model name on the server
        model_version: "1"                  # Optional model version
        num_instances: 4                    # Worker pool size
        timeout: 30                         # Request timeout in seconds
```

### Advanced Configuration with Templates

```yaml
nodes:
  - type: "service"
    name: "image-classifier"
    config:
      kind: "openinference"
      spec:
        address: "triton-server:8001"
        model_name: "resnet50"
        model_version: "1"
        
        # Input tensor specifications (required)
        # Defines the structure of input tensors
        input_tensor_spec:
          - name: "input"
            datatype: "FP32"
            shape: [1, 3, 224, 224]
        
        # Output tensor specifications (required)
        # Defines the structure of output tensors
        output_tensor_spec:
          - name: "output"
            datatype: "FP32"
            shape: [1, 1000]
        
        # Optional: Input templates for data transformation
        # Transforms incoming data into binary tensor format
        input_templates:
          - name: "input"
            template: |
              {{. | jp "$.image_data"}}
        
        # Optional: Output template for result transformation
        # Extracts and transforms inference results
        output_template: |
          {
            "predictions": {{. | jp "$.outputs[0].data"}},
            "model": "{{. | jp "$.model_name"}}",
            "version": "{{. | jp "$.model_version"}}"
          }
```

## Input/Output Format

### Input Format

The node processes input data using the configured tensor specifications and optional templates:

1. **With templates** (recommended for data transformation):
   - Input data can be in any format (JSON, binary, etc.)
   - `input_templates` transform the data into binary tensors
   - Each template corresponds to a tensor in `input_tensor_spec`
   
   Example input with template:
   ```json
   {
     "image_data": [255, 128, 64, ...]
   }
   ```

2. **Without templates** (direct binary data):
   - Input must be raw binary data matching the tensor specification
   - Data is converted directly to tensors using `input_tensor_spec`

### Output Format

The node outputs inference results as JSON:

```json
{
  "model_name": "my_model",
  "model_version": "1",
  "id": "request-id",
  "outputs": [
    {
      "name": "output",
      "datatype": "FP32",
      "shape": [1, 1000],
      "data": [0.1, 0.2, 0.3, ...]
    }
  ]
}
```

Use `output_template` to transform this into your desired format.

## Data Types

Supported Open Inference Protocol v2 data types:

| Type | Description | Go Type | Example |
|------|-------------|---------|---------|
| BOOL | Boolean | bool | true, false |
| INT8 | 8-bit signed integer | int32 | -128 to 127 |
| INT16 | 16-bit signed integer | int32 | -32768 to 32767 |
| INT32 | 32-bit signed integer | int32 | -2147483648 to 2147483647 |
| INT64 | 64-bit signed integer | int64 | Very large integers |
| UINT8 | 8-bit unsigned integer | uint32 | 0 to 255 |
| UINT16 | 16-bit unsigned integer | uint32 | 0 to 65535 |
| UINT32 | 32-bit unsigned integer | uint32 | 0 to 4294967295 |
| UINT64 | 64-bit unsigned integer | uint64 | Very large positive integers |
| FP16 | 16-bit float (half precision) | N/A (use raw) | Use raw_input_contents |
| FP32 | 32-bit float (single precision) | float32 | 1.234 |
| FP64 | 64-bit float (double precision) | float64 | 1.23456789 |
| BYTES | Raw bytes | []byte | Binary data |

**Note**: FP16 is not directly supported in the structured format. Use `raw_input_contents` for FP16 data.

## Template Functions

The node supports the following template functions for data transformation:

### `jp` - JSONPath Extraction

Extract data from JSON using JSONPath expressions:

```go
{{. | jp "$.data.field"}}
```

### `json` - JSON Serialization

Convert Go values to JSON:

```go
{{. | json}}
```

**Note**: Input templates output binary data that is converted to tensors using the datatype specified in `input_tensor_spec`. The conversion is handled automatically by the `util.BytesToTensor` function.

## Example Pipeline

### Image Classification Pipeline

```yaml
input:
  kind: "mjpeg"
  spec:
    url: "http://camera:8080/stream"
    frame_interval: 10

output:
  kind: "stdout"
  spec: {}

flow:
  type: "sequence"
  name: "image-classification"
  nodes:
    # Preprocess image
    - type: "service"
      name: "preprocessor"
      config:
        kind: "http"
        spec:
          url: "http://preprocessor:8000/process"
          method: "POST"
    
    # Inference with Triton
    - type: "service"
      name: "triton-inference"
      config:
        kind: "openinference"
        spec:
          address: "triton:8001"
          model_name: "image_classifier"
          num_instances: 4
          
          # Define tensor specifications
          input_tensor_spec:
            - name: "images"
              datatype: "FP32"
              shape: [1, 3, 224, 224]
          
          output_tensor_spec:
            - name: "probabilities"
              datatype: "FP32"
              shape: [1, 1000]
            - name: "classes"
              datatype: "INT32"
              shape: [1, 1]
          
          # Optional: Transform input data
          input_templates:
            - name: "images"
              template: |
                {{. | jp "$.preprocessed_data"}}
          
          # Optional: Transform output data
          output_template: |
            {
              "class_id": {{. | jp "$.outputs[0].data[0]"}},
              "confidence": {{. | jp "$.outputs[1].data[0]"}},
              "model": "{{. | jp "$.model_name"}}"
            }
```

## Performance Tuning

### Parallel Instances

Increase `num_instances` for higher throughput:

```yaml
num_instances: 8  # 8 parallel workers
```

### Timeout Configuration

Adjust timeout for long-running inferences:

```yaml
timeout: 60  # 60 seconds
```

### Connection Pooling

gRPC automatically manages connection pooling. For optimal performance:

- Use a single client instance per model
- Enable HTTP/2 keep-alive (handled automatically)
- Monitor connection health with server health checks

## Health Checks

The Open Inference Protocol supports health check endpoints:

```go
// Server liveness check
ServerLive(ctx, &ServerLiveRequest{})

// Server readiness check
ServerReady(ctx, &ServerReadyRequest{})

// Model readiness check
ModelReady(ctx, &ModelReadyRequest{Name: "model_name"})
```

These can be used for:
- Startup probes
- Liveness probes
- Readiness probes
- Circuit breaker implementations

## Error Handling

The node returns error events for:

- Connection failures
- Timeout exceeded
- Invalid tensor specifications
- Template processing errors
- Inference server errors

Error events include:
- Error message
- HTTP-style status code (500 for server errors)
- Original context for tracing

## Observability

The node automatically creates OpenTelemetry spans for:

- gRPC connection establishment
- Inference request/response
- Template processing

Span attributes include:
- Model name and version
- Request ID
- Input/output tensor shapes
- Processing duration

## Limitations

- FP16 tensors must use `raw_input_contents` format
- Maximum message size is limited by gRPC configuration (default: 4MB)
- Binary data in templates must be base64 encoded in JSON

## Troubleshooting

### Connection Refused

```
Error: failed to dial gRPC server: connection refused
```

**Solution**: Check that the inference server is running and the address is correct.

### Invalid Tensor Shape

```
Error: tensor shape mismatch
```

**Solution**: Ensure the `shape` array matches the actual data dimensions.

### Template Processing Errors

```
Error: failed to process input template
```

**Solution**: Validate your template syntax and ensure JSONPath expressions are correct.

## References

- [Open Inference Protocol Specification](https://github.com/kserve/open-inference-protocol)
- [Triton Inference Server](https://github.com/triton-inference-server/server)
- [KServe Documentation](https://kserve.github.io/website/)
- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)

## License

SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
