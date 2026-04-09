// Package health provides an HTTP health check handler and dependency checkers.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

const checkTimeout = 5 * time.Second

// Checker is a named health check that can verify a dependency is reachable.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Handler runs all registered Checkers concurrently and returns a JSON summary.
type Handler struct {
	checkers []Checker
}

// NewHandler constructs a Handler with the given checkers.
func NewHandler(checkers ...Checker) *Handler {
	return &Handler{checkers: checkers}
}

// ServeHTTP implements http.Handler.
// It runs all checkers concurrently within a 5-second timeout.
// Returns 200 if ALL pass, 503 if ANY fail.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
	defer cancel()

	type result struct {
		name string
		err  error
	}

	results := make([]result, len(h.checkers))
	var wg sync.WaitGroup

	for i, c := range h.checkers {
		wg.Add(1)
		go func(idx int, checker Checker) {
			defer wg.Done()
			results[idx] = result{
				name: checker.Name(),
				err:  checker.Check(ctx),
			}
		}(i, c)
	}

	wg.Wait()

	checks := make(map[string]string, len(results))
	healthy := true

	for _, res := range results {
		if res.err != nil {
			healthy = false
			checks[res.name] = fmt.Sprintf("error: %s", res.err.Error())
		} else {
			checks[res.name] = "ok"
		}
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !healthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	body := struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}{
		Status: status,
		Checks: checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(body)
}

// ---------------------------------------------------------------------------
// Concrete checkers
// ---------------------------------------------------------------------------

// DBChecker wraps a pgxpool.Pool ping.
type DBChecker struct {
	pool *pgxpool.Pool
}

// NewDBChecker creates a DBChecker from the given pool.
func NewDBChecker(pool *pgxpool.Pool) *DBChecker {
	return &DBChecker{pool: pool}
}

// Name returns the checker name.
func (c *DBChecker) Name() string { return "db" }

// Check pings the database.
func (c *DBChecker) Check(ctx context.Context) error { return c.pool.Ping(ctx) }

// ---------------------------------------------------------------------------

// RedisChecker wraps a redis client ping.
type RedisChecker struct {
	client *redis.Client
}

// NewRedisChecker creates a RedisChecker from the given client.
func NewRedisChecker(client *redis.Client) *RedisChecker {
	return &RedisChecker{client: client}
}

// Name returns the checker name.
func (c *RedisChecker) Name() string { return "redis" }

// Check pings Redis.
func (c *RedisChecker) Check(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// ---------------------------------------------------------------------------

// NATSChecker checks the NATS connection status.
type NATSChecker struct {
	conn *nats.Conn
}

// NewNATSChecker creates a NATSChecker from the given connection.
func NewNATSChecker(conn *nats.Conn) *NATSChecker {
	return &NATSChecker{conn: conn}
}

// Name returns the checker name.
func (c *NATSChecker) Name() string { return "nats" }

// Check verifies that the NATS connection is in the CONNECTED state.
func (c *NATSChecker) Check(_ context.Context) error {
	if c.conn.Status() != nats.CONNECTED {
		return fmt.Errorf("nats connection is not connected (status: %v)", c.conn.Status())
	}
	return nil
}

// ---------------------------------------------------------------------------

// S3Checker pings a MinIO/S3-compatible endpoint.
type S3Checker struct {
	endpoint string
	useSSL   bool
}

// NewS3Checker creates an S3Checker for the given endpoint.
func NewS3Checker(endpoint string, useSSL bool) *S3Checker {
	return &S3Checker{endpoint: endpoint, useSSL: useSSL}
}

// Name returns the checker name.
func (c *S3Checker) Name() string { return "s3" }

// Check performs an HTTP GET to the MinIO health live endpoint.
func (c *S3Checker) Check(ctx context.Context) error {
	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/minio/health/live", scheme, c.endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("s3: failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("s3: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("s3: unexpected status %d", resp.StatusCode)
	}
	return nil
}
