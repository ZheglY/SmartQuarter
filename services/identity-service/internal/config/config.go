package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
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
		return nil, fmt.Errorf("DATABASE_URL required")
	}
	u, err := url.Parse(dbURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || (u.Path != "/identity_db" && !strings.HasPrefix(u.Path, "/identity_test")) {
		return nil, fmt.Errorf("DATABASE_URL must address identity_db or identity_test*")
	}
	for _, port := range []string{grpcPort, httpPort} {
		n, e := strconv.Atoi(strings.TrimPrefix(port, ":"))
		if e != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid listen port")
		}
	}
	if formatPort(grpcPort) == formatPort(httpPort) {
		return nil, fmt.Errorf("HTTP and gRPC ports must differ")
	}
	var level slog.Level
	if err = level.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, fmt.Errorf("invalid LOG_LEVEL")
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
