# Detections Consensus Example

This example runs two object detection model steps and combines their predictions through a vote-based consensus stage.

It demonstrates the scenario where both models detect the same high-level objects (for example `dog`) and a detection is accepted only when multiple models agree.

Both detector services load weights from the local file `yolov8n.pt` placed in this directory.

Detector images are configured for CPU-only PyTorch wheels, so Docker should not download CUDA/cuDNN runtime packages.

## How it works

1. `detector-a` performs object detection on the incoming image.
2. `detector-b` performs object detection on the same image.
3. `detections-consensus` clusters overlapping boxes and accepts only clusters with enough model votes.

Consensus is configurable with:

- `iou_threshold`: minimum IoU to consider two boxes part of the same cluster.
- `vote_threshold`: minimum number of distinct model votes required.
- `same_label_only`: if `true`, only boxes with matching labels can vote together.

## Run

From this folder:

```sh
docker compose up --build
```

In another terminal from repository root:

```sh
go run main.go examples/detections_consensus/pipeline.yaml
```

Then send a request:

```sh
curl -X POST -H "Content-Type: image/jpeg" --data-binary @examples/detections_consensus/dogs.jpg http://localhost:8080/
```

## Expected output fields

- `consensus`: active fusion configuration.
- `input_counts`: number of detections produced by each model.
- `fused_count`: number of consensus detections accepted.
- `fused_detections`: final boxes accepted by consensus.
