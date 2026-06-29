package telemetry

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var Tracer trace.Tracer

// InitTracer initializes OpenTelemetry distributed tracing.
// By default, traces go to io.Discard. Set OTEL_EXPORTER_STDOUT=true to
// dump spans as JSON to stdout (useful for local debugging or log-shipping).
func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
	var w io.Writer = io.Discard
	if os.Getenv("OTEL_EXPORTER_STDOUT") == "true" {
		w = os.Stdout
	}

	exporter, err := stdouttrace.New(stdouttrace.WithWriter(w))
	if err != nil {
		return nil, err
	}

	res := resource.NewWithAttributes(
		"https://opentelemetry.io/schemas/1.4.0",
		attribute.String("service.name", serviceName),
		attribute.String("deployment.environment", os.Getenv("APP_ENV")),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	Tracer = otel.Tracer(serviceName)

	log.Println("[Telemetry] OpenTelemetry tracer initialized (service=" + serviceName + ")")
	return tp, nil
}

// TraceMiddleware instruments each Gin HTTP request as an OpenTelemetry span.
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if Tracer == nil {
			c.Next()
			return
		}

		spanName := c.FullPath()
		if spanName == "" {
			spanName = c.Request.URL.Path
		}

		ctx, span := Tracer.Start(
			c.Request.Context(),
			c.Request.Method+" "+spanName,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Propagate trace context into the request
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// Shutdown gracefully shuts down the trace provider, flushing pending spans.
func Shutdown(tp *sdktrace.TracerProvider) {
	if tp == nil {
		return
	}
	if err := tp.Shutdown(context.Background()); err != nil {
		log.Printf("[Telemetry] Error shutting down tracer provider: %v", err)
	}
}
