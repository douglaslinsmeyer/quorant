package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/config"
	"github.com/quorant/quorant/internal/platform/db"
	"github.com/quorant/quorant/internal/platform/health"
	"github.com/quorant/quorant/internal/platform/logging"
	"github.com/quorant/quorant/internal/platform/middleware"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Logger
	logger := logging.NewLogger(cfg.Log.Level)
	slog.SetDefault(logger)

	// 3. Database
	pool, err := db.NewPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// 4. Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	// 5. NATS
	var nc *nats.Conn
	nc, err = nats.Connect(cfg.NATS.URL)
	if err != nil {
		// Don't fail startup on NATS — log warning, health check will show unhealthy
		logger.Warn("failed to connect to NATS", "error", err)
	} else {
		defer nc.Close()
	}

	// 6. JWT validator
	var tokenValidator auth.TokenValidator
	jwksURL := fmt.Sprintf("http://%s/oauth/v2/keys", cfg.Zitadel.Domain)
	validator, err := auth.NewJWKSValidator(ctx, jwksURL, cfg.Zitadel.Issuer)
	if err != nil {
		// Don't fail startup on Zitadel unavailable — use a fallback that rejects all tokens
		logger.Warn("failed to initialize JWKS validator, auth will reject all tokens", "error", err)
		tokenValidator = &auth.StaticValidator{Err: fmt.Errorf("JWKS not available")}
	} else {
		tokenValidator = validator
	}

	// 7. Health handler
	healthHandler := health.NewHandler(
		health.NewDBChecker(pool),
		health.NewRedisChecker(rdb),
		health.NewNATSChecker(nc),
		health.NewS3Checker(cfg.S3.Endpoint, cfg.S3.UseSSL),
	)

	// 8. Routes
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/health", healthHandler)

	// IAM module
	userRepo := iam.NewPostgresUserRepository(pool)
	userService := iam.NewUserService(userRepo)
	iamHandler := iam.NewHandler(userService, logger)
	iam.RegisterRoutes(mux, iamHandler, tokenValidator)

	// 9. Middleware chain (innermost to outermost)
	var handler http.Handler = mux
	handler = middleware.Logging(logger, handler)
	handler = middleware.Recovery(logger, handler)
	handler = middleware.RequestID(handler)
	handler = middleware.CORS([]string{"*"}, handler) // permissive for dev; configured per-env in production

	// 10. HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 11. Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting API server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 12. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logger.Info("shutting down server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	logger.Info("server stopped")
	return nil
}
