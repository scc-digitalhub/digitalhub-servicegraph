<!--
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
-->

# Quick Reference Guide

## Starting the Service Graph Designer

### Option 1: Using the start script (recommended)
```bash
cd web
./start-dev.sh
```

### Option 2: Manual start
```bash
cd web
npm install  # First time only
npm run dev
```

## Building for Production

```bash
cd web
npm run build
```

The built files will be in the `dist/` directory.

## Port Configuration

The development server runs on port 3000 by default. To change this:

1. Edit `vite.config.js`
2. Update the `server.port` value

## Common Tasks

### Import an Existing YAML File
1. Click "Import" button
2. Select your YAML file
3. The graph will be visualized

### Export Your Graph
1. Click "Export" button
2. Choose a filename
3. Save the YAML file

### Configure Input Source
1. Use the left panel "Input Source"
2. Select source type (HTTP, WebSocket, MJPEG)
3. Fill in configuration details
4. Click "Save"

### Configure Output Sink
1. Use the right panel "Output Sink"
2. Select sink type
3. Fill in configuration details
4. Click "Save"

### Add Service Nodes
1. Click on the graph canvas or existing nodes
2. Configure node type and settings
3. Nodes are automatically connected in sequence

## Testing Your Configuration

1. Export your graph as YAML
2. Run it with the service graph tool:
```bash
go run main.go path/to/your/config.yaml
```

## Keyboard Shortcuts

- `Ctrl/Cmd + Z`: Undo (if implemented)
- `Delete`: Remove selected node (if implemented)
- Mouse wheel: Zoom in/out on canvas
- Click + Drag: Pan the canvas

## Troubleshooting

### Port 3000 Already in Use
Kill the process using port 3000:
```bash
lsof -ti:3000 | xargs kill
```

Or change the port in `vite.config.js`.

### Dependencies Not Installing
Try clearing the npm cache:
```bash
npm cache clean --force
rm -rf node_modules package-lock.json
npm install
```

### Build Errors
Ensure you're using Node.js 18 or higher:
```bash
node --version
```

## Support

For issues or questions, please refer to:
- [Web Designer README](README.md)
- [Main Project README](../README.md)
- Project repository issues
