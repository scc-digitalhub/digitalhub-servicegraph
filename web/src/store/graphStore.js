// SPDX-FileCopyrightText: � 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import { create } from 'zustand'

// Available types based on the Go registries
export const NODE_KINDS       = ['http', 'websocket', 'openinference']
export const SOURCE_KINDS     = ['http', 'websocket', 'mjpeg', 'rtsp']
export const SINK_KINDS       = ['stdout', 'file', 'folder', 'ignore', 'webhook', 'websocket']
export const ERROR_SINK_KINDS = ['errorlog']
export const NODE_TYPES       = ['sequence', 'ensemble', 'switch', 'service']

// Default blank node for the "add node" dialog
export const DEFAULT_NEW_NODE = {
  type: 'service',
  name: '',
  config: { kind: 'http', spec: { url: '', method: 'POST' } },
}

// ── Path helpers ─────────────────────────────────────────────────────────────
// path === null  → the root flow node (graph.flow)
// path === []    → (unused – reserved)
// path === [i]   → graph.flow.nodes[i]
// path === [i,j] → graph.flow.nodes[i].nodes[j]  etc.

function nodeAt(flow, path) {
  if (path === null) return flow
  let cur = flow
  for (const idx of path) cur = cur.nodes[idx]
  return cur
}

function parentOf(flow, path) {
  if (path === null || path.length === 0) return null
  return nodeAt(flow, path.slice(0, -1))
}

const initialState = {
  graph: {
    input: {
      kind: 'http',
      spec: { port: 8080 },
    },
    flow: {
      type: 'sequence',
      name: 'root',
      nodes: [],
    },
    output: null,
    error:  null,
  },
  selectedNode: null,

  // Config dialog
  configDialogOpen: false,
  configDialogType: null,   // 'input' | 'output' | 'node' | 'add_node'
  configDialogData: null,
  configDialogPath: null,   // null = root flow, array = path to node being edited/parent for add_node
}

export const useGraphStore = create((set, get) => ({
  ...initialState,

  // ── Source / sink ──────────────────────────────────────────────────────────
  updateInput:  (input)  => set((s) => ({ graph: { ...s.graph, input } })),
  updateOutput: (output) => set((s) => ({ graph: { ...s.graph, output } })),
  updateError:  (error)  => set((s) => ({ graph: { ...s.graph, error } })),

  // ── Node CRUD ──────────────────────────────────────────────────────────────

  /** Update the node at `path` (null = root) with merged fields from `updates`. */
  updateNode: (path, updates) => set((s) => {
    const g = JSON.parse(JSON.stringify(s.graph))
    const node = nodeAt(g.flow, path)
    Object.assign(node, updates)
    return { graph: g }
  }),

  /** Add `node` as a child of the node at `parentPath` (null = root). */
  addNode: (node, parentPath) => set((s) => {
    const g = JSON.parse(JSON.stringify(s.graph))
    const parent = nodeAt(g.flow, parentPath)
    if (!parent.nodes) parent.nodes = []
    parent.nodes.push(node)
    return { graph: g }
  }),

  /** Remove the node at `path`. Cannot remove root (path === null). */
  deleteNode: (path) => set((s) => {
    if (path === null) return s
    const g = JSON.parse(JSON.stringify(s.graph))
    const lastIdx = path[path.length - 1]
    const parent = path.length === 1 ? g.flow : nodeAt(g.flow, path.slice(0, -1))
    parent.nodes.splice(lastIdx, 1)
    return { graph: g }
  }),

  // ── Config dialog ──────────────────────────────────────────────────────────

  /** Open dialog:
   *  type='input'    → data=inputSpec,    path=undefined
   *  type='output'   → data=outputSpec,   path=undefined
   *  type='error'    → data=errorSpec,    path=undefined
   *  type='node'     → data=nodeObject,   path=null|[...]  (null=root)
   *  type='add_node' → data=newNodeStub,  path=null|[...]  (parent path)
   */
  openConfigDialog: (type, data, path) => set({
    configDialogOpen: true,
    configDialogType: type,
    configDialogData: data,
    configDialogPath: path ?? null,
  }),

  closeConfigDialog: () => set({
    configDialogOpen: false,
    configDialogType: null,
    configDialogData: null,
    configDialogPath: null,
  }),

  // ── Selection ──────────────────────────────────────────────────────────────
  setSelectedNode: (node) => set({ selectedNode: node }),

  // ── Import / Export ────────────────────────────────────────────────────────
  exportGraph: () => {
    const g = JSON.parse(JSON.stringify(get().graph))
    // Strip null top-level fields so the exported YAML is clean
    Object.keys(g).forEach(k => { if (g[k] === null || g[k] === undefined) delete g[k] })
    return g
  },

  importGraph: (graph) => set({ graph }),

  // ── Reset ──────────────────────────────────────────────────────────────────
  reset: () => set(initialState),
}))

