// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	defaultServiceName            = "servicegraph"
	OtelTracesExporterKey         = "OTEL_TRACES_EXPORTER"
	OtelExporterOtlpEndpointKey   = "OTEL_EXPORTER_OTLP_ENDPOINT"
	OtelServiceNameKey            = "OTEL_SERVICE_NAME"
	ResponceBodyValueAttributeKey = "http.response.body.value"
	RequestBodyValueAttributeKey  = "http.request.body.value"
)

// IsOTelTracingEnabled checks whether OpenTelemetry is enabled via environment variables.
func IsOTelTracingEnabled() bool {
	// For simplicity, we consider OTel enabled if theOTEL_TRACES_EXPORTER is defined and OTEL_EXPORTER_OTLP_ENDPOINT env var is set.
	exporter := os.Getenv(OtelTracesExporterKey)
	if exporter == "" || exporter == "none" || exporter == "otlp" && os.Getenv(OtelExporterOtlpEndpointKey) == "" {
		return false
	}
	return true
}

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupOTelSDK(ctx context.Context) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	var err error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	name := os.Getenv(OtelServiceNameKey)
	if name == "" {
		name = defaultServiceName
	}
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(name),
		),
	)
	if err != nil {
		return shutdown, err
	}

	// Set up trace provider.
	exp, err := newTraceExporter(ctx)
	if err != nil {
		return shutdown, err
	}
	tracerProvider := newTracerProvider(r, exp)
	otel.SetTracerProvider(tracerProvider)

	metricReader, err := newMetricExporter(ctx)
	if err != nil {
		return shutdown, err
	}

	meterProvider, err := newMeterProvider(r, metricReader)
	if err != nil {
		return shutdown, err
	}
	otel.SetMeterProvider(meterProvider)

	return shutdown, err
}

// FinalizeOTelSpans ends the span associated with the provided context, if any.
func FinalizeOTelSpans(ctx context.Context) {
	if !IsOTelTracingEnabled() {
		return
	}
	span := oteltrace.SpanFromContext(ctx)
	if span != nil {
		span.End()
	}
}

func AddResponseToSpan(ctx context.Context, body []byte, contentType string) {
	addAttrToSpan(ctx, body, contentType, ResponceBodyValueAttributeKey)
}
func AddRequestToSpan(ctx context.Context, body []byte, contentType string) {
	addAttrToSpan(ctx, body, contentType, RequestBodyValueAttributeKey)
}
func addAttrToSpan(ctx context.Context, body []byte, contentType string, attributeKey string) {
	if !IsOTelTracingEnabled() {
		return
	}
	if len(body) == 0 {
		return
	}
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && isTextContentType(contentType) {
		span.SetAttributes(attribute.String(attributeKey, string(body)))
	}
}

func isTextContentType(contentType string) bool {
	return contentType == "text/plain" || contentType == "application/json" || contentType == "application/xml" || contentType == "application/x-www-form-urlencoded"
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// newExporter creates an exporter based on the value of the OTEL_TRACES_EXPORTER env vars.
func newTraceExporter(ctx context.Context) (trace.SpanExporter, error) {
	return autoexport.NewSpanExporter(ctx)
}

func newTracerProvider(r *resource.Resource, exp trace.SpanExporter) *trace.TracerProvider {

	return trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(r),
	)
}

// newMetricExporter creates a metric reader based on the value of the OTEL_METRICS_EXPORTER env vars.
func newMetricExporter(ctx context.Context) (metric.Reader, error) {
	// enc := json.NewEncoder(os.Stdout)
	// enc.SetIndent("", "  ")
	// exp, err := stdoutmetric.New(
	// 	stdoutmetric.WithEncoder(enc),
	// 	stdoutmetric.WithoutTimestamps(),
	// )
	// if err != nil {
	// 	panic(err)
	// }
	// return metric.NewPeriodicReader(exp), nil
	return autoexport.NewMetricReader(ctx)
}

func newMeterProvider(r *resource.Resource, metricReader metric.Reader) (*metric.MeterProvider, error) {

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(r),
		metric.WithReader(metricReader),
	)
	return meterProvider, nil
}
