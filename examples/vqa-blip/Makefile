# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
# SPDX-License-Identifier: Apache-2.0

.PHONY: help setup start-service test clean docker-build docker-up docker-down

help:
	@echo "VQA BLIP Example - Available targets:"
	@echo ""
	@echo "  setup          - Set up Python virtual environment and install dependencies"
	@echo "  start-service  - Start the VQA HTTP service"
	@echo "  test           - Test the VQA service (requires IMAGE=path/to/image.jpg)"
	@echo "  run-pipeline   - Run the full pipeline (requires service running)"
	@echo "  clean          - Remove virtual environment and cache"
	@echo ""
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-up      - Start services with Docker Compose"
	@echo "  docker-down    - Stop Docker services"
	@echo ""

setup:
	@echo "Setting up environment..."
	@bash setup.sh

start-service:
	@echo "Starting VQA service..."
	@bash start_service.sh

test:
	@if [ -z "$(IMAGE)" ]; then \
		echo "Error: IMAGE variable not set"; \
		echo "Usage: make test IMAGE=path/to/image.jpg"; \
		exit 1; \
	fi
	@python test_service.py $(IMAGE)

run-pipeline:
	@echo "Building service graph..."
	@cd ../.. && go build -o digitalhub-servicegraph main.go
	@echo "Running pipeline..."
	@cd ../.. && ./digitalhub-servicegraph examples/vqa-blip/vqa-pipeline.yaml

clean:
	@echo "Cleaning up..."
	@rm -rf venv __pycache__ *.pyc
	@find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true

docker-build:
	@echo "Building Docker image..."
	@docker build -t vqa-blip:latest .

docker-up:
	@echo "Starting services with Docker Compose..."
	@docker-compose up --build

docker-down:
	@echo "Stopping Docker services..."
	@docker-compose down
