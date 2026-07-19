package telemetry

import (
	"context"
	"errors"
	"net/http"

	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials/insecure"
)

type Shutdown func(context.Context) error

func Setup(ctx context.Context, settings appconfig.Settings) (Shutdown, error) {
	if !settings.Telemetry.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exporterOptions := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(settings.Telemetry.Endpoint)}
	if settings.Telemetry.Insecure {
		exporterOptions = append(exporterOptions, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
	}
	exporter, err := otlptracegrpc.New(ctx, exporterOptions...)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		"",
		attribute.String("service.name", settings.ServiceName),
		attribute.String("deployment.environment", settings.Environment),
	))
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(shutdownCtx context.Context) error {
		return errors.Join(provider.ForceFlush(shutdownCtx), provider.Shutdown(shutdownCtx))
	}, nil
}

func HTTPMiddleware(serviceName string) func(http.HandlerFunc) http.HandlerFunc {
	middleware := otelhttp.NewMiddleware(serviceName)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return middleware(next).ServeHTTP
	}
}
