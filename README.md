# Service Graph

## Description

Service Graph is a Go-based tool for building and executing data flow graphs that process streaming data. It allows users to define graphs consisting of input sources, processing flows with various transformation nodes, and output sinks. The tool supports both synchronous and asynchronous execution modes, enabling real-time data processing pipelines for applications such as API gateways, data integration, and event-driven architectures.

The core architecture revolves around:
- **Sources**: Entry points that ingest data (e.g., HTTP requests, WebSocket messages)
- **Flows**: Processing pipelines composed of nodes that transform, filter, aggregate, or route data
- **Sinks**: Output destinations that handle processed data (e.g., files, webhooks, WebSocket clients)

## Sources

Service Graph supports the following source types:

### HTTP Source
Receives HTTP requests and converts them into stream events.

**Configuration:**
- `port` (int): Server port (default: 8080)
- `read_timeout` (int): Read timeout in seconds (default: 10)
- `write_timeout` (int): Write timeout in seconds (default: 10)
- `process_timeout` (int): Processing timeout in seconds (default: 30)
- `max_input_size` (int64): Maximum input size in bytes (default: 10MB)

**Usage:**
```yaml
input:
  kind: "http"
  spec:
    port: 8080
    read_timeout: 15
```

### WebSocket Source
Receives WebSocket messages and converts them into stream events.

**Configuration:**
- `port` (int): Server port (default: 8080)
- `capacity` (int): Channel capacity for message buffering (default: 10)

**Usage:**
```yaml
input:
  kind: "websocket"
  spec:
    port: 8080
    capacity: 20
```
### MJPEG Source
Reads multipart MJPEG streams (commonly from IP cameras or video endpoints) and emits individual JPEG frames as events.

**Configuration:**
- `url` (string): MJPEG stream URL (required)
- `frame_interval` (int): Process every Nth frame (default: 1 — all frames)
- `read_timeout` (int): Read timeout in seconds (default: 10)

**Usage:**
```yaml
input:
  kind: "mjpeg"
  spec:
    url: "http://camera:8080/stream"
    frame_interval: 10
    read_timeout: 5
```

The MJPEG source emits events where `GetBody()` returns the JPEG bytes and `GetFrameNumber()` exposes the 1-based frame index.

## Sinks

Service Graph supports the following sink types:


### Webhook Sink
Sends processed data via HTTP POST to a specified URL.

**Configuration:**
- `url` (string): Target URL
- `params` (map[string]string): Query parameters
- `headers` (map[string]string): HTTP headers
- `parallelism` (int): Number of concurrent requests

**Usage:**
```yaml
output:
  kind: "webhook"
  spec:
    url: "https://api.example.com/webhook"
    headers:
      Authorization: "Bearer token"
    parallelism: 5
```

### WebSocket Sink
Sends processed data via WebSocket connection.

**Configuration:**
- `url` (string): WebSocket URL
- `msg_type` (int): Message type (1 for text, 2 for binary)
- `params` (map[string]string): Query parameters
- `headers` (map[string]string): HTTP headers for initial connection

**Usage:**
```yaml
output:
  kind: "websocket"
  spec:
    url: "ws://example.com/ws"
    msg_type: 1
    headers:
      Authorization: "Bearer token"
```

### Stdout Sink
Writes processed data to standard output.

**Configuration:** None required.

**Usage:**
```yaml
output:
  kind: "stdout"
```

### File Sink
Writes processed data to a file.

**Configuration:**
- `file_name` (string): Path to the output file

**Usage:**
```yaml
output:
  kind: "file"
  spec:
    file_name: "output.txt"
```

The `folder` sink writes each incoming event to a separate file in a specified folder using an incremental filename pattern.

**Configuration**

The folder sink requires two configuration parameters:

- `folder_path`: The path to the folder where files will be created
- `filename_pattern`: The pattern for generating filenames with placeholders

**Filename Pattern Placeholders**

The following placeholders are supported in the `filename_pattern`:

- `{counter}` or `{index}`: Incremental counter starting from 1
- `{timestamp}`: Unix timestamp in seconds
- `{timestamp_ms}`: Unix timestamp in milliseconds  
- `{timestamp_ns}`: Unix timestamp in nanoseconds

**Usage:**

```yaml
output:
  kind: "folder"
  spec:
    folder_path: "./output/data"
    filename_pattern: "data_{index}.json"
```


### Ignore Sink
Discards processed data (useful for testing or when no output is needed).

**Configuration:** None required.

**Usage:**
```yaml
output:
  kind: "ignore"
```

## Flow Nodes

Flow nodes define the processing logic of the data pipeline. Nodes can be composed hierarchically.

### Sequence Node
Executes child nodes in sequence, passing data from one to the next.

**Configuration:**
- `type`: "sequence"
- `name` (string): Optional node name
- `nodes` ([]Node): Array of child nodes

### Ensemble Node
Executes multiple child nodes in parallel and merges their outputs.

**Configuration:**
- `type`: "ensemble"
- `name` (string): Optional node name
- `nodes` ([]Node): Array of child nodes
- `config`:
  -  `spec` (map[string]interface{}): Configuration options with
     -  `merge_mode` (string): Merge mode ("concat" or "concat_template")
     -  `template` (string): Go text template in case of `concat_template`. It accepts the slice of maps as input, where each map represents the JSON-structured ouputs of the corresponding branches.

### Switch Node
Routes data to different child nodes based on conditions.

**Configuration:**
- `type`: "switch"
- `name` (string): Optional node name
- `nodes` ([]Node): Array of child nodes with conditions
  - Each node can have a `condition` (string): JSONPath expression

### Service Node
Applies a specific service transformation (e.g., HTTP call, WebSocket message).

**Configuration:**
- `type`: "service"
- `name` (string): Optional node name
- `config`:
  - `kind` (string): Service type ("http", "websocket")
  - `spec`: Service-specific configuration

#### HTTP Service
Makes HTTP requests.

**Spec Configuration:**
- `url` (string): Target URL
- `method` (string): HTTP method (default: "POST")
- `params` (map[string]string): Query parameters
- `headers` (map[string]string): HTTP headers
- `num_instances` (int): Number of concurrent instances
- `input_template` (string): Optional Go text template for input data
- `output_template` (string): Optional Go text template for output data

#### OpenInference Node
Integrates with Open Inference Protocol v2 servers (for example Triton Inference Server or KServe) to perform model inference.

**Spec Configuration (under `config.spec`):**
- `address` (string): server address (host:port)
- `model_name` (string): model name to call (required)
- `model_version` (string): optional model version
- `num_instances` (int): number of parallel workers (default: 1)
- `timeout` (int): request timeout in seconds (default: 30)
- `protocol` (string): `grpc` (default) or `rest`
- `input_tensor_spec` ([]): list of input tensor specs (name, datatype, shape)
- `output_tensor_spec` ([]): list of output tensor specs
- `input_templates` / `output_template`: optional templates for input/output transformation

**Usage:**
```yaml
- type: "service"
  name: "triton-inference"
  config:
    kind: "openinference"
    spec:
      address: "triton:8001"
      model_name: "resnet50"
      num_instances: 4
      protocol: "grpc"
      input_tensor_spec:
        - name: "input"
          datatype: "FP32"
          shape: [1, 3, 224, 224]
```

#### WebSocket Service
Sends WebSocket messages.

**Spec Configuration:**
- `url` (string): WebSocket URL
- `msg_type` (int): Message type
- `params` (map[string]string): Query parameters
- `headers` (map[string]string): HTTP headers
- `input_template` (string): Optional Go text template for input data
- `output_template` (string): Optional Go text template for output data

## Template Processing

Service Graph supports Go text templates with custom functions for data transformation in `input_template` and `output_template` fields. Templates allow you to transform data flowing through the pipeline using standard Go template syntax enhanced with specialized functions.

### Template Syntax

Templates use the standard [Go text/template](https://pkg.go.dev/text/template) syntax:

```
{{.}}              # Access the input data
{{. | jp "$.name"}}  # Use custom functions with pipes
```

### Custom Template Functions

#### jp - JSONPath Evaluation

Evaluates a JSONPath expression on JSON data.

**Signature:** `jp(jsonPath string, jsonData any) any`

**Parameters:**
- `jsonPath` (string): JSONPath expression 
- `jsonData` (any): JSON data (string, []byte, or streams.Event)

**Returns:** The matched value(s) from the JSON data

**Examples:**
```yaml
# Extract a single field
output_template: "Name: {{. | jp \"$.user.name\"}}"

# Extract nested data
output_template: "Age: {{. | jp \"$.user.profile.age\"}}"

# Extract array element
output_template: "First: {{. | jp \"$.items[0]\"}}"

# Complex expression
output_template: "{{. | jp \"$.users[?(@.active==true)].name\"}}"
```

**Input:**
```json
{"user": {"name": "Alice", "profile": {"age": 30}}}
```

**Output:**
```
Name: Alice
Age: 30
```

#### json - JSON Serialization

Converts a Go value to its JSON string representation.

**Signature:** `json(v any) string`

**Parameters:**
- `v` (any): Any Go value

**Returns:** JSON string representation

**Examples:**
```yaml
# Convert extracted data to JSON
output_template: "{{. | jp \"$.user\" | json}}"

# Create JSON structure
output_template: "{\"result\": {{. | json}}}"
```

**Input:**
```json
{"user": {"name": "Bob", "age": 25}}
```

**Output:**
```json
{"name":"Bob","age":25}
```

#### tensor - Binary Data to Numerical Array

Converts byte arrays to numerical arrays based on Inference Protocol v2 data types. Useful for ML model inference and binary data processing.

**Signature:** `tensor(dataType string, data any) string`

**Parameters:**
- `dataType` (string): Data type name (case-insensitive)
- `data` (any): Binary data ([]byte, string, or streams.Event)

**Returns:** JSON array of numerical values

**Supported Data Types:**

| Type | Description | Bytes per Element | Output Type |
|------|-------------|-------------------|-------------|
| BOOL | Boolean values | 1 | []bool |
| UINT8 | Unsigned 8-bit integers | 1 | []int |
| INT8 | Signed 8-bit integers | 1 | []int |
| UINT16 | Unsigned 16-bit integers | 2 | []uint16 |
| INT16 | Signed 16-bit integers | 2 | []int16 |
| UINT32 | Unsigned 32-bit integers | 4 | []uint32 |
| INT32 | Signed 32-bit integers | 4 | []int32 |
| UINT64 | Unsigned 64-bit integers | 8 | []uint64 |
| INT64 | Signed 64-bit integers | 8 | []int64 |
| FP16 | Half-precision floats | 2 | []float32 |
| FP32 | Single-precision floats | 4 | []float32 |
| FP64 | Double-precision floats | 8 | []float64 |
| BYTES | Raw byte array | 1 | []int |

**Examples:**
```yaml
# Convert binary float data to JSON array
output_template: "{{. | tensor \"FP32\"}}"

# Convert with JSONPath extraction
output_template: "{{. | jp \"$.data\" | tensor \"INT32\"}}"

# Use in JSON structure
output_template: "{\"values\": {{. | tensor \"UINT8\"}}}"
```

**Input (binary):** 4 bytes representing float32 values [1.0, 2.0]

**Output:**
```json
[1.0, 2.0]
```

**Notes:**
- All multi-byte types use little-endian byte order
- FP16 values are converted to FP32 for JSON representation
- Data length must be a multiple of the element size
- BYTES and UINT8 produce the same output

### Combining Functions

Functions can be chained using pipes for complex transformations:

```yaml
# Extract JSON, convert to specific format, serialize back
output_template: |
  {
    "user": {{. | jp "$.user" | json}},
    "tensor_data": {{. | jp "$.binary_data" | tensor "FP32" | json}}
  }

# Multiple stages of processing
input_template: "{{. | jp \"$.input\" | json}}"
output_template: "Result: {{. | jp \"$.output.value\"}}"
```

### Template Usage in Services

Templates can be applied to both input and output data in service nodes:

#### Input Template Example

Transform request data before sending to a service:

```yaml
flow:
  type: "service"
  name: "api-call"
  config:
    kind: "http"
    spec:
      url: "https://api.example.com/process"
      method: "POST"
      input_template: |
        {
          "user_id": {{. | jp "$.user.id"}},
          "data": {{. | jp "$.payload" | json}}
        }
```

#### Output Template Example

Transform response data from a service:

```yaml
flow:
  type: "service"
  name: "api-call"
  config:
    kind: "http"
    spec:
      url: "https://api.example.com/model/predict"
      method: "POST"
      output_template: |
        {
          "predictions": {{. | jp "$.outputs[0].data" | tensor "FP32"}},
          "model_name": "{{. | jp "$.model_name\"}}"
        }
```

#### Complete Example

```yaml
input:
  kind: "http"
  spec:
    port: 8080

flow:
  type: "sequence"
  nodes:
    - type: "service"
      name: "ml-inference"
      config:
        kind: "http"
        spec:
          url: "http://triton-server:8000/v2/models/mymodel/infer"
          method: "POST"
          headers:
            Content-Type: "application/json"
          input_template: |
            {
              "inputs": [{
                "name": "input_tensor",
                "shape": [1, {{. | jp "$.shape[0]"}}, {{. | jp "$.shape[1]"}}],
                "datatype": "FP32",
                "data": {{. | jp "$.image_data" | tensor "FP32"}}
              }]
            }
          output_template: |
            {
              "predictions": {{. | jp "$.outputs[0].data" | tensor "FP32"}},
              "shape": {{. | jp "$.outputs[0].shape" | json}}
            }

output:
  kind: "stdout"
```

### Template Error Handling

If a template function fails (e.g., invalid JSONPath, type mismatch), the pipeline will return an error and the request will fail. Ensure:

- JSONPath expressions are valid
- Data types match expected formats
- Binary data lengths are correct multiples for tensor operations

## Configuration YAML Structure

The configuration is defined in YAML format with the following structure:

```yaml
input:          # Input source configuration
  kind: string  # Source type
  spec:         # Source-specific configuration
    # ... source config

flow:           # Processing flow
  type: string  # Node type ("sequence", "ensemble", "switch", "service")
  name: string  # Optional name
  # ... additional node config

output:         # Optional output sink
  kind: string  # Sink type
  spec:         # Sink-specific configuration
    # ... sink config
```

## Quick Start

1. Create a configuration file (e.g., `config.yaml`):

```yaml
input:
  kind: "http"
  spec:
    port: 8080
flow:
  type: "sequence"
  nodes:
    - type: "service"
      config:
        kind: "http"
        spec:
          url: "https://httpbin.org/post"
          method: "POST"
output:
  kind: "stdout"
```

2. Run the application:

```bash
go run main.go <config.yaml>
```

If no configuration file is provided, the application will display usage instructions.

3. Send a test request:

```bash
curl -X POST http://localhost:8080/ -d "Hello, Service Graph!"
```

The request will be processed through the flow and the response will be printed to stdout.

## Visual Service Graph Designer

Service Graph includes a web-based visual designer for creating and configuring service graphs with an intuitive drag-and-drop interface.

### Features

- **Visual Graph Editor**: Drag-and-drop interface for designing service graphs
- **Component Configuration**: Easy-to-use dialogs for configuring sources, sinks, and nodes
- **YAML Import/Export**: Load existing configurations and export new ones
- **Real-time Validation**: Instant feedback on configuration errors
- **Support for All Components**: 
  - Sources: HTTP, WebSocket, MJPEG
  - Sinks: Stdout, File, Folder, Ignore, Webhook, WebSocket
  - Nodes: HTTP, WebSocket, OpenInference
  - Flow Types: Sequence, Ensemble, Switch

### Getting Started

1. Navigate to the web directory:
```bash
cd web
```

2. Install dependencies:
```bash
npm install
```

3. Start the development server:
```bash
npm run dev
```

4. Open your browser to `http://localhost:3000`

### Usage

- **Create New Graph**: Use the left panel to configure input sources and the right panel for output sinks
- **Add Nodes**: Click on the graph canvas to add and configure service nodes
- **Configure Components**: Click on any component to open its configuration dialog
- **Import YAML**: Click the "Import" button to load an existing configuration file
- **Export YAML**: Click "Export" to download your graph as a YAML file
- **View YAML**: Toggle the YAML view to see the generated configuration in real-time

See the [Web Designer README](web/README.md) for detailed documentation.

## Examples

The `examples/` directory contains complete, ready-to-run applications demonstrating various use cases:

### Visual Question Answering (VQA) Pipeline

**Location:** `examples/vqa-blip/`

A complete ML pipeline that processes video streams and generates image captions using the BLIP vision-language model.

**Features:**
- MJPEG video stream processing
- BLIP model inference service (Flask/PyTriton)
- Real-time image captioning
- Docker support for easy deployment

**Quick Start:**
```bash
cd examples/vqa-blip
./setup.sh          # Set up Python environment
./start_service.sh  # Start VQA service

# In another terminal:
go build -o digitalhub-servicegraph main.go
./digitalhub-servicegraph examples/vqa-blip/vqa-pipeline.yaml
```

See the [VQA Example README](examples/vqa-blip/README.md) for detailed documentation.


## Security Policy

The current release is the supported version. Security fixes are released together with all other fixes in each new release.

If you discover a security vulnerability in this project, please do not open a public issue.

Instead, report it privately by emailing us at digitalhub@fbk.eu. Include as much detail as possible to help us understand and address the issue quickly and responsibly.

## Contributing

To report a bug or request a feature, please first check the existing issues to avoid duplicates. If none exist, open a new issue with a clear title and a detailed description, including any steps to reproduce if it's a bug.

To contribute code, start by forking the repository. Clone your fork locally and create a new branch for your changes. Make sure your commits follow the [Conventional Commits v1.0](https://www.conventionalcommits.org/en/v1.0.0/) specification to keep history readable and consistent.

Once your changes are ready, push your branch to your fork and open a pull request against the main branch. Be sure to include a summary of what you changed and why. If your pull request addresses an issue, mention it in the description (e.g., “Closes #123”).

Please note that new contributors may be asked to sign a Contributor License Agreement (CLA) before their pull requests can be merged. This helps us ensure compliance with open source licensing standards.

We appreciate contributions and help in improving the project!

## Authors

This project is developed and maintained by **DSLab – Fondazione Bruno Kessler**, with contributions from the open source community. A complete list of contributors is available in the project’s commit history and pull requests.

For questions or inquiries, please contact: [digitalhub@fbk.eu](mailto:digitalhub@fbk.eu)

## Copyright and license

Copyright © 2025 DSLab – Fondazione Bruno Kessler and individual contributors.

This project is licensed under the Apache License, Version 2.0.