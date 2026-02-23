.PHONY: setup start test clean docker-build docker-up docker-down help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

setup: ## Setup Python environment and download models
	chmod +x setup.sh
	./setup.sh

start: ## Start the anonymization service
	chmod +x start_service.sh
	./start_service.sh

test: ## Run service tests
	@if [ ! -d "venv" ]; then \
		echo "Please run 'make setup' first"; \
		exit 1; \
	fi
	@. venv/bin/activate && python3 test_service.py

docker-build: ## Build Docker image
	docker-compose build

docker-up: ## Start services with Docker Compose
	docker-compose up -d

docker-down: ## Stop Docker Compose services
	docker-compose down

docker-logs: ## View Docker Compose logs
	docker-compose logs -f

clean: ## Clean generated files
	rm -rf venv __pycache__ *.pyc output/ test_output*.jpg
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete 2>/dev/null || true

clean-all: clean ## Clean everything including models
	rm -rf models/
