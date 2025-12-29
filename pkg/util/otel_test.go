// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"os"
	"testing"
)

func TestIsOTelTracingEnabled(t *testing.T) {
	// Save original env vars
	originalExporter := os.Getenv(OtelTracesExporterKey)
	originalEndpoint := os.Getenv(OtelExporterOtlpEndpointKey)
	defer func() {
		os.Setenv(OtelTracesExporterKey, originalExporter)
		os.Setenv(OtelExporterOtlpEndpointKey, originalEndpoint)
	}()

	tests := []struct {
		name     string
		exporter string
		endpoint string
		expected bool
	}{
		{"disabled - empty exporter", "", "", false},
		{"disabled - none exporter", "none", "", false},
		{"disabled - otlp without endpoint", "otlp", "", false},
		{"enabled - jaeger exporter", "jaeger", "", true},
		{"enabled - otlp with endpoint", "otlp", "http://localhost:4317", true},
		{"enabled - zipkin exporter", "zipkin", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(OtelTracesExporterKey, tt.exporter)
			os.Setenv(OtelExporterOtlpEndpointKey, tt.endpoint)

			result := IsOTelTracingEnabled()
			if result != tt.expected {
				t.Errorf("IsOTelTracingEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSetupOTelSDK(t *testing.T) {
	// Save original env vars
	originalExporter := os.Getenv(OtelTracesExporterKey)
	originalEndpoint := os.Getenv(OtelExporterOtlpEndpointKey)
	originalServiceName := os.Getenv(OtelServiceNameKey)
	defer func() {
		os.Setenv(OtelTracesExporterKey, originalExporter)
		os.Setenv(OtelExporterOtlpEndpointKey, originalEndpoint)
		os.Setenv(OtelServiceNameKey, originalServiceName)
	}()

	// Test with tracing disabled
	os.Setenv(OtelTracesExporterKey, "")
	shutdown, err := SetupOTelSDK(context.Background())
	if err != nil {
		t.Errorf("SetupOTelSDK() with tracing disabled should not error, got %v", err)
	}
	if shutdown == nil {
		t.Error("SetupOTelSDK() should return a shutdown function even when disabled")
	}

	// Test shutdown function
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown function should not error, got %v", err)
	}
}

func TestFinalizeOTelSpans(t *testing.T) {
	// Save original env vars
	originalExporter := os.Getenv(OtelTracesExporterKey)
	defer func() {
		os.Setenv(OtelTracesExporterKey, originalExporter)
	}()

	// Test with tracing disabled
	os.Setenv(OtelTracesExporterKey, "")
	ctx := context.Background()
	// Should not panic
	FinalizeOTelSpans(ctx)

	// Test with tracing enabled but no span in context
	os.Setenv(OtelTracesExporterKey, "console")
	ctx = context.Background()
	// Should not panic
	FinalizeOTelSpans(ctx)
}

func TestIsTextContentType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"text/plain", true},
		{"application/json", true},
		{"application/xml", true},
		{"application/x-www-form-urlencoded", true},
		{"application/octet-stream", false},
		{"image/png", false},
		{"", false},
		{"text/html", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := isTextContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("isTextContentType(%q) = %v, want %v", tt.contentType, result, tt.expected)
			}
		})
	}
}

func TestAddResponseToSpan(t *testing.T) {
	// Save original env vars
	originalExporter := os.Getenv(OtelTracesExporterKey)
	defer func() {
		os.Setenv(OtelTracesExporterKey, originalExporter)
	}()

	// Test with tracing disabled
	os.Setenv(OtelTracesExporterKey, "")
	ctx := context.Background()
	body := []byte("test response")
	AddResponseToSpan(ctx, body, "application/json")
	// Should not panic

	// Test with tracing enabled but no span
	os.Setenv(OtelTracesExporterKey, "console")
	ctx = context.Background()
	AddResponseToSpan(ctx, body, "application/json")
	// Should not panic
}

func TestAddRequestToSpan(t *testing.T) {
	// Save original env vars
	originalExporter := os.Getenv(OtelTracesExporterKey)
	defer func() {
		os.Setenv(OtelTracesExporterKey, originalExporter)
	}()

	// Test with tracing disabled
	os.Setenv(OtelTracesExporterKey, "")
	ctx := context.Background()
	body := []byte("test request")
	AddRequestToSpan(ctx, body, "application/json")
	// Should not panic

	// Test with tracing enabled but no span
	os.Setenv(OtelTracesExporterKey, "console")
	ctx = context.Background()
	AddRequestToSpan(ctx, body, "application/json")
	// Should not panic
}

func TestAddAttrToSpan(t *testing.T) {
	// Save original env vars
	originalExporter := os.Getenv(OtelTracesExporterKey)
	defer func() {
		os.Setenv(OtelTracesExporterKey, originalExporter)
	}()

	// Test with tracing disabled
	os.Setenv(OtelTracesExporterKey, "")
	ctx := context.Background()
	body := []byte("test body")
	addAttrToSpan(ctx, body, "application/json", "test.attribute")
	// Should not panic

	// Test with empty body
	os.Setenv(OtelTracesExporterKey, "console")
	ctx = context.Background()
	addAttrToSpan(ctx, []byte{}, "application/json", "test.attribute")
	// Should not add attribute due to empty body

	// Test with non-text content type
	addAttrToSpan(ctx, body, "application/octet-stream", "test.attribute")
	// Should not add attribute due to content type
}

func TestNewPropagator(t *testing.T) {
	prop := newPropagator()
	if prop == nil {
		t.Error("newPropagator() should return a non-nil propagator")
	}
}

func TestNewTraceExporter(t *testing.T) {
	// Save original env vars
	originalExporter := os.Getenv(OtelTracesExporterKey)
	originalEndpoint := os.Getenv(OtelExporterOtlpEndpointKey)
	defer func() {
		os.Setenv(OtelTracesExporterKey, originalExporter)
		os.Setenv(OtelExporterOtlpEndpointKey, originalEndpoint)
	}()

	// Test with default (no env vars set)
	os.Unsetenv(OtelTracesExporterKey)
	os.Unsetenv(OtelExporterOtlpEndpointKey)

	ctx := context.Background()
	exporter, err := newTraceExporter(ctx)
	// This might return an error depending on the autoexport configuration
	// We just test that it doesn't panic
	_ = exporter
	_ = err
}

func TestNewTracerProvider(t *testing.T) {
	// This is a simple constructor test
	tp := newTracerProvider(nil, nil)
	if tp == nil {
		t.Error("newTracerProvider() should return a non-nil TracerProvider")
	}
}

func TestNewMetricExporter(t *testing.T) {
	ctx := context.Background()
	reader, err := newMetricExporter(ctx)
	// This might return an error depending on the autoexport configuration
	// We just test that it doesn't panic
	_ = reader
	_ = err
}

func TestNewMeterProvider(t *testing.T) {
	mp, err := newMeterProvider(nil, nil)
	if err != nil {
		t.Errorf("newMeterProvider() should not return an error, got %v", err)
	}
	if mp == nil {
		t.Error("newMeterProvider() should return a non-nil MeterProvider")
	}
}

func TestConstants(t *testing.T) {
	// Test that constants are defined and have expected values
	if defaultServiceName != "servicegraph" {
		t.Errorf("defaultServiceName = %q, want %q", defaultServiceName, "servicegraph")
	}

	if OtelTracesExporterKey != "OTEL_TRACES_EXPORTER" {
		t.Errorf("OtelTracesExporterKey = %q, want %q", OtelTracesExporterKey, "OTEL_TRACES_EXPORTER")
	}

	if OtelExporterOtlpEndpointKey != "OTEL_EXPORTER_OTLP_ENDPOINT" {
		t.Errorf("OtelExporterOtlpEndpointKey = %q, want %q", OtelExporterOtlpEndpointKey, "OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	if OtelServiceNameKey != "OTEL_SERVICE_NAME" {
		t.Errorf("OtelServiceNameKey = %q, want %q", OtelServiceNameKey, "OTEL_SERVICE_NAME")
	}

	if ResponceBodyValueAttributeKey != "http.response.body.value" {
		t.Errorf("ResponceBodyValueAttributeKey = %q, want %q", ResponceBodyValueAttributeKey, "http.response.body.value")
	}

	if RequestBodyValueAttributeKey != "http.request.body.value" {
		t.Errorf("RequestBodyValueAttributeKey = %q, want %q", RequestBodyValueAttributeKey, "http.request.body.value")
	}
}
