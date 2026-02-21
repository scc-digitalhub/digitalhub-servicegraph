#!/bin/bash
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
# SPDX-License-Identifier: Apache-2.0

# Setup script for VQA service

set -e

echo "Setting up VQA service environment..."

# Create virtual environment if it doesn't exist
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
source venv/bin/activate

# Upgrade pip
echo "Upgrading pip..."
pip install --upgrade pip

# Install dependencies
echo "Installing dependencies..."
pip install -r requirements.txt

echo ""
echo "Setup complete!"
echo ""
echo "To activate the environment, run:"
echo "  source venv/bin/activate"
echo ""
echo "To start the VQA service, run:"
echo "  python vqa_http_service.py"
echo ""
