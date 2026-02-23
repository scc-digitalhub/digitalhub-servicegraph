#!/bin/bash
# Start script for Video Anonymization Service

set -e

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    echo "Error: Virtual environment not found!"
    echo "Please run ./setup.sh first"
    exit 1
fi

# Activate virtual environment
source venv/bin/activate

# Check if models exist
if [ -f "models/deploy.prototxt" ] && [ -f "models/res10_300x300_ssd_iter_140000.caffemodel" ]; then
    echo "Using DNN-based face detector (more accurate)"
else
    echo "DNN models not found - using Haar Cascade detector (fallback)"
    echo "For better accuracy, run ./setup.sh to download DNN models"
fi

# Create output directory if it doesn't exist
mkdir -p output

# Start the service
echo "Starting Video Anonymization Service on http://localhost:8000"
echo "Endpoints:"
echo "  - POST /anonymize - Anonymize image (returns image bytes)"
echo "  - POST /anonymize/info - Anonymize and return JSON with info"
echo "  - GET /health - Health check"
echo ""
python3 anonymization_service.py
