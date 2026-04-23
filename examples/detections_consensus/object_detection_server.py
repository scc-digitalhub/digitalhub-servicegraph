#!/usr/bin/env python3

# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

"""
Simple HTTP object detection service with optional JSON input support.

Endpoints:
- GET /health
- POST /detect (raw image bytes or JSON payload with image_b64)
"""

import base64
import logging
import os
from io import BytesIO

from flask import Flask, jsonify, request
from PIL import Image
from ultralytics import YOLO


logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

detector = None
device = None


def init_model():
    """Load object detection model on startup."""
    global detector, device

    if detector is not None:
        return

    model_name = os.getenv("OBJECT_DETECTION_MODEL", "yolov8n.pt")
    device = "cpu"

    logger.info("Loading object detector model: %s", model_name)
    detector = YOLO(model_name)
    detector.to(device)
    logger.info("Object detector is ready")


def _parse_class_filter(class_filter_raw):
    if not class_filter_raw:
        return set()
    return {item.strip().lower() for item in class_filter_raw.split(",") if item.strip()}


def _load_image_and_passthrough():
    """Accept image from raw bytes or JSON payload and return optional passthrough data."""
    passthrough = None

    if request.is_json:
        payload = request.get_json(silent=True) or {}
        image_b64 = payload.get("image_b64")
        passthrough = payload.get("passthrough")
        if image_b64:
            image_bytes = base64.b64decode(image_b64)
            if not image_bytes:
                raise ValueError("Empty image_b64 payload")
            image = Image.open(BytesIO(image_bytes)).convert("RGB")
            return image, image_bytes, passthrough

    image_bytes = request.data
    if not image_bytes:
        raise ValueError("No image data provided")

    image = Image.open(BytesIO(image_bytes)).convert("RGB")
    return image, image_bytes, passthrough


@app.route("/health", methods=["GET"])
def health():
    return (
        jsonify(
            {
                "status": "healthy",
                "model_loaded": detector is not None,
                "device": device or "cpu",
            }
        ),
        200,
    )


@app.route("/detect", methods=["POST"])
def detect():
    global detector

    if detector is None:
        init_model()

    try:
        threshold = float(request.args.get("threshold", "0.4"))
        if threshold < 0 or threshold > 1:
            return jsonify({"error": "threshold must be between 0 and 1"}), 400

        class_filter = _parse_class_filter(request.args.get("class_filter", ""))

        image, image_bytes, passthrough = _load_image_and_passthrough()
        prediction = detector(image, verbose=False)[0]

        detections = []
        for box in prediction.boxes:
            score = float(box.conf.item())
            if score < threshold:
                continue

            x1, y1, x2, y2 = box.xyxy[0].tolist()
            class_id = int(box.cls.item())
            label = str(prediction.names.get(class_id, class_id))

            if class_filter and label.lower() not in class_filter:
                continue

            detections.append(
                {
                    "label": label,
                    "score": score,
                    "box": {
                        "xmin": int(x1),
                        "ymin": int(y1),
                        "xmax": int(x2),
                        "ymax": int(y2),
                    },
                }
            )

        response = {
            "status": "success",
            "model": os.getenv("OBJECT_DETECTION_MODEL", "yolov8n.pt"),
            "count": len(detections),
            "image_b64": base64.b64encode(image_bytes).decode("ascii"),
            "detections": detections,
        }
        if passthrough is not None:
            response["passthrough"] = passthrough

        return jsonify(response), 200
    except ValueError as exc:
        return jsonify({"status": "error", "error": str(exc)}), 400
    except Exception as exc:
        logger.exception("Error during detection")
        return jsonify({"status": "error", "error": str(exc)}), 500


def main():
    port = int(os.getenv("PORT", "8081"))
    logger.info("Starting object detection service on http://0.0.0.0:%s", port)
    init_model()
    app.run(host="0.0.0.0", port=port, debug=False)


if __name__ == "__main__":
    main()
