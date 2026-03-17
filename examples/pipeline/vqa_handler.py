"""
Visual Question Answering Service using BLIP model and PyTriton.

This microservice serves a BLIP model for visual question answering tasks.
It accepts images (JPEG/PNG) and returns text descriptions.
"""

import base64

import numpy as np
from io import BytesIO
from PIL import Image
from typing import List
import json

# Import transformers for BLIP model
from transformers import BlipProcessor, BlipForConditionalGeneration
import torch

# Configure logging
import time


class VQAModel:
    """Visual Question Answering model wrapper for BLIP."""
    
    def __init__(self, model_name: str = "Salesforce/blip-image-captioning-base"):
        """
        Initialize the BLIP model and processor.
        
        Args:
            model_name: HuggingFace model identifier
        """
        print(f"Loading BLIP model: {model_name}")
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        print(f"Using device: {self.device}")
        
        self.processor = BlipProcessor.from_pretrained(model_name)
        self.model = BlipForConditionalGeneration.from_pretrained(model_name).to(self.device)
        print("Model loaded successfully")
    
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


def init_model(context):
    """Initialize the VQA model at server startup."""
    vqa_model = VQAModel()
    setattr(context, "model", vqa_model)

def init_context(context):
    init_model(context)

def handler(context, event):
    """
    Inference function for the VQA model.
    
    Args:
        images: Batch of image bytes as numpy array
        
    Returns:
        List containing numpy array of text descriptions
    """
    request = InferenceRequest(event.body)
    vqa_model = getattr(context, 'model', None)
    if vqa_model is None:
        init_model(context)
    
    vqa_model = context.model
    image_bytes = request.inputs[0].data

    caption = ""
    try:
        # Convert bytes to PIL Image
        image_bytes_clean = bytes(image_bytes)
        image = Image.open(BytesIO(image_bytes_clean)).convert('RGB')
        # time.sleep(2)  # Simulate processing time
        # Generate caption

        if request.parameters and "question" in request.parameters:
            question = request.parameters["question"]
            caption = vqa_model.generate_caption(image, question)
        else:
            caption = vqa_model.generate_caption(image)
        
        context.logger.info(f"Generated caption: {caption}")
        
    except Exception as e:
        context.logger.error(f"Error processing image: {e}")
        # caption = f"Error: {str(e)}"
    

    # Convert results to numpy array with object dtype for variable-length strings
    return {
        "outputs": 
            [
                {"name": "caption", "datatype": "BYTES", "data": [caption], "shape": [1, len(caption)]}
            ]
        
    }

class InferenceRequest:
    id: str | None = None
    parameters: dict | None = None
    inputs: list = []
    outputs: list = []

    def __init__(self, request: dict) -> None:
        self.id = request.get("id")
        self.parameters = request.get("parameters")
        self.inputs = [RequestInput(**input) for input in request.get("inputs", [])]
        self.outputs = [RequestOutput(**output) for output in request.get("outputs", [])]

    def __repr__(self) -> str:
        return self.__str__()

    def __str__(self) -> str:
        return f"InferenceRequest(id={self.id}, parameters={self.parameters}, inputs={self.inputs}, outputs={self.outputs})"

    def dict(self) -> dict:
        return {
            "id": self.id,
            "parameters": self.parameters,
            "inputs": [i.dict() for i in self.inputs],
            "outputs": [o.dict() for o in self.outputs],
        }

    def json(self) -> str:
        return json.dumps(self.dict())


class RequestInput:
    name: str
    datatype: str
    shape: list[int]
    data: list[any]
    parameters: dict | None = {}

    def __init__(self, **kwargs) -> None:
        self.name = kwargs.get("name")
        self.datatype = kwargs.get("datatype")
        self.shape = kwargs.get("shape")
        self.data = kwargs.get("data")
        if self.datatype == "BYTES" and "data" in kwargs:
            self.data = [base64.b64decode(d) for d in kwargs.get("data", [])]
        self.parameters = kwargs.get("parameters")

    def __repr__(self) -> str:
        return self.__str__()

    def __str__(self) -> str:
        return f"RequestInput(name={self.name}, datatype={self.datatype}, shape={self.shape}, data={self.data}, parameters={self.parameters})"

    def dict(self) -> dict:
        return {
            "name": self.name,
            "datatype": self.datatype,
            "shape": self.shape,
            "data": self.data,
            "parameters": self.parameters,
        }

    def json(self) -> str:
        return json.dumps(self.dict())


class RequestOutput:
    name: str
    datatype: str
    shape: list[int]
    parameters: dict | None = {}

    def __init__(self, **kwargs) -> None:
        self.name = kwargs.get("name")
        self.datatype = kwargs.get("datatype")
        self.shape = kwargs.get("shape")
        self.parameters = kwargs.get("parameters")

    def __repr__(self) -> str:
        return self.__str__()

    def __str__(self) -> str:
        return f"RequestOutput(name={self.name}, datatype={self.datatype}, shape={self.shape}, parameters={self.parameters})"

    def dict(self) -> dict:
        return {
            "name": self.name,
            "datatype": self.datatype,
            "shape": self.shape,
            "parameters": self.parameters,
        }

    def json(self) -> str:
        return json.dumps(self.dict())
