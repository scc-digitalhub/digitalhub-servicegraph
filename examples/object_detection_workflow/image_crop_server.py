#!/usr/bin/env python3

# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

"""
Simple HTTP image cropping service.

Takes an image and bounding box coordinates, crops the image(s), and saves locally.

Endpoints:
- GET /health
- POST /crop (image bytes + JSON with box coordinates)
"""

import logging
import os
import json
import base64
from io import BytesIO
from pathlib import Path
from datetime import datetime

from PIL import Image
from flask import Flask, jsonify, request


logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Create images folder if it doesn't exist
IMAGES_FOLDER = Path("images")
IMAGES_FOLDER.mkdir(exist_ok=True)

logger.info("Images will be saved to: %s", IMAGES_FOLDER.absolute())


@app.route("/health", methods=["GET"])
def health():
	"""Basic health endpoint."""
	return (
		jsonify(
			{
				"status": "healthy",
				"images_folder": str(IMAGES_FOLDER.absolute()),
			}
		),
		200,
	)


@app.route("/crop", methods=["POST"])
def crop():
	"""
	Crop image(s) based on bounding box coordinates.

	Expects:
	- image bytes in request data (multipart form 'image')
	- JSON with box coordinates in request form 'boxes' or query param 'boxes'

	Box format (array of objects):
	[
		{
			"label": "person",
			"score": 0.89,
			"box": {
				"xmin": 593,
				"ymin": 2,
				"xmax": 1121,
				"ymax": 800
			}
		},
		... more boxes ...
	]
	"""
	try:
		# Get image from request
		json_body = request.get_json(silent=True) if request.is_json else None
		if "image" in request.files:
			image_file = request.files["image"]
			image = Image.open(BytesIO(image_file.read())).convert("RGB")
		elif json_body and json_body.get("image_b64"):
			image_bytes = base64.b64decode(json_body["image_b64"])
			image = Image.open(BytesIO(image_bytes)).convert("RGB")
		elif request.data:
			image = Image.open(BytesIO(request.data)).convert("RGB")
		else:
			return jsonify({"error": "No image file provided"}), 400
		logger.info("Loaded image with size: %s", image.size)

		# Get boxes from request
		boxes_data = None
		
		if "boxes" in request.form:
			boxes_data = json.loads(request.form["boxes"])
		elif json_body and "boxes" in json_body:
			boxes_data = json_body["boxes"]
		elif "boxes" in request.args:
			boxes_data = json.loads(request.args.get("boxes"))
		else:
			return jsonify({"error": "No boxes data provided"}), 400

		if not isinstance(boxes_data, list):
			boxes_data = [boxes_data]

		# Crop and save images
		timestamp = datetime.now().strftime("%Y%m%d_%H%M%S_%f")[:-3]
		saved_files = []

		for idx, box_info in enumerate(boxes_data):
			try:
				# Extract box coordinates
				if isinstance(box_info, dict) and "box" in box_info:
					box = box_info["box"]
					label = box_info.get("label", f"object_{idx}")
					score = box_info.get("score", 0)
				else:
					# Direct box format
					box = box_info
					label = f"object_{idx}"
					score = 0

				xmin = int(box["xmin"])
				ymin = int(box["ymin"])
				xmax = int(box["xmax"])
				ymax = int(box["ymax"])

				# Validate coordinates
				if xmin < 0 or ymin < 0 or xmax > image.width or ymax > image.height:
					logger.warning(
						"Box %d out of bounds: (%d, %d, %d, %d) for image size %s. Clipping.",
						idx,
						xmin,
						ymin,
						xmax,
						ymax,
						image.size,
					)
					xmin = max(0, xmin)
					ymin = max(0, ymin)
					xmax = min(image.width, xmax)
					ymax = min(image.height, ymax)

				# Crop image
				cropped = image.crop((xmin, ymin, xmax, ymax))

				# Save cropped image and keep encoded bytes for downstream services.
				buffer = BytesIO()
				cropped.save(buffer, format="JPEG", quality=95)
				encoded_crop = base64.b64encode(buffer.getvalue()).decode("ascii")

				# Save cropped image
				filename = f"{timestamp}_{idx:02d}_{label}.jpg"
				filepath = IMAGES_FOLDER / filename
				cropped.save(filepath, quality=95)

				saved_files.append(
					{
						"index": idx,
						"label": label,
						"score": score,
						"filename": filename,
						"path": str(filepath),
						"size": cropped.size,
						"image_b64": encoded_crop,
						"box": {
							"xmin": xmin,
							"ymin": ymin,
							"xmax": xmax,
							"ymax": ymax,
						},
					}
				)
				logger.info("Saved cropped image: %s (size: %s)", filename, cropped.size)

			except Exception as e:
				logger.exception("Error processing box %d", idx)
				saved_files.append(
					{
						"index": idx,
						"error": str(e),
					}
				)

		return (
			jsonify(
				{
					"status": "success",
					"count": len(saved_files),
					"saved_images": saved_files,
				}
			),
			200,
		)

	except Exception as exc:
		logger.exception("Error while processing crop request")
		return jsonify({"status": "error", "error": str(exc)}), 500


def main():
	"""Run the service."""
	port = int(os.getenv("PORT", "8082"))
	logger.info("Starting image crop service on http://0.0.0.0:%s", port)
	logger.info("Cropped images will be saved to: %s", IMAGES_FOLDER.absolute())
	app.run(host="0.0.0.0", port=port, debug=False)


if __name__ == "__main__":
	main()
