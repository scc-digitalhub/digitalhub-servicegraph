package sources

import "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"

type FlowFactory interface {
	GenerateFlow(source streams.Source, sink streams.Sink)
}

type Source interface {
	Start(generator FlowFactory)
	StartAsync(generator FlowFactory, sink streams.Sink)
}
