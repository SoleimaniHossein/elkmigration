package clients

import (
	"context"
	"elkmigration/config"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"sync"
	"time"
)

var RC *RedisClient

type RedisClient struct {
	Ctx    context.Context
	Mu     *sync.Mutex
	Client *redis.Client
}

func NewRedisClient(ctx context.Context, mu *sync.Mutex, client *redis.Client) *RedisClient {
	return &RedisClient{
		Ctx:    ctx,
		Mu:     mu,
		Client: client,
	}
}

// InitRedis initializes the Redis client with a retry mechanism.
func InitRedis(ctx context.Context, mu *sync.Mutex, config config.Redis) {
	RC = NewRedisClient(ctx, mu,
		redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
			Password: config.Pass,
			DB:       config.DB,
		}))

	if err := RC.Client.Ping(ctx).Err(); err != nil {
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
	rc.Mu.Lock()
	defer rc.Mu.Unlock()

	if err := rc.Client.Set(rc.Ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set key %q: %w", key, err)
	}
	return nil
}

// Get retrieves a value from Redis by its key.
func (rc *RedisClient) Get(key string) (string, error) {
	rc.Mu.Lock()
	defer rc.Mu.Unlock()

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
