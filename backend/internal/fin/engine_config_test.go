package fin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Stub config repository ──────────────────────────────────────────────────

type stubConfigRepo struct {
	configs []OrgAccountingConfig
}

func (s *stubConfigRepo) CreateConfig(_ context.Context, cfg *OrgAccountingConfig) (*OrgAccountingConfig, error) {
	cfg.ID = uuid.New()
	cfg.CreatedAt = time.Now()
	s.configs = append(s.configs, *cfg)
	return cfg, nil
}

func (s *stubConfigRepo) GetEffectiveConfig(_ context.Context, orgID uuid.UUID, asOfDate time.Time) (*OrgAccountingConfig, error) {
	var best *OrgAccountingConfig
	for i := range s.configs {
		c := &s.configs[i]
		if c.OrgID == orgID && !c.EffectiveDate.After(asOfDate) {
			if best == nil || c.EffectiveDate.After(best.EffectiveDate) {
				best = c
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no config found for org %s at %s", orgID, asOfDate.Format("2006-01-02"))
	}
	return best, nil
}

func (s *stubConfigRepo) ListConfigsByOrg(_ context.Context, orgID uuid.UUID) ([]OrgAccountingConfig, error) {
	var result []OrgAccountingConfig
	for _, c := range s.configs {
		if c.OrgID == orgID {
			result = append(result, c)
		}
	}
	return result, nil
}

// ── EngineFactory tests ─────────────────────────────────────────────────────

func TestEngineFactory_ForOrg(t *testing.T) {
	orgID := uuid.New()
	repo := &stubConfigRepo{
		configs: []OrgAccountingConfig{
			{
				OrgID:                  orgID,
				Standard:               AccountingStandardGAAP,
				RecognitionBasis:       RecognitionBasisAccrual,
				FiscalYearStart:        time.January,
				AvailabilityPeriodDays: 60,
				EffectiveDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	builders := map[AccountingStandard]EngineBuilder{
		AccountingStandardGAAP: func(config EngineConfig) AccountingEngine {
			return NewGaapEngine(nil, nil, config)
		},
	}

	factory := NewEngineFactory(builders, repo)

	engine, err := factory.ForOrg(context.Background(), orgID)
	require.NoError(t, err)
	require.NotNil(t, engine)

	assert.Equal(t, AccountingStandardGAAP, engine.Standard())

	gaap, ok := engine.(*GaapEngine)
	require.True(t, ok, "engine should be *GaapEngine")
	assert.Equal(t, RecognitionBasisAccrual, gaap.config.RecognitionBasis)
}

func TestEngineFactory_ForOrgAtDate_EffectiveDating(t *testing.T) {
	orgID := uuid.New()
	repo := &stubConfigRepo{
		configs: []OrgAccountingConfig{
			{
				OrgID:                  orgID,
				Standard:               AccountingStandardGAAP,
				RecognitionBasis:       RecognitionBasisCash,
				FiscalYearStart:        time.January,
				AvailabilityPeriodDays: 60,
				EffectiveDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				OrgID:                  orgID,
				Standard:               AccountingStandardGAAP,
				RecognitionBasis:       RecognitionBasisAccrual,
				FiscalYearStart:        time.July,
				AvailabilityPeriodDays: 90,
				EffectiveDate:          time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	builders := map[AccountingStandard]EngineBuilder{
		AccountingStandardGAAP: func(config EngineConfig) AccountingEngine {
			return NewGaapEngine(nil, nil, config)
		},
	}

	factory := NewEngineFactory(builders, repo)

	t.Run("before switch returns cash basis", func(t *testing.T) {
		engine, err := factory.ForOrgAtDate(context.Background(), orgID, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		require.NotNil(t, engine)

		gaap := engine.(*GaapEngine)
		assert.Equal(t, RecognitionBasisCash, gaap.config.RecognitionBasis)
		assert.Equal(t, time.January, gaap.config.FiscalYearStart)
		assert.Equal(t, 60, gaap.config.AvailabilityPeriodDays)
	})

	t.Run("on switch date returns accrual basis", func(t *testing.T) {
		engine, err := factory.ForOrgAtDate(context.Background(), orgID, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		require.NotNil(t, engine)

		gaap := engine.(*GaapEngine)
		assert.Equal(t, RecognitionBasisAccrual, gaap.config.RecognitionBasis)
		assert.Equal(t, time.July, gaap.config.FiscalYearStart)
		assert.Equal(t, 90, gaap.config.AvailabilityPeriodDays)
	})

	t.Run("well after switch returns accrual basis", func(t *testing.T) {
		engine, err := factory.ForOrgAtDate(context.Background(), orgID, time.Date(2027, 3, 15, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		require.NotNil(t, engine)

		gaap := engine.(*GaapEngine)
		assert.Equal(t, RecognitionBasisAccrual, gaap.config.RecognitionBasis)
	})
}

func TestEngineFactory_UnsupportedStandard(t *testing.T) {
	orgID := uuid.New()
	repo := &stubConfigRepo{
		configs: []OrgAccountingConfig{
			{
				OrgID:            orgID,
				Standard:         AccountingStandardIFRS,
				RecognitionBasis: RecognitionBasisAccrual,
				FiscalYearStart:  time.January,
				EffectiveDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	// Only register a GAAP builder.
	builders := map[AccountingStandard]EngineBuilder{
		AccountingStandardGAAP: func(config EngineConfig) AccountingEngine {
			return NewGaapEngine(nil, nil, config)
		},
	}

	factory := NewEngineFactory(builders, repo)

	engine, err := factory.ForOrg(context.Background(), orgID)
	assert.Nil(t, engine)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported accounting standard")
	assert.Contains(t, err.Error(), "ifrs")
}

func TestEngineFactory_NoConfig(t *testing.T) {
	orgID := uuid.New()
	repo := &stubConfigRepo{} // no configs

	builders := map[AccountingStandard]EngineBuilder{
		AccountingStandardGAAP: func(config EngineConfig) AccountingEngine {
			return NewGaapEngine(nil, nil, config)
		},
	}

	factory := NewEngineFactory(builders, repo)

	engine, err := factory.ForOrg(context.Background(), orgID)
	assert.Nil(t, engine)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get config for org")
}
