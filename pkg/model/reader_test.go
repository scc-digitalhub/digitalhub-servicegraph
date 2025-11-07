package model

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadYAML_ValidGraph(t *testing.T) {
	reader, err := NewReader()
	assert.NoError(t, err)

	validYAML := `
input:
  kind: "http"
  spec:
    url: "http://example.com"
flow:
  type: "sequence"
  name: "example-flow"
  nodes:
    - type: "service"
      name: "service1"
output:
  kind: "http"
  spec:
    url: "http://output.com"
`

	graph, err := reader.ReadYAML(bytes.NewReader([]byte(validYAML)))
	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, "http", graph.Input.Kind)
	assert.Equal(t, "sequence", string(graph.Flow.Type))
	assert.Equal(t, "service1", graph.Flow.Nodes[0].Name)
}

func TestReadYAML_InvalidGraph(t *testing.T) {
	reader, err := NewReader()
	assert.NoError(t, err)

	invalidYAML := `
input:
  kind: "http"
  spec:
    url: "http://example.com"
flow:
  type: "unknown-type"
  name: "example-flow"
`

	graph, err := reader.ReadYAML(bytes.NewReader([]byte(invalidYAML)))
	assert.Error(t, err)
	assert.Nil(t, graph)
}
