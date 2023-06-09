package main

import (
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"picket-answersheet-service/src/cmd"
	"picket-answersheet-service/src/config"
)

func main() {
	log.Logger = log.With().Caller().Logger()

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	tp, err := tracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	otel.SetTracerProvider(tp)
	err = cmd.GetRoot(cfg).Execute()
	if err != nil {
		log.Fatal().Err(err).Send()
	}
}

func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("picket-answersheet-service"),
			attribute.String("environment", "development"),
			attribute.Int64("ID", 1),
		)),
	)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}

//func Init() (*sdktrace.TracerProvider, error) {
//	exporter, err := stdout.New(stdout.WithPrettyPrint())
//	if err != nil {
//		return nil, err
//	}
//	tp := sdktrace.NewTracerProvider(
//		sdktrace.WithSampler(sdktrace.AlwaysSample()),
//		sdktrace.WithBatcher(exporter),
//	)
//	otel.SetTracerProvider(tp)
//	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
//	return tp, nil
//}
