package config

import (
	"elkmigration/logger"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
	"time"
)

type App struct {
	BulkSize            int
	MaxBulkPayloadBytes int
	ScrollTTL           time.Duration
	TTL                 time.Duration
	MaxRetries          int
	LogPath             string
}
type Elk2 struct {
	Url   string
	Index string
	User  string
	Pass  string
}

type Elk7 struct {
	Url   string
	Index string
	User  string
	Pass  string
}

type Elk8 struct {
	Url   string
	Index string
	User  string
	Pass  string
}

type Redis struct {
	Addr              string
	DB                int
	Pass              string
	KeyScrollID       string
	KeyTotalProcessed string
	TTL               time.Duration
}

// Config holds the application configuration
type Config struct {
	App   App
	Elk2  Elk2
	Elk7  Elk7
	Elk8  Elk8
	Redis Redis
}

// LoadConfig initializes the application configuration from environment variables
func LoadConfig() (*Config, error) {
	viper.SetConfigName(".env") // Use .env for configuration
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		logger.Log.Warn("Error reading config file", zap.Error(err))
		logger.Log.Info("Environment variable not set, using default")
	}

	// Set up Viper to read environment variables
	viper.AutomaticEnv()

	// Define a Config struct to hold the configuration
	var config Config

	// Unmarshal environment variables into the config struct
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Unable to unmarshal config: %v", err)
		return nil, err
	}

	// Log the loaded configuration (optional)
	configLogger, _ := zap.NewProduction() // Adjust logging based on your setup
	defer configLogger.Sync()
	configLogger.Info("Configuration loaded",
		zap.String("ELK2 URL", config.Elk2.Url),
		zap.String("ELK7 URL", config.Elk7.Url),
		zap.String("ELK8 URL", config.Elk8.Url),
		zap.String("ELK INDEX FROM", config.Elk2.Index),
		zap.String("ELK INDEX TO", config.Elk8.Index),
		zap.Int("BULK SIZE", config.App.BulkSize),
		zap.Int("MAX BULK PAYLOAD BYTES", config.App.MaxBulkPayloadBytes),
		zap.Int("MAX RETRIES", config.App.MaxRetries),
		zap.Duration("SCROLL TIMEOUT", config.App.ScrollTTL),
		zap.String("REDIS URL", config.Redis.Addr),
		zap.Duration("REDIS TTL", config.Redis.TTL),
		zap.String("REDIS KEY SCROLL ID", config.Redis.KeyScrollID),
		zap.String("REDIS KEY TOTAL PROCESSED", config.Redis.KeyTotalProcessed),
		zap.Duration("TIMEOUT", config.App.TTL),
		zap.String("LOG PATH", config.App.LogPath),
	)

	return &config, nil
}
