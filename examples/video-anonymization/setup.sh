#!/bin/bash
# Setup script for Video Anonymization Service
#
# This script:
# 1. Creates a Python virtual environment
# 2. Installs required dependencies
# 3. Downloads DNN face detection models (optional, for better accuracy)

set -e

echo "=== Video Anonymization Service Setup ==="

# Create virtual environment
echo "Creating Python virtual environment..."
python3 -m venv venv

# Activate virtual environment
echo "Activating virtual environment..."
source venv/bin/activate

# Upgrade pip
echo "Upgrading pip..."
pip install --upgrade pip

# Install requirements
echo "Installing Python dependencies..."
pip install -r requirements.txt

# Download DNN face detection models (optional but recommended)
echo ""
echo "=== Downloading DNN Face Detection Models (optional) ==="
echo "These models provide better accuracy than Haar Cascades."
echo ""

mkdir -p models
cd models

# Download Caffe model for face detection
if [ ! -f "deploy.prototxt" ]; then
    echo "Downloading model architecture..."
    curl -L -o deploy.prototxt \
        https://raw.githubusercontent.com/opencv/opencv/master/samples/dnn/face_detector/deploy.prototxt
fi

if [ ! -f "res10_300x300_ssd_iter_140000.caffemodel" ]; then
    echo "Downloading model weights (10MB)..."
    curl -L -o res10_300x300_ssd_iter_140000.caffemodel \
        https://raw.githubusercontent.com/opencv/opencv_3rdparty/dnn_samples_face_detector_20170830/res10_300x300_ssd_iter_140000.caffemodel
fi

cd ..

echo ""
echo "=== Setup Complete! ==="
echo ""
echo "To activate the virtual environment:"
echo "  source venv/bin/activate"
echo ""
echo "To start the service:"
echo "  ./start_service.sh"
echo ""
echo "Or run manually:"
echo "  python3 anonymization_service.py"
echo ""
