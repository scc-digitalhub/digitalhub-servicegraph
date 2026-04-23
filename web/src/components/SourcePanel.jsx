// SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import React, { useState } from 'react'
import {
  Box, Typography, FormControl, InputLabel,
  Select, MenuItem, Button, Paper,
} from '@mui/material'
import SettingsIcon from '@mui/icons-material/Settings'
import { useGraphStore, SOURCE_KINDS } from '../store/graphStore'

function SourcePanel() {
  const { graph, updateInput, openConfigDialog } = useGraphStore()
  const [selectedKind, setSelectedKind] = useState(graph.input.kind)

  const handleKindChange = (e) => {
    const kind = e.target.value
    setSelectedKind(kind)
    const defaults = {
      http:      { port: 8080 },
      websocket: { port: 8080, path: '/ws' },
      mjpeg:     { url: 'http://localhost:8081/video' },
      rtsp:      { url: 'rtsp://localhost:8554/stream', media_type: 'video' },
    }
    updateInput({ kind, spec: defaults[kind] || {} })
  }

  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={700} gutterBottom>Input Source</Typography>

      <FormControl fullWidth size="small" sx={{ mb: 2 }}>
        <InputLabel>Source Type</InputLabel>
        <Select value={selectedKind} label="Source Type" onChange={handleKindChange}>
          {SOURCE_KINDS.map((k) => <MenuItem key={k} value={k}>{k}</MenuItem>)}
        </Select>
      </FormControl>

      <Paper variant="outlined" sx={{ p: 1, mb: 2, bgcolor: 'grey.50' }}>
        <Typography variant="caption" component="pre"
          sx={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap', m: 0, fontSize: '0.7rem' }}>
          {JSON.stringify(graph.input.spec, null, 2)}
        </Typography>
      </Paper>

      <Button fullWidth variant="outlined" size="small"
        startIcon={<SettingsIcon />}
        onClick={() => openConfigDialog('input', graph.input)}
        sx={{ mb: 2 }}>
        Configure
      </Button>

      <Typography variant="caption" color="text.secondary" fontWeight={700} display="block" sx={{ mb: 0.5 }}>
        Available Sources
      </Typography>
      {[['http','HTTP server endpoint'],['websocket','WebSocket server'],['mjpeg','MJPEG stream source'],['rtsp','RTSP stream (video/audio)']].map(([n,d]) => (
        <Typography key={n} variant="caption" color="text.secondary" display="block" sx={{ mb: 0.25 }}>
          <strong>{n}</strong>: {d}
        </Typography>
      ))}
    </Box>
  )
}

export default SourcePanel
