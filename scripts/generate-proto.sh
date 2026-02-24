#!/bin/bash
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

# Script to generate Go code from proto files

set -e

echo "=== Proto Code Generation for Open Inference v2 ==="
echo

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "❌ Error: protoc is not installed"
    echo
    echo "Please install Protocol Buffers compiler (protoc) version 3.0 or later:"
    echo
    echo "  macOS:"
    echo "    brew install protobuf"
    echo
    echo "  Linux (Debian/Ubuntu):"
    echo "    apt-get install protobuf-compiler"
    echo
    echo "  Or download from:"
    echo "    https://github.com/protocolbuffers/protobuf/releases"
    echo
    exit 1
fi

# Check protoc version
PROTOC_VERSION=$(protoc --version 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "0.0.0")
MAJOR_VERSION=$(echo "$PROTOC_VERSION" | cut -d. -f1)

echo "Found protoc version: $PROTOC_VERSION"

if [ "$MAJOR_VERSION" -lt 3 ]; then
    echo
    echo "❌ Error: protoc version $PROTOC_VERSION is too old"
    echo
    echo "Proto3 syntax requires protoc version 3.0 or later."
    echo "Your version: $PROTOC_VERSION"
    echo
    echo "Please upgrade protoc:"
    echo
    echo "  macOS:"
    echo "    brew upgrade protobuf"
    echo
    echo "  Linux:"
    echo "    # Download latest from https://github.com/protocolbuffers/protobuf/releases"
    echo
    exit 1
fi

echo "✓ protoc version OK"

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    echo "✓ protoc-gen-go installed"
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    echo "✓ protoc-gen-go-grpc installed"
fi

echo
echo "Generating Go code from proto files..."
echo

# Generate Go code for inference v2 proto
protoc --go_out=. --go-grpc_out=. \
    pkg/proto/inference/v2/grpc_service.proto

echo
echo "✅ Proto generation complete!"
echo
echo "Generated files:"
echo "  - pkg/proto/inference/v2/grpc_service.pb.go"
echo "  - pkg/proto/inference/v2/grpc_service_grpc.pb.go"
echo
