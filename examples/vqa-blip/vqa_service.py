#!/usr/bin/env python3
"""
Visual Question Answering Service using BLIP model and PyTriton.

This microservice serves a BLIP model for visual question answering tasks.
It accepts images (JPEG/PNG) and returns text descriptions.
"""

import logging
import numpy as np
from io import BytesIO
from PIL import Image
from typing import List

from pytriton.decorators import batch
from pytriton.model_config import ModelConfig, Tensor
from pytriton.triton import Triton

# Import transformers for BLIP model
from transformers import BlipProcessor, BlipForConditionalGeneration
import torch

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class VQAModel:
    """Visual Question Answering model wrapper for BLIP."""
    
    def __init__(self, model_name: str = "Salesforce/blip-image-captioning-base"):
        """
        Initialize the BLIP model and processor.
        
        Args:
            model_name: HuggingFace model identifier
        """
        logger.info(f"Loading BLIP model: {model_name}")
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        logger.info(f"Using device: {self.device}")
        
        self.processor = BlipProcessor.from_pretrained(model_name)
        self.model = BlipForConditionalGeneration.from_pretrained(model_name).to(self.device)
        logger.info("Model loaded successfully")
    
    def generate_caption(self, image: Image.Image, question: str = None) -> str:
        """
        Generate a caption or answer for the given image.
        
        Args:
            image: PIL Image
            question: Optional question text for VQA
            
        Returns:
            Generated text description
        """
        if question:
            # Visual question answering
            inputs = self.processor(image, question, return_tensors="pt").to(self.device)
        else:
            # Image captioning
            inputs = self.processor(image, return_tensors="pt").to(self.device)
        
        with torch.no_grad():
            output = self.model.generate(**inputs, max_new_tokens=50)
        
        caption = self.processor.decode(output[0], skip_special_tokens=True)
        return caption


# Global model instance
vqa_model = None


def init_model():
    """Initialize the VQA model at server startup."""
    global vqa_model
    if vqa_model is None:
        vqa_model = VQAModel()


@batch
def infer_fn(images: np.ndarray) -> List[np.ndarray]:
    """
    Inference function for the VQA model.
    
    Args:
        images: Batch of image bytes as numpy array
        
    Returns:
        List containing numpy array of text descriptions
    """
    global vqa_model
    
    if vqa_model is None:
        init_model()
    
    results = []
    
    for image_bytes in images:
        try:
            # Convert bytes to PIL Image
            image_bytes_clean = image_bytes.tobytes()
            image = Image.open(BytesIO(image_bytes_clean)).convert('RGB')
            
            # Generate caption
            caption = vqa_model.generate_caption(image)
            logger.info(f"Generated caption: {caption}")
            
            results.append(caption)
        except Exception as e:
            logger.error(f"Error processing image: {e}")
            results.append(f"Error: {str(e)}")
    
    # Convert results to numpy array with object dtype for variable-length strings
    return [np.array(results, dtype=object)]


def main():
    """Start the PyTriton server with the VQA model."""
    logger.info("Starting VQA service with PyTriton")
    
    # Initialize model before starting server
    init_model()
    
    # Create Triton server
    with Triton() as triton:
        logger.info("Binding VQA model to Triton")
        
        triton.bind(
            model_name="vqa_blip",
            infer_func=infer_fn,
            inputs=[
                Tensor(name="image", dtype=np.uint8, shape=(-1,)),  # Variable-length byte array
            ],
            outputs=[
                Tensor(name="caption", dtype=object, shape=(-1,)),  # Variable-length string array
            ],
            config=ModelConfig(max_batch_size=16),
        )
        
        logger.info("VQA service is ready and serving on http://localhost:8000")
        logger.info("Model endpoint: http://localhost:8000/v2/models/vqa_blip")
        triton.serve()


if __name__ == "__main__":
    main()
