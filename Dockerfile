# Tools stage for protobuf and other build tools
FROM golang:1.23 AS tools
WORKDIR /tools
RUN apt-get update && apt-get install -y protobuf-compiler libprotobuf-dev && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Build stage
FROM golang:1.23 AS builder
WORKDIR /app

# Copy tools from tools stage
COPY --from=tools /usr/bin/protoc /usr/bin/
COPY --from=tools /usr/lib/x86_64-linux-gnu/libprotoc.so* /usr/lib/x86_64-linux-gnu/
COPY --from=tools /usr/lib/x86_64-linux-gnu/libprotobuf.so* /usr/lib/x86_64-linux-gnu/
COPY --from=tools /go/bin/* /go/bin/

# Copy and build
COPY . .
RUN go mod download && \
    protoc --proto_path=proto \
        --go_out=proto --go-grpc_out=proto \
        --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
        proto/rate_limiter.proto && \
    CGO_ENABLED=0 GOOS=linux go build -o ratelimiter ./cmd/ratelimiter

# Final stage
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/ratelimiter .
EXPOSE 50051
USER nonroot
ENTRYPOINT ["/app/ratelimiter"]
