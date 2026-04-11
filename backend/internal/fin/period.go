package fin

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PeriodStatus controls what transactions are allowed in an accounting period.
type PeriodStatus string

const (
	PeriodStatusOpen       PeriodStatus = "open"
	PeriodStatusSoftClosed PeriodStatus = "soft_closed"
	PeriodStatusClosed     PeriodStatus = "closed"
)

// AccountingPeriod represents a single accounting period within a fiscal year.
type AccountingPeriod struct {
	ID           uuid.UUID    `json:"id"`
	OrgID        uuid.UUID    `json:"org_id"`
	FiscalYear   int          `json:"fiscal_year"`
	PeriodNumber int          `json:"period_number"`
	StartDate    time.Time    `json:"start_date"`
	EndDate      time.Time    `json:"end_date"`
	Status       PeriodStatus `json:"status"`
	ClosedBy     *uuid.UUID   `json:"closed_by,omitempty"`
	ClosedAt     *time.Time   `json:"closed_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// AccountingPeriodRepository defines persistence for accounting periods.
type AccountingPeriodRepository interface {
	CreatePeriod(ctx context.Context, p *AccountingPeriod) (*AccountingPeriod, error)
	GetPeriodForDate(ctx context.Context, orgID uuid.UUID, date time.Time) (*AccountingPeriod, error)
	ListPeriodsByFiscalYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) ([]AccountingPeriod, error)
	UpdatePeriodStatus(ctx context.Context, id uuid.UUID, status PeriodStatus, closedBy *uuid.UUID) error
	AllPeriodsClosedForYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) (bool, error)
}
