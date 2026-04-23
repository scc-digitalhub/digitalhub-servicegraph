// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"expvar"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

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
	source  sources.Source
}

// SourcePort returns the TCP port the configured source is listening on, if
// it exposes a Port() method. Returns 0 if the source is not yet started or
// does not implement port reporting. Useful for tests using port 0.
func (a *App) SourcePort() int {
	if a.source == nil {
		return 0
	}
	if p, ok := a.source.(interface{ Port() int }); ok {
		return p.Port()
	}
	return 0
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
			metricEventsErr.Add(1)
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
		converter, err := nodes.RegistrySingleton.GetConverter(node.Config.Kind)
		if err != nil {
			return nil, fmt.Errorf("unknown node kind %q: %w", node.Config.Kind, err)
		}
		nodeFlow, err := converter.Convert(&node.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create node %q: %w", node.Config.Kind, err)
		}
		return outlet.Via(nodeFlow), nil
	}
	return nil, fmt.Errorf("unsupported node type: %s", node.Type)
}

// startHealthServer starts a lightweight HTTP server that exposes GET /health.
// The listening port is read from the HEALTH_PORT environment variable
// (default: 8090). Set HEALTH_PORT=0 (or to the literal value "disabled") to
// skip starting the health server, which is required when running multiple
// App instances in parallel (e.g. tests).
func startHealthServer() {
	port := os.Getenv("HEALTH_PORT")
	if port == "" {
		port = "8090"
	}
	if port == "0" || port == "disabled" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	// Expose process and servicegraph counters at /debug/vars.
	mux.Handle("/debug/vars", expvar.Handler())
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
	return a.RunWithContext(context.Background())
}

// RunWithContext runs the App and returns when the source completes.
// If ctx is canceled (e.g. via signal handling at the call site), the source
// is asked to stop gracefully via the optional sources.Stoppable interface.
func (a *App) RunWithContext(ctx context.Context) error {
	go startHealthServer()

	// Initialise the error fan-in channel and resolve (or create) the error sink.
	a.errorCh = make(chan any, 64)
	var errSink streams.Sink
	if a.graph.Error != nil {
		c, err := sinks.RegistrySingleton.GetConverter(a.graph.Error.Kind)
		if err != nil {
			return fmt.Errorf("failed to get error sink converter for kind %s: %w", a.graph.Error.Kind, err)
		}
		errSink, err = c.Convert(*a.graph.Error)
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

	converter, err := sources.RegistrySingleton.GetConverter(a.graph.Input.Kind)
	if err != nil {
		return fmt.Errorf("failed to get source converter for kind %s: %w", a.graph.Input.Kind, err)
	}
	source, err := converter.Convert(*a.graph.Input)
	if err != nil {
		return err
	}
	a.source = source

	// If the source supports graceful shutdown, hook it up to ctx cancellation.
	stopDone := make(chan struct{})
	if stoppable, ok := source.(sources.Stoppable); ok {
		go func() {
			defer close(stopDone)
			<-ctx.Done()
			shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := stoppable.Stop(shutCtx); err != nil {
				// Best-effort log; do not fail the run.
				log.Printf("source stop error: %v", err)
			}
		}()
	} else {
		close(stopDone)
	}

	if a.graph.Output != nil {
		outConverter, err := sinks.RegistrySingleton.GetConverter(a.graph.Output.Kind)
		if err != nil {
			return fmt.Errorf("failed to get output sink converter for kind %s: %w", a.graph.Output.Kind, err)
		}
		sink, err := outConverter.Convert(*a.graph.Output)
		if err != nil {
			return err
		}
		source.StartAsync(a, sink)
	} else if len(a.graph.Outputs) > 0 {
		// Collect only enabled outputs.
		var activeSinks []streams.Sink
		for i := range a.graph.Outputs {
			entry := &a.graph.Outputs[i]
			if !entry.Enabled {
				continue
			}
			outConverter, err := sinks.RegistrySingleton.GetConverter(entry.Kind)
			if err != nil {
				return fmt.Errorf("failed to get outputs sink converter for kind %s: %w", entry.Kind, err)
			}
			s, err := outConverter.Convert(entry.OutputSpec)
			if err != nil {
				return fmt.Errorf("failed to create outputs sink %s: %w", entry.Kind, err)
			}
			activeSinks = append(activeSinks, s)
		}
		sink := newMultiSink(activeSinks)
		source.StartAsync(a, sink)
	} else {
		// Synchronous source: Start blocks until the source finishes.
		source.Start(a)
	}

	// All start paths block until the source completes (either naturally or
	// because Stop was called via ctx cancellation). Close the error fan-in
	// channel so the forwarding goroutine exits and the error sink flushes.
	close(a.errorCh)
	<-errDone
	// Wait for the optional ctx-driven Stop goroutine to finish.
	<-stopDone

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
	inputValidator, err := sources.RegistrySingleton.GetValidator(graph.Input.Kind)
	if err != nil {
		return err
	}
	if err := inputValidator.Validate(*graph.Input); err != nil {
		return err
	}
	// validate output
	if graph.Output != nil {
		outputValidator, err := sinks.RegistrySingleton.GetValidator(graph.Output.Kind)
		if err != nil {
			return err
		}
		if err := outputValidator.Validate(*graph.Output); err != nil {
			return err
		}
	}
	// validate outputs list
	if len(graph.Outputs) > 0 {
		seenKinds := make(map[string]int, len(graph.Outputs))
		for i := range graph.Outputs {
			entry := &graph.Outputs[i]
			// Outputs are addressed by kind in run-time params; duplicate
			// kinds make that addressing ambiguous.
			if prev, dup := seenKinds[entry.Kind]; dup {
				return fmt.Errorf("duplicate outputs entry for kind %q (indices %d and %d); each kind must appear at most once", entry.Kind, prev, i)
			}
			seenKinds[entry.Kind] = i
			outputValidator, err := sinks.RegistrySingleton.GetValidator(entry.Kind)
			if err != nil {
				return fmt.Errorf("unknown outputs[%d] sink kind %q: %w", i, entry.Kind, err)
			}
			if err := outputValidator.Validate(entry.OutputSpec); err != nil {
				return fmt.Errorf("invalid outputs[%d] (%s) configuration: %w", i, entry.Kind, err)
			}
		}
		// At least one enabled output must exist when using the outputs list.
		hasEnabled := false
		for _, entry := range graph.Outputs {
			if entry.Enabled {
				hasEnabled = true
				break
			}
		}
		if !hasEnabled {
			return fmt.Errorf("outputs list is present but no entry has 'enabled: true'")
		}
	}
	// validate error_sink (optional)
	if graph.Error != nil {
		errSinkValidator, err := sinks.RegistrySingleton.GetValidator(graph.Error.Kind)
		if err != nil {
			return fmt.Errorf("unknown error sink kind %q: %w", graph.Error.Kind, err)
		}
		if err := errSinkValidator.Validate(*graph.Error); err != nil {
			return fmt.Errorf("invalid error_sink configuration: %w", err)
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
		nodeValidator, err := nodes.RegistrySingleton.GetValidator(node.Config.Kind)
		if err != nil {
			return err
		}
		if err := nodeValidator.Validate(&node.Config); err != nil {
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

// multiSink fans out every incoming event to all underlying sinks in parallel.
// It satisfies streams.Sink and is used when multiple enabled outputs are configured.
//
// Each downstream sink receives events through its own bounded buffer
// (multiSinkBuffer). A slow sink does not block the others or the upstream
// producer: when its buffer is full, new events for that sink are dropped and
// counted; the count is logged once when the sink is closed.
type multiSink struct {
	in   chan any
	done chan struct{}
}

// multiSinkBuffer is the per-sink buffer depth used by newMultiSink. Events
// queued beyond this depth for an individual slow sink are dropped (with a
// counter) so that one slow sink cannot back-pressure the whole pipeline.
const multiSinkBuffer = 256

// newMultiSink creates a multiSink that broadcasts to all provided sinks.
// Each sink must be non-nil. If sinks has exactly one element a plain reference
// to that element is returned instead (avoids wrapping overhead).
func newMultiSink(ss []streams.Sink) streams.Sink {
	if len(ss) == 1 {
		return ss[0]
	}
	ms := &multiSink{
		in:   make(chan any),
		done: make(chan struct{}),
	}
	go ms.process(ss)
	return ms
}

func (ms *multiSink) process(ss []streams.Sink) {
	// Per-sink fan-out buffers and pumps. Each pump owns a single downstream
	// sink; it forwards everything it receives and drops on overflow.
	type pump struct {
		ch      chan any
		dropped uint64
	}
	pumps := make([]*pump, len(ss))
	var wg sync.WaitGroup
	for i, s := range ss {
		p := &pump{ch: make(chan any, multiSinkBuffer)}
		pumps[i] = p
		wg.Add(1)
		go func(p *pump, s streams.Sink) {
			defer wg.Done()
			for msg := range p.ch {
				s.In() <- msg
			}
			close(s.In())
			s.AwaitCompletion()
		}(p, s)
	}

	// Pump from the multiSink input to each per-sink buffer with non-blocking
	// sends so a slow sink cannot block the others.
	for msg := range ms.in {
		for _, p := range pumps {
			select {
			case p.ch <- msg:
			default:
				// Buffer full → drop and count.
				p.dropped++
				metricMultiSinkDrops.Add(1)
			}
		}
	}

	// Close every per-sink buffer and wait for all pumps to drain.
	for i, p := range pumps {
		close(p.ch)
		if p.dropped > 0 {
			slog.Default().Warn("multiSink dropped events for slow sink",
				slog.Int("sink_index", i),
				slog.Uint64("dropped", p.dropped))
		}
	}
	wg.Wait()
	close(ms.done)
}

func (ms *multiSink) In() chan<- any {
	return ms.in
}

func (ms *multiSink) AwaitCompletion() {
	<-ms.done
}
