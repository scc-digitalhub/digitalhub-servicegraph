#!/usr/bin/env python3
"""
Simple HTTP wrapper for Visual Question Answering using BLIP model.

This service provides a simple HTTP endpoint that accepts image data
and returns text descriptions, making it easy to integrate with
the digitalhub-servicegraph framework.
"""

import logging
from io import BytesIO
from PIL import Image
from flask import Flask, request, jsonify
from transformers import BlipProcessor, BlipForConditionalGeneration
import torch

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Global model instance
vqa_model = None
processor = None
device = None


def init_model():
    """Initialize the BLIP model at server startup."""
    global vqa_model, processor, device
    
    if vqa_model is None:
        model_name = "Salesforce/blip-image-captioning-base"
        logger.info(f"Loading BLIP model: {model_name}")
        
        device = "cuda" if torch.cuda.is_available() else "cpu"
        logger.info(f"Using device: {device}")
        
        processor = BlipProcessor.from_pretrained(model_name)
        vqa_model = BlipForConditionalGeneration.from_pretrained(model_name).to(device)
        logger.info("Model loaded successfully")


@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({"status": "healthy"}), 200


@app.route('/caption', methods=['POST'])
def caption():
    """
    Generate caption for the provided image.
    
    Expects: Image bytes in request body
    Returns: JSON with caption text
    """
    global vqa_model, processor, device
    
    if vqa_model is None:
        init_model()
    
    try:
        # Get image from request body
        image_bytes = request.data
        
        if not image_bytes:
            return jsonify({"error": "No image data provided"}), 400
        
        # Convert bytes to PIL Image
        image = Image.open(BytesIO(image_bytes)).convert('RGB')
        
        # Process image and generate caption
        inputs = processor(image, return_tensors="pt").to(device)
        
        with torch.no_grad():
            output = vqa_model.generate(**inputs, max_new_tokens=50)
        
        caption_text = processor.decode(output[0], skip_special_tokens=True)
        logger.info(f"Generated caption: {caption_text}")
        
        return jsonify({
            "caption": caption_text,
            "status": "success"
        }), 200
        
    except Exception as e:
        logger.error(f"Error processing image: {e}")
        return jsonify({
            "error": str(e),
            "status": "error"
        }), 500


@app.route('/vqa', methods=['POST'])
def vqa():
    """
    Visual Question Answering endpoint.
    
    Expects: JSON with 'image' (base64) and 'question' fields
    Returns: JSON with answer text
    """
    global vqa_model, processor, device
    
    if vqa_model is None:
        init_model()
    
    try:
        # For now, just return caption as we're doing image captioning
        # Can be extended for actual VQA with questions
        return caption()
        
    except Exception as e:
        logger.error(f"Error in VQA: {e}")
        return jsonify({
            "error": str(e),
            "status": "error"
        }), 500


def main():
    """Start the Flask server."""
    logger.info("Starting VQA HTTP service")
    init_model()
    logger.info("VQA service is ready on http://localhost:8000")
    app.run(host='0.0.0.0', port=8000, debug=False)


if __name__ == "__main__":
    main()
