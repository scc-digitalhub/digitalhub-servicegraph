package main

import (
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/http"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
)

func main() {
	// mainHttpSync()
	mainHttpAsync()
}

func mainHttpSync() {
	http.NewHTTPSource(http.NewConfiguration(8080, 0, 0, 0, 100000)).Start(&TestFactory{})
}

func mainHttpAsync() {
	http.NewHTTPSource(http.NewConfiguration(8080, 0, 0, 0, 100000)).StartAsync(&TestFactory{}, extension.NewStdoutSink())
}

type TestFactory struct {
	sources.FlowFactory
}

func (f *TestFactory) GenerateFlow(source streams.Source, sink streams.Sink) {
	source.Via(flow.NewMap(mock, 1)).To(sink)
}

func mock(input interface{}) interface{} {
	switch v := input.(type) {
	case string:
		return v + " - processed"
	case streams.Event:
		return string(v.GetBody()) + " - processed"
	}
	return input
}
