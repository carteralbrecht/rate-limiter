package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/carteralbrecht/rate-limiter/internal/server"
	pb "github.com/carteralbrecht/rate-limiter/proto"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
)

type rateLimiterServer struct {
	pb.UnimplementedRateLimiterServer
	rateLimiter *server.RateLimiter
	meter       metric.Meter
	requests    metric.Int64Counter
	remaining   metric.Int64UpDownCounter
	duration    metric.Float64Histogram
	errors      metric.Int64Counter
}

// NewRateLimiterServer creates a new instance of rateLimiterServer with dependency injection.
func NewRateLimiterServer(redisClient *redis.Client, meter metric.Meter) *rateLimiterServer {
	requests, _ := meter.Int64Counter(
		"rate_limiter_requests_total",
		metric.WithDescription("Total number of rate limiter requests"),
	)

	remaining, _ := meter.Int64UpDownCounter(
		"rate_limiter_tokens_remaining",
		metric.WithDescription("Number of tokens remaining in the bucket"),
	)

	duration, _ := meter.Float64Histogram(
		"rate_limiter_request_duration_seconds",
		metric.WithDescription("Duration of rate limiter requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0),
	)

	errors, _ := meter.Int64Counter(
		"rate_limiter_errors_total",
		metric.WithDescription("Total number of rate limiter errors"),
	)

	return &rateLimiterServer{
		rateLimiter: server.NewRateLimiter(redisClient),
		meter:       meter,
		requests:    requests,
		remaining:   remaining,
		duration:    duration,
		errors:      errors,
	}
}

func (s *rateLimiterServer) CheckLimit(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		log.Printf("Request duration for key %s: %.6f seconds", req.Key, duration)
		s.duration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("key", req.Key),
			),
		)
	}()

	allowed, remaining := s.rateLimiter.CheckAndConsumeTokens(ctx, req.Key, int(req.TokenCost))

	s.requests.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("key", req.Key),
			attribute.Bool("allowed", allowed),
		),
	)

	s.remaining.Add(ctx, int64(remaining),
		metric.WithAttributes(
			attribute.String("key", req.Key),
		),
	)

	if !allowed {
		s.errors.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("key", req.Key),
				attribute.String("reason", "rate_limited"),
			),
		)
	}

	return &pb.CheckResponse{Allowed: allowed, Remaining: int32(remaining)}, nil
}

func (s *rateLimiterServer) RefillBucket(ctx context.Context, req *pb.RefillRequest) (*pb.RefillResponse, error) {
	currentTokens := s.rateLimiter.RefillTokens(ctx, req.Key, int(req.LeakRate), int(req.BucketSize))

	s.remaining.Add(ctx, int64(currentTokens),
		metric.WithAttributes(
			attribute.String("key", req.Key),
		),
	)

	return &pb.RefillResponse{CurrentTokens: int32(currentTokens)}, nil
}

func initMeter() (metric.Meter, func(), error) {
	ctx := context.Background()

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:4317"
	}

	exp, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(1*time.Second)),
		),
	)

	otel.SetMeterProvider(meterProvider)

	meter := meterProvider.Meter("rate-limiter")

	shutdown := func() {
		if err := meterProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}

	return meter, shutdown, nil
}

func main() {
	// Initialize OpenTelemetry
	meter, shutdown, err := initMeter()
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer shutdown()

	// Get Redis address from environment variable or use default
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Connected to Redis at %s", redisAddr)

	// Create a new rateLimiterServer instance with the injected Redis client and meter
	server := NewRateLimiterServer(redisClient, meter)

	// Set up gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRateLimiterServer(grpcServer, server)

	log.Println("gRPC server running on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
