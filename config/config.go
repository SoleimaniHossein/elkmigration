package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
	"time"
)

type App struct {
	MonigoPort          int
	BulkSize            int
	MaxBulkPayloadBytes int
	ScrollTTL           string
	TTL                 time.Duration
	MaxRetries          int
}

type Elk2 struct {
	Urls   []string
	Index  string
	User   string
	Pass   string
	SortBy string
	Asc    bool
}

type Elk7 struct {
	Urls    []string
	Index   string
	User    string
	Pass    string
	SortBy  string
	OrderBy string
}

type Elk8 struct {
	Urls    []string
	Index   string
	User    string
	Pass    string
	SortBy  string
	OrderBy string
}

// Config holds the application configuration
type Config struct {
	App  App
	Elk2 Elk2
	Elk7 Elk7
	Elk8 Elk8
}

// LoadConfig initializes the application configuration from environment variables
func LoadConfig() (*Config, error) {
	// Set up Viper to read from config file
	viper.SetConfigFile("env.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Read from .env.yaml if it exists
	if err := viper.ReadInConfig(); err == nil {
		log.Println("Loaded configuration from .env.yaml", err)
	} else {
		log.Println("No env.yaml file found, using only environment variables")
	}

	// Set up Viper to read from environment variables (and override values from the file)
	viper.AutomaticEnv() // This ensures OS env variables take precedence over config file

	// Set multiple prefixes
	prefixes := []string{"APP", "ELK2", "ELK7", "ELK8"}
	for _, prefix := range prefixes {
		viper.SetEnvPrefix(prefix) // Apply the prefix for env vars
		viper.AllowEmptyEnv(true)  // Allow unset environment variables
	}

	viper.BindEnv("App.MonigoPort", "APP_MONIGO_PORT")
	viper.BindEnv("App.BulkSize", "APP_BULK_SIZE")
	viper.BindEnv("App.MaxBulkPayloadBytes", "APP_MAX_BULK_PAYLOAD_BYTES")
	viper.BindEnv("App.ScrollTTL", "APP_SCROLL_TTL")
	viper.BindEnv("App.TTL", "APP_TTL")
	viper.BindEnv("App.MaxRetries", "APP_MAX_RETRIES")

	viper.BindEnv("Elk2.Urls", "ELK2_URLS")
	viper.BindEnv("Elk2.Index", "ELK2_INDEX")
	viper.BindEnv("Elk2.User", "ELK2_USER")
	viper.BindEnv("Elk2.Pass", "ELK2_PASS")
	viper.BindEnv("Elk2.SortBy", "ELK2_SORT_BY")
	viper.BindEnv("Elk2.Asc", "ELK2_ASC")

	viper.BindEnv("Elk8.Urls", "ELK8_URLS")
	viper.BindEnv("Elk8.Index", "ELK8_INDEX")
	viper.BindEnv("Elk8.User", "ELK8_USER")
	viper.BindEnv("Elk8.Pass", "ELK8_PASS")
	viper.BindEnv("Elk8.SortBy", "ELK8_SORT_BY")
	viper.BindEnv("Elk8.OrderBy", "ELK8_ORDER_BY")

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
		zap.String("ELK2 INDEX (FROM)", config.Elk2.Index),
		zap.String("ELK2 SORT BY", config.Elk2.SortBy),
		zap.Bool("Elk2 ORDER BY ASC", config.Elk2.Asc),
		zap.Strings("ELK8 URLs", config.Elk8.Urls),
		zap.String("ELK8 INDEX (TO)", config.Elk8.Index),
		zap.String("ELK8 SORT BY", config.Elk8.SortBy),
		zap.String("ELK8 ORDER BY", config.Elk8.OrderBy),
		zap.Int("BULK SIZE", config.App.BulkSize),
		zap.Int("MAX BULK PAYLOAD BYTES", config.App.MaxBulkPayloadBytes),
		zap.String("SCROLL TIMEOUT", config.App.ScrollTTL),
		zap.Duration("APP TIMEOUT", config.App.TTL),
		zap.Int("MAX RETRIES", config.App.MaxRetries),
	)

	return &config, nil
}
