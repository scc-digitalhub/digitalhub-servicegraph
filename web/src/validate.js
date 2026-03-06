// validate.js — field-level and graph-level validation helpers
// Each validator returns { field: string, message: string }[]

// ── Input (source) ──────────────────────────────────────────────────────────
export function validateInput(data) {
  const errors = []
  if (!data) return errors
  const spec = data.spec || {}
  if (data.kind === 'http' || data.kind === 'websocket') {
    if (!spec.port || Number(spec.port) <= 0)
      errors.push({ field: 'port', message: 'Port is required and must be > 0' })
  }
  if (data.kind === 'mjpeg') {
    if (!spec.url?.trim())
      errors.push({ field: 'url', message: 'URL is required' })
  }
  return errors
}

// ── Output (sink) ───────────────────────────────────────────────────────────
export function validateOutput(data) {
  const errors = []
  if (!data) return errors
  const spec = data.spec || {}
  if (data.kind === 'file') {
    if (!spec.file_name?.trim())
      errors.push({ field: 'file_name', message: 'File name is required' })
  }
  if (data.kind === 'folder') {
    if (!spec.folder_path?.trim())
      errors.push({ field: 'folder_path', message: 'Folder path is required' })
    if (!spec.filename_pattern?.trim())
      errors.push({ field: 'filename_pattern', message: 'Filename pattern is required' })
  }
  if (data.kind === 'webhook' || data.kind === 'websocket') {
    if (!spec.url?.trim())
      errors.push({ field: 'url', message: 'URL is required' })
  }
  return errors
}

// ── Node ────────────────────────────────────────────────────────────────────
export function validateNode(data) {
  const errors = []
  if (!data) return errors
  if (data.type === 'service') {
    const kind = data.config?.kind || 'http'
    const spec = data.config?.spec || {}
    if (kind === 'http' || kind === 'websocket') {
      if (!spec.url?.trim())
        errors.push({ field: 'url', message: 'URL is required' })
    }
    if (kind === 'openinference') {
      if (!spec.address?.trim())
        errors.push({ field: 'address', message: 'Address is required' })
      if (!spec.model_name?.trim())
        errors.push({ field: 'model_name', message: 'Model name is required' })
    }
  }
  if (data.type === 'ensemble') {
    const spec = data.config?.spec || {}
    if (spec.merge_mode === 'concat_template' && !spec.template?.trim())
      errors.push({ field: 'template', message: 'Template is required for concat_template mode' })
  }
  return errors
}

// ── Full graph (for YAML editor) ────────────────────────────────────────────
function validateNodeRecursive(node, path, errors) {
  validateNode(node).forEach(e => errors.push({ ...e, path }))
  if (Array.isArray(node.nodes))
    node.nodes.forEach((child, i) => validateNodeRecursive(child, `${path}.nodes[${i}]`, errors))
}

export function validateGraph(graph) {
  const errors = []
  if (!graph || typeof graph !== 'object') return errors
  validateInput(graph.input).forEach(e => errors.push({ ...e, path: 'input' }))
  if (graph.output)
    validateOutput(graph.output).forEach(e => errors.push({ ...e, path: 'output' }))
  if (graph.error)
    validateOutput(graph.error).forEach(e => errors.push({ ...e, path: 'error' }))
  if (graph.flow)
    validateNodeRecursive(graph.flow, 'flow', errors)
  return errors
}
