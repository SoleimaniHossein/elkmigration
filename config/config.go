package config

import (
	"elkmigration/logger"
	"fmt"
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
}

type Elk2 struct {
	Urls   []string
	Index  string
	SortBy string
	Asc    bool
	User   string
	Pass   string
}

type Elk7 struct {
	Urls  []string
	Index string
	User  string
	Pass  string
}

type Elk8 struct {
	Urls  []string
	Index string
	User  string
	Pass  string
}

type Redis struct {
	Host              string
	Port              int
	Pass              string
	DB                int
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
	viper.SetConfigName(".env.yml") // Use .env for configuration
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		logger.Error("Error reading config file", zap.Error(err))
	}

	//// Load the correct .env file dynamically
	//envFile := ".env"
	//if err := godotenv.Load(envFile); err != nil {
	//	log.Printf("Warning: No %s file found, using system environment variables", envFile)
	//}

	// Configure Viper for environment variables
	//viper.SetEnvPrefix("APP")                              // Prefix all environment variables with APP_
	//viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // Convert app.bulk_size → APP_BULK_SIZE
	viper.AutomaticEnv() // Read OS environment variables

	viper.BindEnv("App.BulkSize", "APP_BULK_SIZE")
	viper.BindEnv("App.MaxBulkPayloadBytes", "APP_MAX_BULK_PAYLOAD_BYTES")
	viper.BindEnv("App.ScrollTTL", "APP_SCROLL_TTL")
	viper.BindEnv("App.TTL", "TTL")
	viper.BindEnv("App.MaxRetries", "APP_MAX_RETRIES")

	viper.BindEnv("Elk2.Urls", "ELK2_URLS")
	viper.BindEnv("Elk2.Index", "ELK2_INDEX")
	viper.BindEnv("Elk2.SortBy", "ELK2_SORT_BY")
	viper.BindEnv("Elk2.Asc", "ELK2_ASC")
	viper.BindEnv("Elk2.User", "ELK2_USER")
	viper.BindEnv("Elk2.Pass", "ELK2_PASS")

	viper.BindEnv("Elk7.Urls", "ELK7_URLS")
	viper.BindEnv("Elk7.Index", "ELK7_INDEX")
	viper.BindEnv("Elk7.User", "ELK7_USER")
	viper.BindEnv("Elk7.Pass", "ELK7_PASS")

	viper.BindEnv("Elk8.Urls", "ELK8_URLS")
	viper.BindEnv("Elk8.Index", "ELK8_INDEX")
	viper.BindEnv("Elk8.User", "ELK8_USER")
	viper.BindEnv("Elk8.Pass", "ELK8_PASS")

	viper.BindEnv("Redis.Host", "REDIS_HOST")
	viper.BindEnv("Redis.Port", "REDIS_PORT")
	viper.BindEnv("Redis.DB", "REDIS_DB")
	viper.BindEnv("Redis.KeyScrollID", "REDIS_KEYSCROLLID")
	viper.BindEnv("Redis.TTL", "REDIS_TTL")
	viper.BindEnv("Redis.MaxRetries", "REDIS_MAX_RETRIES")
	viper.BindEnv("Redis.KeyTotalProcessed", "REDIS_KEYTPROCESSED")
	viper.BindEnv("Redis.TTL", "REDIS_TTL")

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
		zap.Strings("ELK2 URLs", config.Elk2.Urls),
		zap.Strings("ELK7 URLs", config.Elk7.Urls),
		zap.Strings("ELK8 URLs", config.Elk8.Urls),
		zap.String("ELK INDEX FROM", config.Elk2.Index),
		zap.String("ELK INDEX TO", config.Elk7.Index),
		zap.Int("BULK SIZE", config.App.BulkSize),
		zap.Int("MAX BULK PAYLOAD BYTES", config.App.MaxBulkPayloadBytes),
		zap.Int("MAX RETRIES", config.App.MaxRetries),
		zap.Duration("SCROLL TIMEOUT", config.App.ScrollTTL),
		zap.String("REDIS ADDR", fmt.Sprintf("%s:%d", config.Redis.Host, config.Redis.Port)),
		zap.Duration("REDIS TTL", config.Redis.TTL),
		zap.String("REDIS KEY SCROLL ID", config.Redis.KeyScrollID),
		zap.String("REDIS KEY TOTAL PROCESSED", config.Redis.KeyTotalProcessed),
		zap.Duration("TIMEOUT", config.App.TTL),
	)

	return &config, nil
}
