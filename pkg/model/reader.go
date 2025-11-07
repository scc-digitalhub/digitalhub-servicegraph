package model

import (
	"fmt"
	"io"
	"log/slog"

	"sigs.k8s.io/yaml"
)

type Reader struct {
	logger *slog.Logger
}

func NewReader() (*Reader, error) {
	logger := slog.Default()
	logger = logger.With(slog.Group("model", slog.String("name", "reader")))

	return &Reader{
		logger: logger,
	}, nil
}

func (r *Reader) ReadYAML(reader io.Reader) (*Graph, error) {
	graph := &Graph{}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(bodyBytes, &graph); err != nil {
		return nil, err
	}

	err = r.validateGraph(graph)
	if err != nil {
		return nil, err
	}

	return graph, nil
}

// Enhanced validateGraph to validate nodes recursively
func (r *Reader) validateGraph(graph *Graph) error {
	return r.validateNode(graph.Flow)
}

func (r *Reader) validateNode(node *Node) error {
	validNodeTypes := map[NodeType]bool{
		Sequence: true,
		Ensemble: true,
		Split:    true,
		Service:  true,
	}

	if !validNodeTypes[node.Type] {
		return fmt.Errorf("invalid node type: %s", node.Type)
	}

	for _, child := range node.Nodes {
		if err := r.validateNode(&child); err != nil {
			return err
		}
	}

	return nil
}
