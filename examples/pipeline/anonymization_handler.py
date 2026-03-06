"""
Video Anonymization Service using face detection and blurring.

This microservice provides HTTP endpoints for anonymizing images by detecting
and blurring faces (and optionally license plates). It integrates with the
digitalhub-servicegraph framework for real-time video stream processing.
"""

from urllib import request

import numpy as np
from io import BytesIO
from PIL import Image
import json
import cv2


def init_model(context):
    """Initialize face detection models at server startup."""
    # Fallback to Haar Cascade (comes with OpenCV)
    context.logger.info("Loading Haar Cascade face detector...")
    model = cv2.CascadeClassifier(
        cv2.data.haarcascades + 'haarcascade_frontalface_default.xml'
    )
    setattr(context, "model", model)
    context.logger.info("Haar Cascade face detector loaded successfully")

def init_context(context):
    init_model(context)


def detect_faces_haar(model, image):
    """
    Detect faces using Haar Cascade.
    
    Args:
        image: OpenCV image (BGR format)
        
    Returns:
        List of (x, y, w, h) tuples for detected faces
    """
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    faces = model.detectMultiScale(
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


def anonymize_image(context, model, image_bytes, method='blur', blur_factor=50, pixel_size=15):
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
    faces = detect_faces_haar(model, image)
    

    # Anonymize each face
    for (x, y, w, h) in faces:
        if method == 'pixelate':
            image = pixelate_region(image, x, y, w, h, pixel_size)
        else:  # blur
            image = blur_region(image, x, y, w, h, blur_factor)
    
    # Convert back to bytes
    _, buffer = cv2.imencode('.jpg', image)
    return buffer.tobytes(), len(faces)

def handler(context, event):
    """
    Anonymize faces in the provided image.
    
    Expects: Image bytes in request body
    Query params:
        - method: 'blur' or 'pixelate' (default: blur)
        - blur_factor: Blur intensity (default: 50)
        - pixel_size: Pixelation size (default: 15)
    
    Returns: Anonymized image bytes
    """
    request = InferenceRequest(event.body)
    model  = getattr(context, 'model', None)
    if model is None:
        init_model(context)
            
    try:
        # Get image from request body
        image_bytes = bytes(request.inputs[0].data)
            
        # Get parameters
        method = request.parameters['method'] if  request.parameters and 'method' in request.parameters else 'blur'
        blur_factor = int(request.parameters['blur_factor']) if request.parameters and 'blur_factor' in request.parameters else 50
        pixel_size = int(request.parameters['pixel_size']) if request.parameters and 'pixel_size' in request.parameters else 15
        
        # Validate method
        if method not in ['blur', 'pixelate']:
            return {"error": "Method must be 'blur' or 'pixelate'"}
        
        # Anonymize the image
        anonymized_bytes, face_count = anonymize_image(
            context, model, image_bytes, method, blur_factor, pixel_size
        )
        
        context.logger.info(f"Anonymized {face_count} face(s) using {method} method")

        anonymized_bytes_list = list(anonymized_bytes)

        # Return anonymized image
        return {
            "outputs": 
                [
                    {"name": "image", "datatype": "UINT8", "data": anonymized_bytes_list, "shape": [1, len(anonymized_bytes_list)]}
                ]            
        }
        
    except Exception as e:
        context.logger.error(f"Error processing image: {e}")
        return {
            "error": str(e),
            "status": "error"
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

