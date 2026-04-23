#!/usr/bin/env python3

# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

"""
Simple HTTP object detection service.

Endpoints:
- GET /health
- POST /detect (image bytes in request body)
"""

import logging
import os
import base64
from io import BytesIO

from ultralytics import YOLO
from flask import Flask, jsonify, request
from PIL import Image


logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

detector = None
device = None


def init_model():
	"""Load object detection model on startup (lazy if needed)."""
	global detector, device

	if detector is not None:
		return

	model_name = os.getenv("OBJECT_DETECTION_MODEL", "yolov8n.pt")
	device = "cpu"

	logger.info("Loading object detector model: %s", model_name)
	logger.info("Using device: %s", device)

	detector = YOLO(model_name)
	detector.to(device)
	logger.info("Object detector is ready")


@app.route("/health", methods=["GET"])
def health():
	"""Basic health endpoint."""
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
	"""
	Detect objects in an image.

	Expects image bytes in request body.
	Optional query params:
	- threshold (float): confidence threshold, default 0.5
	"""
	global detector

	if detector is None:
		init_model()

	try:
		image_bytes = request.data
		if not image_bytes:
			return jsonify({"error": "No image data provided"}), 400

		threshold = float(request.args.get("threshold", "0.5"))
		if threshold < 0 or threshold > 1:
			return jsonify({"error": "threshold must be between 0 and 1"}), 400

		image = Image.open(BytesIO(image_bytes)).convert("RGB")
		prediction_batch = detector(image, verbose=False)
		prediction = prediction_batch[0]

		results = []
		for box in prediction.boxes:
			score = float(box.conf.item())
			if score < threshold:
				continue

			x1, y1, x2, y2 = box.xyxy[0].tolist()
			class_id = int(box.cls.item())
			label = prediction.names.get(class_id, str(class_id))

			results.append(
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

		return (
			jsonify(
				{
					"status": "success",
					"count": len(results),
					"image_b64": base64.b64encode(image_bytes).decode("ascii"),
					"detections": results,
				}
			),
			200,
		)
	except Exception as exc:
		logger.exception("Error while processing image")
		return jsonify({"status": "error", "error": str(exc)}), 500


def main():
	"""Run the service."""
	port = int(os.getenv("PORT", "8081"))
	logger.info("Starting object detection service on http://0.0.0.0:%s", port)
	init_model()
	app.run(host="0.0.0.0", port=port, debug=False)


if __name__ == "__main__":
	main()
