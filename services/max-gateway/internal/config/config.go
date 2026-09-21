package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment, HTTPAddr, LogLevel, RedisAddr, RedisPassword                                                     string
	RedisDB                                                                                                       int
	SessionTTL, InitDataTTL, DialTimeout, RequestTimeout, ReadTimeout, WriteTimeout, IdleTimeout, ShutdownTimeout time.Duration
	CookieName, IdentityAddr, IssueAddr, IssueReadyURL, CommunityAddr                                             string
	BotToken, BotBaseURL, MiniAppURL, WebhookSecret, BotUsername                                                  string
	Stream, Group                                                                                                 string
	Origins                                                                                                       []string
	SecureCookie                                                                                                  bool
	AuthRate, BusinessRate, WebhookRate, NotificationRate                                                         int
}

func Load() (c Config, err error) {
	str := func(k, d string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return d
	}
	c.Environment = str("APP_ENV", "local")
	c.HTTPAddr = str("HTTP_ADDR", ":8080")
	c.LogLevel = str("LOG_LEVEL", "info")
	c.RedisAddr = str("REDIS_ADDR", "localhost:6379")
	c.RedisPassword = os.Getenv("REDIS_PASSWORD")
	c.CookieName = str("SESSION_COOKIE_NAME", "sq_session")
	c.IdentityAddr = os.Getenv("IDENTITY_GRPC_ADDR")
	c.IssueAddr = os.Getenv("ISSUE_GRPC_ADDR")
	c.CommunityAddr = os.Getenv("COMMUNITY_GRPC_ADDR")
	c.IssueReadyURL = os.Getenv("ISSUE_READY_URL")
	c.BotToken = os.Getenv("MAX_BOT_TOKEN")
	c.BotBaseURL = str("MAX_BOT_API_BASE_URL", "https://platform-api2.max.ru")
	c.MiniAppURL = os.Getenv("MAX_MINIAPP_URL")
	c.BotUsername = os.Getenv("MAX_BOT_USERNAME")
	c.WebhookSecret = os.Getenv("MAX_WEBHOOK_SECRET")
	c.Stream = str("NOTIFICATION_STREAM", "stream:notifications")
	c.Group = str("NOTIFICATION_CONSUMER_GROUP", "max-gateway")
	c.SecureCookie = c.Environment != "local" && c.Environment != "test"
	for _, d := range []struct {
		k, d string
		p    *time.Duration
	}{{"SESSION_TTL", "12h", &c.SessionTTL}, {"MAX_INIT_DATA_TTL", "5m", &c.InitDataTTL}, {"GRPC_DIAL_TIMEOUT", "5s", &c.DialTimeout}, {"GRPC_REQUEST_TIMEOUT", "10s", &c.RequestTimeout}, {"HTTP_READ_TIMEOUT", "10s", &c.ReadTimeout}, {"HTTP_WRITE_TIMEOUT", "30s", &c.WriteTimeout}, {"HTTP_IDLE_TIMEOUT", "60s", &c.IdleTimeout}, {"GRACEFUL_SHUTDOWN_TIMEOUT", "20s", &c.ShutdownTimeout}} {
		*d.p, err = time.ParseDuration(str(d.k, d.d))
		if err != nil || *d.p <= 0 {
			return c, fmt.Errorf("invalid %s", d.k)
		}
	}
	for _, v := range []struct {
		k, d string
		p    *int
	}{{"REDIS_DB", "0", &c.RedisDB}, {"AUTH_RATE_PER_MINUTE", "20", &c.AuthRate}, {"BUSINESS_RATE_PER_MINUTE", "120", &c.BusinessRate}, {"WEBHOOK_RATE_PER_MINUTE", "600", &c.WebhookRate}, {"NOTIFICATION_RATE_PER_SECOND", "2", &c.NotificationRate}} {
		*v.p, err = strconv.Atoi(str(v.k, v.d))
		if err != nil || *v.p < 0 || (v.k != "REDIS_DB" && *v.p == 0) {
			return c, fmt.Errorf("invalid %s", v.k)
		}
	}
	for _, o := range strings.Split(os.Getenv("TRUSTED_ORIGINS"), ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		u, e := url.Parse(o)
		if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return c, fmt.Errorf("invalid TRUSTED_ORIGINS")
		}
		if c.SecureCookie && u.Scheme != "https" {
			return c, fmt.Errorf("HTTPS origins required")
		}
		c.Origins = append(c.Origins, o)
	}
	for k, v := range map[string]string{"ISSUE_GRPC_ADDR": c.IssueAddr, "ISSUE_READY_URL": c.IssueReadyURL, "MAX_BOT_TOKEN": c.BotToken, "MAX_WEBHOOK_SECRET": c.WebhookSecret, "MAX_BOT_USERNAME": c.BotUsername, "MAX_MINIAPP_URL": c.MiniAppURL} {
		if v == "" {
			return c, fmt.Errorf("%s required", k)
		}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{5,256}$`).MatchString(c.WebhookSecret) {
		return c, fmt.Errorf("invalid MAX_WEBHOOK_SECRET")
	}
	if len(c.Origins) == 0 {
		return c, fmt.Errorf("TRUSTED_ORIGINS required")
	}
	for k, v := range map[string]string{"MAX_BOT_API_BASE_URL": c.BotBaseURL, "MAX_MINIAPP_URL": c.MiniAppURL, "ISSUE_READY_URL": c.IssueReadyURL} {
		u, e := url.Parse(v)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return c, fmt.Errorf("invalid %s", k)
		}
		if c.SecureCookie && k != "ISSUE_READY_URL" && u.Scheme != "https" {
			return c, fmt.Errorf("HTTPS required for %s", k)
		}
	}
	if c.NotificationRate > 2 {
		return c, fmt.Errorf("NOTIFICATION_RATE_PER_SECOND must be <= 2")
	}
	return c, nil
}
