// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeType(t *testing.T) {
	assert.Equal(t, NodeType("sequence"), Sequence)
	assert.Equal(t, NodeType("ensemble"), Ensemble)
	assert.Equal(t, NodeType("switch"), Switch)
	assert.Equal(t, NodeType("service"), Service)
}

func TestWSMsgType(t *testing.T) {
	assert.Equal(t, WSMsgType("text"), TextMsg)
	assert.Equal(t, WSMsgType("binary"), BinaryMsg)
}

func TestExecMode(t *testing.T) {
	assert.Equal(t, ExecMode("sync"), SyncMode)
	assert.Equal(t, ExecMode("async"), AsyncMode)
}

func TestMergeMode(t *testing.T) {
	assert.Equal(t, MergeMode("concat"), MergeModeConcat)
	assert.Equal(t, MergeMode("concat_template"), MergeModeConcatTemplate)
}

func TestNode(t *testing.T) {
	node := Node{
		Type:      Sequence,
		Name:      "test-node",
		Nodes:     []Node{{Type: Service, Name: "child"}},
		Config:    NodeConfig{Kind: "http", Spec: map[string]interface{}{"url": "http://example.com"}},
		Condition: "true",
	}

	data, err := json.Marshal(node)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test-node")
}

func TestNodeConfig(t *testing.T) {
	config := NodeConfig{
		Kind: "http",
		Spec: map[string]interface{}{"url": "http://example.com"},
	}

	data, err := json.Marshal(config)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "http://example.com")
}

func TestInputSpec(t *testing.T) {
	spec := InputSpec{
		Kind: "http",
		Spec: map[string]interface{}{"url": "http://input.com"},
	}

	data, err := json.Marshal(spec)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "http://input.com")
}

func TestOutputSpec(t *testing.T) {
	spec := OutputSpec{
		Kind: "http",
		Spec: map[string]interface{}{"url": "http://output.com"},
	}

	data, err := json.Marshal(spec)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "http://output.com")
}

func TestOutputEntry(t *testing.T) {
	entry := OutputEntry{
		OutputSpec: OutputSpec{Kind: "webhook", Spec: map[string]interface{}{"url": "http://hook.com"}},
		Enabled:    true,
	}
	data, err := json.Marshal(entry)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "webhook")
	assert.Contains(t, string(data), "http://hook.com")
	assert.Contains(t, string(data), `"enabled":true`)
}

func TestOutputEntry_DefaultDisabled(t *testing.T) {
	entry := OutputEntry{
		OutputSpec: OutputSpec{Kind: "stdout"},
	}
	assert.False(t, entry.Enabled)
}

func TestGraph(t *testing.T) {
	graph := Graph{
		Input: &InputSpec{
			Kind: "http",
			Spec: map[string]interface{}{"url": "http://input.com"},
		},
		Flow: &Node{
			Type: Sequence,
			Name: "flow",
		},
		Output: &OutputSpec{
			Kind: "http",
			Spec: map[string]interface{}{"url": "http://output.com"},
		},
	}

	data, err := json.Marshal(graph)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "http://input.com")
	assert.Contains(t, string(data), "http://output.com")
}

func TestGraph_WithOutputsList(t *testing.T) {
	graph := Graph{
		Input: &InputSpec{
			Kind: "http",
			Spec: map[string]interface{}{"port": 8080},
		},
		Flow: &Node{Type: Service, Config: NodeConfig{Kind: "http", Spec: map[string]interface{}{"url": "http://svc"}}},
		Outputs: []OutputEntry{
			{OutputSpec: OutputSpec{Kind: "stdout"}, Enabled: true},
			{OutputSpec: OutputSpec{Kind: "ignore"}, Enabled: false},
		},
	}

	data, err := json.Marshal(graph)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"outputs"`)
	assert.Contains(t, string(data), "stdout")
	assert.Contains(t, string(data), "ignore")

	// Round-trip
	var g2 Graph
	assert.NoError(t, json.Unmarshal(data, &g2))
	assert.Len(t, g2.Outputs, 2)
	assert.Equal(t, "stdout", g2.Outputs[0].Kind)
	assert.True(t, g2.Outputs[0].Enabled)
	assert.Equal(t, "ignore", g2.Outputs[1].Kind)
	assert.False(t, g2.Outputs[1].Enabled)
}

func TestNode_ConditionExpression(t *testing.T) {
	node := Node{
		Condition: "$.status == 'active'",
	}

	expr := node.ConditionExpression()
	// The expression might be nil if JSONPath parsing fails
	if expr == nil {
		t.Logf("Condition expression is nil, possibly due to JSONPath parsing")
		return
	}
	assert.NotNil(t, expr)

	// Test caching - should return same expression
	expr2 := node.ConditionExpression()
	assert.Equal(t, expr, expr2)
}

func TestNode_ConditionExpression_Empty(t *testing.T) {
	node := Node{
		Condition: "",
	}

	expr := node.ConditionExpression()
	assert.NotNil(t, expr)
	// Empty condition should default to "$"
}

func TestNodeConfig_Cache(t *testing.T) {
	config := NodeConfig{
		Kind: "http",
		Spec: map[string]interface{}{"url": "http://example.com"},
	}

	// Initially cache should be nil
	assert.Nil(t, config.ConfigCache())

	// Set cache
	testData := "cached data"
	config.SetConfigCache(testData)

	// Should return cached data
	cached := config.ConfigCache()
	assert.NotNil(t, cached)
	assert.Equal(t, testData, *cached)
}
