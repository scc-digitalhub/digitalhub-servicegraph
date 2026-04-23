#!/usr/bin/env python3

# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

"""
Simple HTTP dog breed classification service.

Endpoints:
- GET /health
- POST /predict (multipart form file field: image)
"""

import json
import logging
import os
import base64
from io import BytesIO

import huggingface_hub
import torch
from flask import Flask, jsonify, request
from PIL import Image
from torchvision import transforms
from torchvision.models import resnet50


logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

classifier = None
id2breed = None
transform = transforms.Compose([
    transforms.Resize((224, 224)),
    transforms.ToTensor(),
])
device = "cpu"


def init_model():
    """Load dog breed classifier model on startup."""
    global classifier, id2breed

    if classifier is not None and id2breed is not None:
        return

    repo_id = os.getenv("DOG_BREED_MODEL_REPO", "djhua0103/dog-breed-resnet50")
    ckpt_file = os.getenv("DOG_BREED_MODEL_FILE", "resnet50_dog_best.pth")
    labels_file = os.getenv("DOG_BREED_LABELS_FILE", "id2breed.json")

    logger.info("Loading dog breed model from %s", repo_id)

    ckpt_path = huggingface_hub.hf_hub_download(repo_id, ckpt_file)
    labels_path = huggingface_hub.hf_hub_download(repo_id, labels_file)

    with open(labels_path, "r", encoding="utf-8") as f:
        id2breed = json.load(f)

    model = resnet50(weights=None)
    model.fc = torch.nn.Linear(model.fc.in_features, len(id2breed))

    state = torch.load(ckpt_path, map_location=device)
    state_dict = state.get("model_state", state)
    model.load_state_dict(state_dict)

    model.eval()
    classifier = model
    logger.info("Dog breed classifier is ready with %d classes", len(id2breed))


def _load_image_from_request():
    """Accept image from multipart file field or raw request body."""
    if request.is_json:
        payload = request.get_json(silent=True) or {}
        if payload.get("image_b64"):
            data = base64.b64decode(payload["image_b64"])
            if not data:
                raise ValueError("Empty image_b64 payload")
            return Image.open(BytesIO(data)).convert("RGB")
        if payload.get("image_path"):
            image_path = payload["image_path"]
            with open(image_path, "rb") as f:
                return Image.open(BytesIO(f.read())).convert("RGB")

    if "image" in request.files:
        data = request.files["image"].read()
        if not data:
            raise ValueError("Empty image file")
        return Image.open(BytesIO(data)).convert("RGB")

    if request.data:
        return Image.open(BytesIO(request.data)).convert("RGB")

    raise ValueError("No image provided. Use multipart field 'image' or raw image bytes")


def _predict_single_image(image, top_k):
    """Run model inference for a single PIL image and return top-k predictions."""
    x = transform(image).unsqueeze(0)

    with torch.no_grad():
        logits = classifier(x)
        probs = torch.softmax(logits, dim=1)
        values, indices = torch.topk(probs, k=min(top_k, probs.shape[1]), dim=1)

    predictions = []
    for score, idx in zip(values[0].tolist(), indices[0].tolist()):
        key = str(int(idx))
        predictions.append(
            {
                "class_id": int(idx),
                "breed": id2breed.get(key, f"class_{idx}"),
                "score": float(score),
            }
        )

    return predictions


@app.route("/health", methods=["GET"])
def health():
    """Basic health endpoint."""
    return (
        jsonify(
            {
                "status": "healthy",
                "model_loaded": classifier is not None,
                "device": device,
            }
        ),
        200,
    )


@app.route("/predict", methods=["POST"])
def predict():
    """Predict dog breed from image."""
    global classifier, id2breed

    if classifier is None or id2breed is None:
        init_model()

    try:
        top_k = int(request.args.get("top_k", "3"))
        if top_k < 1:
            return jsonify({"error": "top_k must be >= 1"}), 400

        if request.is_json:
            payload = request.get_json(silent=True) or {}
            images_b64 = payload.get("images_b64")
            if isinstance(images_b64, list):
                if not images_b64:
                    return jsonify({"error": "images_b64 must not be empty"}), 400

                batch_results = []
                for idx, image_b64 in enumerate(images_b64):
                    if not image_b64:
                        batch_results.append({
                            "index": idx,
                            "status": "error",
                            "error": "empty image_b64 value",
                        })
                        continue

                    try:
                        image_data = base64.b64decode(image_b64)
                        image = Image.open(BytesIO(image_data)).convert("RGB")
                        predictions = _predict_single_image(image, top_k)
                        batch_results.append(
                            {
                                "index": idx,
                                "status": "success",
                                "prediction": predictions[0],
                                "predictions": predictions,
                            }
                        )
                    except Exception as exc:
                        batch_results.append(
                            {
                                "index": idx,
                                "status": "error",
                                "error": str(exc),
                            }
                        )

                return (
                    jsonify(
                        {
                            "status": "success",
                            "top_k": top_k,
                            "count": len(batch_results),
                            "results": batch_results,
                        }
                    ),
                    200,
                )

        image = _load_image_from_request()
        predictions = _predict_single_image(image, top_k)

        return (
            jsonify(
                {
                    "status": "success",
                    "top_k": top_k,
                    "prediction": predictions[0],
                    "predictions": predictions,
                }
            ),
            200,
        )
    except ValueError as exc:
        return jsonify({"status": "error", "error": str(exc)}), 400
    except Exception as exc:
        logger.exception("Error during breed prediction")
        return jsonify({"status": "error", "error": str(exc)}), 500


def main():
    """Run the service."""
    port = int(os.getenv("PORT", "8083"))
    logger.info("Starting dog breed classifier service on http://0.0.0.0:%s", port)
    init_model()
    app.run(host="0.0.0.0", port=port, debug=False)


if __name__ == "__main__":
    main()
