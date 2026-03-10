// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

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

	return graph, nil
}

// ReadYAMLWithParams reads a YAML-encoded graph configuration and applies the
// provided parameter overrides before returning the parsed Graph. Each key is
// a dot-separated path into the graph's JSON representation (e.g.
// "input.spec.port", "flow.nodes.0.config.spec.url"). Values are coerced to
// int64, float64, bool or string in that order of preference.
func (r *Reader) ReadYAMLWithParams(reader io.Reader, params map[string]string) (*Graph, error) {
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Unmarshal into a raw map so individual fields can be overwritten before
	// the final unmarshal into the typed Graph struct.
	raw := map[string]any{}
	if err := yaml.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, err
	}

	if len(params) > 0 {
		if err := applyParams(raw, params); err != nil {
			return nil, fmt.Errorf("failed to apply parameters: %w", err)
		}
	}

	// Re-serialise through JSON to populate the typed Graph struct.
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	graph := &Graph{}
	if err := json.Unmarshal(jsonBytes, graph); err != nil {
		return nil, err
	}

	return graph, nil
}

// applyParams applies parameter overrides to the raw graph map.
// Each key uses the syntax:
//
//	input.<spec-subpath>       → input.spec.<subpath>
//	output.<spec-subpath>      → output.spec.<subpath>
//	error.<spec-subpath>       → error.spec.<subpath>
//	<node-name>.<spec-subpath> → find named node's config.spec.<subpath>
func applyParams(raw map[string]any, params map[string]string) error {
	nodeSpecs, err := collectNodeSpecs(raw)
	if err != nil {
		return err
	}

	for key, strVal := range params {
		dotIdx := strings.IndexByte(key, '.')
		if dotIdx < 1 {
			return fmt.Errorf("parameter %q must be in the form <section>.<subpath>", key)
		}
		section := key[:dotIdx]
		subpath := strings.Split(key[dotIdx+1:], ".")

		var target map[string]any
		switch section {
		case "input", "output", "error":
			target, err = ensureSpec(raw, section)
			if err != nil {
				return fmt.Errorf("param %q: %w", key, err)
			}
		default:
			var ok bool
			target, ok = nodeSpecs[section]
			if !ok {
				return fmt.Errorf("param %q: unknown section or node name %q", key, section)
			}
		}

		if err := setNestedValue(target, subpath, inferValue(strVal)); err != nil {
			return fmt.Errorf("param %q: %w", key, err)
		}
	}
	return nil
}

// ensureSpec returns (creating if absent) the "spec" sub-map of the named
// top-level section ("input", "output", or "error").
func ensureSpec(raw map[string]any, section string) (map[string]any, error) {
	sec, ok := raw[section]
	if !ok {
		return nil, fmt.Errorf("section %q not found in graph", section)
	}
	secMap, ok := sec.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("section %q is not a map", section)
	}
	spec, _ := secMap["spec"].(map[string]any)
	if spec == nil {
		spec = map[string]any{}
		secMap["spec"] = spec
	}
	return spec, nil
}

// collectNodeSpecs traverses the flow subtree of the raw graph map and returns
// a flat map from node name to the node's config.spec map (created if absent).
// Returns an error if any node name appears more than once.
func collectNodeSpecs(raw map[string]any) (map[string]map[string]any, error) {
	result := map[string]map[string]any{}
	flow, ok := raw["flow"]
	if !ok {
		return result, nil
	}
	if err := collectNodeSpecsFromNode(flow, result); err != nil {
		return nil, err
	}
	return result, nil
}

func collectNodeSpecsFromNode(node any, acc map[string]map[string]any) error {
	nodeMap, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	name, _ := nodeMap["name"].(string)
	if name != "" {
		if _, exists := acc[name]; exists {
			return fmt.Errorf("duplicate node name %q", name)
		}
		cfg, _ := nodeMap["config"].(map[string]any)
		if cfg == nil {
			cfg = map[string]any{}
			nodeMap["config"] = cfg
		}
		spec, _ := cfg["spec"].(map[string]any)
		if spec == nil {
			spec = map[string]any{}
			cfg["spec"] = spec
		}
		acc[name] = spec
	}
	nodes, _ := nodeMap["nodes"].([]any)
	for _, child := range nodes {
		if err := collectNodeSpecsFromNode(child, acc); err != nil {
			return err
		}
	}
	return nil
}

// setNestedValue navigates the nested map/slice structure following parts and
// sets the leaf to value. Intermediate maps are created when missing.
func setNestedValue(current any, parts []string, value any) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	switch c := current.(type) {
	case map[string]any:
		if len(parts) == 1 {
			c[parts[0]] = value
			return nil
		}
		next, exists := c[parts[0]]
		if !exists {
			next = map[string]any{}
			c[parts[0]] = next
		}
		return setNestedValue(next, parts[1:], value)

	case []any:
		idx, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("expected array index, got %q", parts[0])
		}
		if idx < 0 || idx >= len(c) {
			return fmt.Errorf("index %d out of bounds (len=%d)", idx, len(c))
		}
		if len(parts) == 1 {
			c[idx] = value
			return nil
		}
		return setNestedValue(c[idx], parts[1:], value)

	default:
		return fmt.Errorf("cannot navigate into %T at path segment %q", current, parts[0])
	}
}

// inferValue coerces a string to the most specific type: int64, float64, bool,
// or string (in that order).
func inferValue(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return s
}
