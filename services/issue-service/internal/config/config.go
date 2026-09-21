package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment, GRPCAddr, HTTPAddr, DatabaseURL, LogLevel  string
	S3Endpoint, S3PublicEndpoint, S3Region, S3Bucket        string
	S3PathStyle                                             bool
	UploadTTL, DownloadTTL, RequestTimeout, ShutdownTimeout time.Duration
	MaxUploadSize                                           int64
	RedisAddr, RedisPassword, NotificationStream            string
	RedisDB, OutboxBatchSize, MaxPageSize                   int
	OutboxPollInterval                                      time.Duration
}

func Load() (Config, error) {
	c := Config{Environment: value("APP_ENV", "local"), GRPCAddr: value("GRPC_ADDR", ":8082"), HTTPAddr: value("HTTP_ADDR", ":8083"), DatabaseURL: os.Getenv("DATABASE_URL"), LogLevel: value("LOG_LEVEL", "info"), S3Endpoint: value("S3_ENDPOINT", "https://storage.yandexcloud.net"), S3Region: value("S3_REGION", "ru-central1"), S3Bucket: os.Getenv("S3_BUCKET"), RedisAddr: value("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"), NotificationStream: value("NOTIFICATION_STREAM", "stream:notifications")}
	var err error
	c.S3PublicEndpoint = os.Getenv("S3_PUBLIC_ENDPOINT")
	for key, target := range map[string]*time.Duration{"S3_UPLOAD_URL_TTL": &c.UploadTTL, "S3_DOWNLOAD_URL_TTL": &c.DownloadTTL, "REQUEST_TIMEOUT": &c.RequestTimeout, "SHUTDOWN_TIMEOUT": &c.ShutdownTimeout, "OUTBOX_POLL_INTERVAL": &c.OutboxPollInterval} {
		defaults := map[string]string{"S3_UPLOAD_URL_TTL": "10m", "S3_DOWNLOAD_URL_TTL": "5m", "REQUEST_TIMEOUT": "30s", "SHUTDOWN_TIMEOUT": "15s", "OUTBOX_POLL_INTERVAL": "1s"}
		*target, err = time.ParseDuration(value(key, defaults[key]))
		if err != nil || *target <= 0 {
			return c, fmt.Errorf("%s must be a positive duration", key)
		}
	}
	for key, target := range map[string]*int{"REDIS_DB": &c.RedisDB, "OUTBOX_BATCH_SIZE": &c.OutboxBatchSize, "MAX_PAGE_SIZE": &c.MaxPageSize} {
		defaults := map[string]string{"REDIS_DB": "0", "OUTBOX_BATCH_SIZE": "100", "MAX_PAGE_SIZE": "100"}
		*target, err = strconv.Atoi(value(key, defaults[key]))
		if err != nil || *target < 0 || (key != "REDIS_DB" && *target == 0) {
			return c, fmt.Errorf("%s must be a valid non-negative integer", key)
		}
	}
	c.MaxUploadSize, err = strconv.ParseInt(value("MAX_UPLOAD_SIZE", "10485760"), 10, 64)
	if err != nil || c.MaxUploadSize < 1 || c.MaxUploadSize > 100<<20 {
		return c, fmt.Errorf("MAX_UPLOAD_SIZE must be between 1 and 104857600")
	}
	c.S3PathStyle, err = strconv.ParseBool(value("S3_USE_PATH_STYLE", "false"))
	if err != nil {
		return c, fmt.Errorf("S3_USE_PATH_STYLE must be boolean")
	}
	for key, addr := range map[string]string{"GRPC_ADDR": c.GRPCAddr, "HTTP_ADDR": c.HTTPAddr, "REDIS_ADDR": c.RedisAddr} {
		if _, _, err = net.SplitHostPort(addr); err != nil {
			return c, fmt.Errorf("%s must be host:port", key)
		}
	}
	if c.GRPCAddr == c.HTTPAddr {
		return c, fmt.Errorf("GRPC_ADDR and HTTP_ADDR must differ")
	}
	if c.DatabaseURL == "" || c.S3Bucket == "" {
		return c, fmt.Errorf("DATABASE_URL and S3_BUCKET are required")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return c, fmt.Errorf("DATABASE_URL must be a PostgreSQL URL")
	}
	// The service must not accidentally connect to another service's database.
	if u.Path != "/issue_db" && !strings.HasPrefix(u.Path, "/issue_test") {
		return c, fmt.Errorf("DATABASE_URL database must be issue_db or issue_test*")
	}
	s, err := url.Parse(c.S3Endpoint)
	if err != nil || s.Host == "" || s.User != nil || s.RawQuery != "" || s.Fragment != "" || (s.Path != "" && s.Path != "/") || (s.Scheme != "https" && s.Scheme != "http") {
		return c, fmt.Errorf("invalid S3_ENDPOINT")
	}
	if c.Environment != "local" && c.Environment != "test" && s.Scheme != "https" {
		return c, fmt.Errorf("S3_ENDPOINT must use HTTPS outside local/test")
	}
	if c.UploadTTL > time.Hour || c.DownloadTTL > time.Hour {
		return c, fmt.Errorf("signed URL TTL cannot exceed one hour")
	}
	if c.MaxPageSize > 1000 || c.OutboxBatchSize > 1000 {
		return c, fmt.Errorf("page and outbox batch limits cannot exceed 1000")
	}
	if c.S3PublicEndpoint != "" {
		p, e := url.Parse(c.S3PublicEndpoint)
		if e != nil || p.Host == "" || p.User != nil || p.RawQuery != "" || p.Fragment != "" || (p.Path != "" && p.Path != "/") || (p.Scheme != "https" && p.Scheme != "http") {
			return c, fmt.Errorf("invalid S3_PUBLIC_ENDPOINT")
		}
		if c.Environment != "local" && c.Environment != "test" && p.Scheme != "https" {
			return c, fmt.Errorf("S3_PUBLIC_ENDPOINT must use HTTPS outside local/test")
		}
	}
	if c.UploadTTL < time.Second || c.DownloadTTL < time.Second {
		return c, fmt.Errorf("signed URL TTL must be at least one second")
	}
	return c, nil
}
func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
