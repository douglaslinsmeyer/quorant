package fin

import (
	"context"
	"fmt"
)

// GaapEngine implements AccountingEngine for US GAAP.
type GaapEngine struct{}

// NewGaapEngine returns a new GAAP accounting engine.
func NewGaapEngine() *GaapEngine {
	return &GaapEngine{}
}

var _ AccountingEngine = (*GaapEngine)(nil)

func (e *GaapEngine) Standard() AccountingStandard {
	return AccountingStandardGAAP
}

func (e *GaapEngine) JournalLines(ctx context.Context, gl *GLService, tx FinancialTransaction) ([]GLJournalLine, error) {
	switch tx.Type {
	case TxTypeAssessment:
		return e.assessmentLines(ctx, gl, tx)
	default:
		return nil, fmt.Errorf("gaap: unsupported transaction type %q", tx.Type)
	}
}

func (e *GaapEngine) ChartOfAccounts() []GLAccountSeed {
	return gaapChartOfAccounts
}

func (e *GaapEngine) assessmentLines(ctx context.Context, gl *GLService, tx FinancialTransaction) ([]GLJournalLine, error) {
	ar, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup account 1100: %w", err)
	}
	if ar == nil {
		return nil, fmt.Errorf("gaap: account 1100 (AR-Assessments) not found for org %s", tx.OrgID)
	}

	revenue, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, 4010)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup account 4010: %w", err)
	}
	if revenue == nil {
		return nil, fmt.Errorf("gaap: account 4010 (Assessment Revenue) not found for org %s", tx.OrgID)
	}

	return []GLJournalLine{
		{AccountID: ar.ID, DebitCents: tx.AmountCents, CreditCents: 0},
		{AccountID: revenue.ID, DebitCents: 0, CreditCents: tx.AmountCents},
	}, nil
}

var gaapChartOfAccounts = []GLAccountSeed{}
