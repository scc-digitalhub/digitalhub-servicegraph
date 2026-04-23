// SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import React, { useState, useEffect, useRef } from 'react'
import {
  AppBar, Toolbar, Typography, Box, Button, Chip, Stack,
  Alert, Tooltip, Divider,
} from '@mui/material'
import UploadFileIcon from '@mui/icons-material/UploadFile'
import DownloadIcon   from '@mui/icons-material/Download'
import CodeIcon       from '@mui/icons-material/Code'
import RestartAltIcon from '@mui/icons-material/RestartAlt'
import RefreshIcon    from '@mui/icons-material/Refresh'
import CheckIcon      from '@mui/icons-material/Check'
import GraphCanvas from './components/GraphCanvas'
import ConfigDialog from './components/ConfigDialog'
import SourcePanel from './components/SourcePanel'
import SinkPanel from './components/SinkPanel'
import { useGraphStore } from './store/graphStore'
import yaml from 'js-yaml'
import { validateGraph } from './validate'
import './App.css'

function App() {
  const { graph, exportGraph, importGraph, reset } = useGraphStore()
  const [showYaml, setShowYaml]   = useState(false)
  const [yamlText, setYamlText]   = useState('')
  const [yamlError, setYamlError] = useState(null)
  const [yamlDirty, setYamlDirty] = useState(false)
  const yamlDirtyRef = useRef(false)

  useEffect(() => {
    if (!yamlDirtyRef.current) {
      setYamlText(yaml.dump(graph, { indent: 2 }))
      setYamlError(null)
    }
  }, [graph, showYaml])

  const handleYamlChange = (e) => {
    const text = e.target.value
    setYamlText(text); setYamlDirty(true); yamlDirtyRef.current = true
    try {
      const parsed = yaml.load(text)
      const graphErrors = validateGraph(parsed)
      if (graphErrors.length > 0)
        setYamlError(graphErrors.map(e => `${e.path ? e.path + ': ' : ''}${e.message}`).join('\n'))
      else
        setYamlError(null)
    } catch (err) { setYamlError(err.message) }
  }

  const handleApply = () => {
    try {
      const parsed = yaml.load(yamlText)
      if (!parsed || typeof parsed !== 'object') throw new Error('YAML must represent an object')
      const graphErrors = validateGraph(parsed)
      if (graphErrors.length > 0) {
        setYamlError(graphErrors.map(e => `${e.path ? e.path + ': ' : ''}${e.message}`).join('\n'))
        return
      }
      importGraph(parsed)
      setYamlDirty(false); yamlDirtyRef.current = false; setYamlError(null)
    } catch (err) { setYamlError(err.message) }
  }

  const handleRefresh = () => {
    setYamlText(yaml.dump(graph, { indent: 2 }))
    setYamlDirty(false); yamlDirtyRef.current = false; setYamlError(null)
  }

  const handleExport = () => {
    const yamlStr = yaml.dump(exportGraph(), { indent: 2 })
    const a = Object.assign(document.createElement('a'), {
      href: URL.createObjectURL(new Blob([yamlStr], { type: 'text/yaml' })),
      download: 'service-graph.yaml',
    })
    a.click(); URL.revokeObjectURL(a.href)
  }

  const handleImport = (e) => {
    const file = e.target.files[0]
    if (!file) return
    e.target.value = ''
    const reader = new FileReader()
    reader.onload = (ev) => {
      try {
        const parsed = yaml.load(ev.target.result)
        const graphErrors = validateGraph(parsed)
        if (graphErrors.length > 0) {
          alert('Invalid YAML:\n' + graphErrors.map(e => `${e.path ? e.path + ': ' : ''}${e.message}`).join('\n'))
          return
        }
        importGraph(parsed)
        setYamlDirty(false); yamlDirtyRef.current = false
      } catch (err) { alert('Error parsing YAML: ' + err.message) }
    }
    reader.readAsText(file)
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh', bgcolor: 'grey.100' }}>
      {/* ── Header ───────────────────────────────────────────────── */}
      <AppBar position="static" color="primary">
        <Toolbar variant="dense" sx={{ gap: 1 }}>
          <Typography variant="h6" sx={{ fontWeight: 700, flexGrow: 1 }}>
            Service Graph Designer
          </Typography>

          <Button component="label" variant="outlined" color="inherit" size="small"
            startIcon={<UploadFileIcon fontSize="small" />}>
            Import
            <input type="file" accept=".yaml,.yml" onChange={handleImport} hidden />
          </Button>

          <Button variant="outlined" color="inherit" size="small"
            startIcon={<DownloadIcon fontSize="small" />}
            onClick={handleExport}>
            Export
          </Button>

          <Button
            variant={showYaml ? 'contained' : 'outlined'}
            color="inherit" size="small"
            startIcon={<CodeIcon fontSize="small" />}
            onClick={() => setShowYaml(!showYaml)}
            sx={showYaml ? { bgcolor: 'rgba(255,255,255,0.2)' } : undefined}>
            {showYaml ? 'Hide YAML' : 'Edit YAML'}
          </Button>

          <Button variant="outlined" color="inherit" size="small"
            startIcon={<RestartAltIcon fontSize="small" />}
            onClick={() => { if (window.confirm('Reset the graph?')) reset() }}>
            Reset
          </Button>
        </Toolbar>
      </AppBar>

      {/* ── Body ─────────────────────────────────────────────────── */}
      <Box sx={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        {/* Left sidebar — source on top, sink on bottom */}
        <Box sx={{ width: 280, flexShrink: 0, bgcolor: 'background.paper', borderRight: 1, borderColor: 'divider', overflowY: 'auto', p: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <SourcePanel />
          <Divider />
          <SinkPanel />
        </Box>

        {/* Main canvas / YAML editor */}
        <Box sx={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
          {showYaml ? (
            <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', bgcolor: '#1e1e1e' }}>
              {/* YAML toolbar */}
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 1.5, py: 0.75, bgcolor: '#252526', borderBottom: '1px solid #3c3c3c', flexShrink: 0 }}>
                <Typography variant="caption" sx={{ fontWeight: 700, color: '#ccc', letterSpacing: '0.04em' }}>YAML Editor</Typography>
                {yamlDirty && <Chip label="unsaved" size="small" color="warning" variant="outlined" sx={{ height: 18, fontSize: '0.65rem' }} />}
                {yamlError && <Chip label="parse error" size="small" color="error" variant="outlined" sx={{ height: 18, fontSize: '0.65rem' }} />}
                <Stack direction="row" spacing={0.75} sx={{ ml: 'auto' }}>
                  <Button size="small" variant="outlined" color="inherit"
                    startIcon={<RefreshIcon sx={{ fontSize: 14 }} />}
                    onClick={handleRefresh}
                    sx={{ color: '#ccc', borderColor: '#555' }}>
                    Refresh
                  </Button>
                  <Button size="small" variant="contained" color="primary"
                    startIcon={<CheckIcon sx={{ fontSize: 14 }} />}
                    onClick={handleApply}
                    disabled={!!yamlError}>
                    Apply
                  </Button>
                </Stack>
              </Box>

              {yamlError && (
                <Alert severity="error" sx={{ borderRadius: 0, py: 0.5 }}>
                  <Typography variant="caption" sx={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>{yamlError}</Typography>
                </Alert>
              )}

              <textarea
                value={yamlText}
                onChange={handleYamlChange}
                spellCheck={false} autoCorrect="off" autoCapitalize="off"
                style={{
                  flex: 1, resize: 'none', border: 'none', outline: 'none',
                  background: '#1e1e1e', color: '#d4d4d4',
                  fontFamily: '"Fira Code","Courier New",monospace',
                  fontSize: '0.875rem', lineHeight: 1.6,
                  padding: '1rem 1.25rem', caretColor: '#aeafad',
                }}
              />
            </Box>
          ) : (
            <GraphCanvas />
          )}
        </Box>

      </Box>

      <ConfigDialog />
    </Box>
  )
}

export default App
