// SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import React from 'react'
import {
  Box, TextField, Select, MenuItem, FormControl, InputLabel,
  Typography, IconButton, Paper, Divider, Stack,
} from '@mui/material'
import AddIcon    from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import KVEditor from './KVEditor'
import { NODE_KINDS } from '../../store/graphStore'

const NODE_TYPE_OPTIONS = [
  { value: 'service',  desc: 'Calls an external service (HTTP / WebSocket / OpenInference)' },
  { value: 'sequence', desc: 'Runs child nodes one after another' },
  { value: 'ensemble', desc: 'Runs child nodes in parallel and merges outputs' },
  { value: 'switch',   desc: 'Routes to first child whose condition matches' },
]

const DATATYPES = ['BOOL','INT8','UINT8','INT16','UINT16','INT32','UINT32',
                   'INT64','UINT64','FP16','FP32','FP64','BYTES']

const DEFAULT_CONFIGS = {
  service:  { kind: 'http', spec: { url: '', method: 'POST' } },
  ensemble: { spec: { merge_mode: 'concat' } },
}

function NodeConfigForm({ data, onChange, isNew = false, errors = [] }) {
  const fe = (field) => {
    const e = errors.find(e => e.field === field)
    return e ? { error: true, helperText: e.message } : {}
  }

  const set = (key, value) => {
    const u = { ...data }
    if (value === '' || value === undefined) delete u[key]
    else u[key] = value
    onChange(u)
  }

  const setSpec = (key, value) => {
    const spec = { ...(data.config?.spec || {}) }
    if (value === '' || value === null || value === undefined) delete spec[key]
    else spec[key] = value
    onChange({ ...data, config: { ...(data.config || {}), spec } })
  }

  const handleTypeChange = (type) => {
    const next = { type, name: data.name, condition: data.condition }
    if (DEFAULT_CONFIGS[type]) next.config = JSON.parse(JSON.stringify(DEFAULT_CONFIGS[type]))
    if (type !== 'service') next.nodes = data.nodes || []
    onChange(next)
  }

  const handleKindChange = (kind) => {
    const specDefaults = {
      http:          { url: '', method: 'POST' },
      websocket:     { url: '', msg_type: 1 },
      openinference: { address: '', model_name: '' },
    }
    onChange({ ...data, config: { kind, spec: specDefaults[kind] || {} } })
  }

  // ── HTTP service ──────────────────────────────────────────────────────
  const renderHTTP = () => {
    const spec = data.config?.spec || {}
    return (<>
      <TextField label="URL" size="small" fullWidth required
        value={spec.url || ''} placeholder="https://api.example.com/endpoint"
        onChange={(e) => setSpec('url', e.target.value)}
        {...fe('url')}
        sx={{ mb: 2 }} />
      <FormControl fullWidth size="small" sx={{ mb: 2 }}>
        <InputLabel>Method</InputLabel>
        <Select value={spec.method || 'POST'} label="Method"
          onChange={(e) => setSpec('method', e.target.value)}>
          {['GET','POST','PUT','PATCH','DELETE'].map(m => <MenuItem key={m} value={m}>{m}</MenuItem>)}
        </Select>
      </FormControl>
      <TextField label="Num Instances" type="number" size="small" fullWidth
        value={spec.num_instances ?? ''} placeholder="1"
        helperText="Number of parallel goroutines. Default: 1"
        onChange={(e) => setSpec('num_instances', e.target.value === '' ? undefined : parseInt(e.target.value))}
        sx={{ mb: 2 }} />
      <KVEditor label="Headers"      value={spec.headers || {}} keyPlaceholder="Header-Name"
        onChange={(v) => setSpec('headers', Object.keys(v).length ? v : undefined)} />
      <KVEditor label="Query Params" value={spec.params  || {}} keyPlaceholder="param"
        onChange={(v) => setSpec('params',  Object.keys(v).length ? v : undefined)} />
      <TextField label="Input Template" multiline rows={3} size="small" fullWidth
        value={spec.input_template || ''} placeholder="Go text/template — transforms input before sending"
        helperText="Functions: jp, json, tensor"
        onChange={(e) => setSpec('input_template', e.target.value || undefined)} sx={{ mb: 2 }} />
      <TextField label="Output Template" multiline rows={3} size="small" fullWidth
        value={spec.output_template || ''} placeholder="Go text/template — transforms the response"
        onChange={(e) => setSpec('output_template', e.target.value || undefined)} sx={{ mb: 2 }} />
    </>)
  }

  // ── WebSocket service ─────────────────────────────────────────────────
  const renderWS = () => {
    const spec = data.config?.spec || {}
    return (<>
      <TextField label="URL" size="small" fullWidth required
        value={spec.url || ''} placeholder="ws://example.com/ws"
        onChange={(e) => setSpec('url', e.target.value)}
        {...fe('url')}
        sx={{ mb: 2 }} />
      <FormControl fullWidth size="small" sx={{ mb: 2 }}>
        <InputLabel>Message Type</InputLabel>
        <Select value={spec.msg_type ?? 1} label="Message Type"
          onChange={(e) => setSpec('msg_type', parseInt(e.target.value))}>
          <MenuItem value={1}>1 — Text</MenuItem>
          <MenuItem value={2}>2 — Binary</MenuItem>
        </Select>
      </FormControl>
      <KVEditor label="Headers"      value={spec.headers || {}} keyPlaceholder="Header-Name"
        onChange={(v) => setSpec('headers', Object.keys(v).length ? v : undefined)} />
      <KVEditor label="Query Params" value={spec.params  || {}} keyPlaceholder="param"
        onChange={(v) => setSpec('params',  Object.keys(v).length ? v : undefined)} />
      <TextField label="Input Template" multiline rows={3} size="small" fullWidth
        value={spec.input_template || ''} placeholder="Go text/template"
        onChange={(e) => setSpec('input_template', e.target.value || undefined)} sx={{ mb: 2 }} />
      <TextField label="Output Template" multiline rows={3} size="small" fullWidth
        value={spec.output_template || ''} placeholder="Go text/template"
        onChange={(e) => setSpec('output_template', e.target.value || undefined)} sx={{ mb: 2 }} />
    </>)
  }

  // ── OpenInference service ─────────────────────────────────────────────
  const renderOI = () => {
    const spec = data.config?.spec || {}

    const mutateTensors = (field, fn) => {
      const arr = [...(spec[field] || [])]; fn(arr)
      setSpec(field, arr.length ? arr : undefined)
    }

    const TensorList = ({ field, label }) => {
      const tensors = spec[field] || []
      return (
        <Box sx={{ mb: 2 }}>
          <Stack direction="row" alignItems="center" sx={{ mb: 0.5 }}>
            <Typography variant="caption" fontWeight={600} color="text.secondary" sx={{ flex: 1 }}>{label}</Typography>
            <IconButton size="small"
              onClick={() => mutateTensors(field, (a) => a.push({ name: '', datatype: 'FP32', shape: [1] }))}>
              <AddIcon fontSize="small" />
            </IconButton>
          </Stack>
          {tensors.map((ts, idx) => (
            <Paper key={idx} variant="outlined" sx={{ p: 1.5, mb: 1 }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
                <Typography variant="caption" fontWeight={600}>Tensor {idx + 1}</Typography>
                <IconButton size="small" color="error"
                  onClick={() => mutateTensors(field, (a) => a.splice(idx, 1))}>
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Stack>
              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1 }}>
                <TextField label="Name" size="small" value={ts.name || ''} placeholder="tensor_name"
                  onChange={(e) => mutateTensors(field, (a) => { a[idx] = { ...a[idx], name: e.target.value } })} />
                <FormControl size="small">
                  <InputLabel>Data Type</InputLabel>
                  <Select value={ts.datatype || 'FP32'} label="Data Type"
                    onChange={(e) => mutateTensors(field, (a) => { a[idx] = { ...a[idx], datatype: e.target.value } })}>
                    {DATATYPES.map(dt => <MenuItem key={dt} value={dt}>{dt}</MenuItem>)}
                  </Select>
                </FormControl>
                <TextField label="Shape (comma-separated)" size="small" placeholder="1,224,224,3"
                  sx={{ gridColumn: '1 / -1' }}
                  value={(ts.shape || []).join(',')}
                  onChange={(e) => {
                    const shape = e.target.value.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
                    mutateTensors(field, (a) => { a[idx] = { ...a[idx], shape } })
                  }} />
                {field === 'input_tensor_spec' && (
                  <TextField label="Data Field (optional)" size="small" placeholder="field name"
                    sx={{ gridColumn: '1 / -1' }}
                    helperText="JSON field for this tensor's data. Empty = whole body."
                    value={ts.data_field || ''}
                    onChange={(e) => mutateTensors(field, (a) => { a[idx] = { ...a[idx], data_field: e.target.value || undefined } })} />
                )}
              </Box>
            </Paper>
          ))}
        </Box>
      )
    }

    const inputTemplates = spec.input_templates || []
    const mutateInputTemplates = (fn) => {
      const arr = [...inputTemplates]; fn(arr)
      setSpec('input_templates', arr.length ? arr : undefined)
    }

    return (<>
      <TextField label="Address" size="small" fullWidth required
        value={spec.address || ''} placeholder="localhost:8001"
        helperText={errors.find(e=>e.field==='address')?.message ?? 'gRPC server address (host:port)'}
        error={!!errors.find(e=>e.field==='address')}
        onChange={(e) => setSpec('address', e.target.value)} sx={{ mb: 2 }} />
      <TextField label="Model Name" size="small" fullWidth required
        value={spec.model_name || ''} placeholder="my-model"
        onChange={(e) => setSpec('model_name', e.target.value)}
        {...fe('model_name')}
        sx={{ mb: 2 }} />
      <TextField label="Model Version" size="small" fullWidth
        value={spec.model_version || ''} placeholder="1"
        onChange={(e) => setSpec('model_version', e.target.value || undefined)} sx={{ mb: 2 }} />
      <TextField label="Num Instances" type="number" size="small" fullWidth
        value={spec.num_instances ?? ''} placeholder="1"
        onChange={(e) => setSpec('num_instances', e.target.value === '' ? undefined : parseInt(e.target.value))} sx={{ mb: 2 }} />
      <TextField label="Timeout (s)" type="number" size="small" fullWidth
        value={spec.timeout ?? ''} placeholder="30"
        onChange={(e) => setSpec('timeout', e.target.value === '' ? undefined : parseInt(e.target.value))} sx={{ mb: 2 }} />
      <KVEditor label="Extra Params" value={spec.params || {}} keyPlaceholder="param"
        onChange={(v) => setSpec('params', Object.keys(v).length ? v : undefined)} />

      <TensorList field="input_tensor_spec"  label="Input Tensor Specs" />
      <TensorList field="output_tensor_spec" label="Output Tensor Specs" />

      {/* Per-tensor input templates */}
      <Box sx={{ mb: 2 }}>
        <Stack direction="row" alignItems="center" sx={{ mb: 0.5 }}>
          <Typography variant="caption" fontWeight={600} color="text.secondary" sx={{ flex: 1 }}>Input Templates</Typography>
          <IconButton size="small" onClick={() => mutateInputTemplates((a) => a.push({ name: '', template: '' }))}>
            <AddIcon fontSize="small" />
          </IconButton>
        </Stack>
        {inputTemplates.map((it, idx) => (
          <Paper key={idx} variant="outlined" sx={{ p: 1.5, mb: 1 }}>
            <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
              <Typography variant="caption" fontWeight={600}>Template {idx + 1}</Typography>
              <IconButton size="small" color="error" onClick={() => mutateInputTemplates((a) => a.splice(idx, 1))}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Stack>
            <TextField label="Tensor Name" size="small" fullWidth
              value={it.name || ''} placeholder="input_tensor_name"
              onChange={(e) => mutateInputTemplates((a) => { a[idx] = { ...a[idx], name: e.target.value } })}
              sx={{ mb: 1 }} />
            <TextField label="Template" multiline rows={3} size="small" fullWidth
              value={it.template || ''} placeholder="Go text/template producing tensor JSON"
              onChange={(e) => mutateInputTemplates((a) => { a[idx] = { ...a[idx], template: e.target.value } })} />
          </Paper>
        ))}
      </Box>

      <TextField label="Output Template" multiline rows={3} size="small" fullWidth
        value={spec.output_template || ''} placeholder="Go text/template for inference response"
        onChange={(e) => setSpec('output_template', e.target.value || undefined)} sx={{ mb: 2 }} />
    </>)
  }

  // ── Ensemble ──────────────────────────────────────────────────────────
  const renderEnsemble = () => {
    const spec = data.config?.spec || {}
    return (<>
      <FormControl fullWidth size="small" sx={{ mb: 2 }}>
        <InputLabel>Merge Mode</InputLabel>
        <Select value={spec.merge_mode || 'concat'} label="Merge Mode"
          onChange={(e) => setSpec('merge_mode', e.target.value)}>
          <MenuItem value="concat">concat — merge outputs as array</MenuItem>
          <MenuItem value="concat_template">concat_template — Go template merge</MenuItem>
        </Select>
      </FormControl>
      {spec.merge_mode === 'concat_template' && (
        <TextField label="Template" multiline rows={4} size="small" fullWidth required
          value={spec.template || ''}
          placeholder="Go text/template. Receives a slice of maps, one per branch."
          onChange={(e) => setSpec('template', e.target.value || undefined)}
          {...fe('template')}
          sx={{ mb: 2 }} />
      )}
    </>)
  }

  const nodeType = data.type || 'service'

  return (
    <Box>
      {isNew && (
        <FormControl fullWidth size="small" sx={{ mb: 3 }}>
          <InputLabel>Node Type *</InputLabel>
          <Select value={nodeType} label="Node Type *"
            onChange={(e) => handleTypeChange(e.target.value)}>
            {NODE_TYPE_OPTIONS.map(({ value, desc }) => (
              <MenuItem key={value} value={value}>
                <Box>
                  <Typography variant="body2" fontWeight={600}>{value}</Typography>
                  <Typography variant="caption" color="text.secondary">{desc}</Typography>
                </Box>
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      )}

      <TextField label="Name" size="small" fullWidth
        value={data.name || ''} placeholder="optional node name"
        onChange={(e) => set('name', e.target.value)} sx={{ mb: 2 }} />

      <TextField label="Condition" size="small" fullWidth
        value={data.condition || ''} placeholder='$.field == "value"'
        helperText='For child nodes inside a switch. JSONPath expression.'
        onChange={(e) => set('condition', e.target.value)} sx={{ mb: 2 }} />

      {nodeType === 'service' && (<>
        <Divider sx={{ mb: 2 }}>
          <Typography variant="caption" color="text.secondary">Service Config</Typography>
        </Divider>
        <FormControl fullWidth size="small" sx={{ mb: 2 }}>
          <InputLabel>Service Kind *</InputLabel>
          <Select value={data.config?.kind || 'http'} label="Service Kind *"
            onChange={(e) => handleKindChange(e.target.value)}>
            {NODE_KINDS.map(k => <MenuItem key={k} value={k}>{k}</MenuItem>)}
          </Select>
        </FormControl>
        {data.config?.kind === 'http'          && renderHTTP()}
        {data.config?.kind === 'websocket'     && renderWS()}
        {data.config?.kind === 'openinference' && renderOI()}
      </>)}

      {nodeType === 'ensemble' && (<>
        <Divider sx={{ mb: 2 }}>
          <Typography variant="caption" color="text.secondary">Ensemble Config</Typography>
        </Divider>
        {renderEnsemble()}
      </>)}

      {(nodeType === 'sequence' || nodeType === 'switch') && (
        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          {nodeType === 'sequence'
            ? 'Child nodes will be executed sequentially. Use "Add Child Node" to add them.'
            : 'Routes to the first child whose condition evaluates to a non-empty match. Use "Add Child Node" to add children.'}
        </Typography>
      )}
    </Box>
  )
}

export default NodeConfigForm
