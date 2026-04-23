#!/usr/bin/env python3

# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

"""
Video Anonymization Service using face detection and blurring.

This microservice provides HTTP endpoints for anonymizing images by detecting
and blurring faces (and optionally license plates). It integrates with the
digitalhub-servicegraph framework for real-time video stream processing.
"""

import logging
import cv2
import numpy as np
from io import BytesIO
from PIL import Image
from flask import Flask, request, jsonify
import os

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Global variables for models
face_cascade = None
face_net = None
face_detector_type = None


def init_models():
    """Initialize face detection models at server startup."""
    global face_cascade, face_net, face_detector_type
    
    # Try to use DNN-based face detector (more accurate)
    model_path = "models/deploy.prototxt"
    weights_path = "models/res10_300x300_ssd_iter_140000.caffemodel"
    
    if os.path.exists(model_path) and os.path.exists(weights_path):
        logger.info("Loading DNN-based face detector...")
        face_net = cv2.dnn.readNetFromCaffe(model_path, weights_path)
        face_detector_type = "dnn"
        logger.info("DNN face detector loaded successfully")
    else:
        # Fallback to Haar Cascade (comes with OpenCV)
        logger.info("Loading Haar Cascade face detector...")
        face_cascade = cv2.CascadeClassifier(
            cv2.data.haarcascades + 'haarcascade_frontalface_default.xml'
        )
        face_detector_type = "haar"
        logger.info("Haar Cascade face detector loaded successfully")


def detect_faces_dnn(image, confidence_threshold=0.5):
    """
    Detect faces using DNN-based detector.
    
    Args:
        image: OpenCV image (BGR format)
        confidence_threshold: Minimum confidence for detection
        
    Returns:
        List of (x, y, w, h) tuples for detected faces
    """
    h, w = image.shape[:2]
    blob = cv2.dnn.blobFromImage(
        cv2.resize(image, (300, 300)), 1.0, (300, 300), (104.0, 177.0, 123.0)
    )
    
    face_net.setInput(blob)
    detections = face_net.forward()
    
    faces = []
    for i in range(detections.shape[2]):
        confidence = detections[0, 0, i, 2]
        if confidence > confidence_threshold:
            box = detections[0, 0, i, 3:7] * np.array([w, h, w, h])
            (x, y, x2, y2) = box.astype("int")
            faces.append((x, y, x2 - x, y2 - y))
    
    return faces


def detect_faces_haar(image):
    """
    Detect faces using Haar Cascade.
    
    Args:
        image: OpenCV image (BGR format)
        
    Returns:
        List of (x, y, w, h) tuples for detected faces
    """
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    faces = face_cascade.detectMultiScale(
        gray,
        scaleFactor=1.1,
        minNeighbors=5,
        minSize=(30, 30)
    )
    return faces


def blur_region(image, x, y, w, h, blur_factor=50):
    """
    Apply Gaussian blur to a specific region of the image.
    
    Args:
        image: OpenCV image
        x, y: Top-left corner of region
        w, h: Width and height of region
        blur_factor: Blur intensity (higher = more blur)
        
    Returns:
        Image with blurred region
    """
    # Ensure coordinates are within image bounds
    height, width = image.shape[:2]
    x = max(0, x)
    y = max(0, y)
    w = min(w, width - x)
    h = min(h, height - y)
    
    # Extract region
    roi = image[y:y+h, x:x+w]
    
    # Apply Gaussian blur (kernel size must be odd)
    kernel_size = blur_factor if blur_factor % 2 == 1 else blur_factor + 1
    blurred_roi = cv2.GaussianBlur(roi, (kernel_size, kernel_size), 0)
    
    # Replace region with blurred version
    image[y:y+h, x:x+w] = blurred_roi
    
    return image


def pixelate_region(image, x, y, w, h, pixel_size=15):
    """
    Apply pixelation effect to a specific region.
    
    Args:
        image: OpenCV image
        x, y: Top-left corner of region
        w, h: Width and height of region
        pixel_size: Size of pixels for pixelation effect
        
    Returns:
        Image with pixelated region
    """
    # Ensure coordinates are within image bounds
    height, width = image.shape[:2]
    x = max(0, x)
    y = max(0, y)
    w = min(w, width - x)
    h = min(h, height - y)
    
    # Extract region
    roi = image[y:y+h, x:x+w]
    
    # Downscale and upscale to create pixelation effect
    temp = cv2.resize(roi, (w // pixel_size, h // pixel_size), interpolation=cv2.INTER_LINEAR)
    pixelated_roi = cv2.resize(temp, (w, h), interpolation=cv2.INTER_NEAREST)
    
    # Replace region
    image[y:y+h, x:x+w] = pixelated_roi
    
    return image


def anonymize_image(image_bytes, method='blur', blur_factor=50, pixel_size=15):
    """
    Anonymize an image by detecting and obscuring faces.
    
    Args:
        image_bytes: Image data as bytes
        method: Anonymization method ('blur' or 'pixelate')
        blur_factor: Blur intensity for blur method
        pixel_size: Pixel size for pixelate method
        
    Returns:
        Tuple of (anonymized_image_bytes, face_count)
    """
    # Convert bytes to OpenCV image
    nparr = np.frombuffer(image_bytes, np.uint8)
    image = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    
    if image is None:
        raise ValueError("Failed to decode image")
    
    # Detect faces
    if face_detector_type == "dnn":
        faces = detect_faces_dnn(image)
    else:
        faces = detect_faces_haar(image)
    
    logger.info(f"Detected {len(faces)} face(s)")
    
    # Anonymize each face
    for (x, y, w, h) in faces:
        if method == 'pixelate':
            image = pixelate_region(image, x, y, w, h, pixel_size)
        else:  # blur
            image = blur_region(image, x, y, w, h, blur_factor)
    
    # Convert back to bytes
    _, buffer = cv2.imencode('.jpg', image)
    return buffer.tobytes(), len(faces)


@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({
        "status": "healthy",
        "detector": face_detector_type
    }), 200


@app.route('/anonymize', methods=['POST'])
def anonymize():
    """
    Anonymize faces in the provided image.
    
    Expects: Image bytes in request body
    Query params:
        - method: 'blur' or 'pixelate' (default: blur)
        - blur_factor: Blur intensity (default: 50)
        - pixel_size: Pixelation size (default: 15)
    
    Returns: Anonymized image bytes
    """
    if face_detector_type is None:
        init_models()
    
    try:
        # Get image from request body
        image_bytes = request.data
        
        if not image_bytes:
            return jsonify({"error": "No image data provided"}), 400
        
        # Get parameters
        method = request.args.get('method', 'blur')
        blur_factor = int(request.args.get('blur_factor', 50))
        pixel_size = int(request.args.get('pixel_size', 15))
        
        # Validate method
        if method not in ['blur', 'pixelate']:
            return jsonify({"error": "Method must be 'blur' or 'pixelate'"}), 400
        
        # Anonymize the image
        anonymized_bytes, face_count = anonymize_image(
            image_bytes, method, blur_factor, pixel_size
        )
        
        logger.info(f"Anonymized {face_count} face(s) using {method} method")
        
        # Return anonymized image
        return anonymized_bytes, 200, {'Content-Type': 'image/jpeg'}
        
    except Exception as e:
        logger.error(f"Error processing image: {e}")
        return jsonify({
            "error": str(e),
            "status": "error"
        }), 500


@app.route('/anonymize/info', methods=['POST'])
def anonymize_info():
    """
    Anonymize image and return JSON with info and base64 image.
    
    Returns: JSON with face count and anonymized image
    """
    if face_detector_type is None:
        init_models()
    
    try:
        image_bytes = request.data
        
        if not image_bytes:
            return jsonify({"error": "No image data provided"}), 400
        
        method = request.args.get('method', 'blur')
        blur_factor = int(request.args.get('blur_factor', 50))
        pixel_size = int(request.args.get('pixel_size', 15))
        
        anonymized_bytes, face_count = anonymize_image(
            image_bytes, method, blur_factor, pixel_size
        )
        
        import base64
        encoded_image = base64.b64encode(anonymized_bytes).decode('utf-8')
        
        return jsonify({
            "faces_detected": face_count,
            "method": method,
            "image": encoded_image,
            "status": "success"
        }), 200
        
    except Exception as e:
        logger.error(f"Error in anonymize_info: {e}")
        return jsonify({
            "error": str(e),
            "status": "error"
        }), 500


def main():
    """Start the Flask server."""
    logger.info("Starting Video Anonymization service")
    init_models()
    logger.info(f"Service ready on http://localhost:8000 (using {face_detector_type} detector)")
    app.run(host='0.0.0.0', port=8000, debug=False)


if __name__ == "__main__":
    main()
