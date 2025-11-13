package model

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
	MergeModeConcat MergeMode = "concat"
)

type Node struct {
	Type      NodeType   `json,yaml:"type"`
	Name      string     `json,yaml:"name,omitempty"`
	Nodes     []Node     `json,yaml:"nodes,omitempty"`
	Config    NodeConfig `json,yaml:"config,omitempty"`
	MergeMode MergeMode  `json,yaml:"merge_mode,omitempty"`
	Condition string     `json,yaml:"condition,omitempty"`
}

type NodeConfig struct {
	Kind string                 `json,yaml:"kind"`
	Spec map[string]interface{} `json,yaml:"spec,omitempty"`
}

type InputSpec struct {
	Kind string                 `json,yaml:"kind"`
	Spec map[string]interface{} `json,yaml:"spec,omitempty"`
}
type OutputSpec struct {
	Kind string                 `json,yaml:"kind"`
	Spec map[string]interface{} `json,yaml:"spec,omitempty"`
}

type Graph struct {
	Input  *InputSpec  `json,yaml:"input"`
	Flow   *Node       `json,yaml:"flow"`
	Output *OutputSpec `json,yaml:"output,omitempty"`
}
