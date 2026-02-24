# Proto Generation Required

## Status

⚠️ The Open Inference client node requires generated Go code from Protocol Buffer definitions, but generation has not been completed yet on this system.

## Why This Is Needed

The `openinferenceclient` node communicates with inference servers using the [Open Inference Protocol v2](https://github.com/kserve/open-inference-protocol), which is defined using Protocol Buffers (protobuf). The Go code needs to be generated from these `.proto` files to enable gRPC communication.

## Requirements

To generate the required code, you need:

1. **Protocol Buffers Compiler (protoc)** version **3.0 or later**
   - Current system has: `libprotoc 2.4.1` ❌ (too old)
   - Required: `libprotoc 3.0+` ✅

2. **Go plugins for protoc**:
   - `protoc-gen-go`
   - `protoc-gen-go-grpc`

## How to Generate

### Step 1: Install/Upgrade protoc

#### macOS
```bash
# Using Homebrew
brew install protobuf

# Or upgrade if already installed
brew upgrade protobuf

# Verify version (should be 3.x or later)
protoc --version
```

#### Linux (Debian/Ubuntu)
```bash
# Using apt (may not have latest version)
sudo apt-get install -y protobuf-compiler

# Or install manually from official releases:
PROTOC_VERSION=25.1
wget https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip
unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d $HOME/.local
export PATH="$PATH:$HOME/.local/bin"

# Verify
protoc --version
```

#### Linux (Other)
Download the appropriate release for your system from:
https://github.com/protocolbuffers/protobuf/releases

### Step 2: Generate Code

Once protoc 3.0+ is installed:

```bash
# From repository root
./scripts/generate-proto.sh
```

This will:
1. Check protoc version
2. Install Go plugins if needed
3. Generate Go code in `pkg/proto/inference/v2/`

## Alternative: Use Pre-Generated Code

If you cannot upgrade protoc, you can use pre-generated code from a system that has protoc 3.0+:

1. On a system with protoc 3.0+, run:
   ```bash
   ./scripts/generate-proto.sh
   ```

2. Copy the generated files to your system:
   ```bash
   # Generated files:
   pkg/proto/inference/v2/grpc_service.pb.go
   pkg/proto/inference/v2/grpc_service_grpc.pb.go
   ```

3. Add these files to your project

## What Gets Generated

After successful generation, you will have:

- `grpc_service.pb.go`: Protocol Buffer message definitions
  - `ModelInferRequest`
  - `ModelInferResponse`
  - `InferInputTensor`
  - `InferOutputTensor`
  - `InferTensorContents`
  - etc.

- `grpc_service_grpc.pb.go`: gRPC client and server interfaces
  - `GRPCInferenceServiceClient`
  - `ModelInfer()` method
  - `ServerLive()`, `ServerReady()`, `ModelReady()` methods
  - Connection management

## Testing After Generation

Once generated, test the import:

```bash
cd pkg/nodes/openinferenceclient
go build
```

If successful, you should see no import errors.

## Current System Information

```
protoc version: libprotoc 2.4.1
Status: ❌ Too old (requires 3.0+)
Action needed: Upgrade protoc to version 3.0 or later
```

## Support

For issues or questions:
- Check protoc installation: https://grpc.io/docs/protoc-installation/
- Open Inference Protocol: https://github.com/kserve/open-inference-protocol
- gRPC Go Quick Start: https://grpc.io/docs/languages/go/quickstart/

---

**Note**: The proto definition file is already present at:
`pkg/proto/inference/v2/grpc_service.proto`

Only the generated Go code is missing.
