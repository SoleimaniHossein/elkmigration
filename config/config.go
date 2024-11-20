package config

import (
	"elkmigration/logger"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
)

// Config holds the application configuration
type Config struct {
	ElkIndexFrom string `mapstructure:"ELK_INDEX_FROM"`
	ElkIndexTo   string `mapstructure:"ELK_INDEX_TO"`

	Elk2Url  string `mapstructure:"ELK2_URL"`
	Elk2User string `mapstructure:"ELK2_USER"`
	Elk2Pass string `mapstructure:"ELK2_PASS"`

	Elk7Url  string `mapstructure:"ELK7_URL"`
	Elk7User string `mapstructure:"ELK7_USER"`
	Elk7Pass string `mapstructure:"ELK7_PASS"`

	Elk8Url  string `mapstructure:"ELK8_URL"`
	ELK8User string `mapstructure:"ELK8_USER"`
	Elk8Pass string `mapstructure:"ELK8_PASS"`

	RedisUrl  string `mapstructure:"REDIS_URL"`
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
	viper.SetDefault("ELK_INDEX_FROM", "idx_from")
	viper.SetDefault("ELK_INDEX_TO", "idx_to")

	viper.SetDefault("ELK2_URL", "http://127.0.0.1:9202")
	viper.SetDefault("ELK2_USER", "elastic")
	viper.SetDefault("ELK2_PASS", "changeme")

	viper.SetDefault("ELK7_URL", "http://127.0.0.1:9207")
	viper.SetDefault("ELK7_USER", "elastic")
	viper.SetDefault("ELK7_PASS", "changeme")

	viper.SetDefault("ELK8_URL", "http://127.0.0.1:9208")
	viper.SetDefault("ELK8_USER", "elastic")
	viper.SetDefault("ELK8_PASS", "changeme")

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
		zap.String("ELK2 URL", config.Elk2Url),
		zap.String("ELK7 URL", config.Elk7Url),
		zap.String("ELK8 URL", config.Elk8Url),
		zap.String("ELK INDEX FROM", config.ElkIndexFrom),
		zap.String("ELK INDEX TO", config.ElkIndexTo),
		zap.String("Redis URL", config.RedisUrl),
	)

	return &config, nil
}
