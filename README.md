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

#### WebSocket Service
Sends WebSocket messages.

**Spec Configuration:**
- `url` (string): WebSocket URL
- `msg_type` (int): Message type
- `params` (map[string]string): Query parameters
- `headers` (map[string]string): HTTP headers
- `input_template` (string): Optional Go text template for input data
- `output_template` (string): Optional Go text template for output data

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