package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/quorant/quorant/internal/platform/config"
	"github.com/quorant/quorant/internal/platform/db"
	"github.com/quorant/quorant/internal/platform/logging"
	"github.com/quorant/quorant/internal/platform/queue"

	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/scheduler"
	"github.com/quorant/quorant/internal/task"
	"github.com/quorant/quorant/internal/webhook"
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

	// 8. Register event handlers

	// Context lake ingestion — embeds domain events into the vector store
	contextChunkRepo := ai.NewPostgresContextChunkRepository(pool)
	ingestionEmbedFn := ai.EmbeddingFunc(func(ctx context.Context, text string) ([]float32, error) {
		// Worker uses the same LLM config as the API server
		// In production, this would be initialized from config
		// For now, use StubEmbeddingFunc as fallback when LLM is not configured
		return ai.StubEmbeddingFunc(ctx, text)
	})
	ingestionWorker := ai.NewIngestionWorker(contextChunkRepo, ingestionEmbedFn, logger)
	ingestionWorker.RegisterHandlers(consumer)

	// Compliance worker — re-evaluates orgs on rule/org/unit/document changes
	jurisdictionRuleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
	complianceCheckRepo := ai.NewPostgresComplianceCheckRepository(pool)
	orgRepo := org.NewPostgresOrgRepository(pool)
	complianceService := ai.NewComplianceService(jurisdictionRuleRepo, complianceCheckRepo, orgRepo, logger)
	complianceService.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	complianceService.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	complianceService.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	complianceService.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	complianceService.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	complianceService.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	complianceService.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	taskRepo := task.NewPostgresTaskRepository(pool)
	taskService := task.NewTaskService(taskRepo, audit.NewNoopAuditor(), natsPublisher, logger)

	complianceWorker := ai.NewComplianceWorker(complianceService, jurisdictionRuleRepo, complianceCheckRepo, orgRepo, taskService, natsPublisher, logger)
	complianceWorker.RegisterHandlers(consumer)

	logger.Info("registered event handlers", "count", 12)

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

	// Webhook relay
	webhookRepo := webhook.NewPostgresWebhookRepository(pool)
	webhookRelay := webhook.NewRelay(nc, webhookRepo, logger)
	go func() {
		if err := webhookRelay.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error("webhook relay error", "error", err)
		}
	}()

	// Webhook retry worker
	webhookRetryWorker := webhook.NewRetryWorker(webhookRepo, logger)
	go func() {
		if err := webhookRetryWorker.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error("webhook retry worker error", "error", err)
		}
	}()

	// Scheduler
	sched := scheduler.New(logger)
	sched.Register(scheduler.NewAssessmentGeneratorJob(pool, logger), 1*time.Hour)
	sched.Register(scheduler.NewLateFeeJob(pool, logger), 24*time.Hour)
	sched.Register(scheduler.NewCollectionEscalationJob(pool, logger), 24*time.Hour)
	sched.Register(scheduler.NewARBAutoApprovalJob(pool, logger), 1*time.Hour)
	sched.Register(scheduler.NewSLABreachMonitorJob(pool, logger), 5*time.Minute)
	sched.Register(scheduler.NewAnnouncementPublisherJob(pool, logger), 1*time.Minute)
	sched.Register(scheduler.NewNotificationDispatchJob(pool, logger), 30*time.Second)
	complianceAlertJob := ai.NewComplianceAlertJob(jurisdictionRuleRepo, complianceCheckRepo, orgRepo, complianceService, taskService, natsPublisher, logger)
	sched.Register(complianceAlertJob, 24*time.Hour)
	go sched.Start(ctx)

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
