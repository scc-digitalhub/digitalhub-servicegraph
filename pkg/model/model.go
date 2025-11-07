package model

type NodeType string

const (
	Sequence NodeType = "sequence"
	Ensemble NodeType = "ensemble"
	Split    NodeType = "split"
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

type Node struct {
	Type      NodeType   `json,yaml:"type"`
	Name      string     `json,yaml:"name,omitempty"`
	Condition string     `json,yaml:"condition,omitempty"`
	Nodes     []Node     `json,yaml:"nodes,omitempty"`
	Config    NodeConfig `json,yaml:"config,omitempty"`
}

type NodeConfig struct {
	Kind string                 `json,yaml:"kind"`
	Spec map[string]interface{} `json,yaml:"spec,omitempty"`
}

type HTTPSpec struct {
	URL     string            `json,yaml:"url"`
	Method  string            `json,yaml:"method"`
	Headers map[string]string `json,yaml:"headers,omitempty"`
	Params  map[string]string `json,yaml:"params,omitempty"`
}

type WebSocketSpec struct {
	URL     string            `json,yaml:"url"`
	Params  map[string]string `json,yaml:"params,omitempty"`
	MsgType WSMsgType         `json,yaml:"msg_type,omitempty"`
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
