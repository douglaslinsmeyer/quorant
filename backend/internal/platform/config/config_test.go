package config_test

import (
	"testing"

	"github.com/quorant/quorant/internal/platform/config"
)

// TestLoad_Defaults verifies that Load() returns the expected default values
// when no environment variables are set.
func TestLoad_Defaults(t *testing.T) {
	// Unset all relevant env vars so we get pure defaults
	envVars := []string{
		"SERVER_HOST", "SERVER_PORT",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "DB_MAX_CONNS",
		"REDIS_ADDR", "REDIS_PASSWORD", "REDIS_DB",
		"NATS_URL",
		"S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET", "S3_USE_SSL",
		"ZITADEL_DOMAIN", "ZITADEL_ISSUER",
		"LOG_LEVEL",
	}
	for _, v := range envVars {
		t.Setenv(v, "")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	t.Run("ServerConfig defaults", func(t *testing.T) {
		if cfg.Server.Host != "0.0.0.0" {
			t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
		}
		if cfg.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
		}
	})

	t.Run("DatabaseConfig defaults", func(t *testing.T) {
		if cfg.Database.Host != "localhost" {
			t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "localhost")
		}
		if cfg.Database.Port != 5432 {
			t.Errorf("Database.Port = %d, want %d", cfg.Database.Port, 5432)
		}
		if cfg.Database.User != "quorant" {
			t.Errorf("Database.User = %q, want %q", cfg.Database.User, "quorant")
		}
		if cfg.Database.Password != "quorant" {
			t.Errorf("Database.Password = %q, want %q", cfg.Database.Password, "quorant")
		}
		if cfg.Database.Name != "quorant_dev" {
			t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "quorant_dev")
		}
		if cfg.Database.SSLMode != "disable" {
			t.Errorf("Database.SSLMode = %q, want %q", cfg.Database.SSLMode, "disable")
		}
		if cfg.Database.MaxConns != 25 {
			t.Errorf("Database.MaxConns = %d, want %d", cfg.Database.MaxConns, 25)
		}
	})

	t.Run("RedisConfig defaults", func(t *testing.T) {
		if cfg.Redis.Addr != "localhost:6379" {
			t.Errorf("Redis.Addr = %q, want %q", cfg.Redis.Addr, "localhost:6379")
		}
		if cfg.Redis.Password != "" {
			t.Errorf("Redis.Password = %q, want %q", cfg.Redis.Password, "")
		}
		if cfg.Redis.DB != 0 {
			t.Errorf("Redis.DB = %d, want %d", cfg.Redis.DB, 0)
		}
	})

	t.Run("NATSConfig defaults", func(t *testing.T) {
		if cfg.NATS.URL != "nats://localhost:4222" {
			t.Errorf("NATS.URL = %q, want %q", cfg.NATS.URL, "nats://localhost:4222")
		}
	})

	t.Run("S3Config defaults", func(t *testing.T) {
		if cfg.S3.Endpoint != "localhost:9000" {
			t.Errorf("S3.Endpoint = %q, want %q", cfg.S3.Endpoint, "localhost:9000")
		}
		if cfg.S3.AccessKey != "minioadmin" {
			t.Errorf("S3.AccessKey = %q, want %q", cfg.S3.AccessKey, "minioadmin")
		}
		if cfg.S3.SecretKey != "minioadmin" {
			t.Errorf("S3.SecretKey = %q, want %q", cfg.S3.SecretKey, "minioadmin")
		}
		if cfg.S3.Bucket != "quorant-documents" {
			t.Errorf("S3.Bucket = %q, want %q", cfg.S3.Bucket, "quorant-documents")
		}
		if cfg.S3.UseSSL != false {
			t.Errorf("S3.UseSSL = %v, want %v", cfg.S3.UseSSL, false)
		}
	})

	t.Run("ZitadelConfig defaults", func(t *testing.T) {
		if cfg.Zitadel.Domain != "localhost:8085" {
			t.Errorf("Zitadel.Domain = %q, want %q", cfg.Zitadel.Domain, "localhost:8085")
		}
		if cfg.Zitadel.Issuer != "http://localhost:8085" {
			t.Errorf("Zitadel.Issuer = %q, want %q", cfg.Zitadel.Issuer, "http://localhost:8085")
		}
	})

	t.Run("LogConfig defaults", func(t *testing.T) {
		if cfg.Log.Level != "info" {
			t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
		}
	})
}

// TestLoad_EnvVars verifies that Load() reads values from environment variables
// when they are set, overriding defaults.
func TestLoad_EnvVars(t *testing.T) {
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "pguser")
	t.Setenv("DB_PASSWORD", "pgpassword")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("DB_MAX_CONNS", "50")
	t.Setenv("REDIS_ADDR", "redis.example.com:6380")
	t.Setenv("REDIS_PASSWORD", "redispass")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("NATS_URL", "nats://nats.example.com:4222")
	t.Setenv("S3_ENDPOINT", "s3.example.com:443")
	t.Setenv("S3_ACCESS_KEY", "myaccesskey")
	t.Setenv("S3_SECRET_KEY", "mysecretkey")
	t.Setenv("S3_BUCKET", "my-bucket")
	t.Setenv("S3_USE_SSL", "true")
	t.Setenv("ZITADEL_DOMAIN", "auth.example.com")
	t.Setenv("ZITADEL_ISSUER", "https://auth.example.com")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	t.Run("ServerConfig from env", func(t *testing.T) {
		if cfg.Server.Host != "127.0.0.1" {
			t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
		}
	})

	t.Run("DatabaseConfig from env", func(t *testing.T) {
		if cfg.Database.Host != "db.example.com" {
			t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.example.com")
		}
		if cfg.Database.Port != 5433 {
			t.Errorf("Database.Port = %d, want %d", cfg.Database.Port, 5433)
		}
		if cfg.Database.User != "pguser" {
			t.Errorf("Database.User = %q, want %q", cfg.Database.User, "pguser")
		}
		if cfg.Database.Password != "pgpassword" {
			t.Errorf("Database.Password = %q, want %q", cfg.Database.Password, "pgpassword")
		}
		if cfg.Database.Name != "mydb" {
			t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "mydb")
		}
		if cfg.Database.SSLMode != "require" {
			t.Errorf("Database.SSLMode = %q, want %q", cfg.Database.SSLMode, "require")
		}
		if cfg.Database.MaxConns != 50 {
			t.Errorf("Database.MaxConns = %d, want %d", cfg.Database.MaxConns, 50)
		}
	})

	t.Run("RedisConfig from env", func(t *testing.T) {
		if cfg.Redis.Addr != "redis.example.com:6380" {
			t.Errorf("Redis.Addr = %q, want %q", cfg.Redis.Addr, "redis.example.com:6380")
		}
		if cfg.Redis.Password != "redispass" {
			t.Errorf("Redis.Password = %q, want %q", cfg.Redis.Password, "redispass")
		}
		if cfg.Redis.DB != 2 {
			t.Errorf("Redis.DB = %d, want %d", cfg.Redis.DB, 2)
		}
	})

	t.Run("NATSConfig from env", func(t *testing.T) {
		if cfg.NATS.URL != "nats://nats.example.com:4222" {
			t.Errorf("NATS.URL = %q, want %q", cfg.NATS.URL, "nats://nats.example.com:4222")
		}
	})

	t.Run("S3Config from env", func(t *testing.T) {
		if cfg.S3.Endpoint != "s3.example.com:443" {
			t.Errorf("S3.Endpoint = %q, want %q", cfg.S3.Endpoint, "s3.example.com:443")
		}
		if cfg.S3.AccessKey != "myaccesskey" {
			t.Errorf("S3.AccessKey = %q, want %q", cfg.S3.AccessKey, "myaccesskey")
		}
		if cfg.S3.SecretKey != "mysecretkey" {
			t.Errorf("S3.SecretKey = %q, want %q", cfg.S3.SecretKey, "mysecretkey")
		}
		if cfg.S3.Bucket != "my-bucket" {
			t.Errorf("S3.Bucket = %q, want %q", cfg.S3.Bucket, "my-bucket")
		}
		if cfg.S3.UseSSL != true {
			t.Errorf("S3.UseSSL = %v, want %v", cfg.S3.UseSSL, true)
		}
	})

	t.Run("ZitadelConfig from env", func(t *testing.T) {
		if cfg.Zitadel.Domain != "auth.example.com" {
			t.Errorf("Zitadel.Domain = %q, want %q", cfg.Zitadel.Domain, "auth.example.com")
		}
		if cfg.Zitadel.Issuer != "https://auth.example.com" {
			t.Errorf("Zitadel.Issuer = %q, want %q", cfg.Zitadel.Issuer, "https://auth.example.com")
		}
	})

	t.Run("LogConfig from env", func(t *testing.T) {
		if cfg.Log.Level != "debug" {
			t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
		}
	})
}

// TestLoad_InvalidPort verifies that Load() returns an error when an env var
// that should parse as an integer contains a non-numeric value.
func TestLoad_InvalidPort(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		value  string
	}{
		{"invalid SERVER_PORT", "SERVER_PORT", "notaport"},
		{"invalid DB_PORT", "DB_PORT", "notaport"},
		{"invalid DB_MAX_CONNS", "DB_MAX_CONNS", "notanumber"},
		{"invalid REDIS_DB", "REDIS_DB", "notanumber"},
		{"invalid S3_USE_SSL", "S3_USE_SSL", "notabool"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, tc.value)
			_, err := config.Load()
			if err == nil {
				t.Errorf("Load() with %s=%q: expected error, got nil", tc.envKey, tc.value)
			}
		})
	}
}

// TestDatabaseConfig_DSN verifies that DSN() produces the correct PostgreSQL
// connection string from a DatabaseConfig.
func TestDatabaseConfig_DSN(t *testing.T) {
	tests := []struct {
		name     string
		dbCfg    config.DatabaseConfig
		wantDSN  string
	}{
		{
			name: "default values",
			dbCfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "quorant",
				Password: "quorant",
				Name:     "quorant_dev",
				SSLMode:  "disable",
				MaxConns: 25,
			},
			wantDSN: "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable",
		},
		{
			name: "custom values",
			dbCfg: config.DatabaseConfig{
				Host:     "db.example.com",
				Port:     5433,
				User:     "pguser",
				Password: "pgpassword",
				Name:     "mydb",
				SSLMode:  "require",
				MaxConns: 50,
			},
			wantDSN: "postgres://pguser:pgpassword@db.example.com:5433/mydb?sslmode=require",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.dbCfg.DSN()
			if got != tc.wantDSN {
				t.Errorf("DSN() = %q, want %q", got, tc.wantDSN)
			}
		})
	}
}
