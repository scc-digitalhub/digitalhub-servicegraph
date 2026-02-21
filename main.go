// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/app"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/httpclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/wsclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/base"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/http"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/mjpeg"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/websocket"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config.yaml>")
		os.Exit(1)
	}
	configPath := os.Args[1]
	simpleYaml(configPath)
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
