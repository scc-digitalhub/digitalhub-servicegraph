// SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import React, { useState, useEffect } from 'react'
import {
  Dialog, DialogTitle, DialogContent, DialogActions,
  Button, IconButton, Typography, Divider, Box,
} from '@mui/material'
import CloseIcon  from '@mui/icons-material/Close'
import AddIcon    from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import { useGraphStore, DEFAULT_NEW_NODE } from '../store/graphStore'
import { validateInput, validateOutput, validateNode } from '../validate'
import InputConfigForm  from './forms/InputConfigForm'
import OutputConfigForm from './forms/OutputConfigForm'
import NodeConfigForm   from './forms/NodeConfigForm'

function ConfigDialog() {
  const {
    configDialogOpen, configDialogType, configDialogData, configDialogPath,
    closeConfigDialog, openConfigDialog,
    updateInput, updateOutput, updateError, updateNode, addNode, deleteNode,
  } = useGraphStore()

  const [formData, setFormData] = useState(null)

  useEffect(() => {
    if (configDialogData) setFormData(JSON.parse(JSON.stringify(configDialogData)))
  }, [configDialogData])

  if (!configDialogOpen || !formData) return null

  const isNodeType  = configDialogType === 'node' || configDialogType === 'add_node'
  const isAddMode   = configDialogType === 'add_node'
  const isRootNode  = configDialogType === 'node' && configDialogPath === null
  const isContainer = isNodeType && ['sequence', 'ensemble', 'switch'].includes(formData.type)
  const isSinkType  = configDialogType === 'output' || configDialogType === 'error'

  // Compute field-level validation errors for the current form
  const formErrors =
    configDialogType === 'input' ? validateInput(formData)  :
    isSinkType                   ? validateOutput(formData) :
    isNodeType                   ? validateNode(formData)   : []

  const handleSave = () => {
    if (configDialogType === 'input') {
      updateInput(formData)
    } else if (configDialogType === 'output') {
      updateOutput(formData)
    } else if (configDialogType === 'error') {
      updateError(formData)
    } else if (configDialogType === 'node') {
      updateNode(configDialogPath, formData)
    } else if (configDialogType === 'add_node') {
      const clean = JSON.parse(JSON.stringify(formData,
        (_, v) => (v === '' || v === undefined) ? undefined : v))
      addNode(clean, configDialogPath)
    }
    closeConfigDialog()
  }

  const handleDelete = () => {
    if (window.confirm('Delete this node?')) { deleteNode(configDialogPath); closeConfigDialog() }
  }

  const handleAddChild = () => {
    openConfigDialog('add_node', JSON.parse(JSON.stringify(DEFAULT_NEW_NODE)), configDialogPath)
  }

  const titles = {
    input:    'Configure Input Source',
    output:   'Configure Output Sink',
    error:    'Configure Error Sink',
    node:     isRootNode ? 'Configure Root Flow' : 'Configure Node',
    add_node: 'Add New Node',
  }

  return (
    <Dialog
      open={configDialogOpen}
      onClose={closeConfigDialog}
      maxWidth="sm"
      fullWidth
      PaperProps={{ sx: { borderRadius: 2 } }}
    >
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', pb: 1 }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 700, flex: 1 }}>
          {titles[configDialogType] ?? 'Configure'}
        </Typography>
        <IconButton size="small" onClick={closeConfigDialog}><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <Divider />

      <DialogContent sx={{ pt: 2, pb: 1 }}>
        {configDialogType === 'input'  && <InputConfigForm  data={formData} onChange={setFormData} errors={formErrors} />}
        {isSinkType                    && <OutputConfigForm data={formData} onChange={setFormData} errors={formErrors} />}
        {isNodeType                    && <NodeConfigForm   data={formData} onChange={setFormData} isNew={isAddMode} errors={formErrors} />}
      </DialogContent>

      <Divider />
      <DialogActions sx={{ px: 2, py: 1.5, gap: 1 }}>
        {/* left-aligned destructive / structural actions */}
        {configDialogType === 'node' && !isRootNode && (
          <Button
            variant="outlined" color="error" size="small"
            startIcon={<DeleteIcon />}
            onClick={handleDelete}
            sx={{ mr: 'auto' }}
          >
            Delete Node
          </Button>
        )}
        {configDialogType === 'node' && isContainer && (
          <Box sx={{ mr: 'auto' }}>
            <Button
              variant="outlined" size="small"
              startIcon={<AddIcon />}
              onClick={handleAddChild}
            >
              Add Child Node
            </Button>
          </Box>
        )}

        <Button variant="text" size="small" onClick={closeConfigDialog}>
          Cancel
        </Button>
        <Button variant="contained" size="small" onClick={handleSave} disabled={formErrors.length > 0}>
          {isAddMode ? 'Add Node' : 'Save'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

export default ConfigDialog
