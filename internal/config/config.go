package config

import (
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
)

// Config stores all application configuration
type Config struct {
	// General settings
	ServiceName string
	Environment string
	Port        int

	// NATS settings
	NatsURL         string
	NatsStreamName  string
	NatsSubjects    []string
	NatsStorageType nats.StorageType
	NatsMaxAge      time.Duration
	NatsReplicas    int

	// Tracing settings
	JaegerURL string

	// Loki settings
	LokiURL string
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	// Set defaults
	config := &Config{
		ServiceName:     getEnv("SERVICE_NAME", "microservice"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		Port:            getEnvAsInt("PORT", 8080),
		NatsURL:         getEnv("NATS_URL", "nats://localhost:4222"),
		NatsStreamName:  getEnv("NATS_STREAM", "logs"),
		NatsSubjects:    []string{getEnv("NATS_SUBJECT", "logs.>")},
		NatsStorageType: nats.FileStorage,
		NatsMaxAge:      getEnvAsDuration("NATS_MAX_AGE", 7*24*time.Hour), // 7 days
		NatsReplicas:    getEnvAsInt("NATS_REPLICAS", 1),
		JaegerURL:       getEnv("JAEGER_URL", "localhost:4317"),
		LokiURL:         getEnv("LOKI_URL", "http://localhost:3100/loki/api/v1/push"),
	}

	// Parse storage type
	storageTypeStr := getEnv("NATS_STORAGE_TYPE", "file")
	if storageTypeStr == "memory" {
		config.NatsStorageType = nats.MemoryStorage
	}

	return config
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt gets an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsDuration gets an environment variable as a duration or returns a default value
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
