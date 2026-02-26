// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ohler55/ojg/jp"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type App struct {
	sources.FlowFactory
	graph *model.Graph
}

var validNodeTypes = map[model.NodeType]bool{
	model.Sequence: true,
	model.Ensemble: true,
	model.Switch:   true,
	model.Service:  true,
}

func NewApp(graph model.Graph) (*App, error) {
	if err := validateGraph(&graph); err != nil {
		return nil, err
	}
	return &App{
		graph: &graph,
	}, nil
}

func (a *App) GenerateFlow(source streams.Source, sink streams.Sink) {
	flow := generateFlow(source, a.graph.Flow)
	flow.To(sink)
}

func generateFlow(outlet streams.Source, node *model.Node) streams.Flow {
	switch node.Type {
	case model.Sequence:
		var flow streams.Flow = nil
		for _, child := range node.Nodes {
			flow = generateFlow(outlet, &child)
		}
		return flow
	case model.Ensemble:
		fanOut := flow.FanOut(outlet, len(node.Nodes))

		for i, child := range node.Nodes {
			fanOut[i] = generateFlow(fanOut[i], &child)
		}
		spec := node.Config.Spec
		return flow.ZipWith(flow.MergeFunctionFromSpec(spec), fanOut...)
	case model.Switch:
		var conditions []jp.Expr
		for _, child := range node.Nodes {
			x := child.ConditionExpression()
			conditions = append(conditions, x)
		}
		splitFlows := flow.SplitMulti(outlet, len(node.Nodes), func(in any) int {
			for i, condition := range conditions {
				res, err := util.EvaluateJSONPathOnExpr(in, condition)
				if err != nil {
					continue
				}
				if len(res) > 0 {
					return i
				}
			}
			// default to last flow if no condition matches
			return len(conditions) - 1
		})

		for i, child := range node.Nodes {
			splitFlows[i] = generateFlow(splitFlows[i], &child)
		}
		return flow.Merge(splitFlows...)
	case model.Service:
		converter, _ := nodes.RegistrySingleton.Get(node.Config.Kind)
		flow, _ := converter.(nodes.Converter).Convert(node.Config)
		return outlet.Via(flow)
	}
	return nil
}

// startHealthServer starts a lightweight HTTP server that exposes GET /health.
// The listening port is read from the HEALTH_PORT environment variable
// (default: 8090). The server runs in a background goroutine for the
// lifetime of the process.
func startHealthServer() {
	port := os.Getenv("HEALTH_PORT")
	if port == "" {
		port = "8090"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	log.Printf("health server listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("health server error: %v", err)
	}
}

func (a *App) Run() error {
	go startHealthServer()

	converter, err := sources.RegistrySingleton.Get(a.graph.Input.Kind)
	if err != nil {
		return fmt.Errorf("failed to get source converter for kind %s: %w", a.graph.Input.Kind, err)
	}
	source, err := converter.(sources.Converter).Convert(*a.graph.Input)
	if err != nil {
		return err
	}

	if a.graph.Output != nil {
		converter, _ = sinks.RegistrySingleton.Get(a.graph.Output.Kind)
		sink, err := converter.(sinks.Converter).Convert(*a.graph.Output)
		if err != nil {
			return err
		}
		source.StartAsync(a, sink)
	} else {
		source.Start(a)
	}

	return nil
}

// Enhanced validateGraph to validate nodes recursively
func validateGraph(graph *model.Graph) error {
	// validate input
	inputValidator, err := sources.RegistrySingleton.Get(graph.Input.Kind)
	if err != nil {
		return err
	}
	err = inputValidator.(sources.Validator).Validate(*graph.Input)
	if err != nil {
		return err
	}
	// validate output
	if graph.Output != nil {
		outputValidator, err := sinks.RegistrySingleton.Get(graph.Output.Kind)
		if err != nil {
			return err
		}
		if validator, ok := outputValidator.(sinks.Validator); ok {
			err = validator.Validate(*graph.Output)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("output kind %s does not support validation", graph.Output.Kind)
		}
	}
	return validateNode(graph.Flow)
}

func validateNode(node *model.Node) error {
	if !validNodeTypes[node.Type] {
		return fmt.Errorf("invalid node type: %s", node.Type)
	}

	switch node.Type {
	case model.Sequence:
		if len(node.Nodes) == 0 {
			return fmt.Errorf("%s node must have child nodes", node.Type)
		}
		for _, child := range node.Nodes {
			if err := validateNode(&child); err != nil {
				return err
			}
		}
	case model.Ensemble:
		if len(node.Nodes) < 2 {
			return fmt.Errorf("ensemble node must have at least two child nodes")
		}
		spec := node.Config.Spec
		if spec == nil {
			return fmt.Errorf("ensemble node must have merge_mode in config spec")
		}
		mergeMode, ok := spec["merge_mode"].(string)
		if !ok {
			return fmt.Errorf("ensemble node must have merge_mode in config spec")
		}

		if model.MergeMode(mergeMode) != model.MergeModeConcat && model.MergeMode(mergeMode) != model.MergeModeConcatTemplate {
			return fmt.Errorf("ensemble node must have a valid merge mode")
		}
		if model.MergeMode(mergeMode) == model.MergeModeConcatTemplate {
			if _, ok := spec["template"].(string); !ok {
				return fmt.Errorf("ensemble node with concat_template merge mode must have template in config spec")
			}
		}
		for _, child := range node.Nodes {
			if err := validateNode(&child); err != nil {
				return err
			}
		}
	case model.Switch:
		if len(node.Nodes) < 2 {
			return fmt.Errorf("switch node must have at least two child nodes")
		}
		for _, child := range node.Nodes {
			if child.Condition != "" {
				child.Condition = util.NormalizeJSONPath(child.Condition)
				if err := util.ValidateJSONPath(child.Condition); err != nil {
					return fmt.Errorf("invalid JSONPath condition in switch node: %s", err.Error())
				}
			}
		}
		for _, child := range node.Nodes {
			if err := validateNode(&child); err != nil {
				return err
			}
		}
	default:
		// validate node config
		nodeValidator, err := nodes.RegistrySingleton.Get(node.Config.Kind)
		if err != nil {
			return err
		}
		err = nodeValidator.(nodes.Validator).Validate(node.Config)
		if err != nil {
			return err
		}

		for _, child := range node.Nodes {
			if err := validateNode(&child); err != nil {
				return err
			}
		}
	}

	return nil
}
