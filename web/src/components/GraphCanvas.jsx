// SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import React, { useCallback, useEffect, useRef, useState } from 'react'
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  Handle,
  Position,
  NodeResizer,
  useNodesState,
  useEdgesState,
} from 'reactflow'
import {
  Button
} from '@mui/material'

import 'reactflow/dist/style.css'
import { useGraphStore, DEFAULT_NEW_NODE } from '../store/graphStore'
import { Plus } from 'lucide-react'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import CallMergeIcon from '@mui/icons-material/CallMerge'
import AltRouteIcon  from '@mui/icons-material/AltRoute'
import SettingsIcon  from '@mui/icons-material/Settings'
import InputIcon     from '@mui/icons-material/Input'
import OutputIcon    from '@mui/icons-material/Output'
import './GraphCanvas.css'

// ── Palette ────────────────────────────────────────────────────────────────
const NODE_COLORS = {
  sequence: '#667eea',
  ensemble: '#10b981',
  switch:   '#f59e0b',
  service:  '#3b82f6',
  input:    '#8b5cf6',
  output:   '#ec4899',
}

const CONTAINER_BG = {
  sequence: 'rgba(102, 126, 234, 0.12)',
  ensemble: 'rgba(16, 185, 129, 0.12)',
  switch:   'rgba(245, 158, 11, 0.12)',
}

const TYPE_ICONS = {
  sequence: PlayArrowIcon,
  ensemble: CallMergeIcon,
  switch:   AltRouteIcon,
  service:  SettingsIcon,
  input:    InputIcon,
  output:   OutputIcon,
}

// ── Layout constants ────────────────────────────────────────────────────────
const LEAF_W      = 190
const LEAF_H      = 50
const H_PAD       = 28   // left/right padding inside a container
const V_PAD_TOP   = 52   // top padding (label area)
const V_PAD_BOT   = 24   // bottom padding
const CHILD_GAP   = 20   // vertical gap between siblings
const MIN_CONT_W  = 260  // minimum container width
const ROOT_X      = 280  // x of input/output nodes
const ROOT_Y_FLOW = 180  // y where root flow node starts

// ── Custom node components ──────────────────────────────────────────────────
function ContainerNode({ data }) {
  const color = NODE_COLORS[data.nodeData?.type] ?? NODE_COLORS.service
  const Icon  = TYPE_ICONS[data.nodeData?.type]
  return (
    <div className={`cn-wrap ${data.selected ? 'cn-selected' : ''}`}
         style={{ borderColor: color }}>
      <NodeResizer
        minWidth={MIN_CONT_W}
        minHeight={V_PAD_TOP + V_PAD_BOT}
        lineStyle={{ borderColor: color, borderWidth: 1 }}
        handleStyle={{ borderColor: color, backgroundColor: 'white', width: 8, height: 8 }}
      />
      <Handle type="target" position={Position.Top} />
      <div className="cn-label" style={{ color, display: 'flex', alignItems: 'center', gap: 4 }}>
        {Icon && <Icon sx={{ fontSize: 13 }} />}
        {data.label}
      </div>
      <Handle type="source" position={Position.Bottom} />
    </div>
  )
}

function IoNode({ data }) {
  const isInput = data.ioType === 'input'
  const color   = isInput ? NODE_COLORS.input : NODE_COLORS.output
  const Icon    = isInput ? InputIcon : OutputIcon
  return (
    <div className="sn-wrap" style={{ background: color }}>
      {!isInput && <Handle type="target" position={Position.Top} />}
      <Icon sx={{ fontSize: 14, mr: 0.5 }} />
      <span>{data.label}</span>
      {isInput && <Handle type="source" position={Position.Bottom} />}
    </div>
  )
}

function ServiceNode({ data }) {
  const color = NODE_COLORS[data.nodeData?.type] ?? NODE_COLORS.service
  const Icon  = TYPE_ICONS[data.nodeData?.type]
  return (
    <div className={`sn-wrap ${data.selected ? 'sn-selected' : ''}`}
         style={{ background: color }}>
      <Handle type="target" position={Position.Top} />
      {Icon && <Icon sx={{ fontSize: 14, mr: 0.5 }} />}
      <span>{data.label}</span>
      <Handle type="source" position={Position.Bottom} />
    </div>
  )
}

const nodeTypes = {
  containerNode: ContainerNode,
  serviceNode:   ServiceNode,
  ioInput:       IoNode,
  ioOutput:      IoNode,
}

// ── Recursive layout builder ────────────────────────────────────────────────
/**
 * Returns { id, rfNodes, rfEdges, w, h }
 * - rfNodes[0] is always the node itself (position set to {0,0}; caller repositions it)
 * - rfNodes[1..] are all descendants with parentId chains set
 */
function computeLayout(node, nodePath, idGen, parentRfId) {
  const id = `node-${idGen.next()}`
  const isContainer = ['sequence', 'ensemble', 'switch'].includes(node.type)
  const baseName = node.name?.trim() || node.type
  const kindTag  = node.type === 'service' && node.config?.kind ? ` (${node.config.kind})` : ''
  const cond     = node.condition ? ` [${node.condition.slice(0, 18)}…]` : ''
  const label    = `${baseName}${kindTag}${cond}`

  const nodeData = {
    label,
    nodePath,
    nodeData: { type: node.type, name: node.name, condition: node.condition, config: node.config },
    isContainer,
    selected: false,
  }

  // ── Leaf (service, or container with no children yet) ──────────────────
  if (!isContainer || !node.nodes?.length) {
    const rfNode = {
      id,
      type: isContainer ? 'containerNode' : 'serviceNode',
      data: nodeData,
      position: { x: 0, y: 0 },
      style: isContainer
        ? {
            background: CONTAINER_BG[node.type] ?? 'rgba(100,100,100,0.1)',
            borderColor: NODE_COLORS[node.type] ?? '#888',
            width:  MIN_CONT_W,
            height: V_PAD_TOP + V_PAD_BOT,
          }
        : {},
    }
    if (parentRfId) { rfNode.parentId = parentRfId; rfNode.extent = 'parent' }
    const w = isContainer ? MIN_CONT_W : LEAF_W
    const h = isContainer ? V_PAD_TOP + V_PAD_BOT : LEAF_H
    return { id, rfNodes: [rfNode], rfEdges: [], w, h }
  }

  // ── Container with children ────────────────────────────────────────────
  const childLayouts = node.nodes.map((child, idx) => {
    const childPath = nodePath === null ? [idx] : [...nodePath, idx]
    return computeLayout(child, childPath, idGen, id)
  })

  const maxChildW = Math.max(LEAF_W, ...childLayouts.map(l => l.w))
  const containerW = Math.max(MIN_CONT_W, maxChildW + 2 * H_PAD)

  let y = V_PAD_TOP
  const intraEdges = []

  childLayouts.forEach((layout, idx) => {
    // Centre child horizontally within container
    const rootOfChild = layout.rfNodes.find(n => n.id === layout.id)
    rootOfChild.position = { x: Math.round((containerW - layout.w) / 2), y }

    // Connecting edges between siblings only make sense for sequence nodes
    if (node.type === 'sequence' && idx < childLayouts.length - 1) {
      intraEdges.push({
        id: `e-sib-${layout.id}-${childLayouts[idx + 1].id}`,
        source: layout.id,
        target: childLayouts[idx + 1].id,
        animated: true,
        style: { stroke: NODE_COLORS.sequence, strokeWidth: 1.5 },
      })
    }
    y += layout.h + CHILD_GAP
  })

  const containerH = y + V_PAD_BOT

  const containerNode = {
    id,
    type: 'containerNode',
    data: nodeData,
    position: { x: 0, y: 0 },
    style: {
      background: CONTAINER_BG[node.type] ?? 'rgba(100,100,100,0.1)',
      borderColor: NODE_COLORS[node.type] ?? '#888',
      width:  containerW,
      height: containerH,
    },
  }
  if (parentRfId) { containerNode.parentId = parentRfId; containerNode.extent = 'parent' }

  return {
    id,
    rfNodes: [containerNode, ...childLayouts.flatMap(l => l.rfNodes)],
    rfEdges: [...intraEdges, ...childLayouts.flatMap(l => l.rfEdges)],
    w: containerW,
    h: containerH,
  }
}

// ── Full graph builder ──────────────────────────────────────────────────────
function buildGraph(graph) {
  const idGen = { counter: 0, next() { return this.counter++ } }
  const rfNodes = []
  const rfEdges = []

  // Input
  rfNodes.push({
    id: 'input',
    type: 'ioInput',
    data: { label: graph.input?.kind ?? 'http', ioType: 'input', nodePath: undefined, nodeData: graph.input },
    position: { x: ROOT_X, y: 40 },
  })

  let lastFlowId = 'input'
  let flowH = 0

  if (graph.flow) {
    const layout = computeLayout(graph.flow, null, idGen, null)
    const root = layout.rfNodes.find(n => n.id === layout.id)
    // Centre root node over the input node
    root.position = { x: ROOT_X - Math.round((layout.w - 160) / 2), y: ROOT_Y_FLOW }
    rfNodes.push(...layout.rfNodes)
    rfEdges.push(...layout.rfEdges)
    rfEdges.push({
      id: 'e-input-flow',
      source: 'input',
      target: layout.id,
      animated: true,
      style: { stroke: NODE_COLORS.input, strokeWidth: 2 },
    })
    lastFlowId = layout.id
    flowH = layout.h
  }

  if (graph.output) {
    rfNodes.push({
      id: 'output',
      type: 'ioOutput',
      data: { label: graph.output.kind, ioType: 'output', nodePath: undefined, nodeData: graph.output },
      position: { x: ROOT_X, y: ROOT_Y_FLOW + flowH + 80 },
    })
    rfEdges.push({
      id: 'e-flow-output',
      source: lastFlowId,
      target: 'output',
      animated: true,
      style: { stroke: NODE_COLORS.output, strokeWidth: 2 },
    })
  }

  return { rfNodes, rfEdges }
}

// ── Apply selection highlight to a node list ────────────────────────────────
function applySelection(nodes, selectedPath) {
  return nodes.map(n => {
    if (!n.data?.nodePath === undefined) return n
    const isSelected = selectedPath === null
      ? n.data.nodePath === null && n.data.isContainer
      : Array.isArray(n.data.nodePath) &&
        n.data.nodePath.length === selectedPath.length &&
        n.data.nodePath.every((v, i) => v === selectedPath[i])
    if (!n.data?.nodeData) return n
    return {
      ...n,
      data: { ...n.data, selected: isSelected },
    }
  })
}

// ── Merge user resize overrides into a freshly-built node list ─────────────
function applySavedSizes(rfNodes, savedSizes) {
  return rfNodes.map(n => {
    if (!n.data?.isContainer) return n
    const key = JSON.stringify(n.data.nodePath)
    const saved = savedSizes[key]
    if (!saved) return n
    return { ...n, style: { ...n.style, width: saved.width, height: saved.height } }
  })
}

// ── Component ──────────────────────────────────────────────────────────────
function GraphCanvas() {
  const { graph, openConfigDialog } = useGraphStore()
  const [selectedContainerPath, setSelectedContainerPath] = useState(null)

  const { rfNodes: rawNodes, rfEdges } = buildGraph(graph)
  const rfNodes = applySelection(rawNodes, selectedContainerPath)

  const [nodes, setNodes, onNodesChange] = useNodesState(rfNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(rfEdges)

  // Track user-applied resize dimensions keyed by JSON.stringify(nodePath)
  const userSizesRef = useRef({})
  // Mirror nodes into a ref so the change handler can read current values without a stale closure
  const nodesRef = useRef(nodes)
  useEffect(() => { nodesRef.current = nodes }, [nodes])

  useEffect(() => {
    const { rfNodes: n, rfEdges: e } = buildGraph(graph)
    setNodes(applySavedSizes(applySelection(n, selectedContainerPath), userSizesRef.current))
    setEdges(e)
  }, [graph, selectedContainerPath])  // eslint-disable-line react-hooks/exhaustive-deps

  // Intercept resize events and persist them so rebuilds don't reset user-chosen sizes
  const handleNodesChange = useCallback((changes) => {
    changes.forEach(change => {
      if (change.type === 'dimensions' && change.dimensions) {
        const node = nodesRef.current.find(n => n.id === change.id)
        if (node?.data?.isContainer) {
          userSizesRef.current[JSON.stringify(node.data.nodePath)] = change.dimensions
        }
      }
    })
    onNodesChange(changes)
  }, [onNodesChange])

  const onNodeClick = useCallback((_, rfNode) => {
    if (rfNode.id === 'input') {
      openConfigDialog('input', graph.input)
      setSelectedContainerPath(null)
    } else if (rfNode.id === 'output') {
      openConfigDialog('output', graph.output)
      setSelectedContainerPath(null)
    } else {
      const { nodePath, nodeData, isContainer } = rfNode.data
      if (isContainer) setSelectedContainerPath(nodePath)
      else setSelectedContainerPath(null)
      openConfigDialog('node', nodeData, nodePath)
    }
  }, [graph, openConfigDialog])

  const handleAddNode = useCallback(() => {
    openConfigDialog('add_node', JSON.parse(JSON.stringify(DEFAULT_NEW_NODE)), selectedContainerPath)
  }, [openConfigDialog, selectedContainerPath])

  const addTargetLabel = selectedContainerPath === null
    ? 'root'
    : `container (depth ${selectedContainerPath.length})`

  return (
    <div className="graph-canvas">
      <div className="canvas-toolbar">

        <button className="btn btn-primary btn-sm" onClick={handleAddNode}>
          <Plus size={14} /> Add Node
        </button>
        <span className="canvas-hint">
          Adding to: <strong>{addTargetLabel}</strong> — click a container to change target
        </span>
      </div>

      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={handleNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        fitView
        fitViewOptions={{ padding: 0.25 }}
      >
        <Background />
        <Controls />
        <MiniMap
          nodeColor={(n) => {
            if (n.id === 'input')  return NODE_COLORS.input
            if (n.id === 'output') return NODE_COLORS.output
            return n.style?.background?.startsWith('rgba')
              ? (NODE_COLORS[n.data?.nodeData?.type] ?? NODE_COLORS.service)
              : (n.style?.background ?? NODE_COLORS.service)
          }}
        />
      </ReactFlow>
    </div>
  )
}

export default GraphCanvas
