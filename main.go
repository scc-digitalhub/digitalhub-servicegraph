// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/app"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/httpclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/openinferenceclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/wsclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/base"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/http"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/mjpeg"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/websocket"
)

func main() {
	initLogger()
	if len(os.Args) >= 2 && os.Args[1] == "--healthcheck" {
		healthCheck()
		return
	}
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config.yaml>")
		os.Exit(1)
	}
	configPath := os.Args[1]
	simpleYaml(configPath)
}

// healthCheck probes the app's health endpoint and exits non-zero on failure.
// Used by the Docker HEALTHCHECK instruction:
//
//	docker HEALTHCHECK CMD ["/app/servicegraph", "--healthcheck"]
func healthCheck() {
	port := os.Getenv("HEALTH_PORT")
	if port == "" {
		port = "8090"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:" + port + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "health check returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}
}

// initLogger configures the default slog logger based on the LOG_LEVEL
// environment variable. Supported values (case-insensitive): debug, info,
// warn, error. Defaults to info.
func initLogger() {
	var level slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
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
