package tracing

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ErrOTELProvider        = "otel provider setup error"
	FailedToCreateResource = "failed to create resource"
)

func InitTracer(serviceName, endpoint string) func() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}

	// Criar resource com nome do serviço
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		log.Printf("%s: %v", FailedToCreateResource, err)
		return func() {}
	}

	// Tenta conectar com retries
	var conn *grpc.ClientConn
	for i := 1; i <= 5; i++ {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		conn, err = grpc.DialContext(cctx, endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err == nil {
			break
		}
		log.Printf("%s (tentativa %d/5): %v", ErrOTELProvider, i, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		// Não derruba o serviço, apenas avisa
		log.Printf(" Tracing desativado: %v", err)
		return func() {}
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		log.Printf("%s: %v", ErrOTELProvider, err)
		return func() {}
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	log.Printf(" OTEL tracing inicializado para %s em %s", serviceName, endpoint)

	return func() {
		_ = tp.Shutdown(context.Background())
	}
}
