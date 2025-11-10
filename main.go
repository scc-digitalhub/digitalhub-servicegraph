package main

import (
	"os"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/app"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/httpclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/wsclient"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/base"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/http"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/websocket"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
)

func main() {
	// mainWebSocketSync()
	// mainWebSocketAsync()
	// simpleYaml("test/simple-http-sync.yaml")
	simpleYaml("test/simple-http-async.yaml")
}

func simpleYaml(path string) {
	modelReader, _ := model.NewReader()

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	graph, err := modelReader.ReadYAML(file)
	if err != nil {
		panic(err)
	}
	app := app.NewApp(*graph)
	app.Run()
}

func mainHttpAsync() {
	http.NewHTTPSource(http.NewConfiguration(8080, 0, 0, 0, 100000)).StartAsync(&TestFactory{}, base.NewStdoutSink())
}

func mainWebSocketSync() {
	websocket.NewWSSource(websocket.NewConfiguration(8080, 2)).Start(&TestFactory{})
}

func mainWebSocketAsync() {
	websocket.NewWSSource(websocket.NewConfiguration(8080, 2)).StartAsync(&TestFactory{}, base.NewStdoutSink())
}

type TestFactory struct {
	sources.FlowFactory
}

func (f *TestFactory) GenerateFlow(source streams.Source, sink streams.Sink) {
	source.Via(flow.NewMap(mock, 1).Via(flow.NewMap(mock, 1))).To(sink)
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
