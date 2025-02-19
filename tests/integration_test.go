package server

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	pb "github.com/carteralbrecht/rate-limiter/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const serverAddr = "localhost:50051"

func setupTestClient(t *testing.T) pb.RateLimiterClient {
	t.Helper()

	// Connect to the gRPC server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return pb.NewRateLimiterClient(conn)
}

func generateRandomUserID() string {
	return "user_" + strconv.Itoa(rand.Intn(100000))
}

func TestCheckLimit_AllowedInitially(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()
	userID := generateRandomUserID()

	// Arrange
	req := &pb.CheckRequest{Key: userID, TokenCost: 1}

	// Act
	resp, err := client.CheckLimit(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, resp.Allowed, "Request should be allowed initially")
}

func TestCheckLimit_ExceedsLimit(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()
	userID := generateRandomUserID()

	// Arrange - Drain the bucket
	for i := 0; i < 10; i++ {
		client.CheckLimit(ctx, &pb.CheckRequest{Key: userID, TokenCost: 1})
	}

	// Act - Try one more request
	resp, err := client.CheckLimit(ctx, &pb.CheckRequest{Key: userID, TokenCost: 1})

	// Assert
	assert.NoError(t, err)
	assert.False(t, resp.Allowed, "Request should be denied after bucket is empty")
}

func TestRefillBucket_IncreasesTokens(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()
	userID := generateRandomUserID()

	// Arrange
	req := &pb.RefillRequest{Key: userID, LeakRate: 2, BucketSize: 10}

	// Act
	resp, err := client.RefillBucket(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.Greater(t, resp.CurrentTokens, int32(0), "Bucket should have tokens after refill")
}

func TestCheckLimit_AllowedAfterRefill(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()
	userID := generateRandomUserID()

	// Arrange - Empty the bucket
	for i := 0; i < 10; i++ {
		client.CheckLimit(ctx, &pb.CheckRequest{Key: userID, TokenCost: 1})
	}

	// Act - Refill the bucket
	client.RefillBucket(ctx, &pb.RefillRequest{Key: userID, LeakRate: 5, BucketSize: 10})

	// Act - Try a request again
	resp, err := client.CheckLimit(ctx, &pb.CheckRequest{Key: userID, TokenCost: 1})

	// Assert
	assert.NoError(t, err)
	assert.True(t, resp.Allowed, "Request should be allowed after refill")
}
