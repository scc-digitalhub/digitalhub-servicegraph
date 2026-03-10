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
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/errorlog"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

type App struct {
	sources.FlowFactory
	graph   *model.Graph
	errorCh chan any // fan-in channel that receives error events from all ErrorFilters
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
	f, err := generateFlow(source, a.graph.Flow)
	if err != nil {
		// generateFlow is called from inside a running source goroutine via the
		// FlowFactory interface (which cannot return an error). Log and panic so
		// the error is visible rather than producing a silent nil-pointer crash.
		panic(fmt.Sprintf("failed to build processing flow: %v", err))
	}
	// Insert a graph-level ErrorFilter so that error events (status >= 400) are
	// diverted to the dedicated error sink instead of reaching the output sink.
	ef := flow.NewErrorFilter(1)
	go func() {
		for e := range ef.ErrOut() {
			a.errorCh <- e
		}
	}()
	f.Via(ef).To(sink)
}

func generateFlow(outlet streams.Source, node *model.Node) (streams.Flow, error) {
	switch node.Type {
	case model.Sequence:
		var f streams.Flow
		for i := range node.Nodes {
			var err error
			if f == nil {
				f, err = generateFlow(outlet, &node.Nodes[i])
			} else {
				f, err = generateFlow(f, &node.Nodes[i])
			}
			if err != nil {
				return nil, err
			}
		}
		return f, nil
	case model.Ensemble:
		fanOut := flow.FanOut(outlet, len(node.Nodes))
		for i := range node.Nodes {
			f, err := generateFlow(fanOut[i], &node.Nodes[i])
			if err != nil {
				return nil, err
			}
			fanOut[i] = f
		}
		spec := node.Config.Spec
		return flow.ZipWith(flow.MergeFunctionFromSpec(spec), fanOut...), nil
	case model.Switch:
		var conditions []jp.Expr
		for i := range node.Nodes {
			x := node.Nodes[i].ConditionExpression()
			conditions = append(conditions, x)
		}
		splitFlows := flow.SplitMulti(outlet, len(node.Nodes), func(in any) int {
			// Error events bypass condition evaluation so they propagate to the
			// default branch and on to the graph-level ErrorFilter unchanged.
			if ev, ok := in.(streams.Event); ok && ev.GetStatus() >= 400 {
				return len(conditions) - 1
			}
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
		for i := range node.Nodes {
			f, err := generateFlow(splitFlows[i], &node.Nodes[i])
			if err != nil {
				return nil, err
			}
			splitFlows[i] = f
		}
		return flow.Merge(splitFlows...), nil
	case model.Service:
		converter, err := nodes.RegistrySingleton.Get(node.Config.Kind)
		if err != nil {
			return nil, fmt.Errorf("unknown node kind %q: %w", node.Config.Kind, err)
		}
		nodeFlow, err := converter.(nodes.Converter).Convert(&node.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create node %q: %w", node.Config.Kind, err)
		}
		return outlet.Via(nodeFlow), nil
	}
	return nil, fmt.Errorf("unsupported node type: %s", node.Type)
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

	// Initialise the error fan-in channel and resolve (or create) the error sink.
	a.errorCh = make(chan any, 64)
	var errSink streams.Sink
	if a.graph.Error != nil {
		c, err := sinks.RegistrySingleton.Get(a.graph.Error.Kind)
		if err != nil {
			return fmt.Errorf("failed to get error sink converter for kind %s: %w", a.graph.Error.Kind, err)
		}
		errSink, err = c.(sinks.Converter).Convert(*a.graph.Error)
		if err != nil {
			return fmt.Errorf("failed to create error sink %s: %w", a.graph.Error.Kind, err)
		}
	} else {
		errSink = errorlog.NewLogSink()
	}
	// Forward error events from the fan-in channel to the error sink.
	// errDone is closed once the error sink has flushed all events.
	errDone := make(chan struct{})
	go func() {
		defer close(errDone)
		for e := range a.errorCh {
			errSink.In() <- e
		}
		close(errSink.In())
		errSink.AwaitCompletion()
	}()

	converter, err := sources.RegistrySingleton.Get(a.graph.Input.Kind)
	if err != nil {
		return fmt.Errorf("failed to get source converter for kind %s: %w", a.graph.Input.Kind, err)
	}
	source, err := converter.(sources.Converter).Convert(*a.graph.Input)
	if err != nil {
		return err
	}

	if a.graph.Output != nil {
		outConverter, err := sinks.RegistrySingleton.Get(a.graph.Output.Kind)
		if err != nil {
			return fmt.Errorf("failed to get output sink converter for kind %s: %w", a.graph.Output.Kind, err)
		}
		sink, err := outConverter.(sinks.Converter).Convert(*a.graph.Output)
		if err != nil {
			return err
		}
		source.StartAsync(a, sink)
	} else {
		// Synchronous source: Start blocks until the source finishes.
		// Close the error fan-in channel afterwards so the forwarding goroutine
		// exits and the error sink flushes any remaining events.
		source.Start(a)
		close(a.errorCh)
		<-errDone
	}

	return nil
}

// Enhanced validateGraph to validate nodes recursively
func validateGraph(graph *model.Graph) error {
	// Guard against missing required fields to avoid nil-pointer panics.
	if graph.Input == nil {
		return fmt.Errorf("graph is missing required 'input' field")
	}
	if graph.Flow == nil {
		return fmt.Errorf("graph is missing required 'flow' field")
	}
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
	// validate error_sink (optional)
	if graph.Error != nil {
		errSinkValidator, err := sinks.RegistrySingleton.Get(graph.Error.Kind)
		if err != nil {
			return fmt.Errorf("unknown error sink kind %q: %w", graph.Error.Kind, err)
		}
		if validator, ok := errSinkValidator.(sinks.Validator); ok {
			if err = validator.Validate(*graph.Error); err != nil {
				return fmt.Errorf("invalid error_sink configuration: %w", err)
			}
		}
	}
	if err := validateNodeNamesUnique(graph.Flow); err != nil {
		return err
	}
	return validateNode(graph.Flow)
}

// validateNodeNamesUnique checks that every named node in the flow tree has a
// unique name, which is required for unambiguous parameter resolution.
func validateNodeNamesUnique(flow *model.Node) error {
	seen := map[string]bool{}
	var check func(*model.Node) error
	check = func(node *model.Node) error {
		if node.Name != "" {
			if seen[node.Name] {
				return fmt.Errorf("duplicate node name %q in graph", node.Name)
			}
			seen[node.Name] = true
		}
		for i := range node.Nodes {
			if err := check(&node.Nodes[i]); err != nil {
				return err
			}
		}
		return nil
	}
	return check(flow)
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
		for i := range node.Nodes {
			if err := validateNode(&node.Nodes[i]); err != nil {
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
		for i := range node.Nodes {
			if err := validateNode(&node.Nodes[i]); err != nil {
				return err
			}
		}
	case model.Switch:
		if len(node.Nodes) < 2 {
			return fmt.Errorf("switch node must have at least two child nodes")
		}
		for i := range node.Nodes {
			if node.Nodes[i].Condition != "" {
				node.Nodes[i].Condition = util.NormalizeJSONPath(node.Nodes[i].Condition)
				if err := util.ValidateJSONPath(node.Nodes[i].Condition); err != nil {
					return fmt.Errorf("invalid JSONPath condition in switch node: %s", err.Error())
				}
			}
		}
		for i := range node.Nodes {
			if err := validateNode(&node.Nodes[i]); err != nil {
				return err
			}
		}
	default:
		// validate node config
		nodeValidator, err := nodes.RegistrySingleton.Get(node.Config.Kind)
		if err != nil {
			return err
		}
		err = nodeValidator.(nodes.Validator).Validate(&node.Config)
		if err != nil {
			return err
		}

		for i := range node.Nodes {
			if err := validateNode(&node.Nodes[i]); err != nil {
				return err
			}
		}
	}

	return nil
}
