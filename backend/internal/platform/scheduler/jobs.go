package scheduler

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AssessmentGeneratorJob generates assessments for active schedules.
// It checks each active assessment_schedule and creates assessments for units
// in the org that do not already have an assessment for the current period.
// Runs every hour.
type AssessmentGeneratorJob struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewAssessmentGeneratorJob creates a new AssessmentGeneratorJob.
func NewAssessmentGeneratorJob(pool *pgxpool.Pool, logger *slog.Logger) *AssessmentGeneratorJob {
	return &AssessmentGeneratorJob{pool: pool, logger: logger}
}

func (j *AssessmentGeneratorJob) Name() string { return "assessment_generator" }

func (j *AssessmentGeneratorJob) Run(ctx context.Context) error {
	rows, err := j.pool.Query(ctx, `
		SELECT id, org_id, name, frequency, base_amount_cents, day_of_month, grace_days
		FROM assessment_schedules
		WHERE is_active = TRUE AND deleted_at IS NULL
		AND (ends_at IS NULL OR ends_at > now())
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type schedule struct {
		id              string
		orgID           string
		name            string
		frequency       string
		baseAmountCents int64
		dayOfMonth      int
		graceDays       int
	}

	var schedules []schedule
	for rows.Next() {
		var s schedule
		if err := rows.Scan(&s.id, &s.orgID, &s.name, &s.frequency, &s.baseAmountCents, &s.dayOfMonth, &s.graceDays); err != nil {
			return err
		}
		schedules = append(schedules, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	generated := 0
	for _, s := range schedules {
		// Insert assessments for all units in the org that don't already have one
		// for this schedule in the current billing period.
		result, err := j.pool.Exec(ctx, `
			INSERT INTO assessments (org_id, unit_id, schedule_id, amount_cents, due_date, grace_days, status, created_at, updated_at)
			SELECT
				u.org_id,
				u.id,
				$1::uuid,
				$2,
				date_trunc('month', now()) + make_interval(days => $3 - 1),
				$4,
				'pending',
				now(),
				now()
			FROM units u
			WHERE u.org_id = $5::uuid
			AND u.deleted_at IS NULL
			AND NOT EXISTS (
				SELECT 1 FROM assessments a
				WHERE a.unit_id = u.id
				AND a.schedule_id = $1::uuid
				AND date_trunc('month', a.due_date) = date_trunc('month', now())
				AND a.deleted_at IS NULL
			)
		`, s.id, s.baseAmountCents, s.dayOfMonth, s.graceDays, s.orgID)
		if err != nil {
			j.logger.Error("failed to generate assessments", "schedule_id", s.id, "error", err)
			continue
		}
		generated += int(result.RowsAffected())
	}

	if generated > 0 {
		j.logger.Info("assessments generated", "count", generated)
	}
	return nil
}

// LateFeeJob applies late fees to overdue assessments.
// Finds assessments past due_date + grace_days that have no late_fee ledger entry
// and creates the corresponding ledger entries.
// Runs daily.
type LateFeeJob struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewLateFeeJob creates a new LateFeeJob.
func NewLateFeeJob(pool *pgxpool.Pool, logger *slog.Logger) *LateFeeJob {
	return &LateFeeJob{pool: pool, logger: logger}
}

func (j *LateFeeJob) Name() string { return "late_fee_applicator" }

func (j *LateFeeJob) Run(ctx context.Context) error {
	result, err := j.pool.Exec(ctx, `
		INSERT INTO ledger_entries (org_id, unit_id, assessment_id, entry_type, amount_cents, description, created_at, updated_at)
		SELECT
			a.org_id,
			a.unit_id,
			a.id,
			'late_fee',
			COALESCE(sch.late_fee_cents, 0),
			'Automatic late fee',
			now(),
			now()
		FROM assessments a
		LEFT JOIN assessment_schedules sch ON sch.id = a.schedule_id AND sch.deleted_at IS NULL
		WHERE a.status NOT IN ('paid', 'waived', 'cancelled')
		AND a.deleted_at IS NULL
		AND (a.due_date + make_interval(days => COALESCE(a.grace_days, 0))) < now()
		AND NOT EXISTS (
			SELECT 1 FROM ledger_entries le
			WHERE le.assessment_id = a.id
			AND le.entry_type = 'late_fee'
			AND le.deleted_at IS NULL
		)
	`)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		j.logger.Info("late fees applied", "count", result.RowsAffected())
	}
	return nil
}

// CollectionEscalationJob advances collection case status for units with overdue balances.
// Checks elapsed time since last action and advances status based on policy.
// Runs daily.
type CollectionEscalationJob struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewCollectionEscalationJob creates a new CollectionEscalationJob.
func NewCollectionEscalationJob(pool *pgxpool.Pool, logger *slog.Logger) *CollectionEscalationJob {
	return &CollectionEscalationJob{pool: pool, logger: logger}
}

func (j *CollectionEscalationJob) Name() string { return "collection_escalator" }

func (j *CollectionEscalationJob) Run(ctx context.Context) error {
	// Escalate 'late' → 'delinquent' after 30 days of no activity
	result, err := j.pool.Exec(ctx, `
		UPDATE collection_cases
		SET status = 'delinquent', updated_at = now()
		WHERE status = 'late'
		AND deleted_at IS NULL
		AND updated_at < now() - interval '30 days'
	`)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		j.logger.Info("collection cases escalated late→delinquent", "count", result.RowsAffected())
	}

	// Escalate 'delinquent' → 'demand_sent' after 60 days of no activity
	result, err = j.pool.Exec(ctx, `
		UPDATE collection_cases
		SET status = 'demand_sent', updated_at = now()
		WHERE status = 'delinquent'
		AND deleted_at IS NULL
		AND updated_at < now() - interval '60 days'
	`)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		j.logger.Info("collection cases escalated delinquent→demand_sent", "count", result.RowsAffected())
	}

	return nil
}

// ARBAutoApprovalJob auto-approves ARB requests that have passed their review deadline.
// Runs hourly.
type ARBAutoApprovalJob struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewARBAutoApprovalJob creates a new ARBAutoApprovalJob.
func NewARBAutoApprovalJob(pool *pgxpool.Pool, logger *slog.Logger) *ARBAutoApprovalJob {
	return &ARBAutoApprovalJob{pool: pool, logger: logger}
}

func (j *ARBAutoApprovalJob) Name() string { return "arb_auto_approval" }

func (j *ARBAutoApprovalJob) Run(ctx context.Context) error {
	result, err := j.pool.Exec(ctx, `
		UPDATE arb_requests
		SET status = 'approved', auto_approved = TRUE, decided_at = now(), updated_at = now()
		WHERE status IN ('submitted', 'under_review')
		AND auto_approved = FALSE
		AND review_deadline IS NOT NULL
		AND review_deadline < now()
		AND deleted_at IS NULL
	`)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		j.logger.Info("ARB requests auto-approved", "count", result.RowsAffected())
	}
	return nil
}

// SLABreachMonitorJob monitors tasks for SLA deadline breaches.
// Sets sla_breached = TRUE for tasks that have passed their SLA deadline.
// Runs every 5 minutes.
type SLABreachMonitorJob struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewSLABreachMonitorJob creates a new SLABreachMonitorJob.
func NewSLABreachMonitorJob(pool *pgxpool.Pool, logger *slog.Logger) *SLABreachMonitorJob {
	return &SLABreachMonitorJob{pool: pool, logger: logger}
}

func (j *SLABreachMonitorJob) Name() string { return "sla_breach_monitor" }

func (j *SLABreachMonitorJob) Run(ctx context.Context) error {
	result, err := j.pool.Exec(ctx, `
		UPDATE tasks SET sla_breached = TRUE, updated_at = now()
		WHERE sla_breached = FALSE
		AND sla_deadline IS NOT NULL
		AND sla_deadline < now()
		AND status NOT IN ('completed', 'cancelled')
	`)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		j.logger.Info("SLA breaches detected", "count", result.RowsAffected())
	}
	return nil
}

// AnnouncementPublisherJob publishes scheduled announcements when their scheduled_for time arrives.
// Runs every minute.
type AnnouncementPublisherJob struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewAnnouncementPublisherJob creates a new AnnouncementPublisherJob.
func NewAnnouncementPublisherJob(pool *pgxpool.Pool, logger *slog.Logger) *AnnouncementPublisherJob {
	return &AnnouncementPublisherJob{pool: pool, logger: logger}
}

func (j *AnnouncementPublisherJob) Name() string { return "announcement_publisher" }

func (j *AnnouncementPublisherJob) Run(ctx context.Context) error {
	result, err := j.pool.Exec(ctx, `
		UPDATE announcements SET published_at = now(), updated_at = now()
		WHERE scheduled_for IS NOT NULL
		AND scheduled_for <= now()
		AND published_at IS NULL
		AND deleted_at IS NULL
	`)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		j.logger.Info("announcements published", "count", result.RowsAffected())
	}
	return nil
}

// NotificationDispatchJob dispatches pending notifications.
// In production this reads NotificationRequested events and fans out to push/email/SMS.
// For now, it is a no-op stub pending external service integration.
// Runs every 30 seconds.
type NotificationDispatchJob struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewNotificationDispatchJob creates a new NotificationDispatchJob.
func NewNotificationDispatchJob(pool *pgxpool.Pool, logger *slog.Logger) *NotificationDispatchJob {
	return &NotificationDispatchJob{pool: pool, logger: logger}
}

func (j *NotificationDispatchJob) Name() string { return "notification_dispatch" }

func (j *NotificationDispatchJob) Run(ctx context.Context) error {
	// In production: query pending notifications, resolve delivery channels
	// from notification_preferences, render templates, send via FCM/SendGrid/Twilio.
	// For now: no-op (notification infrastructure requires external service integration).
	return nil
}
