package clients

import (
	"context"
	"elkmigration/config"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"strconv"
	"time"
)

var RC *RedisClient

type RedisClient struct {
	Client *redis.Client
	Ctx    context.Context
}

func NewRedisClient(ctx context.Context, client *redis.Client) *RedisClient {
	return &RedisClient{
		Client: client,
		Ctx:    ctx,
	}
}

// InitRedis initializes the Redis client with a retry mechanism.
func InitRedis(ctx context.Context, config *config.Config) {
	RC = NewRedisClient(ctx,
		redis.NewClient(&redis.Options{
			Addr:     config.RedisUrl,
			Password: config.RedisPass,
			DB:       config.RedisDb,
		}))

	if err := RC.Client.Ping(RC.Ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
}

// CloseRedis closes the Redis client connection.
func CloseRedis() {
	if RC != nil {
		RC.Client.Close()
	}
}

func (rc *RedisClient) Set(key string, value interface{}, expiration time.Duration) error {
	if err := rc.Client.Set(rc.Ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set key %q: %w", key, err)
	}
	return nil
}

// Get retrieves a value from Redis by its key.
func (rc *RedisClient) Get(key string) (string, error) {
	val, err := rc.Client.Get(rc.Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // Key does not exist
	} else if err != nil {
		return "", fmt.Errorf("failed to get key %q: %w", key, err)
	}
	return val, nil
}

// Close shuts down the Redis client connection.
func (rc *RedisClient) Close() error {
	if err := rc.Client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}
	return nil
}

// GetOffsetFromRedis retrieves the saved offset from Redis.
func (rc *RedisClient) GetOffsetFromRedis(key string) (int, error) {
	val, err := rc.Client.Get(rc.Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// Key does not exist, start from offset 0
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	// Parse the offset value
	offset, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid offset value in Redis: %w", err)
	}

	return offset, nil
}

// SaveOffsetToRedis saves the current offset to Redis.
func (rc *RedisClient) SaveOffsetToRedis(key string, offset int) error {
	err := rc.Client.Set(rc.Ctx, key, strconv.Itoa(offset), 0).Err()
	return err
}
