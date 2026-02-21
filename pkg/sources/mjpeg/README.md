# MJPEG Source

The MJPEG source implementation allows you to connect to MJPEG (Motion JPEG) streams and process individual JPEG frames as events in the servicegraph pipeline.

## Overview

MJPEG is a video streaming format where each frame is sent as a separate JPEG image within an HTTP multipart stream. This source implementation:

- Connects to an MJPEG stream via HTTP
- Parses the multipart stream to extract individual JPEG frames
- Wraps each JPEG frame as an Event
- Supports frame sampling to reduce processing load
- Passes frames to the configured processing flow

## Configuration

The MJPEG source is configured using the following parameters:

```yaml
input:
  kind: "mjpeg"
  spec:
    url: "http://example.com/mjpeg/stream"  # Required: MJPEG stream URL
    frame_interval: 1                        # Optional: Process every Nth frame (default: 1)
    read_timeout: 10                         # Optional: Read timeout in seconds (default: 10)
```

### Parameters

- **url** (required): The HTTP URL of the MJPEG stream
- **frame_interval** (optional): Process every Nth frame. For example:
  - `1` = process every frame (default)
  - `2` = process every 2nd frame (skip odd frames)
  - `5` = process every 5th frame (useful for reducing CPU load)
- **read_timeout** (optional): Maximum time in seconds to wait for data (default: 10)

## Usage Example

### Basic Frame Processing

```yaml
input:
  kind: "mjpeg"
  spec:
    url: "http://camera.example.com/mjpeg"
    frame_interval: 1
flow:
  type: "sequence"
  name: "frame-processor"
  nodes:
    - type: "service"
      name: "image-analyzer"
      config:
        kind: "http"
        spec:
          url: "http://localhost:9000/analyze"
          method: "POST"
          headers:
            Content-Type: "image/jpeg"
```

### Frame Sampling

Process only every 10th frame to reduce load:

```yaml
input:
  kind: "mjpeg"
  spec:
    url: "http://camera.example.com/mjpeg"
    frame_interval: 10
    read_timeout: 30
flow:
  type: "sequence"
  name: "sampled-processor"
  nodes:
    - type: "service"
      name: "motion-detector"
      config:
        kind: "http"
        spec:
          url: "http://localhost:9000/detect-motion"
          method: "POST"
```

## Event Structure

Each MJPEG frame is wrapped as an `MJPEGEvent` with the following properties:

- **Body**: The raw JPEG image data (bytes)
- **Content-Type**: Always `"image/jpeg"`
- **Frame Number**: Sequential frame number (1-indexed)
- **Timestamp**: When the frame was received
- **URL**: The source MJPEG stream URL
- **Context**: Go context for cancellation and tracing

### Accessing Event Properties

Events can be accessed in the processing flow:

```go
event := msg.(*mjpeg.MJPEGEvent)
jpegData := event.GetBody()           // []byte
contentType := event.GetContentType() // "image/jpeg"
frameNum := event.GetFrameNumber()    // int64
timestamp := event.GetTimestamp()     // time.Time
url := event.GetURL()                 // string
```

## Features

### Automatic Stream Parsing

The source automatically:
- Detects multipart boundaries
- Extracts JPEG frames from the stream
- Handles connection errors gracefully
- Supports graceful shutdown on SIGINT/SIGTERM

### Frame Sampling

Use the `frame_interval` parameter to reduce processing load:
- Useful for high frame rate streams
- Reduces CPU and network bandwidth usage
- Maintains temporal consistency

### Error Handling

The source handles various error conditions:
- Connection failures
- Invalid content types
- Malformed multipart streams
- Read timeouts

## Testing

Run the tests:

```bash
go test ./pkg/sources/mjpeg/... -v
```

## Implementation Notes

- The source uses Go's standard `mime/multipart` package for parsing
- Each frame is processed in a streaming fashion (no buffering of entire stream)
- Context cancellation is supported for clean shutdown
- Compatible with both synchronous and asynchronous flow modes

## Common MJPEG Sources

MJPEG is commonly used for:
- IP security cameras
- Webcam streams
- Video surveillance systems
- Computer vision pipelines

Many IP cameras expose MJPEG streams at URLs like:
- `http://camera-ip/mjpeg`
- `http://camera-ip/video.mjpg`
- `http://camera-ip/cgi-bin/mjpeg`

Check your camera's documentation for the specific MJPEG endpoint.
