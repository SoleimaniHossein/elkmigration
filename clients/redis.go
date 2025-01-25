package clients

import (
	"context"
	"elkmigration/config"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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
func InitRedis(ctx context.Context, logger *zap.Logger, config *config.Config) {
	var err error
	RC = NewRedisClient(ctx,
		redis.NewClient(&redis.Options{
			Addr:     config.RedisUrl,
			Password: config.RedisPass,
			DB:       config.RedisDb,
		}))

	for attempts := 0; attempts < 5; attempts++ {

		_, err = RC.Client.Ping(ctx).Result()
		if err == nil {
			logger.Info("Connected to Redis successfully")
			return
		}
		logger.Warn("Failed to connect to Redis, retrying...", zap.Int("attempt", attempts+1))
		time.Sleep(2 * time.Second)
	}
	logger.Fatal("Unable to connect to Redis after multiple attempts", zap.Error(err))
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
