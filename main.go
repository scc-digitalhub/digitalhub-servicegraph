package main

import (
	"os"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/app"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/httpclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/wsclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/base"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/http"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/websocket"
)

func main() {

	// simpleYaml("test/simple-http-sync.yaml")
	// simpleYaml("test/simple-http-async.yaml")
	// simpleYaml("test/simple-ws-sync.yaml")
	// simpleYaml("test/simple-ws-async.yaml")
	simpleYaml("test/simple-http-sync-ensemble.yaml")

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
	app, err := app.NewApp(*graph)
	if err != nil {
		panic(err)
	}
	app.Run()
}
