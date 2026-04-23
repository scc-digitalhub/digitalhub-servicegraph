// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"sync"

	"github.com/ohler55/ojg/jp"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type NodeType string

const (
	Sequence NodeType = "sequence"
	Ensemble NodeType = "ensemble"
	Switch   NodeType = "switch"
	Service  NodeType = "service"
)

type WSMsgType string

const (
	TextMsg   WSMsgType = "text"
	BinaryMsg WSMsgType = "binary"
)

type ExecMode string

const (
	SyncMode  ExecMode = "sync"
	AsyncMode ExecMode = "async"
)

type MergeMode string

const (
	MergeModeConcat         MergeMode = "concat"
	MergeModeConcatTemplate MergeMode = "concat_template"
)

type Node struct {
	Type                NodeType   `json:"type"`
	Name                string     `json:"name,omitempty"`
	Nodes               []Node     `json:"nodes,omitempty"`
	Config              NodeConfig `json:"config,omitempty"`
	Condition           string     `json:"condition,omitempty"`
	conditionExpression *jp.Expr
}

func (n *Node) ConditionExpression() jp.Expr {
	if n.conditionExpression == nil {
		exprText := n.Condition
		if exprText == "" {
			exprText = "$"
		}
		x, _ := util.BuildJSONPathExpression(exprText)
		n.conditionExpression = &x
	}
	return *n.conditionExpression
}

type NodeConfig struct {
	Kind        string                 `json:"kind"`
	Spec        map[string]interface{} `json:"spec,omitempty"`
	mu          sync.Mutex             //nolint:govet // mutex must not be copied; always use *NodeConfig
	configCache *any
}

// ConfigCache returns the cached parsed configuration, or nil if not yet set.
func (c *NodeConfig) ConfigCache() *any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.configCache
}

// SetConfigCache stores a parsed configuration. Only the first call takes effect;
// subsequent calls are no-ops, making this safe for concurrent use.
func (c *NodeConfig) SetConfigCache(t any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.configCache == nil {
		c.configCache = &t
	}
}

type InputSpec struct {
	Kind string                 `json:"kind"`
	Spec map[string]interface{} `json:"spec,omitempty"`
}
type OutputSpec struct {
	Kind string                 `json:"kind"`
	Spec map[string]interface{} `json:"spec,omitempty"`
}

// OutputEntry is an element in the Graph.Outputs list.
// Enabled controls whether this sink is active; defaults to false, so every
// entry must opt-in explicitly (or be enabled at runtime via run arguments).
type OutputEntry struct {
	OutputSpec `json:",inline"`
	Enabled    bool `json:"enabled"`
}

type Graph struct {
	Input   *InputSpec    `json:"input"`
	Flow    *Node         `json:"flow"`
	Output  *OutputSpec   `json:"output,omitempty"`
	Outputs []OutputEntry `json:"outputs,omitempty"`
	Error   *OutputSpec   `json:"error,omitempty"`
}
