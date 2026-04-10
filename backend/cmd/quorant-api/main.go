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

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	"github.com/quorant/quorant/internal/admin"
	"github.com/quorant/quorant/internal/ai"
	platformai "github.com/quorant/quorant/internal/platform/ai"
	"github.com/quorant/quorant/internal/platform/cfgstore"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/billing"
	"github.com/quorant/quorant/internal/com"
	"github.com/quorant/quorant/internal/doc"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/gov"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/license"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/task"
	"github.com/quorant/quorant/internal/webhook"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/config"
	"github.com/quorant/quorant/internal/platform/db"
	"github.com/quorant/quorant/internal/platform/health"
	"github.com/quorant/quorant/internal/platform/i18n"
	"github.com/quorant/quorant/internal/platform/logging"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/platform/storage"
	"github.com/quorant/quorant/internal/platform/telemetry"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// iamUserFinder adapts iam.UserRepository to the middleware.UserFinder interface.
// iam.UserRepository.FindByIDPUserID returns (*iam.User, error), but UserFinder
// returns (uuid.UUID, error) — this adapter bridges the gap.
type iamUserFinder struct {
	repo iam.UserRepository
}

func (f iamUserFinder) FindByIDPUserID(ctx context.Context, idpUserID string) (uuid.UUID, error) {
	user, err := f.repo.FindByIDPUserID(ctx, idpUserID)
	if err != nil {
		return uuid.Nil, err
	}
	if user == nil {
		return uuid.Nil, fmt.Errorf("user not found for idp_user_id: %s", idpUserID)
	}
	return user.ID, nil
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
	outboxPublisher := queue.NewOutboxPublisher(pool)
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

	// Metrics endpoint (no auth — for Prometheus scraper)
	mux.Handle("GET /metrics", promhttp.Handler())

	// i18n module (unauthenticated — language packs are public)
	i18nRegistry, err := i18n.NewRegistry()
	if err != nil {
		logger.Error("failed to load i18n packs", "error", err)
		os.Exit(1)
	}
	i18nHandler := i18n.NewHandler(i18nRegistry)
	mux.HandleFunc("GET /api/v1/i18n/{locale}", i18nHandler.GetPack)
	mux.HandleFunc("GET /api/v1/i18n", i18nHandler.ListLocales)

	// IAM module
	userRepo := iam.NewPostgresUserRepository(pool)
	userService := iam.NewUserService(userRepo)
	iamHandler := iam.NewHandlerWithSecret(userService, logger, cfg.Zitadel.WebhookSecret)
	iam.RegisterRoutes(mux, iamHandler, tokenValidator)

	// RBAC: permission checker and user ID resolver shared across all modules
	permChecker := middleware.NewPostgresPermissionChecker(pool)
	resolveUserID := middleware.NewUserIDResolver(iamUserFinder{repo: userRepo})

	// Org module
	orgRepo := org.NewPostgresOrgRepository(pool)
	membershipRepo := org.NewPostgresMembershipRepository(pool)
	unitRepo := org.NewPostgresUnitRepository(pool)
	amenityRepo := org.NewPostgresAmenityRepository(pool)
	vendorRepo := org.NewPostgresVendorRepository(pool)
	registrationRepo := org.NewPostgresRegistrationRepository(pool)
	orgService := org.NewOrgService(orgRepo, membershipRepo, unitRepo, userRepo, auditor, outboxPublisher, logger).
		WithAmenityRepo(amenityRepo).
		WithVendorRepo(vendorRepo).
		WithRegistrationRepo(registrationRepo)
	orgHandler := org.NewOrgHandler(orgService, logger)
	membershipHandler := org.NewMembershipHandler(orgService, logger)
	unitHandler := org.NewUnitHandler(orgService, logger)
	amenityHandler := org.NewAmenityHandler(orgService, logger)
	vendorHandler := org.NewVendorHandler(orgService, logger)
	registrationHandler := org.NewRegistrationHandler(orgService, logger)
	org.RegisterRoutes(mux, orgHandler, membershipHandler, unitHandler, amenityHandler, vendorHandler, registrationHandler, tokenValidator, permChecker, resolveUserID)

	// License module (initialized early — entitlementChecker is needed by AI and Webhook routes)
	licenseRepo := license.NewPostgresLicenseRepository(pool)
	pgChecker := license.NewPostgresEntitlementChecker(licenseRepo)
	entitlementChecker := license.NewCachedEntitlementChecker(pgChecker, rdb, 60*time.Second)
	licenseService := license.NewLicenseService(licenseRepo, entitlementChecker, auditor, outboxPublisher, logger)
	licenseHandler := license.NewLicenseHandler(licenseService, logger)
	license.RegisterRoutes(mux, licenseHandler, tokenValidator, permChecker, resolveUserID)

	// LLM client (used by AI module for embeddings and completions)
	var llmClient platformai.Client
	llmCfg := platformai.Config{
		Provider: platformai.Provider(cfg.LLM.Provider),
		APIKey:   cfg.LLM.APIKey,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	}
	if cfg.LLM.EmbedProvider != "" {
		embedCfg := &platformai.Config{
			Provider: platformai.Provider(cfg.LLM.EmbedProvider),
			APIKey:   cfg.LLM.EmbedAPIKey,
			BaseURL:  cfg.LLM.EmbedBaseURL,
			Model:    cfg.LLM.EmbedModel,
		}
		llmClient, _ = platformai.NewClientFromEnv(llmCfg, embedCfg)
	} else {
		llmClient, _ = platformai.NewClientFromEnv(llmCfg, nil)
	}
	// Wrap with retry for transient failures (429, 5xx)
	llmClient = platformai.NewRetryClient(llmClient, 3)

	// Bridge: convert platform/ai.Client.Embed into internal/ai.EmbeddingFunc
	embedFn := ai.EmbeddingFunc(func(ctx context.Context, text string) ([]float32, error) {
		resp, err := llmClient.Embed(ctx, platformai.EmbeddingRequest{
			Model: cfg.LLM.EmbedModel,
			Input: text,
		})
		if err != nil {
			return nil, err
		}
		return resp.Embedding, nil
	})

	// AI module (initialized before domain modules that depend on it)
	contextChunkRepo := ai.NewPostgresContextChunkRepository(pool)
	contextLakeService := ai.NewContextLakeService(contextChunkRepo, orgRepo, embedFn, logger)
	policyRepo := ai.NewPostgresPolicyRepository(pool)
	policyService := ai.NewPolicyService(policyRepo, logger)
	cfgStore := cfgstore.Store(cfgstore.NewCachedStore(
		cfgstore.NewPostgresStore(pool), rdb, 60*time.Second,
	))
	aiHandler := ai.NewAIHandler(policyService, contextLakeService, orgRepo, cfgStore, logger)
	ai.RegisterRoutes(mux, aiHandler, tokenValidator, permChecker, resolveUserID, entitlementChecker)

	// Real implementations of AI interfaces injected into domain modules.
	contextRetriever := ai.NewPostgresContextRetriever(contextLakeService)
	_ = contextRetriever // reserved for future com module wiring
	policyResolver := ai.NewPostgresPolicyResolverWithLLM(policyService, contextLakeService, llmClient)

	// Compliance engine
	jurisdictionRuleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
	complianceCheckRepo := ai.NewPostgresComplianceCheckRepository(pool)
	complianceService := ai.NewComplianceService(jurisdictionRuleRepo, complianceCheckRepo, orgRepo, logger)
	complianceService.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	complianceService.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	complianceService.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	complianceService.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	complianceService.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	complianceService.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	complianceService.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)
	complianceHandler := ai.NewComplianceHandler(complianceService, complianceCheckRepo, logger)
	ai.RegisterComplianceRoutes(mux, complianceHandler, tokenValidator, permChecker, resolveUserID, entitlementChecker)
	jurisdictionAdminHandler := ai.NewJurisdictionAdminHandler(jurisdictionRuleRepo, outboxPublisher, logger)
	ai.RegisterJurisdictionAdminRoutes(mux, jurisdictionAdminHandler, tokenValidator, permChecker, resolveUserID)

	// Fin module
	assessmentRepo := fin.NewPostgresAssessmentRepository(pool)
	paymentRepo := fin.NewPostgresPaymentRepository(pool)
	budgetRepo := fin.NewPostgresBudgetRepository(pool)
	fundRepo := fin.NewPostgresFundRepository(pool)
	collectionRepo := fin.NewPostgresCollectionRepository(pool)
	glRepo := fin.NewPostgresGLRepository(pool)
	glService := fin.NewGLService(glRepo, auditor, logger)
	finService := fin.NewFinService(assessmentRepo, paymentRepo, budgetRepo, fundRepo, collectionRepo, glService, auditor, outboxPublisher, policyResolver, complianceService, logger)
	assessmentHandler := fin.NewAssessmentHandler(finService, logger)
	paymentHandler := fin.NewPaymentHandler(finService, logger)
	budgetHandler := fin.NewBudgetHandler(finService, logger)
	fundHandler := fin.NewFundHandler(finService, logger)
	collectionHandler := fin.NewCollectionHandler(finService, logger)
	glHandler := fin.NewGLHandler(glService, logger)
	fin.RegisterRoutes(mux, assessmentHandler, paymentHandler, budgetHandler, fundHandler, collectionHandler, glHandler, tokenValidator, permChecker, resolveUserID)

	// Gov module
	violationRepo := gov.NewPostgresViolationRepository(pool)
	arbRepo := gov.NewPostgresARBRepository(pool)
	ballotRepo := gov.NewPostgresBallotRepository(pool)
	meetingRepo := gov.NewPostgresMeetingRepository(pool)
	govService := gov.NewGovService(violationRepo, arbRepo, ballotRepo, meetingRepo, auditor, outboxPublisher, policyResolver, complianceService, logger)
	violationHandler := gov.NewViolationHandler(govService, logger)
	arbHandler := gov.NewARBHandler(govService, logger)
	ballotHandler := gov.NewBallotHandler(govService, logger)
	meetingHandler := gov.NewMeetingHandler(govService, logger)
	gov.RegisterRoutes(mux, violationHandler, arbHandler, ballotHandler, meetingHandler, tokenValidator, permChecker, resolveUserID)

	// Com module
	announcementRepo := com.NewPostgresAnnouncementRepository(pool)
	threadRepo := com.NewPostgresThreadRepository(pool)
	notificationRepo := com.NewPostgresNotificationRepository(pool)
	calendarRepo := com.NewPostgresCalendarRepository(pool)
	templateRepo := com.NewPostgresTemplateRepository(pool)
	directoryRepo := com.NewPostgresDirectoryRepository(pool)
	commLogRepo := com.NewPostgresCommLogRepository(pool)
	comService := com.NewComService(announcementRepo, threadRepo, notificationRepo, calendarRepo, templateRepo, directoryRepo, commLogRepo, auditor, outboxPublisher, logger)
	announcementHandler := com.NewAnnouncementHandler(comService, logger)
	threadHandler := com.NewThreadHandler(comService, logger)
	calendarHandler := com.NewCalendarHandler(comService, logger)
	notificationHandler := com.NewNotificationHandler(comService, logger)
	commLogHandler := com.NewCommLogHandler(comService, logger)
	com.RegisterRoutes(mux, announcementHandler, threadHandler, calendarHandler, notificationHandler, commLogHandler, tokenValidator, permChecker, resolveUserID)

	// Task module
	taskRepo := task.NewPostgresTaskRepository(pool)
	taskService := task.NewTaskService(taskRepo, auditor, outboxPublisher, logger)
	taskHandler := task.NewTaskHandler(taskService, logger)
	task.RegisterRoutes(mux, taskHandler, tokenValidator, permChecker, resolveUserID)

	// Billing module
	billingRepo := billing.NewPostgresBillingRepository(pool)
	billingService := billing.NewBillingService(billingRepo, auditor, outboxPublisher, logger)
	billingHandler := billing.NewBillingHandlerWithSecret(billingService, logger, cfg.Stripe.WebhookSecret)
	billing.RegisterRoutes(mux, billingHandler, tokenValidator, permChecker, resolveUserID)

	// Admin module
	adminRepo := admin.NewPostgresAdminRepository(pool)
	adminService := admin.NewAdminService(adminRepo, auditor, outboxPublisher, logger)
	adminHandler := admin.NewAdminHandler(adminService, logger)
	admin.RegisterRoutes(mux, adminHandler, tokenValidator, permChecker, resolveUserID)

	// Audit module
	auditHandler := audit.NewHandler(auditor, logger)
	audit.RegisterRoutes(mux, auditHandler, tokenValidator, permChecker, resolveUserID)

	// Webhook module
	webhookRepo := webhook.NewPostgresWebhookRepository(pool)
	webhookService := webhook.NewWebhookService(webhookRepo, auditor, outboxPublisher, logger)
	webhookHandler := webhook.NewWebhookHandler(webhookService, logger)
	webhook.RegisterRoutes(mux, webhookHandler, tokenValidator, permChecker, resolveUserID, entitlementChecker)

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
	var storageClient storage.StorageClient
	if s3Client != nil {
		storageClient = s3Client
	} else if cfg.Server.Environment == "production" {
		return fmt.Errorf("S3 storage is required in production")
	} else {
		logger.Warn("S3 storage not available — using in-memory mock (dev only)")
		storageClient = storage.NewMockStorageClient()
	}
	docService := doc.NewDocService(docRepo, storageClient, cfg.S3.Bucket, auditor, outboxPublisher, logger)
	docHandler := doc.NewDocHandler(docService, logger)
	doc.RegisterRoutes(mux, docHandler, tokenValidator, permChecker, resolveUserID)

	// --- Estoppel module ---
	estoppelRepo := estoppel.NewPostgresRepository(pool)
	financialProvider := fin.NewEstoppelFinancialAdapter(finService)
	complianceProvider := gov.NewEstoppelComplianceAdapter(govService)
	propertyProvider := org.NewEstoppelPropertyAdapter(orgService)
	jurisdictionRulesRepo := estoppel.NewPostgresJurisdictionRulesRepository(pool)
	narrativeGen := estoppel.NewNoopNarrativeGenerator()
	pdfGen := estoppel.NewMarotoGenerator()
	estoppelDocAdapter := doc.NewEstoppelDocumentAdapter(docService)
	estoppelService := estoppel.NewEstoppelService(
		estoppelRepo,
		financialProvider,
		complianceProvider,
		propertyProvider,
		jurisdictionRulesRepo,
		narrativeGen,
		pdfGen,
		estoppelDocAdapter,
		estoppelDocAdapter,
		auditor,
		outboxPublisher,
		logger,
	)
	estoppelHandler := estoppel.NewHandler(estoppelService, logger)
	estoppel.RegisterRoutes(mux, estoppelHandler, tokenValidator, permChecker, entitlementChecker, resolveUserID)

	// 10. Middleware chain (innermost to outermost)
	rateLimiter := middleware.NewRateLimiter(100, 100, time.Minute) // 100 req/min default
	var handler http.Handler = mux
	handler = middleware.RateLimit(rateLimiter)(handler)
	handler = middleware.Logging(logger, handler)
	handler = middleware.Recovery(logger, handler)
	handler = middleware.RequestID(handler)
	handler = middleware.Metrics(handler)  // after RequestID, before Tracing
	handler = middleware.Tracing(handler)  // outermost: creates root span before RequestID
	handler = middleware.CORS(cfg.CORSAllowedOrigins(), handler) // permissive for dev; configured per-env in production

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
