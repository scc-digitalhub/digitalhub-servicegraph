# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

IMAGE_NAME   ?= ghcr.io/scc-digitalhub/digitalhub-servicegraph
IMAGE_TAG    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PLATFORMS    ?= linux/amd64,linux/arm64
BUILDER_NAME ?= servicegraph-builder

.PHONY: help build test docker-build docker-buildx docker-push docker-buildx-push setup-buildx clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# ─── Go targets ───────────────────────────────────────────────────────────────

build: ## Build the servicegraph binary for the current platform
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o servicegraph .

test: ## Run all Go tests
	go test ./...

# ─── Single-platform Docker targets ──────────────────────────────────────────

docker-build: ## Build a single-arch Docker image for the current platform
	podman build --tag $(IMAGE_NAME):$(IMAGE_TAG)  --tag $(IMAGE_NAME):latest  .

docker-push: ## Push the single-arch image to the registry
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(IMAGE_NAME):latest

# ─── Multi-arch Docker targets (requires buildx) ─────────────────────────────

setup-buildx: ## Create (or reuse) a multi-arch buildx builder instance
	@if ! docker buildx inspect $(BUILDER_NAME) > /dev/null 2>&1; then \
		docker buildx create \
			--name $(BUILDER_NAME) \
			--driver docker-container \
			--bootstrap; \
	fi
	docker buildx use $(BUILDER_NAME)

## Build multi-arch image and load it into the local Docker daemon (single platform only).
## Use docker-buildx-push to build & push all platforms at once.
docker-buildx: setup-buildx ## Build multi-arch image (local load, single arch only)
	docker buildx build \
		--builder $(BUILDER_NAME) \
		--platform linux/$(shell go env GOARCH) \
		--tag $(IMAGE_NAME):$(IMAGE_TAG) \
		--tag $(IMAGE_NAME):latest \
		--load \
		.

docker-buildx-push: setup-buildx ## Build multi-arch image and push to registry ($(PLATFORMS))
	docker buildx build \
		--builder $(BUILDER_NAME) \
		--platform $(PLATFORMS) \
		--tag $(IMAGE_NAME):$(IMAGE_TAG) \
		--tag $(IMAGE_NAME):latest \
		--push \
		.

# ─── Cleanup ─────────────────────────────────────────────────────────────────

clean: ## Remove the local binary and Docker builder
	rm -f servicegraph
	-docker buildx rm $(BUILDER_NAME) 2>/dev/null || true
