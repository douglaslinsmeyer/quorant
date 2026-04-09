package webhook

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
)

// WebhookService provides business logic for the webhook domain.
type WebhookService struct {
	repo      WebhookRepository
	auditor   audit.Auditor
	publisher queue.Publisher
	logger    *slog.Logger
}

// NewWebhookService constructs a WebhookService backed by the given repository.
func NewWebhookService(repo WebhookRepository, auditor audit.Auditor, publisher queue.Publisher, logger *slog.Logger) *WebhookService {
	return &WebhookService{repo: repo, auditor: auditor, publisher: publisher, logger: logger}
}

// CreateSubscription creates a new webhook subscription with a generated secret.
func (s *WebhookService) CreateSubscription(ctx context.Context, orgID, createdBy uuid.UUID, req CreateSubscriptionRequest) (*Subscription, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("webhook service: CreateSubscription: generate secret: %w", err)
	}

	retryPolicy := DefaultRetryPolicy()
	if req.RetryPolicy != nil {
		retryPolicy = *req.RetryPolicy
	}

	headers := req.Headers
	if headers == nil {
		headers = map[string]string{}
	}

	sub := &Subscription{
		OrgID:         orgID,
		Name:          req.Name,
		EventPatterns: req.EventPatterns,
		TargetURL:     req.TargetURL,
		Secret:        secret,
		Headers:       headers,
		IsActive:      true,
		RetryPolicy:   retryPolicy,
		CreatedBy:     createdBy,
	}

	created, err := s.repo.CreateSubscription(ctx, sub)
	if err != nil {
		return nil, fmt.Errorf("webhook service: CreateSubscription: %w", err)
	}

	s.logger.InfoContext(ctx, "webhook subscription created", "subscription_id", created.ID, "org_id", orgID)
	return created, nil
}

// ListSubscriptions returns all webhook subscriptions for the given org.
func (s *WebhookService) ListSubscriptions(ctx context.Context, orgID uuid.UUID) ([]Subscription, error) {
	subs, err := s.repo.ListSubscriptionsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("webhook service: ListSubscriptions: %w", err)
	}
	return subs, nil
}

// GetSubscription returns a subscription by ID, or NotFoundError if it does not exist.
func (s *WebhookService) GetSubscription(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	sub, err := s.repo.FindSubscriptionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("webhook service: GetSubscription: %w", err)
	}
	if sub == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("webhook subscription %s not found", id))
	}
	return sub, nil
}

// UpdateSubscription applies the request fields to the given subscription.
func (s *WebhookService) UpdateSubscription(ctx context.Context, id uuid.UUID, req UpdateSubscriptionRequest) (*Subscription, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	sub, err := s.GetSubscription(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		sub.Name = *req.Name
	}
	if len(req.EventPatterns) > 0 {
		sub.EventPatterns = req.EventPatterns
	}
	if req.TargetURL != nil {
		sub.TargetURL = *req.TargetURL
	}
	if req.IsActive != nil {
		sub.IsActive = *req.IsActive
	}
	if req.Headers != nil {
		sub.Headers = req.Headers
	}

	updated, err := s.repo.UpdateSubscription(ctx, sub)
	if err != nil {
		return nil, fmt.Errorf("webhook service: UpdateSubscription: %w", err)
	}

	s.logger.InfoContext(ctx, "webhook subscription updated", "subscription_id", id)
	return updated, nil
}

// DeleteSubscription soft-deletes a webhook subscription.
func (s *WebhookService) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	// Verify it exists first.
	if _, err := s.GetSubscription(ctx, id); err != nil {
		return err
	}

	if err := s.repo.SoftDeleteSubscription(ctx, id); err != nil {
		return fmt.Errorf("webhook service: DeleteSubscription: %w", err)
	}

	s.logger.InfoContext(ctx, "webhook subscription deleted", "subscription_id", id)
	return nil
}

// ListDeliveries returns all deliveries for the given subscription.
func (s *WebhookService) ListDeliveries(ctx context.Context, subscriptionID uuid.UUID) ([]Delivery, error) {
	// Ensure the subscription exists.
	if _, err := s.GetSubscription(ctx, subscriptionID); err != nil {
		return nil, err
	}

	deliveries, err := s.repo.ListDeliveriesBySubscription(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("webhook service: ListDeliveries: %w", err)
	}
	return deliveries, nil
}

// GetEventTypes returns the list of available event types for subscriptions.
func (s *WebhookService) GetEventTypes(_ context.Context) []string {
	return AvailableEventTypes()
}

// SendTestEvent creates a test delivery record for a subscription using a synthetic payload.
func (s *WebhookService) SendTestEvent(ctx context.Context, subscriptionID uuid.UUID) (*Delivery, error) {
	if _, err := s.GetSubscription(ctx, subscriptionID); err != nil {
		return nil, err
	}

	now := time.Now()
	delivery := &Delivery{
		SubscriptionID: subscriptionID,
		EventID:        uuid.New(),
		EventType:      "quorant.test.PingEvent",
		Status:         DeliveryStatusPending,
		Attempts:       0,
		NextRetryAt:    &now,
	}

	created, err := s.repo.CreateDelivery(ctx, delivery)
	if err != nil {
		return nil, fmt.Errorf("webhook service: SendTestEvent: %w", err)
	}

	s.logger.InfoContext(ctx, "test delivery created", "subscription_id", subscriptionID, "delivery_id", created.ID)
	return created, nil
}

// generateSecret creates a cryptographically random 32-byte hex string.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}
