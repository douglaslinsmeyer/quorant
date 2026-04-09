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

	"github.com/quorant/quorant/internal/admin"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/billing"
	"github.com/quorant/quorant/internal/com"
	"github.com/quorant/quorant/internal/doc"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/gov"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/license"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/task"
	"github.com/quorant/quorant/internal/webhook"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/platform/config"
	"github.com/quorant/quorant/internal/platform/db"
	"github.com/quorant/quorant/internal/platform/health"
	"github.com/quorant/quorant/internal/platform/logging"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/storage"
	"github.com/quorant/quorant/internal/platform/telemetry"
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

	// 3. Telemetry
	shutdownTracer, err := telemetry.InitTracer(ctx, telemetry.Config{
		ServiceName: cfg.Telemetry.ServiceName,
		Endpoint:    cfg.Telemetry.Endpoint,
		Enabled:     cfg.Telemetry.Enabled,
	})
	if err != nil {
		logger.Warn("failed to initialize tracer", "error", err)
	} else {
		defer shutdownTracer(ctx)
	}

	// 4. Database
	pool, err := db.NewPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// 5. Audit and event infrastructure
	auditor := audit.NewPostgresAuditor(pool)
	_ = auditor
	outboxPublisher := queue.NewOutboxPublisher(pool)
	_ = outboxPublisher
	logger.Info("audit and event infrastructure initialized")

	// 5. Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	// 6. NATS
	var nc *nats.Conn
	nc, err = nats.Connect(cfg.NATS.URL)
	if err != nil {
		// Don't fail startup on NATS — log warning, health check will show unhealthy
		logger.Warn("failed to connect to NATS", "error", err)
	} else {
		defer nc.Close()
	}

	// 7. JWT validator
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

	// 8. Health handler
	healthHandler := health.NewHandler(
		health.NewDBChecker(pool),
		health.NewRedisChecker(rdb),
		health.NewNATSChecker(nc),
		health.NewS3Checker(cfg.S3.Endpoint, cfg.S3.UseSSL),
	)

	// 9. Routes
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/health", healthHandler)

	// IAM module
	userRepo := iam.NewPostgresUserRepository(pool)
	userService := iam.NewUserService(userRepo)
	iamHandler := iam.NewHandler(userService, logger)
	iam.RegisterRoutes(mux, iamHandler, tokenValidator)

	// Org module
	orgRepo := org.NewPostgresOrgRepository(pool)
	membershipRepo := org.NewPostgresMembershipRepository(pool)
	unitRepo := org.NewPostgresUnitRepository(pool)
	orgService := org.NewOrgService(orgRepo, membershipRepo, unitRepo, userRepo, logger)
	orgHandler := org.NewOrgHandler(orgService, logger)
	membershipHandler := org.NewMembershipHandler(orgService, logger)
	unitHandler := org.NewUnitHandler(orgService, logger)
	org.RegisterRoutes(mux, orgHandler, membershipHandler, unitHandler, tokenValidator)

	// Fin module
	assessmentRepo := fin.NewPostgresAssessmentRepository(pool)
	paymentRepo := fin.NewPostgresPaymentRepository(pool)
	budgetRepo := fin.NewPostgresBudgetRepository(pool)
	fundRepo := fin.NewPostgresFundRepository(pool)
	collectionRepo := fin.NewPostgresCollectionRepository(pool)
	finService := fin.NewFinService(assessmentRepo, paymentRepo, budgetRepo, fundRepo, collectionRepo, logger)
	assessmentHandler := fin.NewAssessmentHandler(finService, logger)
	paymentHandler := fin.NewPaymentHandler(finService, logger)
	budgetHandler := fin.NewBudgetHandler(finService, logger)
	fundHandler := fin.NewFundHandler(finService, logger)
	collectionHandler := fin.NewCollectionHandler(finService, logger)
	fin.RegisterRoutes(mux, assessmentHandler, paymentHandler, budgetHandler, fundHandler, collectionHandler, tokenValidator)

	// Gov module
	violationRepo := gov.NewPostgresViolationRepository(pool)
	arbRepo := gov.NewPostgresARBRepository(pool)
	ballotRepo := gov.NewPostgresBallotRepository(pool)
	meetingRepo := gov.NewPostgresMeetingRepository(pool)
	govService := gov.NewGovService(violationRepo, arbRepo, ballotRepo, meetingRepo, logger)
	violationHandler := gov.NewViolationHandler(govService, logger)
	arbHandler := gov.NewARBHandler(govService, logger)
	ballotHandler := gov.NewBallotHandler(govService, logger)
	meetingHandler := gov.NewMeetingHandler(govService, logger)
	gov.RegisterRoutes(mux, violationHandler, arbHandler, ballotHandler, meetingHandler, tokenValidator)

	// Com module
	announcementRepo := com.NewPostgresAnnouncementRepository(pool)
	threadRepo := com.NewPostgresThreadRepository(pool)
	notificationRepo := com.NewPostgresNotificationRepository(pool)
	calendarRepo := com.NewPostgresCalendarRepository(pool)
	templateRepo := com.NewPostgresTemplateRepository(pool)
	directoryRepo := com.NewPostgresDirectoryRepository(pool)
	commLogRepo := com.NewPostgresCommLogRepository(pool)
	comService := com.NewComService(announcementRepo, threadRepo, notificationRepo, calendarRepo, templateRepo, directoryRepo, commLogRepo, logger)
	announcementHandler := com.NewAnnouncementHandler(comService, logger)
	threadHandler := com.NewThreadHandler(comService, logger)
	calendarHandler := com.NewCalendarHandler(comService, logger)
	notificationHandler := com.NewNotificationHandler(comService, logger)
	commLogHandler := com.NewCommLogHandler(comService, logger)
	com.RegisterRoutes(mux, announcementHandler, threadHandler, calendarHandler, notificationHandler, commLogHandler, tokenValidator)

	// Task module
	taskRepo := task.NewPostgresTaskRepository(pool)
	taskService := task.NewTaskService(taskRepo, logger)
	taskHandler := task.NewTaskHandler(taskService, logger)
	task.RegisterRoutes(mux, taskHandler, tokenValidator)

	// License module
	licenseRepo := license.NewPostgresLicenseRepository(pool)
	entitlementChecker := license.NewPostgresEntitlementChecker(licenseRepo)
	_ = entitlementChecker // Will be injected into middleware in a future phase
	licenseService := license.NewLicenseService(licenseRepo, entitlementChecker, logger)
	licenseHandler := license.NewLicenseHandler(licenseService, logger)
	license.RegisterRoutes(mux, licenseHandler, tokenValidator)

	// Billing module
	billingRepo := billing.NewPostgresBillingRepository(pool)
	billingService := billing.NewBillingService(billingRepo, logger)
	billingHandler := billing.NewBillingHandler(billingService, logger)
	billing.RegisterRoutes(mux, billingHandler, tokenValidator)

	// Admin module
	adminRepo := admin.NewPostgresAdminRepository(pool)
	adminService := admin.NewAdminService(adminRepo, logger)
	adminHandler := admin.NewAdminHandler(adminService, logger)
	admin.RegisterRoutes(mux, adminHandler, tokenValidator)

	// AI module
	contextChunkRepo := ai.NewPostgresContextChunkRepository(pool)
	contextLakeService := ai.NewContextLakeService(contextChunkRepo, orgRepo, ai.StubEmbeddingFunc, logger)
	policyRepo := ai.NewPostgresPolicyRepository(pool)
	policyService := ai.NewPolicyService(policyRepo, logger)
	aiHandler := ai.NewAIHandler(policyService, contextLakeService, orgRepo, logger)
	ai.RegisterRoutes(mux, aiHandler, tokenValidator)

	// Replace stub resolvers with real implementations (for future module injection).
	_ = ai.NewPostgresContextRetriever(contextLakeService)
	_ = ai.NewPostgresPolicyResolver(policyService)

	// Webhook module
	webhookRepo := webhook.NewPostgresWebhookRepository(pool)
	webhookService := webhook.NewWebhookService(webhookRepo, logger)
	webhookHandler := webhook.NewWebhookHandler(webhookService, logger)
	webhook.RegisterRoutes(mux, webhookHandler, tokenValidator)

	// Doc module
	s3Client, err := storage.NewS3Client(cfg.S3)
	if err != nil {
		logger.Warn("failed to initialize S3 client", "error", err)
	}
	if s3Client != nil {
		if err := s3Client.EnsureBucket(ctx); err != nil {
			logger.Warn("failed to ensure S3 bucket", "error", err)
		}
	}
	docRepo := doc.NewPostgresDocRepository(pool)
	// Use MockStorageClient if s3Client is nil (S3 unavailable)
	var storageClient storage.StorageClient = s3Client
	if storageClient == nil {
		storageClient = storage.NewMockStorageClient()
	}
	docService := doc.NewDocService(docRepo, storageClient, cfg.S3.Bucket, logger)
	docHandler := doc.NewDocHandler(docService, logger)
	doc.RegisterRoutes(mux, docHandler, tokenValidator)

	// 10. Middleware chain (innermost to outermost)
	var handler http.Handler = mux
	handler = middleware.Logging(logger, handler)
	handler = middleware.Recovery(logger, handler)
	handler = middleware.RequestID(handler)
	handler = middleware.Tracing(handler) // outermost: creates root span before RequestID
	handler = middleware.CORS([]string{"*"}, handler) // permissive for dev; configured per-env in production

	// 11. HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 12. Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting API server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 13. Graceful shutdown
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
