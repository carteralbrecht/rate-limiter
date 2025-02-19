package server

import (
	"context"
	"errors"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestCheckAndConsumeTokens_NewBucket(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:123"
	tokenCost := 1

	// Mock Redis calls for a new bucket
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	mock.ExpectSet("bucket:"+key, 10, 0).SetVal("OK") // Initialize with default size
	mock.ExpectSet("bucket:"+key, 9, 0).SetVal("OK")  // After consuming 1 token

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.True(t, success, "Request should be allowed for new bucket")
	assert.Equal(t, 9, remaining, "Should have 9 tokens remaining after consuming 1")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_NewBucketInsufficientTokens(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:insufficient"
	tokenCost := 15 // More than the default bucket size of 10

	// Mock Redis calls for a new bucket
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	mock.ExpectSet("bucket:"+key, 10, 0).SetVal("OK") // Initialize with default size

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.False(t, success, "Request should be denied for new bucket with insufficient tokens")
	assert.Equal(t, 10, remaining, "Should have 10 tokens remaining")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_ExistingBucket(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:456"
	tokenCost := 2

	// Mock Redis calls for existing bucket with 5 tokens
	mock.ExpectGet("bucket:" + key).SetVal("5")
	mock.ExpectSet("bucket:"+key, 3, 0).SetVal("OK") // After consuming 2 tokens

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.True(t, success, "Request should be allowed")
	assert.Equal(t, 3, remaining, "Should have 3 tokens remaining after consuming 2")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_InsufficientTokens(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:789"
	tokenCost := 3

	// Mock Redis calls for bucket with insufficient tokens
	mock.ExpectGet("bucket:" + key).SetVal("2")

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.False(t, success, "Request should be denied")
	assert.Equal(t, 2, remaining, "Should still have 2 tokens")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_GetError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:error"
	tokenCost := 1

	// Mock Redis calls with error
	mock.ExpectGet("bucket:" + key).SetErr(errors.New("redis connection error"))

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.False(t, success, "Request should be denied on error")
	assert.Equal(t, 0, remaining, "Should return 0 tokens on error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_SetError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:seterror"
	tokenCost := 1

	// Mock Redis calls with set error
	mock.ExpectGet("bucket:" + key).SetVal("5")
	mock.ExpectSet("bucket:"+key, 4, 0).SetErr(errors.New("redis set error"))

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.False(t, success, "Request should be denied on set error")
	assert.Equal(t, 5, remaining, "Should return original token count")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_NewBucketSetError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:newbucketerror"
	tokenCost := 1

	// Mock Redis calls with error on new bucket initialization
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	mock.ExpectSet("bucket:"+key, 10, 0).SetErr(errors.New("redis set error"))

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.False(t, success, "Request should be denied on initialization error")
	assert.Equal(t, 0, remaining, "Should return 0 tokens on error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_Normal(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:101"
	leakRate := 3
	bucketSize := 10

	// Mock Redis calls
	mock.ExpectGet("bucket:" + key).SetVal("5")
	mock.ExpectSet("bucket:"+key, 8, 0).SetVal("OK") // 5 + 3 = 8

	// Act
	newTokenCount := rateLimiter.RefillTokens(ctx, key, leakRate, bucketSize)

	// Assert
	assert.Equal(t, 8, newTokenCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_OverBucketSize(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:102"
	leakRate := 5
	bucketSize := 10

	// Mock Redis calls
	mock.ExpectGet("bucket:" + key).SetVal("8")
	mock.ExpectSet("bucket:"+key, 10, 0).SetVal("OK") // Would be 13, capped at 10

	// Act
	newTokenCount := rateLimiter.RefillTokens(ctx, key, leakRate, bucketSize)

	// Assert
	assert.Equal(t, 10, newTokenCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_NewBucket(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:103"
	leakRate := 3
	bucketSize := 10

	// Mock Redis calls for non-existent bucket
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	mock.ExpectSet("bucket:"+key, 3, 0).SetVal("OK") // Start with leakRate tokens

	// Act
	newTokenCount := rateLimiter.RefillTokens(ctx, key, leakRate, bucketSize)

	// Assert
	assert.Equal(t, 3, newTokenCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_GetError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:geterror"
	leakRate := 3
	bucketSize := 10

	// Mock Redis calls with get error
	mock.ExpectGet("bucket:" + key).SetErr(errors.New("redis connection error"))

	// Act
	newTokenCount := rateLimiter.RefillTokens(ctx, key, leakRate, bucketSize)

	// Assert
	assert.Equal(t, 0, newTokenCount, "Should return 0 tokens on error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_SetError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:seterror"
	leakRate := 3
	bucketSize := 10

	// Mock Redis calls with set error
	mock.ExpectGet("bucket:" + key).SetVal("5")
	mock.ExpectSet("bucket:"+key, 8, 0).SetErr(errors.New("redis set error"))

	// Act
	newTokenCount := rateLimiter.RefillTokens(ctx, key, leakRate, bucketSize)

	// Assert
	assert.Equal(t, 5, newTokenCount, "Should return original token count on error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_NewBucketSetError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:newbucketerror"
	leakRate := 3
	bucketSize := 10

	// Mock Redis calls with error on new bucket initialization
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	mock.ExpectSet("bucket:"+key, 3, 0).SetErr(errors.New("redis set error"))

	// Act
	newTokenCount := rateLimiter.RefillTokens(ctx, key, leakRate, bucketSize)

	// Assert
	assert.Equal(t, 0, newTokenCount, "Should return 0 tokens on initialization error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_NewBucketConsumeError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:newbucketconsumeerror"
	tokenCost := 5 // Less than the default bucket size of 10

	// Mock Redis calls for a new bucket with error on consume
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	mock.ExpectSet("bucket:"+key, 10, 0).SetVal("OK")                     // Initialize with default size
	mock.ExpectSet("bucket:"+key, 5, 0).SetErr(errors.New("redis error")) // Error when consuming tokens

	// Act
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, tokenCost)

	// Assert
	assert.False(t, success, "Request should be denied when consume fails")
	assert.Equal(t, 10, remaining, "Should return initial token count")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNewRateLimiter(t *testing.T) {
	// Test with valid client
	client, _ := redismock.NewClientMock()
	limiter := NewRateLimiter(client)
	assert.NotNil(t, limiter)
	assert.Equal(t, client, limiter.redisClient)

	// Test with nil client
	limiter = NewRateLimiter(nil)
	assert.NotNil(t, limiter)
	assert.Nil(t, limiter.redisClient)
}

func TestCheckAndConsumeTokens_ZeroOrNegativeTokens(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:zerotokens"

	// Test with zero tokens
	mock.ExpectGet("bucket:" + key).SetVal("5")
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, 0)
	assert.True(t, success, "Request should be allowed for zero tokens")
	assert.Equal(t, 5, remaining)

	// Test with negative tokens
	mock.ExpectGet("bucket:" + key).SetVal("5")
	success, remaining = rateLimiter.CheckAndConsumeTokens(ctx, key, -1)
	assert.True(t, success, "Request should be allowed for negative tokens")
	assert.Equal(t, 5, remaining)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_EdgeCases(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:edgecases"

	// Test with zero leak rate
	mock.ExpectGet("bucket:" + key).SetVal("5")
	newTokens := rateLimiter.RefillTokens(ctx, key, 0, 10)
	assert.Equal(t, 5, newTokens)

	// Test with negative leak rate
	mock.ExpectGet("bucket:" + key).SetVal("5")
	newTokens = rateLimiter.RefillTokens(ctx, key, -1, 10)
	assert.Equal(t, 5, newTokens)

	// Test with zero bucket size
	mock.ExpectGet("bucket:" + key).SetVal("5")
	newTokens = rateLimiter.RefillTokens(ctx, key, 3, 0)
	assert.Equal(t, 5, newTokens)

	// Test with negative bucket size
	mock.ExpectGet("bucket:" + key).SetVal("5")
	newTokens = rateLimiter.RefillTokens(ctx, key, 3, -1)
	assert.Equal(t, 5, newTokens)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_ZeroTokensRedisError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:zerotokens:error"

	// Test Redis error when checking token count
	mock.ExpectGet("bucket:" + key).SetErr(errors.New("redis error"))
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, 0)
	assert.False(t, success, "Request should be denied on Redis error")
	assert.Equal(t, 0, remaining)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAndConsumeTokens_ZeroTokensInitError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:zerotokens:initerror"

	// Test Redis error when initializing bucket
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	mock.ExpectSet("bucket:"+key, 10, 0).SetErr(errors.New("redis error"))
	success, remaining := rateLimiter.CheckAndConsumeTokens(ctx, key, 0)
	assert.False(t, success, "Request should be denied on initialization error")
	assert.Equal(t, 0, remaining)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefillTokens_InvalidParamsRedisError(t *testing.T) {
	// Arrange
	client, mock := redismock.NewClientMock()
	rateLimiter := NewRateLimiter(client)
	ctx := context.Background()
	key := "user:invalid:error"

	// Test Redis error when checking token count with invalid parameters
	mock.ExpectGet("bucket:" + key).SetErr(errors.New("redis error"))
	newTokens := rateLimiter.RefillTokens(ctx, key, 0, 10)
	assert.Equal(t, 0, newTokens)

	// Test Redis Nil when checking token count with invalid parameters
	mock.ExpectGet("bucket:" + key).SetErr(redis.Nil)
	newTokens = rateLimiter.RefillTokens(ctx, key, -1, 10)
	assert.Equal(t, 0, newTokens)

	assert.NoError(t, mock.ExpectationsWereMet())
}
