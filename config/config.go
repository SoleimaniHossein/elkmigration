package config

import (
	"elkmigration/logger"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
)

// Config holds the application configuration
type Config struct {
	Elk2URL   string `mapstructure:"ELK2_URL"`
	Elk7URL   string `mapstructure:"ELK7_URL"`
	Elk8URL   string `mapstructure:"ELK8_URL"`
	RedisURL  string `mapstructure:"REDIS_URL"`
	RedisDb   int    `mapstructure:"REDIS_DB"`
	RedisPass string `mapstructure:"REDIS_PASSWORD"`
}

// LoadConfig initializes the application configuration from environment variables
func LoadConfig() (*Config, error) {
	viper.SetConfigName(".env") // Use .env for configuration
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		logger.Log.Warn("Error reading config file", zap.Error(err))
		logger.Log.Info("Environment variable not set, using default")
	}

	// Set up Viper to read environment variables
	viper.AutomaticEnv()
	// Provide default values
	viper.SetDefault("ELK2_URL", "http://127.0.0.1:9200")
	viper.SetDefault("ELK7_URL", "http://127.0.0.1:9200")
	viper.SetDefault("ELK8_URL", "http://127.0.0.1:9200")
	viper.SetDefault("REDIS_URL", "127.0.0.1:6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_PASSWORD", nil)

	// Define a Config struct to hold the configuration
	var config Config

	// Unmarshal environment variables into the config struct
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Unable to unmarshal config: %v", err)
		return nil, err
	}

	// Log the loaded configuration (optional)
	logger, _ := zap.NewProduction() // Adjust logging based on your setup
	defer logger.Sync()
	logger.Info("Configuration loaded",
		zap.String("ELK2 URL", config.Elk2URL),
		zap.String("ELK7 URL", config.Elk7URL),
		zap.String("ELK8 URL", config.Elk8URL),
		zap.String("Redis URL", config.RedisURL),
		zap.Int("Redis DB", config.RedisDb),
		zap.String("Redis Pass", config.RedisPass),
	)

	return &config, nil
}
