package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWebhookRepository implements WebhookRepository using a pgxpool.
type PostgresWebhookRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresWebhookRepository creates a new PostgresWebhookRepository.
func NewPostgresWebhookRepository(pool *pgxpool.Pool) *PostgresWebhookRepository {
	return &PostgresWebhookRepository{pool: pool}
}

// Pool returns the underlying pgxpool.Pool for test helpers.
func (r *PostgresWebhookRepository) Pool() *pgxpool.Pool { return r.pool }

// ─── Subscriptions ────────────────────────────────────────────────────────────

func (r *PostgresWebhookRepository) CreateSubscription(ctx context.Context, s *Subscription) (*Subscription, error) {
	headersJSON, err := marshalHeaders(s.Headers)
	if err != nil {
		return nil, fmt.Errorf("webhook: CreateSubscription marshal headers: %w", err)
	}
	retryJSON, err := marshalRetryPolicy(s.RetryPolicy)
	if err != nil {
		return nil, fmt.Errorf("webhook: CreateSubscription marshal retry_policy: %w", err)
	}

	const q = `
		INSERT INTO webhook_subscriptions
			(org_id, name, event_patterns, target_url, secret, headers, is_active, retry_policy, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, org_id, name, event_patterns, target_url, secret, headers,
		          is_active, retry_policy, created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		s.OrgID, s.Name, s.EventPatterns, s.TargetURL, s.Secret,
		headersJSON, s.IsActive, retryJSON, s.CreatedBy,
	)
	result, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("webhook: CreateSubscription: %w", err)
	}
	return result, nil
}

func (r *PostgresWebhookRepository) FindSubscriptionByID(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	const q = `
		SELECT id, org_id, name, event_patterns, target_url, secret, headers,
		       is_active, retry_policy, created_by, created_at, updated_at, deleted_at
		FROM webhook_subscriptions WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanSubscription(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: FindSubscriptionByID: %w", err)
	}
	return result, nil
}

func (r *PostgresWebhookRepository) ListSubscriptionsByOrg(ctx context.Context, orgID uuid.UUID) ([]Subscription, error) {
	const q = `
		SELECT id, org_id, name, event_patterns, target_url, secret, headers,
		       is_active, retry_policy, created_by, created_at, updated_at, deleted_at
		FROM webhook_subscriptions
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("webhook: ListSubscriptionsByOrg: %w", err)
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		sub, err := scanSubscriptionRow(rows)
		if err != nil {
			return nil, fmt.Errorf("webhook: ListSubscriptionsByOrg scan: %w", err)
		}
		subs = append(subs, *sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webhook: ListSubscriptionsByOrg rows: %w", err)
	}
	return subs, nil
}

func (r *PostgresWebhookRepository) UpdateSubscription(ctx context.Context, s *Subscription) (*Subscription, error) {
	headersJSON, err := marshalHeaders(s.Headers)
	if err != nil {
		return nil, fmt.Errorf("webhook: UpdateSubscription marshal headers: %w", err)
	}
	retryJSON, err := marshalRetryPolicy(s.RetryPolicy)
	if err != nil {
		return nil, fmt.Errorf("webhook: UpdateSubscription marshal retry_policy: %w", err)
	}

	const q = `
		UPDATE webhook_subscriptions SET
			name           = $1,
			event_patterns = $2,
			target_url     = $3,
			headers        = $4,
			is_active      = $5,
			retry_policy   = $6,
			updated_at     = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING id, org_id, name, event_patterns, target_url, secret, headers,
		          is_active, retry_policy, created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		s.Name, s.EventPatterns, s.TargetURL,
		headersJSON, s.IsActive, retryJSON, s.ID,
	)
	result, err := scanSubscription(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("webhook: UpdateSubscription: subscription %s not found", s.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: UpdateSubscription: %w", err)
	}
	return result, nil
}

func (r *PostgresWebhookRepository) SoftDeleteSubscription(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE webhook_subscriptions SET deleted_at = now(), is_active = FALSE WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("webhook: SoftDeleteSubscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook: SoftDeleteSubscription: subscription %s not found", id)
	}
	return nil
}

// FindActiveSubscriptionsForEvent returns active subscriptions whose event_patterns match
// the given eventType. Pattern matching is done in Go: "*" within a pattern segment acts
// as a wildcard (e.g. "quorant.gov.*" matches "quorant.gov.ViolationCreated").
func (r *PostgresWebhookRepository) FindActiveSubscriptionsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]Subscription, error) {
	const q = `
		SELECT id, org_id, name, event_patterns, target_url, secret, headers,
		       is_active, retry_policy, created_by, created_at, updated_at, deleted_at
		FROM webhook_subscriptions
		WHERE org_id = $1 AND is_active = TRUE AND deleted_at IS NULL`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("webhook: FindActiveSubscriptionsForEvent: %w", err)
	}
	defer rows.Close()

	var matched []Subscription
	for rows.Next() {
		sub, err := scanSubscriptionRow(rows)
		if err != nil {
			return nil, fmt.Errorf("webhook: FindActiveSubscriptionsForEvent scan: %w", err)
		}
		if matchesAnyPattern(sub.EventPatterns, eventType) {
			matched = append(matched, *sub)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webhook: FindActiveSubscriptionsForEvent rows: %w", err)
	}
	return matched, nil
}

// matchesAnyPattern returns true if eventType matches at least one pattern.
// Patterns support a single trailing wildcard segment (e.g. "quorant.gov.*").
func matchesAnyPattern(patterns []string, eventType string) bool {
	for _, p := range patterns {
		if matchPattern(p, eventType) {
			return true
		}
	}
	return false
}

// matchPattern matches an event type against a single pattern.
// Only trailing "*" wildcards are supported (e.g. "quorant.*", "quorant.gov.*").
func matchPattern(pattern, eventType string) bool {
	if pattern == eventType {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(eventType, prefix)
	}
	return false
}

// ─── Deliveries ───────────────────────────────────────────────────────────────

func (r *PostgresWebhookRepository) CreateDelivery(ctx context.Context, d *Delivery) (*Delivery, error) {
	const q = `
		INSERT INTO webhook_deliveries
			(subscription_id, event_id, event_type, status, attempts, last_attempt_at, response_code, response_body, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, subscription_id, event_id, event_type, status, attempts,
		          last_attempt_at, response_code, response_body, next_retry_at, created_at`

	row := r.pool.QueryRow(ctx, q,
		d.SubscriptionID, d.EventID, d.EventType, d.Status, d.Attempts,
		d.LastAttemptAt, d.ResponseCode, truncateResponseBody(d.ResponseBody), d.NextRetryAt,
	)
	result, err := scanDelivery(row)
	if err != nil {
		return nil, fmt.Errorf("webhook: CreateDelivery: %w", err)
	}
	return result, nil
}

func (r *PostgresWebhookRepository) UpdateDelivery(ctx context.Context, d *Delivery) (*Delivery, error) {
	const q = `
		UPDATE webhook_deliveries SET
			status          = $1,
			attempts        = $2,
			last_attempt_at = $3,
			response_code   = $4,
			response_body   = $5,
			next_retry_at   = $6
		WHERE id = $7
		RETURNING id, subscription_id, event_id, event_type, status, attempts,
		          last_attempt_at, response_code, response_body, next_retry_at, created_at`

	row := r.pool.QueryRow(ctx, q,
		d.Status, d.Attempts, d.LastAttemptAt,
		d.ResponseCode, truncateResponseBody(d.ResponseBody), d.NextRetryAt, d.ID,
	)
	result, err := scanDelivery(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("webhook: UpdateDelivery: delivery %s not found", d.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: UpdateDelivery: %w", err)
	}
	return result, nil
}

func (r *PostgresWebhookRepository) ListDeliveriesBySubscription(ctx context.Context, subscriptionID uuid.UUID) ([]Delivery, error) {
	const q = `
		SELECT id, subscription_id, event_id, event_type, status, attempts,
		       last_attempt_at, response_code, response_body, next_retry_at, created_at
		FROM webhook_deliveries
		WHERE subscription_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("webhook: ListDeliveriesBySubscription: %w", err)
	}
	defer rows.Close()

	var deliveries []Delivery
	for rows.Next() {
		d, err := scanDeliveryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("webhook: ListDeliveriesBySubscription scan: %w", err)
		}
		deliveries = append(deliveries, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webhook: ListDeliveriesBySubscription rows: %w", err)
	}
	return deliveries, nil
}

func (r *PostgresWebhookRepository) FindPendingDeliveries(ctx context.Context) ([]Delivery, error) {
	const q = `
		SELECT id, subscription_id, event_id, event_type, status, attempts,
		       last_attempt_at, response_code, response_body, next_retry_at, created_at
		FROM webhook_deliveries
		WHERE status IN ('pending', 'retrying')
		  AND (next_retry_at IS NULL OR next_retry_at <= now())
		ORDER BY next_retry_at ASC NULLS FIRST, created_at ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("webhook: FindPendingDeliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []Delivery
	for rows.Next() {
		d, err := scanDeliveryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("webhook: FindPendingDeliveries scan: %w", err)
		}
		deliveries = append(deliveries, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webhook: FindPendingDeliveries rows: %w", err)
	}
	return deliveries, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func marshalHeaders(h map[string]string) ([]byte, error) {
	if h == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(h)
}

func unmarshalHeaders(raw []byte, dst *map[string]string) error {
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil {
			return fmt.Errorf("unmarshal headers: %w", err)
		}
	}
	if *dst == nil {
		*dst = map[string]string{}
	}
	return nil
}

func marshalRetryPolicy(p RetryPolicy) ([]byte, error) {
	return json.Marshal(p)
}

func unmarshalRetryPolicy(raw []byte, dst *RetryPolicy) error {
	if len(raw) == 0 {
		*dst = DefaultRetryPolicy()
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("unmarshal retry_policy: %w", err)
	}
	return nil
}

func scanSubscription(row pgx.Row) (*Subscription, error) {
	var s Subscription
	var headersRaw []byte
	var retryRaw []byte
	err := row.Scan(
		&s.ID, &s.OrgID, &s.Name, &s.EventPatterns, &s.TargetURL, &s.Secret,
		&headersRaw, &s.IsActive, &retryRaw, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := unmarshalHeaders(headersRaw, &s.Headers); err != nil {
		return nil, err
	}
	if err := unmarshalRetryPolicy(retryRaw, &s.RetryPolicy); err != nil {
		return nil, err
	}
	return &s, nil
}

func scanSubscriptionRow(rows pgx.Rows) (*Subscription, error) {
	var s Subscription
	var headersRaw []byte
	var retryRaw []byte
	err := rows.Scan(
		&s.ID, &s.OrgID, &s.Name, &s.EventPatterns, &s.TargetURL, &s.Secret,
		&headersRaw, &s.IsActive, &retryRaw, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := unmarshalHeaders(headersRaw, &s.Headers); err != nil {
		return nil, err
	}
	if err := unmarshalRetryPolicy(retryRaw, &s.RetryPolicy); err != nil {
		return nil, err
	}
	return &s, nil
}

func scanDelivery(row pgx.Row) (*Delivery, error) {
	var d Delivery
	err := row.Scan(
		&d.ID, &d.SubscriptionID, &d.EventID, &d.EventType, &d.Status,
		&d.Attempts, &d.LastAttemptAt, &d.ResponseCode, &d.ResponseBody,
		&d.NextRetryAt, &d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func scanDeliveryRow(rows pgx.Rows) (*Delivery, error) {
	var d Delivery
	err := rows.Scan(
		&d.ID, &d.SubscriptionID, &d.EventID, &d.EventType, &d.Status,
		&d.Attempts, &d.LastAttemptAt, &d.ResponseCode, &d.ResponseBody,
		&d.NextRetryAt, &d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

const maxResponseBodyBytes = 4096

// truncateResponseBody caps response body storage at maxResponseBodyBytes bytes.
// It operates on bytes to avoid splitting multi-byte UTF-8 sequences mid-rune.
func truncateResponseBody(body *string) *string {
	if body == nil {
		return nil
	}
	b := []byte(*body)
	if len(b) <= maxResponseBodyBytes {
		return body
	}
	truncated := string(b[:maxResponseBodyBytes])
	return &truncated
}
