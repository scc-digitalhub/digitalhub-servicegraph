package app

import (
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
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
	// source.Via()
	// TODO
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
		source.StartAsync(a.FlowFactory, sink)
	} else {
		source.Start(a.FlowFactory)
	}

	return nil
}
