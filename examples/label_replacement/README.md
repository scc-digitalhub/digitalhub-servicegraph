<!--
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
-->

# Label Replacement Example

This example shows how to combine a generic object detector with a more specific classifier and keep the result attached to the original object locations.

It demonstrates the scenario where the detector finds high-level objects such as `dog`, while the classifier predicts more specific labels such as dog breeds. Instead of returning a detached list of crop classifications, the workflow replaces the original detection labels with the classifier predictions and preserves the original bounding boxes.

The detector loads weights from the local file `yolov8n.pt` placed in this directory.

The detector image is configured for CPU-only PyTorch wheels, so Docker should not download CUDA/cuDNN runtime packages. The dog breed classifier downloads its model from Hugging Face on first startup.

## How it works

1. `object-detection` runs object detection on the input image and returns bounding boxes.
2. `image-cropper` crops each detected object in memory and forwards the crop payloads for classification.
3. `dog-breed-classifier` predicts a breed for each crop.
4. The classifier replaces the original `dog` labels with breed labels, draws the relabeled boxes onto the original image, and saves one annotated result image.

## Run

From this folder:

```sh
docker compose up --build
```

In another terminal from repository root:

```sh
go run main.go examples/label_replacement/pipeline.yaml
```

Then send a request:

```sh
curl -X POST -H "Content-Type: image/jpeg" --data-binary @examples/label_replacement/dogs.jpg http://localhost:8080/
```

## Expected output fields

- `count`: number of classified detections.
- `results`: raw classifier outputs for each cropped detection.
- `replaced_detections`: original boxes with labels replaced by breed predictions.
- `annotated_image`: saved annotated image metadata including filename and path.