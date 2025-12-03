package model

import (
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
	configCache *any
}

func (c *NodeConfig) ConfigCache() *any {
	return c.configCache
}

func (c *NodeConfig) SetConfigCache(t any) {
	c.configCache = &t
}

type InputSpec struct {
	Kind string                 `json:"kind"`
	Spec map[string]interface{} `json:"spec,omitempty"`
}
type OutputSpec struct {
	Kind string                 `json:"kind"`
	Spec map[string]interface{} `json:"spec,omitempty"`
}

type Graph struct {
	Input  *InputSpec  `json:"input"`
	Flow   *Node       `json:"flow"`
	Output *OutputSpec `json:"output,omitempty"`
}
