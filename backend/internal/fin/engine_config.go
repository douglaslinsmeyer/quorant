package fin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EngineConfig holds per-org accounting configuration.
type EngineConfig struct {
	RecognitionBasis       RecognitionBasis
	FiscalYearStart        time.Month
	AvailabilityPeriodDays int
}

// EngineBuilder constructs an engine for a given config.
type EngineBuilder func(config EngineConfig) AccountingEngine

// EngineFactory resolves an org to its configured engine instance.
type EngineFactory struct {
	builders   map[AccountingStandard]EngineBuilder
	configRepo OrgAccountingConfigRepository
}

func NewEngineFactory(builders map[AccountingStandard]EngineBuilder, configRepo OrgAccountingConfigRepository) *EngineFactory {
	return &EngineFactory{builders: builders, configRepo: configRepo}
}

func (f *EngineFactory) ForOrg(ctx context.Context, orgID uuid.UUID) (AccountingEngine, error) {
	return f.ForOrgAtDate(ctx, orgID, time.Now())
}

func (f *EngineFactory) ForOrgAtDate(ctx context.Context, orgID uuid.UUID, date time.Time) (AccountingEngine, error) {
	cfg, err := f.configRepo.GetEffectiveConfig(ctx, orgID, date)
	if err != nil {
		return nil, fmt.Errorf("engine factory: get config for org %s at %s: %w", orgID, date.Format("2006-01-02"), err)
	}
	builder, ok := f.builders[cfg.Standard]
	if !ok {
		return nil, fmt.Errorf("engine factory: unsupported accounting standard %q", cfg.Standard)
	}
	return builder(EngineConfig{
		RecognitionBasis:       cfg.RecognitionBasis,
		FiscalYearStart:        cfg.FiscalYearStart,
		AvailabilityPeriodDays: cfg.AvailabilityPeriodDays,
	}), nil
}

// OrgAccountingConfig is an effective-dated, versioned accounting configuration per org.
type OrgAccountingConfig struct {
	ID                     uuid.UUID          `json:"id"`
	OrgID                  uuid.UUID          `json:"org_id"`
	Standard               AccountingStandard `json:"standard"`
	RecognitionBasis       RecognitionBasis   `json:"recognition_basis"`
	FiscalYearStart        time.Month         `json:"fiscal_year_start"`
	AvailabilityPeriodDays int                `json:"availability_period_days"`
	EffectiveDate          time.Time          `json:"effective_date"`
	CreatedAt              time.Time          `json:"created_at"`
	CreatedBy              uuid.UUID          `json:"created_by"`
}

// OrgAccountingConfigRepository defines persistence for org accounting configs.
type OrgAccountingConfigRepository interface {
	CreateConfig(ctx context.Context, cfg *OrgAccountingConfig) (*OrgAccountingConfig, error)
	GetEffectiveConfig(ctx context.Context, orgID uuid.UUID, asOfDate time.Time) (*OrgAccountingConfig, error)
	ListConfigsByOrg(ctx context.Context, orgID uuid.UUID) ([]OrgAccountingConfig, error)
}
