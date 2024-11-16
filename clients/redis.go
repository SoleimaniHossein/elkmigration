package clients

import (
	"context"
	"elkmigration/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

var RedisClient *redis.Client

// InitRedis initializes the Redis client with a retry mechanism.
func InitRedis(logger *zap.Logger, config *config.Config) {
	var err error
	for attempts := 0; attempts < 5; attempts++ {
		RedisClient = redis.NewClient(&redis.Options{
			Addr:     config.RedisURL,
			Password: config.RedisPass,
			DB:       config.RedisDb,
		})

		_, err = RedisClient.Ping(context.Background()).Result()
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
	if RedisClient != nil {
		RedisClient.Close()
	}
}
