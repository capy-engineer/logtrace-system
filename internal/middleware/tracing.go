package middleware

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// InitTracer initializes an OTLP exporter and configures the OpenTelemetry trace provider
func InitTracer(serviceName, jaegerEndpoint string) (func(context.Context) error, error) {
	ctx := context.Background()

	// Create OTLP exporter
	conn, err := grpc.DialContext(ctx, jaegerEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to Jaeger: %w", err)
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Configure the trace provider with the exporter
	res, err := resource.New(ctx,
		resource.WithAttributes(
			// Service name used to identify this service
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create a trace provider with a batch span processor
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator to propagate trace context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return shutdown function that can be deferred
	return func(ctx context.Context) error {
		// Shutdown will flush any remaining spans
		ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		if err := tracerProvider.Shutdown(ctxWithTimeout); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
			return err
		}
		return nil
	}, nil
}

// Tracing returns middleware for Gin that creates spans for each request
func Tracing(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
