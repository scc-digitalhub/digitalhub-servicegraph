#!/usr/bin/env python3
"""
HTTP service that fuses detections from two models using vote-based consensus.

Endpoints:
- GET /health
- POST /fuse
"""

import logging
import os
from collections import Counter

from flask import Flask, jsonify, request


logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)


def _validate_box(box):
    try:
        return {
            "xmin": int(box["xmin"]),
            "ymin": int(box["ymin"]),
            "xmax": int(box["xmax"]),
            "ymax": int(box["ymax"]),
        }
    except Exception:
        return None


def _iou(box1, box2):
    x_left = max(box1["xmin"], box2["xmin"])
    y_top = max(box1["ymin"], box2["ymin"])
    x_right = min(box1["xmax"], box2["xmax"])
    y_bottom = min(box1["ymax"], box2["ymax"])

    if x_right <= x_left or y_bottom <= y_top:
        return 0.0

    intersection = (x_right - x_left) * (y_bottom - y_top)
    area1 = (box1["xmax"] - box1["xmin"]) * (box1["ymax"] - box1["ymin"])
    area2 = (box2["xmax"] - box2["xmin"]) * (box2["ymax"] - box2["ymin"])
    union = area1 + area2 - intersection

    if union <= 0:
        return 0.0
    return float(intersection / union)


def _normalize_detections(detections, model_idx):
    normalized = []
    for detection in detections or []:
        if not isinstance(detection, dict):
            continue

        box = _validate_box(detection.get("box", {}))
        if box is None:
            continue

        normalized.append(
            {
                "model_idx": model_idx,
                "label": str(detection.get("label", "unknown")),
                "score": float(detection.get("score", 0.0)),
                "box": box,
            }
        )
    return normalized


def _find_best_cluster(clusters, candidate, iou_threshold, same_label_only):
    best_idx = -1
    best_iou = 0.0

    for idx, cluster in enumerate(clusters):
        iou_val = _iou(candidate["box"], cluster["box_avg"])
        if iou_val < iou_threshold:
            continue

        if same_label_only and candidate["label"].lower() != cluster["label_majority"].lower():
            continue

        if iou_val > best_iou:
            best_iou = iou_val
            best_idx = idx

    return best_idx


def _recompute_cluster_stats(cluster):
    members = cluster["members"]
    total_weight = sum(max(member["score"], 1e-6) for member in members)

    def weighted_coord(coord):
        weighted_sum = sum(member["box"][coord] * max(member["score"], 1e-6) for member in members)
        return int(round(weighted_sum / total_weight))

    cluster["box_avg"] = {
        "xmin": weighted_coord("xmin"),
        "ymin": weighted_coord("ymin"),
        "xmax": weighted_coord("xmax"),
        "ymax": weighted_coord("ymax"),
    }

    label_counter = Counter(member["label"] for member in members)
    cluster["label_majority"] = label_counter.most_common(1)[0][0]


def _cluster_detections(all_detections, iou_threshold, same_label_only):
    clusters = []
    for candidate in all_detections:
        cluster_idx = _find_best_cluster(clusters, candidate, iou_threshold, same_label_only)

        if cluster_idx < 0:
            clusters.append(
                {
                    "members": [candidate],
                    "box_avg": dict(candidate["box"]),
                    "label_majority": candidate["label"],
                }
            )
            continue

        clusters[cluster_idx]["members"].append(candidate)
        _recompute_cluster_stats(clusters[cluster_idx])

    return clusters


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "healthy", "mode": "detections-consensus"}), 200


@app.route("/fuse", methods=["POST"])
def fuse():
    try:
        payload = request.get_json(silent=True) or {}
        detections_a = payload.get("detections_a", [])
        detections_b = payload.get("detections_b", [])
        model_names = payload.get("model_names", ["detector-a", "detector-b"])

        iou_threshold = float(request.args.get("iou_threshold", "0.5"))
        vote_threshold = int(request.args.get("vote_threshold", "2"))
        same_label_only = str(request.args.get("same_label_only", "true")).lower() in {
            "1",
            "true",
            "yes",
            "on",
        }

        if not 0.0 <= iou_threshold <= 1.0:
            return jsonify({"error": "iou_threshold must be between 0 and 1"}), 400
        if vote_threshold < 1:
            return jsonify({"error": "vote_threshold must be >= 1"}), 400

        normalized = _normalize_detections(detections_a, 0) + _normalize_detections(detections_b, 1)
        clusters = _cluster_detections(normalized, iou_threshold, same_label_only)

        fused = []
        for cluster in clusters:
            member_models = sorted({member["model_idx"] for member in cluster["members"]})
            votes = len(member_models)
            if votes < vote_threshold:
                continue

            scores = [member["score"] for member in cluster["members"]]
            contributing_models = [
                model_names[idx] if idx < len(model_names) else f"model_{idx}"
                for idx in member_models
            ]

            fused.append(
                {
                    "label": cluster["label_majority"],
                    "score": sum(scores) / max(len(scores), 1),
                    "votes": votes,
                    "consensus_models": contributing_models,
                    "box": cluster["box_avg"],
                }
            )

        return (
            jsonify(
                {
                    "status": "success",
                    "consensus": {
                        "iou_threshold": iou_threshold,
                        "vote_threshold": vote_threshold,
                        "same_label_only": same_label_only,
                    },
                    "input_counts": {
                        "model_a": len(detections_a or []),
                        "model_b": len(detections_b or []),
                    },
                    "fused_count": len(fused),
                    "fused_detections": fused,
                }
            ),
            200,
        )
    except Exception as exc:
        logger.exception("Error while fusing detections")
        return jsonify({"status": "error", "error": str(exc)}), 500


def main():
    port = int(os.getenv("PORT", "8083"))
    logger.info("Starting detections consensus service on http://0.0.0.0:%s", port)
    app.run(host="0.0.0.0", port=port, debug=False)


if __name__ == "__main__":
    main()
