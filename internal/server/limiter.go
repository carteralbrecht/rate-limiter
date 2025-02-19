// Package server implements the rate limiter service using a token bucket algorithm.
// It provides functionality for checking and consuming tokens, as well as refilling buckets.
package server

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	redisClient *redis.Client
}

func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
	}
}

// CheckAndConsumeTokens checks if there are enough tokens in the bucket and consumes them if available.
// Returns whether the request can proceed and the number of tokens remaining.
func (r *RateLimiter) CheckAndConsumeTokens(ctx context.Context, key string, tokenCost int) (bool, int) {
	// Handle zero or negative token cost
	if tokenCost <= 0 {
		log.Printf("CheckAndConsumeTokens: Token cost is %d, treating as no-op", tokenCost)
		currentTokens, err := r.redisClient.Get(ctx, "bucket:"+key).Int()
		if err != nil && err != redis.Nil {
			log.Printf("Failed to get bucket %s: %v", key, err)
			return false, 0
		}
		if err == redis.Nil {
			currentTokens = 10
			err = r.redisClient.Set(ctx, "bucket:"+key, currentTokens, 0).Err()
			if err != nil {
				log.Printf("Failed to initialize bucket %s: %v", key, err)
				return false, 0
			}
		}
		return true, currentTokens
	}

	bucketKey := "bucket:" + key
	log.Printf("CheckAndConsumeTokens: Checking bucket %s for %d tokens", bucketKey, tokenCost)

	// Get current token count
	currentTokens, err := r.redisClient.Get(ctx, bucketKey).Int()
	if err == redis.Nil {
		log.Printf("CheckAndConsumeTokens: Bucket %s not found, initializing with default size of 10", bucketKey)
		// Initialize new bucket with default size of 10
		currentTokens = 10
		err = r.redisClient.Set(ctx, bucketKey, currentTokens, 0).Err()
		if err != nil {
			log.Printf("Failed to initialize bucket %s: %v", bucketKey, err)
			return false, 0
		}
		log.Printf("CheckAndConsumeTokens: Successfully initialized bucket %s with %d tokens", bucketKey, currentTokens)

		// For a new bucket, consume tokens immediately
		if currentTokens >= tokenCost {
			newTokens := currentTokens - tokenCost
			err = r.redisClient.Set(ctx, bucketKey, newTokens, 0).Err()
			if err != nil {
				log.Printf("Failed to consume tokens from bucket %s: %v", bucketKey, err)
				return false, currentTokens
			}
			log.Printf("CheckAndConsumeTokens: Successfully consumed %d tokens from new bucket %s, %d tokens remaining", tokenCost, bucketKey, newTokens)
			return true, newTokens
		}
		log.Printf("CheckAndConsumeTokens: Not enough tokens in new bucket %s. Required: %d, Available: %d", bucketKey, tokenCost, currentTokens)
		return false, currentTokens
	} else if err != nil {
		log.Printf("Failed to get bucket %s: %v", bucketKey, err)
		return false, 0
	}

	log.Printf("CheckAndConsumeTokens: Found bucket %s with %d tokens", bucketKey, currentTokens)

	// Check if enough tokens are available
	if currentTokens >= tokenCost {
		// Consume tokens
		newTokens := currentTokens - tokenCost
		err = r.redisClient.Set(ctx, bucketKey, newTokens, 0).Err()
		if err != nil {
			log.Printf("Failed to consume tokens from bucket %s: %v", bucketKey, err)
			return false, currentTokens
		}
		log.Printf("CheckAndConsumeTokens: Successfully consumed %d tokens from bucket %s, %d tokens remaining", tokenCost, bucketKey, newTokens)
		return true, newTokens
	}

	log.Printf("CheckAndConsumeTokens: Not enough tokens in bucket %s. Required: %d, Available: %d", bucketKey, tokenCost, currentTokens)
	return false, currentTokens
}

// RefillTokens adds tokens to the bucket based on the leak rate, up to the bucket size.
// Returns the new token count.
func (r *RateLimiter) RefillTokens(ctx context.Context, key string, leakRate int, bucketSize int) int {
	// Handle invalid leak rate or bucket size
	if leakRate <= 0 || bucketSize <= 0 {
		log.Printf("RefillTokens: Invalid parameters - leak rate: %d, bucket size: %d, treating as no-op", leakRate, bucketSize)
		currentTokens, err := r.redisClient.Get(ctx, "bucket:"+key).Int()
		if err != nil && err != redis.Nil {
			log.Printf("Failed to get bucket %s: %v", key, err)
			return 0
		}
		if err == redis.Nil {
			return 0
		}
		return currentTokens
	}

	bucketKey := "bucket:" + key
	log.Printf("RefillTokens: Attempting to refill bucket %s with leak rate %d and bucket size %d", bucketKey, leakRate, bucketSize)

	// Get current token count
	currentTokens, err := r.redisClient.Get(ctx, bucketKey).Int()
	if err == redis.Nil {
		log.Printf("RefillTokens: Bucket %s not found, initializing with leak rate %d", bucketKey, leakRate)
		// If bucket doesn't exist, start with leakRate tokens
		err = r.redisClient.Set(ctx, bucketKey, leakRate, 0).Err()
		if err != nil {
			log.Printf("Failed to initialize bucket %s during refill: %v", bucketKey, err)
			return 0
		}
		log.Printf("RefillTokens: Successfully initialized bucket %s with %d tokens", bucketKey, leakRate)
		return leakRate
	} else if err != nil {
		log.Printf("Failed to get bucket %s during refill: %v", bucketKey, err)
		return 0
	}

	log.Printf("RefillTokens: Current tokens in bucket %s: %d", bucketKey, currentTokens)

	// Calculate new token count, not exceeding bucket size
	newTokens := currentTokens + leakRate
	if newTokens > bucketSize {
		newTokens = bucketSize
	}

	// Update bucket
	err = r.redisClient.Set(ctx, bucketKey, newTokens, 0).Err()
	if err != nil {
		log.Printf("Failed to update bucket %s during refill: %v", bucketKey, err)
		return currentTokens
	}

	log.Printf("RefillTokens: Successfully refilled bucket %s. Old count: %d, New count: %d", bucketKey, currentTokens, newTokens)
	return newTokens
}
