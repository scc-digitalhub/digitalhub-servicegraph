import React from 'react'
import { Box, TextField } from '@mui/material'

function InputConfigForm({ data, onChange, errors = [] }) {
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

  const numField = (label, key, placeholder, hint) => (
    <TextField key={key} label={label} type="number" size="small" fullWidth
      value={data.spec[key] ?? ''}
      placeholder={placeholder}
      helperText={hint}
      onChange={(e) => updateSpec(key, e.target.value === '' ? undefined : parseInt(e.target.value))}
      sx={{ mb: 2 }}
    />
  )

  const renderHTTP = () => (<>
    <TextField label="Port" type="number" size="small" fullWidth required
      value={data.spec.port || 8080}
      onChange={(e) => updateSpec('port', parseInt(e.target.value))}
      {...fe('port')}
      sx={{ mb: 2 }} />
    {numField('Read Timeout (s)',    'read_timeout',    '10',      'Default: 10 s')}
    {numField('Write Timeout (s)',   'write_timeout',   '10',      'Default: 10 s')}
    {numField('Process Timeout (s)', 'process_timeout', '30',      'Default: 30 s')}
    {numField('Max Input Size (B)',  'max_input_size',  '10485760','Default: 10 MB')}
  </>)

  const renderWebSocket = () => (<>
    <TextField label="Port" type="number" size="small" fullWidth required
      value={data.spec.port || 8080}
      onChange={(e) => updateSpec('port', parseInt(e.target.value))}
      {...fe('port')}
      sx={{ mb: 2 }} />
    {numField('Capacity', 'capacity', '10', 'Channel buffer capacity. Default: 10')}
  </>)

  const renderMJPEG = () => (<>
    <TextField label="URL" size="small" fullWidth required
      value={data.spec.url || ''} placeholder="http://camera-host/mjpeg"
      onChange={(e) => updateSpec('url', e.target.value)}
      {...fe('url')}
      sx={{ mb: 2 }} />
    {numField('Frame Interval', 'frame_interval', '1', 'Process every Nth frame. Default: 1')}
    {numField('Read Timeout (s)', 'read_timeout', '10', 'Default: 10 s')}
  </>)

  return (
    <Box>
      <TextField label="Source Type" size="small" fullWidth disabled
        value={data.kind} sx={{ mb: 3 }} />
      {data.kind === 'http'      && renderHTTP()}
      {data.kind === 'websocket' && renderWebSocket()}
      {data.kind === 'mjpeg'     && renderMJPEG()}
    </Box>
  )
}

export default InputConfigForm

