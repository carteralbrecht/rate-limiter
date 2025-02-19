# 🚦 High-Performance Rate Limiter

<div align="center">
  <a href="https://go.dev/" target="_blank"><img src="https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://redis.io/" target="_blank"><img src="https://img.shields.io/badge/redis-%23DD0031.svg?style=for-the-badge&logo=redis&logoColor=white" alt="Redis"></a>
  <a href="https://grpc.io/" target="_blank"><img src="https://img.shields.io/badge/gRPC-%23244c5a.svg?style=for-the-badge&logo=google&logoColor=white" alt="gRPC"></a>
  <a href="https://www.docker.com/" target="_blank"><img src="https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white" alt="Docker"></a>
  <a href="https://grafana.com/" target="_blank"><img src="https://img.shields.io/badge/grafana-%23F46800.svg?style=for-the-badge&logo=grafana&logoColor=white" alt="Grafana"></a>
  <a href="https://prometheus.io/" target="_blank"><img src="https://img.shields.io/badge/Prometheus-E6522C?style=for-the-badge&logo=Prometheus&logoColor=white" alt="Prometheus"></a>
</div>

A robust, Redis-backed rate limiter service implementing the token bucket algorithm. Built with Go and gRPC for high performance and scalability.

## ✨ Features

- **Token Bucket Algorithm**: Efficient and flexible rate limiting implementation
- **Redis Backend**: Distributed rate limiting with persistent storage
- **gRPC Interface**: High-performance API with protocol buffer definitions
- **Configurable Limits**: Adjustable token bucket size and leak rates
- **Thread-Safe**: Concurrent request handling with Redis atomic operations
- **Comprehensive Testing**: 100% test coverage with both unit and integration tests
- **Complete Observability**: Integrated metrics, logs, and traces with:
  - OpenTelemetry for metrics collection
  - Prometheus for metrics storage
  - Grafana for visualization
  - Loki for log aggregation
  - Promtail for log collection

## 🚀 Quick Start

### Prerequisites

- Go 1.23 or higher
- Docker and Docker Compose

### Running with Docker Compose

1. Clone the repository:
   ```bash
   git clone https://github.com/carteralbrecht/rate-limiter.git
   cd rate-limiter
   ```

2. Start the service and observability stack:
   ```bash
   docker-compose up
   ```

The following services will be available:
- Rate Limiter: `localhost:50051` (gRPC)
- Grafana: `localhost:3000`
- Prometheus: `localhost:9090`
- Loki: `localhost:3100`

### Running Locally

1. Start Redis:
   ```bash
   docker-compose up redis
   ```

2. Build and run the service:
   ```bash
   make build
   ./ratelimiter
   ```

### Environment Variables

The service supports the following environment variables:
- `REDIS_ADDR`: Redis server address (default: "localhost:6379")
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry collector endpoint (default: "http://localhost:4317")
- `OTEL_SERVICE_NAME`: Service name for telemetry (default: "rate-limiter")

### Available Make Commands

- `make all`                    - Run all checks and build (default)
- `make build`                  - Build the binary
- `make clean`                  - Clean up generated files and containers
- `make dev-tools`              - Build development tools Docker image
- `make fmt`                    - Format code
- `make lint`                   - Run linters
- `make proto`                  - Generate protobuf code
- `make test`                   - Run unit tests
- `make integration-test`       - Run integration tests and stop services after completion
- `make integration-test-keep`  - Run integration tests and keep services running
- `make up`                     - Start application and dependencies
- `make down`                   - Stop application and dependencies

For a full list of available commands, run:
```bash
make help
```

## 🔧 Usage

The rate limiter provides two main gRPC operations:

### Check and Consume Tokens

Checks if there are enough tokens in the bucket and consumes them if available:

```go
success, remaining := rateLimiter.CheckLimit(ctx, &pb.CheckRequest{
    Key: "user:123",
    TokenCost: 1,
})
if success.Allowed {
    // Request allowed, proceed
    // remaining shows tokens left in bucket
} else {
    // Rate limit exceeded
}
```

### Refill Tokens

Adds tokens to the bucket based on the leak rate:

```go
response := rateLimiter.RefillBucket(ctx, &pb.RefillRequest{
    Key: "user:123",
    LeakRate: 5,
    BucketSize: 10,
})
// response.CurrentTokens shows updated token count
```

## 🏗️ Architecture

The rate limiter uses the token bucket algorithm with the following components:

- **Bucket**: Each rate-limited key has a bucket with a maximum capacity
- **Tokens**: Consumed for each request
- **Leak Rate**: Rate at which tokens are replenished
- **Redis**: Stores bucket state and handles atomic operations
- **OpenTelemetry**: Collects and exports metrics
- **Loki**: Aggregates logs from all services

```
┌─────────────┐    gRPC     ┌──────────────┐    Redis    ┌──────────┐
│   Client    │─────────────► Rate Limiter │────────────►│  Bucket  │
└─────────────┘             └──────────────┘             └──────────┘
                                   │
                                   ├─────────────┐
                                   │             │
                            ┌──────▼──────┐      │
                            │    OTEL     │      │
                            │  Collector  │      │
                            └──────┬──────┘      │
                                   │             │
                            ┌──────▼──────┐ ┌────▼─────┐
                            │ Prometheus  │ │   Loki   │
                            └──────┬──────┘ └────┬─────┘
                                   │             │
                                   └──────┐ ┌────┘
                                          │ │
                                    ┌─────▼─▼────┐
                                    │  Grafana   │
                                    │ Dashboards │
                                    └────────────┘
```

## 📊 Observability

The rate limiter includes comprehensive observability features:

### Metrics (OpenTelemetry + Prometheus)

Built-in metrics include:
- `rate_limiter_requests_total`: Total number of rate limiter requests
- `rate_limiter_tokens_remaining`: Number of tokens remaining in buckets
- `rate_limiter_request_duration_seconds`: Request duration histogram
- `rate_limiter_errors_total`: Total number of rate limiter errors

### Logging (Loki + Promtail)

Centralized logging with:
- Request/response logging
- Error tracking
- Performance metrics
- Container logs from all services

### Dashboards (Grafana)

Pre-configured Grafana dashboards for:
1. Rate Limiter Overview:
   - Request rates and latencies
   - Token bucket levels
   - Error rates
2. System Health:
   - Redis metrics
   - Service metrics
   - Container metrics

Access Grafana at `http://localhost:3000`:
- Username: admin
- Password: admin

## 🧪 Testing

Run unit tests:
```bash
make test
```

Run integration tests:
```bash
make integration-test
```
---

Run integration tests and keep the observability stack running to examine metrics:
```bash
make integration-test-keep
```

When finished examining metrics, stop the stack:
```bash
make down
```

## 📈 Performance

The rate limiter is designed for high performance:

- Minimal latency with Redis operations
- Efficient token bucket algorithm
- Concurrent request handling
- Scalable through Redis clustering
- Optimized metric collection

## 📝 License

This is free and unencumbered software released into the public domain - see the [LICENSE](LICENSE) file for details.

