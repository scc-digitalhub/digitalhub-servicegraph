// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"io"
	"log/slog"

	"sigs.k8s.io/yaml"
)

type Reader struct {
	logger *slog.Logger
}

func NewReader() (*Reader, error) {
	logger := slog.Default()
	logger = logger.With(slog.Group("model", slog.String("name", "reader")))

	return &Reader{
		logger: logger,
	}, nil
}

func (r *Reader) ReadYAML(reader io.Reader) (*Graph, error) {
	graph := &Graph{}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(bodyBytes, &graph); err != nil {
		return nil, err
	}

	return graph, nil
}
