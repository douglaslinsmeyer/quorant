// Package config provides application configuration loaded from environment variables.
// Each field maps to a specific env var; when the env var is absent or empty,
// a sensible default is used instead.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config is the root configuration struct for the Quorant backend service.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	NATS      NATSConfig
	S3        S3Config
	Zitadel   ZitadelConfig
	Stripe    StripeConfig
	Log       LogConfig
	Telemetry TelemetryConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host        string // env: SERVER_HOST, default: "0.0.0.0"
	Port        int    // env: SERVER_PORT, default: 8080
	Environment string // env: APP_ENV, default: "development"
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string // env: DB_HOST, default: "localhost"
	Port     int    // env: DB_PORT, default: 5432
	User     string // env: DB_USER, default: "quorant"
	Password string // env: DB_PASSWORD, default: "quorant"
	Name     string // env: DB_NAME, default: "quorant_dev"
	SSLMode  string // env: DB_SSLMODE, default: "disable"
	MaxConns int32  // env: DB_MAX_CONNS, default: 25
}

// DSN returns a PostgreSQL connection string for use with pgx or database/sql.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string // env: REDIS_ADDR, default: "localhost:6379"
	Password string // env: REDIS_PASSWORD, default: ""
	DB       int    // env: REDIS_DB, default: 0
}

// NATSConfig holds NATS messaging settings.
type NATSConfig struct {
	URL string // env: NATS_URL, default: "nats://localhost:4222"
}

// S3Config holds object storage (MinIO/S3) settings.
type S3Config struct {
	Endpoint  string // env: S3_ENDPOINT, default: "localhost:9000"
	AccessKey string // env: S3_ACCESS_KEY, default: "minioadmin"
	SecretKey string // env: S3_SECRET_KEY, default: "minioadmin"
	Bucket    string // env: S3_BUCKET, default: "quorant-documents"
	UseSSL    bool   // env: S3_USE_SSL, default: false
}

// ZitadelConfig holds Zitadel identity provider settings.
type ZitadelConfig struct {
	Domain          string // env: ZITADEL_DOMAIN, default: "localhost:8085"
	Issuer          string // env: ZITADEL_ISSUER, default: "http://localhost:8085"
	WebhookSecret   string // env: ZITADEL_WEBHOOK_SECRET, default: ""
}

// StripeConfig holds Stripe payment provider settings.
type StripeConfig struct {
	WebhookSecret string // env: STRIPE_WEBHOOK_SECRET, default: ""
}

// LogConfig holds structured logging settings.
type LogConfig struct {
	Level string // env: LOG_LEVEL, default: "info"
}

// TelemetryConfig holds OpenTelemetry settings.
type TelemetryConfig struct {
	Enabled     bool   // env: OTEL_ENABLED, default: false
	Endpoint    string // env: OTEL_ENDPOINT, default: "localhost:4318"
	ServiceName string // env: OTEL_SERVICE_NAME, default: "quorant-api"
}

// getEnv returns the value of the named environment variable, or fallback when
// the variable is unset or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt returns the integer value of the named environment variable, or
// fallback when the variable is unset or empty. An error is returned when the
// variable is set but cannot be parsed as a base-10 integer.
func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not a valid integer: %w", key, v, err)
	}
	return n, nil
}

// getEnvInt32 is like getEnvInt but returns int32.
func getEnvInt32(key string, fallback int32) (int32, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not a valid int32: %w", key, v, err)
	}
	return int32(n), nil
}

// getEnvBool returns the boolean value of the named environment variable, or
// fallback when the variable is unset or empty. An error is returned when the
// variable is set but cannot be parsed as a boolean.
func getEnvBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: %s=%q is not a valid boolean: %w", key, v, err)
	}
	return b, nil
}

// Load reads configuration from environment variables, applying defaults where
// variables are absent or empty. It returns an error if any variable is present
// but cannot be parsed into its expected type.
func Load() (*Config, error) {
	serverPort, err := getEnvInt("SERVER_PORT", 8080)
	if err != nil {
		return nil, err
	}

	dbPort, err := getEnvInt("DB_PORT", 5432)
	if err != nil {
		return nil, err
	}

	dbMaxConns, err := getEnvInt32("DB_MAX_CONNS", 25)
	if err != nil {
		return nil, err
	}

	redisDB, err := getEnvInt("REDIS_DB", 0)
	if err != nil {
		return nil, err
	}

	s3UseSSL, err := getEnvBool("S3_USE_SSL", false)
	if err != nil {
		return nil, err
	}

	otelEnabled, err := getEnvBool("OTEL_ENABLED", false)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Host:        getEnv("SERVER_HOST", "0.0.0.0"),
			Port:        serverPort,
			Environment: getEnv("APP_ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "quorant"),
			Password: getEnv("DB_PASSWORD", "quorant"),
			Name:     getEnv("DB_NAME", "quorant_dev"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxConns: dbMaxConns,
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: os.Getenv("REDIS_PASSWORD"), // empty string is a valid default
			DB:       redisDB,
		},
		NATS: NATSConfig{
			URL: getEnv("NATS_URL", "nats://localhost:4222"),
		},
		S3: S3Config{
			Endpoint:  getEnv("S3_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("S3_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("S3_SECRET_KEY", "minioadmin"),
			Bucket:    getEnv("S3_BUCKET", "quorant-documents"),
			UseSSL:    s3UseSSL,
		},
		Zitadel: ZitadelConfig{
			Domain:        getEnv("ZITADEL_DOMAIN", "localhost:8085"),
			Issuer:        getEnv("ZITADEL_ISSUER", "http://localhost:8085"),
			WebhookSecret: os.Getenv("ZITADEL_WEBHOOK_SECRET"),
		},
		Stripe: StripeConfig{
			WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Telemetry: TelemetryConfig{
			Enabled:     otelEnabled,
			Endpoint:    getEnv("OTEL_ENDPOINT", "localhost:4318"),
			ServiceName: getEnv("OTEL_SERVICE_NAME", "quorant-api"),
		},
	}

	return cfg, nil
}
