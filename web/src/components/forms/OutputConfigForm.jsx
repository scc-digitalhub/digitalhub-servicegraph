import React from 'react'
import {
  Box, TextField, Select, MenuItem, FormControl,
  InputLabel, Typography,
} from '@mui/material'
import KVEditor from './KVEditor'

function OutputConfigForm({ data, onChange, errors = [] }) {
  const fe = (field) => {
    const e = errors.find(e => e.field === field)
    return e ? { error: true, helperText: e.message } : {}
  }

  const updateSpec = (key, value) => {
    const s = { ...data.spec }
    if (value === '' || value === null || value === undefined) delete s[key]
    else s[key] = value
    onChange({ ...data, spec: s })
  }

  const renderFile = () => (
    <TextField label="File Name" size="small" fullWidth required
      value={data.spec.file_name || ''} placeholder="output.txt"
      onChange={(e) => updateSpec('file_name', e.target.value)}
      {...fe('file_name')}
      sx={{ mb: 2 }} />
  )

  const renderFolder = () => (<>
    <TextField label="Folder Path" size="small" fullWidth required
      value={data.spec.folder_path || ''} placeholder="./output"
      onChange={(e) => updateSpec('folder_path', e.target.value)}
      {...fe('folder_path')}
      sx={{ mb: 2 }} />
    <TextField label="Filename Pattern" size="small" fullWidth required
      value={data.spec.filename_pattern || ''} placeholder="event_{counter}.json"
      helperText={errors.find(e => e.field==='filename_pattern')?.message ?? '{counter}/{index}, {timestamp}, {timestamp_ms}, {timestamp_ns}'}
      error={!!errors.find(e => e.field==='filename_pattern')}
      onChange={(e) => updateSpec('filename_pattern', e.target.value)}
      sx={{ mb: 2 }} />
  </>)

  const renderWebhook = () => (<>
    <TextField label="URL" size="small" fullWidth required
      value={data.spec.url || ''} placeholder="https://example.com/webhook"
      onChange={(e) => updateSpec('url', e.target.value)}
      {...fe('url')}
      sx={{ mb: 2 }} />
    <TextField label="Parallelism" type="number" size="small" fullWidth
      value={data.spec.parallelism ?? ''} placeholder="1"
      helperText="Number of concurrent outgoing requests"
      onChange={(e) => updateSpec('parallelism', e.target.value === '' ? undefined : parseInt(e.target.value))}
      sx={{ mb: 2 }} />
    <KVEditor label="Headers"      value={data.spec.headers || {}} keyPlaceholder="Header-Name"
      onChange={(v) => updateSpec('headers', Object.keys(v).length ? v : undefined)} />
    <KVEditor label="Query Params" value={data.spec.params  || {}} keyPlaceholder="param"
      onChange={(v) => updateSpec('params',  Object.keys(v).length ? v : undefined)} />
  </>)

  const renderWebSocket = () => (<>
    <TextField label="URL" size="small" fullWidth required
      value={data.spec.url || ''} placeholder="ws://example.com/ws"
      onChange={(e) => updateSpec('url', e.target.value)}
      {...fe('url')}
      sx={{ mb: 2 }} />
    <FormControl fullWidth size="small" sx={{ mb: 2 }}>
      <InputLabel>Message Type</InputLabel>
      <Select value={data.spec.msg_type ?? 1} label="Message Type"
        onChange={(e) => updateSpec('msg_type', parseInt(e.target.value))}>
        <MenuItem value={1}>1 — Text</MenuItem>
        <MenuItem value={2}>2 — Binary</MenuItem>
      </Select>
    </FormControl>
    <KVEditor label="Headers"      value={data.spec.headers || {}} keyPlaceholder="Header-Name"
      onChange={(v) => updateSpec('headers', Object.keys(v).length ? v : undefined)} />
    <KVEditor label="Query Params" value={data.spec.params  || {}} keyPlaceholder="param"
      onChange={(v) => updateSpec('params',  Object.keys(v).length ? v : undefined)} />
  </>)

  return (
    <Box>
      <TextField label="Sink Type" size="small" fullWidth disabled
        value={data.kind} sx={{ mb: 3 }} />
      {data.kind === 'file'      && renderFile()}
      {data.kind === 'folder'    && renderFolder()}
      {data.kind === 'webhook'   && renderWebhook()}
      {data.kind === 'websocket' && renderWebSocket()}
      {(data.kind === 'stdout' || data.kind === 'ignore') && (
        <Typography variant="body2" color="text.secondary">
          No additional configuration required for this sink type.
        </Typography>
      )}
    </Box>
  )
}

export default OutputConfigForm

