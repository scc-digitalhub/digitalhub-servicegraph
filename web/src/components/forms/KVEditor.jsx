// SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import React from 'react'
import {
  Box, Typography, TextField, IconButton, Divider,
} from '@mui/material'
import AddIcon    from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'

function KVEditor({ label, value = {}, onChange, keyPlaceholder = 'key', valuePlaceholder = 'value' }) {
  const entries = Object.entries(value || {})

  const handleKeyChange = (idx, newKey) => {
    const newEntries = entries.map(([k, v], i) => [i === idx ? newKey : k, v])
    onChange(Object.fromEntries(newEntries))
  }

  const handleValueChange = (idx, newValue) => {
    const newEntries = entries.map(([k, v], i) => [k, i === idx ? newValue : v])
    onChange(Object.fromEntries(newEntries))
  }

  const handleAdd = () => {
    let key = keyPlaceholder, n = 1
    while (Object.prototype.hasOwnProperty.call(value, key)) key = `${keyPlaceholder}${n++}`
    onChange({ ...value, [key]: '' })
  }

  const handleRemove = (idx) => {
    const newEntries = entries.filter((_, i) => i !== idx)
    onChange(Object.fromEntries(newEntries))
  }

  return (
    <Box sx={{ mb: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 0.5 }}>
        <Typography variant="caption" fontWeight={600} color="text.secondary" sx={{ flex: 1 }}>
          {label}
        </Typography>
        <IconButton size="small" onClick={handleAdd} title="Add entry">
          <AddIcon sx={{ fontSize: 16 }} />
        </IconButton>
      </Box>

      {entries.length === 0 ? (
        <Typography variant="caption" color="text.disabled" sx={{ pl: 0.5 }}>
          No entries — click + to add
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
          {entries.map(([k, v], idx) => (
            <Box key={idx} sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
              <TextField size="small" value={k} placeholder={keyPlaceholder}
                onChange={(e) => handleKeyChange(idx, e.target.value)}
                sx={{ flex: 1 }} />
              <Typography variant="body2" color="text.disabled">:</Typography>
              <TextField size="small" value={v} placeholder={valuePlaceholder}
                onChange={(e) => handleValueChange(idx, e.target.value)}
                sx={{ flex: 1 }} />
              <IconButton size="small" color="error" onClick={() => handleRemove(idx)} title="Remove">
                <DeleteIcon sx={{ fontSize: 14 }} />
              </IconButton>
            </Box>
          ))}
        </Box>
      )}
      <Divider sx={{ mt: 1 }} />
    </Box>
  )
}

export default KVEditor

