package middleware

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// InitTracer initializes an OTLP exporter and configures the trace provider
func InitTracer(serviceName, jaegerEndpoint string) (func(), error) {
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

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	// Set global propagator to propagate trace context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}, nil
}

// Tracing middleware for Gin that creates a span for each request
func Tracing(tracerName string) gin.HandlerFunc {
	tracer := otel.Tracer(tracerName)

	return func(c *gin.Context) {
		// Extract trace context from HTTP headers
		ctx := c.Request.Context()
		carrier := propagation.HeaderCarrier(c.Request.Header)
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

		// Create a new span for this request
		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		ctx, span := tracer.Start(ctx, spanName)
		defer span.End()

		// Set trace context in the Gin context
		c.Request = c.Request.WithContext(ctx)

		// Add trace information to span
		span.SetAttributes(
			semconv.HTTPMethodKey.String(c.Request.Method),
			semconv.HTTPURLKey.String(c.Request.URL.String()),
			semconv.HTTPRouteKey.String(c.FullPath()),
			semconv.HTTPUserAgentKey.String(c.Request.UserAgent()),
			semconv.HTTPClientIPKey.String(c.ClientIP()),
		)

		// Process request
		c.Next()

		// Update span with response information
		span.SetAttributes(
			semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
		)

		// Record error if status code is 4xx or 5xx
		if c.Writer.Status() >= 400 {
			span.RecordError(fmt.Errorf("HTTP %d: %s", c.Writer.Status(), c.Errors.String()))
		}
	}
}
