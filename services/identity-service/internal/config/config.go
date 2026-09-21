package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCPort    string
	HTTPPort    string
	DatabaseURL string
	LogLevel    string
}

func Load() (*Config, error) {
	grpcPort := getEnv("IDENTITY_GRPC_PORT", "50051")
	httpPort := getEnv("IDENTITY_HTTP_PORT", "8081")
	logLevel := getEnv("LOG_LEVEL", "info")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Дефолтная строка подключения под локальный Docker Compose
		dbURL = "postgres://postgres:postgres@localhost:5432/identity_db?sslmode=disable"
	}

	cfg := &Config{
		GRPCPort:    formatPort(grpcPort),
		HTTPPort:    formatPort(httpPort),
		DatabaseURL: dbURL,
		LogLevel:    logLevel,
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func formatPort(port string) string {
	if len(port) > 0 && port[0] == ':' {
		return port
	}
	return fmt.Sprintf(":%s", port)
}
