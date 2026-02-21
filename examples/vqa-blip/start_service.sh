#!/bin/bash
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
# SPDX-License-Identifier: Apache-2.0

# Start the VQA HTTP service

# Activate virtual environment if it exists
if [ -d "venv" ]; then
    source venv/bin/activate
fi

echo "Starting VQA HTTP service on http://localhost:8000"
echo "Endpoints:"
echo "  - POST /caption - Image captioning"
echo "  - GET  /health  - Health check"
echo ""

python vqa_http_service.py
