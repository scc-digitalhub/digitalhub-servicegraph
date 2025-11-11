package app

import (
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
)

type App struct {
	sources.FlowFactory
	graph *model.Graph
}

func NewApp(graph model.Graph) *App {
	return &App{
		graph: &graph,
	}
}

func (a *App) GenerateFlow(source streams.Source, sink streams.Sink) {
	flow := generateFlow(source, a.graph.Flow)
	flow.To(sink)
}

func generateFlow(outlet streams.Source, node *model.Node) streams.Flow {
	switch node.Type {
	// TODO: implement other node types
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
		mergeMode, _ := node.Config.Spec["merge_mode"].(string)
		return flow.ZipWith(flow.MergeFunctionByName(mergeMode), fanOut...)
	case model.Service:
		converter, _ := nodes.RegistrySingleton.Get(node.Config.Kind)
		flow, _ := converter.(nodes.Converter).Convert(node.Config)
		return outlet.Via(flow)
	}
	return nil
}

func (a *App) Run() error {
	converter, _ := sources.RegistrySingleton.Get(a.graph.Input.Kind)
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
