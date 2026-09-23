package config

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	RedisAddr, RedisPassword, NotificationStream string
	GRPCPort                                     string
	HTTPPort                                     string
	DatabaseURL                                  string
	LogLevel                                     string
	AdminUserIDs                                 map[string]bool
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
		RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"), NotificationStream: getEnv("NOTIFICATION_STREAM", "stream:notifications"),
		GRPCPort:     formatPort(grpcPort),
		HTTPPort:     formatPort(httpPort),
		DatabaseURL:  dbURL,
		LogLevel:     logLevel,
		AdminUserIDs: map[string]bool{},
	}
	for _, raw := range strings.Split(os.Getenv("ADMIN_USER_IDS"), ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		parsed, e := uuid.Parse(id)
		if e != nil || parsed == uuid.Nil || parsed.String() != id {
			return nil, fmt.Errorf("ADMIN_USER_IDS must contain canonical user UUIDs")
		}
		cfg.AdminUserIDs[id] = true
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
