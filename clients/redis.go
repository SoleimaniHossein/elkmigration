package clients

import (
	"context"
	"elkmigration/config"
	"errors"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

var ctx = context.Background()
var RedisClient *Redis

type Redis struct {
	Client *redis.Client
}

func NewRedisClient(client *redis.Client) *Redis {
	return &Redis{Client: client}
}

// InitRedis initializes the Redis client with a retry mechanism.
func InitRedis(logger *zap.Logger, config *config.Config) {
	var err error
	RedisClient = NewRedisClient(
		redis.NewClient(&redis.Options{
			Addr:     config.RedisUrl,
			Password: config.RedisPass,
			DB:       config.RedisDb,
		}))

	for attempts := 0; attempts < 5; attempts++ {

		_, err = RedisClient.Client.Ping(context.Background()).Result()
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
		RedisClient.Client.Close()
	}
}

func (r *Redis) SaveLastProcessedID(key string, value any) error {
	return r.Client.Set(ctx, key, value, 0).Err()
}

func (r *Redis) GetLastProcessedID(key string) (string, error) {
	result, err := r.Client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return result, err
}
