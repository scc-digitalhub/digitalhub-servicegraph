<!--
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
-->

# Video Anonymization Pipeline Example

This example demonstrates a real-time video anonymization pipeline using the **digitalhub-servicegraph** framework. The pipeline processes MJPEG video streams and automatically detects and anonymizes faces using advanced computer vision techniques.

## Overview

The Video Anonymization pipeline consists of:

1. **MJPEG Source**: Captures frames from a video stream (webcam, IP camera, or video file)
2. **HTTP Client Node**: Sends frames to the anonymization service
3. **Anonymization Service**: Detects and blurs/pixelates faces using OpenCV
4. **Output Sinks**: Displays results (console) or saves anonymized frames (file)

### Features

- **Real-time face detection** using DNN-based or Haar Cascade detectors
- **Multiple anonymization methods**: Gaussian blur or pixelation
- **Configurable processing**: Adjustable blur intensity, pixel size, and frame rate
- **Privacy-focused**: Processes video streams locally without cloud dependencies
- **Production-ready**: Docker support for easy deployment
- **Extensible**: Easy to add detection for other objects (e.g., license plates)

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌────────────────────┐     ┌──────────┐
│ MJPEG Source │────>│ HTTP Client  │────>│ Anonymization      │────>│  Stdout  │
│ (Video       │     │ Node         │     │ Service            │     │  Sink    │
│  Stream)     │     │              │     │ (Face Detection)   │     │          │
└──────────────┘     └──────────────┘     └────────────────────┘     └──────────┘
                                                    │
                                                    v
                                          ┌──────────────────┐
                                          │ OpenCV DNN       │
                                          │ Face Detector    │
                                          └──────────────────┘
```

## Face Detection Technology

The service supports two face detection methods:

### DNN-based Face Detector (Recommended)
- **Model**: ResNet-10 SSD face detector
- **Accuracy**: Higher detection rate, works better with varied angles and lighting
- **Performance**: Slightly slower but more reliable
- **Auto-downloaded**: Models are fetched automatically during setup

### Haar Cascade Detector (Fallback)
- **Model**: OpenCV built-in Haar Cascade classifier
- **Accuracy**: Good for frontal faces in good lighting
- **Performance**: Faster processing
- **No downloads**: Comes pre-installed with OpenCV

The service automatically uses DNN detector if models are available, otherwise falls back to Haar Cascade.

## Quick Start

### Prerequisites

- Python 3.8 or higher
- Go 1.20 or higher (for running the service graph)
- MJPEG video stream source (see Testing section)

### Installation

1. **Setup the anonymization service**:
```bash
cd examples/video-anonymization
make setup
```

This will:
- Create a Python virtual environment
- Install dependencies (OpenCV, Flask, etc.)
- Download DNN face detection models (~10MB)

2. **Build the service graph** (from repository root):
```bash
cd ../..
go build -o servicegraph main.go
```

### Running the Pipeline

#### Option 1: Using Make (Recommended)

1. Start the anonymization service:
```bash
make start
```

2. In a separate terminal, run the service graph:
```bash
../../servicegraph -config anonymization-pipeline.yaml
```

#### Option 2: Manual Execution

1. Start the anonymization service:
```bash
source venv/bin/activate
python3 anonymization_service.py
```

2. In a separate terminal, run the service graph:
```bash
../../servicegraph -config anonymization-pipeline.yaml
```

#### Option 3: Docker Compose

```bash
# Build and start all services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

### Testing the Service

Before running the full pipeline, you can test the anonymization service:

```bash
# Start the service
make start

# In another terminal, run tests
make test

# Or test with a specific image
python3 test_service.py path/to/image.jpg
```

**Testing endpoints manually**:

```bash
# Health check
curl http://localhost:8000/health

# Anonymize an image (blur method)
curl -X POST \
  http://localhost:8000/anonymize?method=blur&blur_factor=50 \
  --data-binary @test_image.jpg \
  -H "Content-Type: image/jpeg" \
  -o anonymized.jpg

# Anonymize with pixelation
curl -X POST \
  http://localhost:8000/anonymize?method=pixelate&pixel_size=15 \
  --data-binary @test_image.jpg \
  -H "Content-Type: image/jpeg" \
  -o anonymized_pixelated.jpg

# Get anonymization info (JSON response)
curl -X POST \
  http://localhost:8000/anonymize/info?method=blur \
  --data-binary @test_image.jpg \
  -H "Content-Type: image/jpeg"
```

## Configuration

### Pipeline Configuration

Edit [anonymization-pipeline.yaml](anonymization-pipeline.yaml) to customize the pipeline:

#### MJPEG Source Settings

```yaml
input:
  kind: "mjpeg"
  spec:
    url: "http://localhost:8080/?action=stream"  # MJPEG stream URL
    frame_interval: 5  # Process every Nth frame (1 = all frames)
    read_timeout: 30   # Connection timeout in seconds
```

**Frame Interval Tips**:
- `frame_interval: 1` - Process all frames (highest quality, highest CPU)
- `frame_interval: 5` - Process every 5th frame (good balance)
- `frame_interval: 10` - Process every 10th frame (lower CPU usage)

#### Anonymization Settings

```yaml
flow:
  type: "sequence"
  name: "anonymization-pipeline"
  nodes:
    - type: "service"
      name: "anonymization-service"
      config:
        kind: "http"
        spec:
          url: "http://localhost:8000/anonymize?method=blur&blur_factor=50"
          method: "POST"
          headers:
            Content-Type: "image/jpeg"
```

**Anonymization Methods** (change URL query parameters):
- **blur**: `url: "http://localhost:8000/anonymize?method=blur&blur_factor=50"`
  - `blur_factor`: Controls blur intensity (10-100 recommended)
  - Higher values = more blurring
- **pixelate**: `url: "http://localhost:8000/anonymize?method=pixelate&pixel_size=15"`
  - `pixel_size`: Size of pixels (5-20 recommended)
  - Higher values = more pixelation

#### Output Configuration

The current configuration outputs to stdout:

```yaml
output:
  kind: "stdout"
  spec: {}
```

**Note**: To save anonymized frames to files, you can redirect stdout or modify the anonymization service to save files directly by calling the service with additional parameters.

## MJPEG Stream Sources

You need an MJPEG stream to process. Here are common options:

### Option 1: IP Camera
Many IP cameras support MJPEG streams:
```yaml
url: "http://camera-ip:8080/video"
```

### Option 2: Webcam with Motion
Install and run [Motion](https://motion-project.github.io/):
```bash
# Install Motion
brew install motion  # macOS
# or
sudo apt-get install motion  # Linux

# Edit /etc/motion/motion.conf
# Set: stream_localhost off

# Run Motion
motion

# Use stream URL
url: "http://localhost:8081/"
```

### Option 3: Webcam with FFmpeg
Stream your webcam using FFmpeg:
```bash
# macOS
ffmpeg -f avfoundation -i "0" -f mjpeg -q:v 2 http://localhost:8080/stream

# Linux
ffmpeg -f v4l2 -i /dev/video0 -f mjpeg -q:v 2 http://localhost:8080/stream
```

### Option 4: Test with Video File
Convert a video file to MJPEG stream:
```bash
ffmpeg -re -i input_video.mp4 -f mjpeg -q:v 2 http://localhost:8080/stream
```

## API Reference

### Endpoints

#### `GET /health`
Health check endpoint.

**Response**:
```json
{
  "status": "healthy",
  "detector": "dnn"  // or "haar"
}
```

#### `POST /anonymize`
Anonymize faces in the provided image.

**Request**:
- Body: Image bytes (JPEG/PNG)
- Query Parameters:
  - `method`: `blur` or `pixelate` (default: `blur`)
  - `blur_factor`: Blur intensity (default: `50`)
  - `pixel_size`: Pixel size for pixelation (default: `15`)

**Response**: Anonymized image bytes (JPEG)

**Example**:
```bash
curl -X POST \
  "http://localhost:8000/anonymize?method=blur&blur_factor=60" \
  --data-binary @image.jpg \
  -H "Content-Type: image/jpeg" \
  -o output.jpg
```

#### `POST /anonymize/info`
Anonymize image and return JSON with metadata.

**Response**:
```json
{
  "faces_detected": 2,
  "method": "blur",
  "image": "base64_encoded_image...",
  "status": "success"
}
```

## Performance Tuning

### For Real-time Processing

1. **Adjust frame interval**: Process fewer frames
   ```yaml
   frame_interval: 10  # Process every 10th frame
   ```

2. **Reduce image resolution**: Resize before anonymization
   ```yaml
   # Add preprocessing step to resize images
   ```

3. **Use Haar Cascade**: Faster but less accurate
   ```bash
   # Don't download DNN models, service will use Haar Cascade
   ```

4. **Optimize blur factor**: Lower values are faster
   ```yaml
   blur_factor: "25"  # Lower = faster processing
   ```

### For High Accuracy

1. **Use DNN detector**: More accurate face detection
   ```bash
   ./setup.sh  # Downloads DNN models
   ```

2. **Process all frames**:
   ```yaml
   frame_interval: 1
   ```

3. **Higher blur factor**: Better privacy protection
   ```yaml
   blur_factor: "75"
   ```

## Project Structure

```
video-anonymization/
├── anonymization_service.py      # Flask service with face detection
├── anonymization-pipeline.yaml   # Service graph configuration
├── requirements.txt              # Python dependencies
├── setup.sh                      # Environment setup script
├── start_service.sh             # Service start script
├── test_service.py              # Test suite
├── Dockerfile                   # Docker image definition
├── docker-compose.yml           # Docker Compose configuration
├── Makefile                     # Build automation
├── .gitignore                   # Git ignore rules
└── README.md                    # This file
```

## Troubleshooting

### Service won't start

**Error**: `ModuleNotFoundError: No module named 'cv2'`
- **Solution**: Run `make setup` to install dependencies

**Error**: `Address already in use`
- **Solution**: Port 8000 is in use. Kill the process or change the port in `anonymization_service.py`

### Poor detection accuracy

**Issue**: Faces not detected or false positives
- **Solution 1**: Download DNN models with `./setup.sh`
- **Solution 2**: Adjust lighting and camera angle
- **Solution 3**: Ensure faces are clearly visible and frontal

### Slow processing

**Issue**: Pipeline lagging behind stream
- **Solution 1**: Increase `frameInterval` to process fewer frames
- **Solution 2**: Reduce blur factor: `blur_factor: "25"`
- **Solution 3**: Use Haar Cascade instead of DNN
- **Solution 4**: Reduce stream resolution

### No MJPEG stream available

**Issue**: Don't have an MJPEG source for testing
- **Solution**: Use provided test script to create a simple stream server:
  ```bash
  # Install motion or use ffmpeg (see MJPEG Stream Sources section)
  ```

## Extending the Pipeline

### Add License Plate Detection

Modify `anonymization_service.py` to detect and blur license plates:

```python
# Add license plate detection using OpenCV or pytesseract
def detect_license_plates(image):
    # Your implementation here
    pass

# In anonymize_image function:
plates = detect_license_plates(image)
for (x, y, w, h) in plates:
    image = blur_region(image, x, y, w, h, blur_factor)
```

### Add Custom Preprocessing

Add image preprocessing service to enhance detection:

```yaml
flow:
  type: "sequence"
  name: "anonymization-pipeline"
  nodes:
    - type: "service"
      name: "enhance-image"
      config:
        kind: "http"
        spec:
          url: "http://localhost:8001/enhance"  # Custom enhancement service
          method: "POST"
    - type: "service"
      name: "anonymization-service"
      config:
        kind: "http"
        spec:
          url: "http://localhost:8000/anonymize?method=blur&blur_factor=50"
          method: "POST"
```

### Add Webhook Integration

Modify the anonymization service to send webhook alerts when faces are detected, or create a separate notification service in the pipeline chain.

## Use Cases

- **Privacy Protection**: Anonymize faces in public surveillance footage
- **Content Moderation**: Blur faces in user-generated content before review
- **Research**: De-identify participants in video research studies
- **Live Streaming**: Protect bystander privacy in live streams
- **Archive Processing**: Batch anonymize historical video archives

## Security and Privacy Considerations

- **Local Processing**: All processing happens locally, no data sent to cloud
- **No Persistent Storage**: Service doesn't store images by default
- **Configurable Output**: Control where anonymized frames are saved
- **Audit Logging**: Add logging to track processing activities
- **Access Control**: Deploy behind authentication/authorization layer for production

## License

This example is part of the digitalhub-servicegraph project. See the parent repository for license information.

## Contributing

Contributions are welcome! Areas for improvement:
- Additional anonymization methods (masking, blacking out)
- Support for other object detection (license plates, logos)
- Performance optimizations
- Additional test cases
- Documentation improvements

## Support

For issues and questions:
- Check the [main repository README](../../README.md)
- Review the [service graph documentation](../../README.md#template-processing)
- Open an issue in the GitHub repository

## Acknowledgments

- OpenCV for computer vision capabilities
- Flask for the HTTP service framework
- digitalhub-servicegraph for the streaming framework
