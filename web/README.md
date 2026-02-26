# Service Graph Designer

A visual web-based editor for creating and configuring DigitalHub Service Graphs.

## Features

- **Visual Graph Editor**: Interactive drag-and-drop interface using React Flow
- **Configuration Dialogs**: Easy-to-use forms for configuring sources, nodes, and sinks
- **Input Sources**: Configure HTTP, WebSocket, and MJPEG sources
- **Output Sinks**: Configure stdout, file, folder, webhook, and WebSocket sinks
- **Service Nodes**: Configure HTTP, WebSocket, and OpenInference service nodes
- **Import/Export**: Load and save configurations as YAML files
- **YAML Preview**: View the generated YAML configuration in real-time

## Getting Started

### Prerequisites

- Node.js 18+ and npm

### Installation

```bash
cd web
npm install
```

### Development

Start the development server:

```bash
npm run dev
```

The application will be available at http://localhost:3000

### Build

Build for production:

```bash
npm run build
```

The built files will be in the `dist` directory.

## Usage

### Creating a Service Graph

1. **Configure Input Source**
   - Select the source type (HTTP, WebSocket, MJPEG) from the left panel
   - Click "Configure" to set specific parameters
   - The input node will appear at the top of the graph

2. **Add Service Nodes**
   - Click on nodes in the graph to configure them
   - Set the service kind (http, websocket, openinference)
   - Configure service-specific parameters like URLs, methods, headers

3. **Configure Output Sink**
   - In the right panel, click "Add Output" if no output exists
   - Select the sink type
   - Click "Configure" to set parameters like file paths, URLs, etc.

4. **Export Configuration**
   - Click "Export" to download the YAML configuration
   - Click "View YAML" to see the configuration in real-time

5. **Import Configuration**
   - Click "Import" to load an existing YAML file
   - The graph will be updated to reflect the imported configuration

### Available Components

#### Input Sources
- **http**: HTTP server endpoint
  - `port`: Server port (default: 8080)
  - `path`: Optional URL path

- **websocket**: WebSocket server
  - `port`: Server port
  - `path`: WebSocket path
  - `msg_type`: Message type (text/binary)

- **mjpeg**: MJPEG stream source
  - `url`: Stream URL
  - `boundary`: Optional boundary string

#### Service Nodes
- **http**: HTTP client for making requests
  - `url`: Target URL
  - `method`: HTTP method (GET, POST, PUT, etc.)
  - `headers`: Request headers
  - `timeout`: Request timeout

- **websocket**: WebSocket client
  - `url`: WebSocket URL
  - `msg_type`: Message type (text/binary)

- **openinference**: OpenInference Protocol v2 client
  - `address`: Server address
  - `model_name`: Model name
  - `model_version`: Optional model version
  - `timeout`: Request timeout

#### Output Sinks
- **stdout**: Print to console
- **file**: Write to a single file
  - `file_name`: Output file path

- **folder**: Write to multiple files with pattern
  - `folder_path`: Output folder
  - `filename_pattern`: Pattern with placeholders ({counter}, {timestamp}, etc.)

- **ignore**: Discard output
- **webhook**: Send to HTTP endpoint
  - `url`: Webhook URL
  - `method`: HTTP method
  - `headers`: Request headers

- **websocket**: WebSocket client output
  - `url`: WebSocket URL
  - `msg_type`: Message type

## Development

### Project Structure

```
web/
├── src/
│   ├── components/
│   │   ├── GraphCanvas.jsx       # Main graph visualization
│   │   ├── SourcePanel.jsx       # Input source configuration
│   │   ├── SinkPanel.jsx         # Output sink configuration
│   │   ├── ConfigDialog.jsx      # Configuration modal
│   │   └── forms/
│   │       ├── InputConfigForm.jsx
│   │       ├── OutputConfigForm.jsx
│   │       └── NodeConfigForm.jsx
│   ├── store/
│   │   └── graphStore.js         # Zustand state management
│   ├── App.jsx                   # Main app component
│   ├── App.css                   # App styles
│   ├── main.jsx                  # Entry point
│   └── index.css                 # Global styles
├── index.html
├── package.json
└── vite.config.js
```

### Technologies Used

- **React 18**: UI framework
- **React Flow**: Graph visualization library
- **Zustand**: State management
- **Vite**: Build tool and dev server
- **js-yaml**: YAML parsing and serialization
- **Lucide React**: Icon library

## License

SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler  
SPDX-License-Identifier: Apache-2.0
