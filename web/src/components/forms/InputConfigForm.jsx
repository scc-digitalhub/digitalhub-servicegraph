// SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import React from 'react'
import { Box, FormControl, FormHelperText, InputLabel, MenuItem, Select, TextField } from '@mui/material'

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

  const renderRTSP = () => {
    const mediaType = data.spec.media_type || 'video'
    const isVideo = mediaType === 'video'
    const isAudio = mediaType === 'audio'
    const mtErr = errors.find(e => e.field === 'media_type')
    return (<>
      <TextField label="URL" size="small" fullWidth required
        value={data.spec.url || ''} placeholder="rtsp://camera-host/stream"
        onChange={(e) => updateSpec('url', e.target.value)}
        {...fe('url')}
        sx={{ mb: 2 }} />
      <FormControl size="small" fullWidth required sx={{ mb: 2 }} error={!!mtErr}>
        <InputLabel>Media Type</InputLabel>
        <Select label="Media Type" value={mediaType}
          onChange={(e) => updateSpec('media_type', e.target.value)}>
          <MenuItem value="video">Video</MenuItem>
          <MenuItem value="audio">Audio</MenuItem>
        </Select>
        {mtErr && <FormHelperText>{mtErr.message}</FormHelperText>}
      </FormControl>
      {isVideo && numField('Frame Interval', 'frame_interval', '1', 'Process every Nth frame. Default: 1')}
      {isVideo && numField('JPEG Quality', 'jpeg_quality', '80', 'JPEG encoding quality for H.264 frames (1–100). Default: 80')}
      {isAudio && numField('Audio Max Size (B)',          'audio_max_size',            '1048576', 'Max audio buffer size in bytes. Default: 1 MB')}
      {isAudio && numField('Audio Processing Interval (ms)', 'audio_processing_interval', '1000',    'Interval to flush audio buffer. Default: 1000 ms')}
      {isAudio && numField('Audio Chunk Size (B)',         'audio_chunk_size',          '0',       'Sliding-window chunk size (0 = disabled). Default: 0')}
      {numField('Max Retries',    'max_retries',   '0',    'Reconnect attempts before giving up (0 = unlimited). Default: 0')}
      {numField('Retry Backoff (ms)', 'retry_backoff', '1000', 'Delay between reconnect attempts. Default: 1000 ms')}
    </>)
  }

  return (
    <Box>
      <TextField label="Source Type" size="small" fullWidth disabled
        value={data.kind} sx={{ mb: 3 }} />
      {data.kind === 'http'      && renderHTTP()}
      {data.kind === 'websocket' && renderWebSocket()}
      {data.kind === 'mjpeg'     && renderMJPEG()}
      {data.kind === 'rtsp'      && renderRTSP()}
    </Box>
  )
}

export default InputConfigForm

