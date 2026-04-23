// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/app"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/httpclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/openinferenceclient"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/nodes/wsclient"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/base"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/errorlog"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks/mjpeg"

	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/http"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/mjpeg"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/rtsp"
	_ "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sources/websocket"
)

func main() {
	initLogger()
	if len(os.Args) >= 2 && os.Args[1] == "--healthcheck" {
		healthCheck()
		return
	}
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config.yaml> [key=value ...]")
		os.Exit(1)
	}
	configPath := os.Args[1]
	params, err := parseParams(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid parameter: %v\n", err)
		os.Exit(1)
	}
	simpleYaml(configPath, params)
}

// parseParams converts a slice of "key=value" strings into a map.
// The key must be a non-empty dot-separated path; values are passed as-is and
// interpreted during YAML parameter application.
func parseParams(args []string) (map[string]string, error) {
	params := make(map[string]string, len(args))
	for _, arg := range args {
		idx := strings.IndexByte(arg, '=')
		if idx < 1 {
			return nil, fmt.Errorf("parameter %q must use key=value format", arg)
		}
		params[arg[:idx]] = arg[idx+1:]
	}
	return params, nil
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

func simpleYaml(path string, params map[string]string) {
	modelReader, err := model.NewReader()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create model reader: %v\n", err)
		os.Exit(1)
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open config %s: %v\n", path, err)
		os.Exit(1)
	}
	defer file.Close()

	graph, err := modelReader.ReadYAMLWithParams(file, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse config: %v\n", err)
		os.Exit(1)
	}
	a, err := app.NewApp(*graph)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build app: %v\n", err)
		os.Exit(1)
	}
	// Tie the App's lifetime to OS signals (SIGINT/SIGTERM) so that the
	// configured source can shut down gracefully (see sources.Stoppable).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := a.RunWithContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "app exited with error: %v\n", err)
		os.Exit(1)
	}
}
