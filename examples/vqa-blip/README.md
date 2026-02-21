# Visual Question Answering Pipeline Example

This example demonstrates a complete visual question answering (VQA) pipeline using:
- **BLIP model** (Salesforce/blip-image-captioning-base) for image captioning
- **Flask HTTP service** for easy integration
- **digitalhub-servicegraph** for stream processing
- **MJPEG source** for video stream input

## Table of Contents

- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Architecture](#architecture)
- [Technology Stack](#technology-stack)
- [Testing the VQA Service](#testing-the-vqa-service)
- [Configuration](#configuration)
- [Performance](#performance)
- [Advanced Usage](#advanced-usage)
- [Use Cases](#use-cases)
- [Extending the Example](#extending-the-example)
- [Troubleshooting](#troubleshooting)
- [References](#references)

## Quick Start

### One-Minute Setup

**1. Start VQA Service**
```bash
cd examples/vqa-blip
./setup.sh
./start_service.sh
```

**2. Run Pipeline** (in another terminal)
```bash
# Build service graph
cd /Users/raman/workspace/digitalhub-servicegraph
go build -o digitalhub-servicegraph main.go

# Edit MJPEG URL in examples/vqa-blip/vqa-pipeline.yaml
# Then run:
./digitalhub-servicegraph examples/vqa-blip/vqa-pipeline.yaml
```

### Local Setup (Detailed)

1. **Set up the Python environment:**
   ```bash
   chmod +x setup.sh
   ./setup.sh
   ```

2. **Start the VQA service:**
   ```bash
   chmod +x start_service.sh
   ./start_service.sh
   ```
   
   The service will be available at `http://localhost:8000`

3. **In another terminal, build the service graph:**
   ```bash
   cd ../..  # Navigate to digitalhub-servicegraph root
   go build -o digitalhub-servicegraph main.go
   ```

4. **Configure your MJPEG source:**
   
   Edit `vqa-pipeline.yaml` and update the MJPEG URL:
   ```yaml
   input:
     kind: "mjpeg"
     spec:
       url: "http://your-camera-ip:port/video.mjpeg"
   ```

5. **Run the pipeline:**
   ```bash
   ./digitalhub-servicegraph examples/vqa-blip/vqa-pipeline.yaml
   ```

### Docker Setup

**Build and start services:**
```bash
cd examples/vqa-blip
docker-compose up --build
```

This starts:
- VQA service on port 8000
- Service graph with MJPEG processing

**View captions:**
```bash
docker-compose logs -f servicegraph
```

## Project Structure

```
vqa-blip/
├── README.md                 # This file
├── Makefile                 # Build and run commands
├── requirements.txt         # Python dependencies
├── .gitignore              # Git ignore patterns
│
├── vqa_http_service.py     # Flask-based VQA service (RECOMMENDED)
├── vqa_service.py          # PyTriton-based VQA service (Advanced)
├── test_service.py         # Service testing utility
│
├── vqa-pipeline.yaml       # Service graph configuration
│
├── setup.sh                # Environment setup script
├── start_service.sh        # Service startup script
│
├── Dockerfile              # Container image definition
└── docker-compose.yml      # Multi-service orchestration
```

### What This Example Provides

**1. Visual Question Answering Microservice**

Two implementations:

- **Simple HTTP Service** (`vqa_http_service.py`) - **RECOMMENDED**
  - Flask-based REST API
  - Easy to integrate with service graph
  - Endpoints:
    - `POST /caption` - Generate image captions
    - `GET /health` - Health check
  - ~150 lines of code

- **Advanced PyTriton Service** (`vqa_service.py`)
  - NVIDIA Triton Inference Server protocol
  - Better for production/high-throughput
  - Batching support
  - ~150 lines of code

**2. Service Graph Pipeline Configuration**

`vqa-pipeline.yaml` defines:
- **Input**: MJPEG video stream source
- **Processing**: HTTP client calling VQA service
- **Output**: Stdout sink for captions
- **Flow control**: Frame rate limiting, templating

**3. Complete Development Environment**

- **Setup automation**: One-command environment setup
- **Testing utilities**: Service health checks and image testing
- **Docker support**: Containerized deployment
- **Documentation**: Comprehensive README

## Architecture

### High-Level Flow

```
MJPEG Stream → Service Graph → VQA Service (BLIP) → stdout
                               (Flask/HTTP)
```

The pipeline:
1. Reads frames from an MJPEG video stream
2. Sends selected frames to the VQA service
3. Receives image captions from the BLIP model
4. Outputs captions to stdout

### Detailed Architecture

```
┌─────────────────┐
│  MJPEG Stream   │  Video source (camera, file, etc.)
│  (Input)        │
└────────┬────────┘
         │ Frame every N seconds
         ▼
┌─────────────────┐
│ Service Graph   │  Go-based orchestration
│  - Frame        │
│    extraction   │
│  - HTTP client  │
└────────┬────────┘
         │ POST /caption
         ▼
┌─────────────────┐
│  VQA Service    │  Python Flask/PyTriton
│  - BLIP model   │
│  - Image        │
│    processing   │
└────────┬────────┘
         │ {"caption": "..."}
         ▼
┌─────────────────┐
│  Stdout Sink    │  Terminal output
│  (Output)       │
└─────────────────┘
```

### Key Features

- **Real-time Processing**: Processes MJPEG video streams frame-by-frame
- **Configurable Frame Rate**: Process every Nth frame
- **Async Processing**: Continuous streaming support
- **ML Model Integration**: BLIP state-of-the-art vision-language model
- **Automatic GPU/CPU Detection**: Uses CUDA if available
- **Production-Ready**: Health checks, error handling, logging
- **Docker Containerization**: Easy deployment
- **Scalable Architecture**: Horizontal and vertical scaling options

## Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| ML Model | BLIP (Salesforce) | Image captioning |
| Model Size | 250M parameters | Vision-language understanding |
| Serving | Flask / PyTriton | HTTP API / Triton protocol |
| Framework | Hugging Face Transformers | Model inference |
| Orchestration | digitalhub-servicegraph | Stream processing |
| Container | Docker / Docker Compose | Deployment |

## Testing the VQA Service

### Health Check
```bash
curl http://localhost:8000/health
```

### Image Captioning
```bash
# Using a local image
curl -X POST \
  -H "Content-Type: image/jpeg" \
  --data-binary @test-image.jpg \
  http://localhost:8000/caption

# Expected response:
# {"caption": "a dog sitting on a couch", "status": "success"}
```

### Using the Test Script
```bash
python test_service.py path/to/test-image.jpg
```

## Configuration

### MJPEG Source Settings

In `vqa-pipeline.yaml`:

```yaml
input:
  kind: "mjpeg"
  spec:
    url: "http://localhost:1984/api/stream.mjpeg?src=camera"
    frame_interval: 10        # Process every 10th frame
    read_timeout: 10          # Read timeout (seconds)
```

- **frame_interval**: Controls processing rate. Higher = fewer frames processed
- **url**: Your MJPEG stream source (IP camera, video server, etc.)

### VQA Service Configuration

The service uses the BLIP model from Hugging Face:
- **Model**: `Salesforce/blip-image-captioning-base`
- **Device**: Automatically selects CUDA if available, otherwise CPU
- **Max tokens**: 50 tokens per caption

To use a different model, edit `vqa_http_service.py`:
```python
model_name = "Salesforce/blip-vqa-base"  # For VQA with questions
```

## Performance Considerations

### Performance Characteristics

**Throughput:**
- **CPU**: ~2-5 frames/second (i7 processor)
- **GPU**: ~10-20 frames/second (NVIDIA RTX)
- **Latency**: 200ms-2s per frame (model dependent)

**Resource Usage:**
- **Memory**: ~2GB RAM, ~4GB VRAM (GPU mode)
- **Disk**: ~2.5GB (model + dependencies)
- **CPU**: 2+ cores recommended

**Scalability:**
- **Horizontal**: Multiple VQA service instances
- **Vertical**: GPU acceleration
- **Optimization**: Batch processing (PyTriton)

### Frame Rate
- **frame_interval**: Adjust based on your needs
  - `1`: Process every frame (high CPU/GPU usage)
  - `10`: Process every 10th frame (recommended for real-time)
  - `30`: Process every 30th frame (low resource usage)

### GPU Acceleration
- CUDA required for GPU support
- GPU provides 5-10x faster inference
- Model automatically detects and uses GPU if available

### Resource Usage
- **Memory**: ~2GB RAM, ~4GB VRAM (with GPU)
- **CPU**: 2+ cores recommended
- **GPU**: Optional but recommended (NVIDIA with CUDA support)

## Advanced Usage

### Using PyTriton (More Scalable)

For production deployments with better performance:

1. Start the PyTriton service:
   ```bash
   python vqa_service.py
   ```

2. Update `vqa-pipeline.yaml` to use Triton inference protocol:
   ```yaml
   url: "http://localhost:8000/v2/models/vqa_blip/infer"
   ```

### Custom Questions (VQA)

To ask specific questions about images, extend `vqa_http_service.py`:

```python
@app.route('/vqa', methods=['POST'])
def vqa():
    data = request.json
    image_bytes = base64.b64decode(data['image'])
    question = data['question']
    
    image = Image.open(BytesIO(image_bytes)).convert('RGB')
    inputs = processor(image, question, return_tensors="pt").to(device)
    # ... generate answer
```

## Use Cases

This example demonstrates patterns for:

### 1. Video Analytics Pipelines
- Security camera monitoring
- Content moderation
- Scene understanding

### 2. ML Model Serving
- HTTP API wrapping ML models
- Integration with streaming frameworks
- Batched inference

### 3. Stream Processing
- Real-time video analysis
- Event-driven architectures
- Data pipeline orchestration

### 4. Microservices Architecture
- Service composition
- HTTP-based communication
- Containerized deployment

## Extending the Example

### Add Question Answering
Modify service to accept questions:
```python
inputs = processor(image, question, return_tensors="pt")
```

### Use Different Models
Change model in `vqa_http_service.py`:
```python
model_name = "Salesforce/blip-vqa-base"  # VQA variant
model_name = "Salesforce/blip-image-captioning-large"  # Better quality
```

### Add Additional Processing
Extend the service graph:
```yaml
flow:
  nodes:
    - type: "service"
      name: "vqa"
      # ... VQA config
    - type: "service"
      name: "sentiment-analysis"
      # ... analyze captions
```

### Store Results
Change output sink:
```yaml
output:
  kind: "file"
  spec:
    path: "captions.txt"
```

## Troubleshooting

### Quick Reference

| Issue | Solution |
|-------|----------|
| Port 8000 in use | Kill process: `lsof -ti:8000 \| xargs kill -9` |
| Out of memory | Increase `frame_interval` to 30+ |
| Slow captions | Enable GPU or use smaller model |
| MJPEG fails | Check URL with: `curl -I <mjpeg-url>` |

### Service won't start
- Check Python version (3.8+ required)
- Ensure dependencies installed: `pip install -r requirements.txt`
- Check port 8000 is available: `lsof -i :8000`

### MJPEG connection fails
- Verify MJPEG URL is accessible: `curl -I <mjpeg-url>`
- Check network connectivity

### Out of memory errors
- Increase `frame_interval` to process fewer frames
- Use CPU instead of GPU (edit `vqa_http_service.py`)
- Reduce `max_new_tokens` in model.generate()

### Slow performance
- Enable GPU (CUDA) if available
- Increase `frame_interval`
- Use smaller model: `Salesforce/blip-image-captioning-small`

## Example MJPEG Sources

### Testing with a webcam stream
```bash
# Using ffmpeg to create MJPEG stream from webcam
ffmpeg -f avfoundation -i "0" -f mjpeg \
  -r 10 http://localhost:1984/video.mjpeg
```

### IP Camera examples
```yaml
# Axis camera
url: "http://camera-ip/mjpg/video.mjpg"

# Generic IP camera
url: "http://camera-ip:8080/video"

# RTSP to MJPEG (using converter)
url: "http://localhost:1984/api/stream.mjpeg?src=rtsp://camera-ip/stream"
```

## Model Information

**BLIP** (Bootstrapping Language-Image Pre-training):
- Developed by Salesforce Research
- State-of-the-art vision-language model
- Supports both image captioning and VQA
- Paper: https://arxiv.org/abs/2201.12086
- Model on Hugging Face: https://huggingface.co/Salesforce/blip-image-captioning-base

Available BLIP models:
- `Salesforce/blip-image-captioning-base` (default, 250M params)
- `Salesforce/blip-image-captioning-large` (better quality, 500M params)
- `Salesforce/blip-vqa-base` (for VQA tasks)

## References

- **BLIP Paper**: https://arxiv.org/abs/2201.12086
- **Model Card**: https://huggingface.co/Salesforce/blip-image-captioning-base
- **Service Graph**: https://github.com/scc-digitalhub/digitalhub-servicegraph
- **PyTriton**: https://github.com/triton-inference-server/pytriton

## License

SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
