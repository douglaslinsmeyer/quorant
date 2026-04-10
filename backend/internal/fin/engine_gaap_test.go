package fin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGaapEngine_Standard(t *testing.T) {
	engine := fin.NewGaapEngine()
	assert.Equal(t, fin.AccountingStandardGAAP, engine.Standard())
}

func TestGaapEngine_JournalLines_Assessment(t *testing.T) {
	engine := fin.NewGaapEngine()
	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)

	orgID := uuid.New()
	ctx := context.Background()

	arAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 1100,
		Name: "AR-Assessments", AccountType: fin.GLAccountTypeAsset,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	revenueAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 4010,
		Name: "Assessment Revenue-Operating", AccountType: fin.GLAccountTypeRevenue,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	glRepo.SetAccounts(arAccount, revenueAccount)

	tx := fin.FinancialTransaction{
		Type:          fin.TxTypeAssessment,
		OrgID:         orgID,
		AmountCents:   15000,
		EffectiveDate: time.Now(),
		SourceID:      uuid.New(),
	}

	lines, err := engine.JournalLines(ctx, glService, tx)
	require.NoError(t, err)
	require.Len(t, lines, 2)

	assert.Equal(t, arAccount.ID, lines[0].AccountID)
	assert.Equal(t, int64(15000), lines[0].DebitCents)
	assert.Equal(t, int64(0), lines[0].CreditCents)

	assert.Equal(t, revenueAccount.ID, lines[1].AccountID)
	assert.Equal(t, int64(0), lines[1].DebitCents)
	assert.Equal(t, int64(15000), lines[1].CreditCents)
}

func TestGaapEngine_JournalLines_Assessment_MissingAccounts(t *testing.T) {
	engine := fin.NewGaapEngine()
	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)

	tx := fin.FinancialTransaction{
		Type:          fin.TxTypeAssessment,
		OrgID:         uuid.New(),
		AmountCents:   15000,
		EffectiveDate: time.Now(),
		SourceID:      uuid.New(),
	}

	_, err := engine.JournalLines(context.Background(), glService, tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1100")
}
