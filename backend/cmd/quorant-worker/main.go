package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"

	"github.com/quorant/quorant/internal/platform/config"
	"github.com/quorant/quorant/internal/platform/db"
	"github.com/quorant/quorant/internal/platform/logging"
	"github.com/quorant/quorant/internal/platform/queue"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// 4. NATS
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer nc.Close()

	// 5. NATS Publisher
	natsPublisher, err := queue.NewNATSPublisher(nc, logger)
	if err != nil {
		return fmt.Errorf("creating NATS publisher: %w", err)
	}

	// 6. Outbox Poller
	poller := queue.NewOutboxPoller(pool, natsPublisher, logger)

	// 7. Event Consumer
	consumer, err := queue.NewConsumer(nc, pool, logger)
	if err != nil {
		return fmt.Errorf("creating event consumer: %w", err)
	}

	// 8. Register handlers (modules will register their handlers here in future phases)
	// Example: consumer.Register(queue.HandlerRegistration{
	//     Name:    "com.send_violation_notice",
	//     Subject: "quorant.violation.ViolationCreated.>",
	//     Handler: comModule.HandleViolationCreated,
	// })
	logger.Info("no event handlers registered yet — handlers will be added in future phases")

	// 9. Start consumer
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("starting consumer: %w", err)
	}

	// 10. Start outbox poller in background
	go func() {
		if err := poller.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error("outbox poller error", "error", err)
		}
	}()

	logger.Info("worker started")

	// 11. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	sig := <-quit
	logger.Info("received shutdown signal", "signal", sig)

	cancel() // cancel context → stops poller and consumers
	consumer.Stop()

	logger.Info("worker stopped")
	return nil
}
