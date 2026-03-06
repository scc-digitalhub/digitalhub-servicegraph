import React, { useState } from 'react'
import {
  Box, Typography, FormControl, InputLabel,
  Select, MenuItem, Button, Paper, Stack, IconButton, Tooltip,
} from '@mui/material'
import SettingsIcon from '@mui/icons-material/Settings'
import AddIcon      from '@mui/icons-material/Add'
import DeleteIcon   from '@mui/icons-material/Delete'
import { useGraphStore, SINK_KINDS, ERROR_SINK_KINDS } from '../store/graphStore'

function SinkPanel() {
  const { graph, updateOutput, updateError, openConfigDialog } = useGraphStore()
  const [selectedKind, setSelectedKind] = useState(graph.output?.kind || 'stdout')
  const [selectedErrorKind, setSelectedErrorKind] = useState(graph.error?.kind || 'errorlog')

  const handleKindChange = (e) => {
    const kind = e.target.value
    setSelectedKind(kind)
    const defaults = {
      stdout:    {},
      file:      { file_name: 'output.txt' },
      folder:    { folder_path: './output', filename_pattern: 'event_{counter}.txt' },
      ignore:    {},
      webhook:   { url: 'http://localhost:8081/webhook', method: 'POST' },
      websocket: { url: 'ws://localhost:8081/ws' },
    }
    updateOutput({ kind, spec: defaults[kind] || {} })
  }

  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={700} gutterBottom>Output Sink</Typography>

      {graph.output ? (
        <>
          <FormControl fullWidth size="small" sx={{ mb: 2 }}>
            <InputLabel>Sink Type</InputLabel>
            <Select value={selectedKind} label="Sink Type" onChange={handleKindChange}>
              {SINK_KINDS.map((k) => <MenuItem key={k} value={k}>{k}</MenuItem>)}
            </Select>
          </FormControl>

          <Paper variant="outlined" sx={{ p: 1, mb: 2, bgcolor: 'grey.50' }}>
            <Typography variant="caption" component="pre"
              sx={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap', m: 0, fontSize: '0.7rem' }}>
              {JSON.stringify(graph.output.spec, null, 2)}
            </Typography>
          </Paper>

          <Stack direction="row" spacing={1} sx={{ mb: 2 }}>
            <Button variant="outlined" size="small"
              startIcon={<SettingsIcon />}
              onClick={() => openConfigDialog('output', graph.output)}
              sx={{ flex: 1 }}>
              Configure
            </Button>
            <Tooltip title="Remove output sink">
              <IconButton size="small" color="error" onClick={() => updateOutput(null)}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Stack>
        </>
      ) : (
        <Button fullWidth variant="outlined" color="secondary" size="small"
          startIcon={<AddIcon />}
          onClick={() => updateOutput({ kind: 'stdout', spec: {} })}
          sx={{ mb: 2 }}>
          Add Output
        </Button>
      )}

      <Typography variant="caption" color="text.secondary" fontWeight={700} display="block" sx={{ mb: 0.5 }}>
        Available Sinks
      </Typography>
      {[
        ['stdout','Print to console'],['file','Write to single file'],
        ['folder','Write to multiple files'],['ignore','Discard output'],
        ['webhook','HTTP webhook'],['websocket','WebSocket client'],
      ].map(([n,d]) => (
        <Typography key={n} variant="caption" color="text.secondary" display="block" sx={{ mb: 0.25 }}>
          <strong>{n}</strong>: {d}
        </Typography>
      ))}

      {/* ── Error Sink ─────────────────────────────────────────── */}
      <Typography variant="subtitle2" fontWeight={700} gutterBottom sx={{ mt: 2 }}>Error Sink</Typography>
      <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
        Receives events with status &ge; 400 produced by the pipeline.
      </Typography>

      {graph.error ? (
        <>
          <FormControl fullWidth size="small" sx={{ mb: 2 }}>
            <InputLabel>Error Sink Type</InputLabel>
            <Select value={selectedErrorKind} label="Error Sink Type"
              onChange={(e) => {
                const kind = e.target.value
                setSelectedErrorKind(kind)
                updateError({ kind, spec: {} })
              }}>
              {ERROR_SINK_KINDS.map((k) => <MenuItem key={k} value={k}>{k}</MenuItem>)}
            </Select>
          </FormControl>

          <Stack direction="row" spacing={1} sx={{ mb: 2 }}>
            <Button variant="outlined" size="small"
              startIcon={<SettingsIcon />}
              onClick={() => openConfigDialog('error', graph.error)}
              sx={{ flex: 1 }}>
              Configure
            </Button>
            <Tooltip title="Remove error sink">
              <IconButton size="small" color="error" onClick={() => updateError(null)}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Stack>
        </>
      ) : (
        <Button fullWidth variant="outlined" color="warning" size="small"
          startIcon={<AddIcon />}
          onClick={() => { setSelectedErrorKind('errorlog'); updateError({ kind: 'errorlog', spec: {} }) }}
          sx={{ mb: 2 }}>
          Add Error Sink
        </Button>
      )}

      <Typography variant="caption" color="text.secondary" fontWeight={700} display="block" sx={{ mb: 0.5 }}>
        Available Error Sinks
      </Typography>
      <Typography variant="caption" color="text.secondary" display="block">
        <strong>errorlog</strong>: Log errors to application output
      </Typography>
    </Box>
  )
}

export default SinkPanel
