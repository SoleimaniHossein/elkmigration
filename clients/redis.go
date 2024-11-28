package clients

import (
	"context"
	"elkmigration/config"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

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

// Save a value of any type
func (r *Redis) Save(ctx context.Context, key string, value any) error {
	data, err := json.Marshal(value) // Serialize the value
	if err != nil {
		return err
	}
	return r.Client.Set(ctx, key, data, 0).Err()
}

//// Get a value and deserialize it to the specified type
//func (r *Redis) Get(ctx context.Context, key string, dest any) error {
//	result, err := r.Client.Get(ctx, key).Result()
//	if errors.Is(err, redis.Nil) {
//		return nil // Key not found, return nil error
//	} else if err != nil {
//		return err
//	}
//
//	return json.Unmarshal([]byte(result), dest) // Deserialize to the specified type
//}

func (r *Redis) Get(ctx context.Context, key string) (dest any, err error) {
	// Retrieve the raw result from Redis
	result, err := r.Client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, errors.New("key not found") // Explicitly handle missing key
	} else if err != nil {
		return nil, err // Return other Redis errors
	}

	// Attempt to unmarshal the JSON string into the provided destination
	err = json.Unmarshal([]byte(result), dest)
	if err != nil {
		return nil, errors.New("failed to deserialize value: " + err.Error())
	}

	return dest, nil
}

//
//func (r *Redis) Save(ctx context.Context, key string, value any) error {
//	return r.Client.Set(ctx, key, value, 0).Err()
//}
//
//func (r *Redis) Get(ctx context.Context, key string) (string, error) {
//	result, err := r.Client.Get(ctx, key).Result()
//	if errors.Is(err, redis.Nil) {
//		return "", nil
//	}
//	return result, err
//}

// SaveJSON saves an interface as a JSON string in Redis
func (r *Redis) SaveJSON(ctx context.Context, key string, value interface{}) error {
	// Marshal the value to a JSON string
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Save the JSON string in Redis
	if err := r.Client.Set(ctx, key, jsonData, 0).Err(); err != nil {
		return fmt.Errorf("failed to save to Redis: %w", err)
	}

	return nil
}

// GetJSON retrieves a JSON string from Redis and unmarshals it into an interface
func (r *Redis) GetJSON(ctx context.Context, key string, dest interface{}) error {
	// Get the JSON string from Redis
	jsonData, err := r.Client.Get(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to get from Redis: %w", err)
	}

	// Unmarshal the JSON string into the destination interface
	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}
