//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/quorant/quorant/internal/platform/config"
	"github.com/quorant/quorant/internal/platform/db"
)

func validTestConfig() config.DatabaseConfig {
	return config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "quorant",
		Password: "quorant",
		Name:     "quorant_dev",
		SSLMode:  "disable",
		MaxConns: 5,
	}
}

func TestNewPool_ConnectsToRealPostgres(t *testing.T) {
	ctx := context.Background()
	cfg := validTestConfig()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool() returned unexpected error: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool.Ping() failed: %v", err)
	}
}

func TestNewPool_ReturnsErrorForInvalidHost(t *testing.T) {
	ctx := context.Background()
	cfg := validTestConfig()
	cfg.Host = "invalid-host-that-does-not-exist"

	pool, err := db.NewPool(ctx, cfg)
	if err == nil {
		pool.Close()
		t.Fatal("NewPool() expected an error for invalid host, got nil")
	}
}
