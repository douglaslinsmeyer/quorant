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
	case TxTypePayment:
		return e.paymentLines(ctx, gl, tx)
	case TxTypeFundTransfer:
		return e.fundTransferLines(ctx, gl, tx)
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

func (e *GaapEngine) paymentLines(ctx context.Context, gl *GLService, tx FinancialTransaction) ([]GLJournalLine, error) {
	cash, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1010)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup account 1010: %w", err)
	}
	if cash == nil {
		return nil, fmt.Errorf("gaap: account 1010 (Cash-Operating) not found for org %s", tx.OrgID)
	}

	ar, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup account 1100: %w", err)
	}
	if ar == nil {
		return nil, fmt.Errorf("gaap: account 1100 (AR-Assessments) not found for org %s", tx.OrgID)
	}

	return []GLJournalLine{
		{AccountID: cash.ID, DebitCents: tx.AmountCents, CreditCents: 0},
		{AccountID: ar.ID, DebitCents: 0, CreditCents: tx.AmountCents},
	}, nil
}

func (e *GaapEngine) fundTransferLines(ctx context.Context, gl *GLService, tx FinancialTransaction) ([]GLJournalLine, error) {
	fromFundType, ok := tx.Metadata["from_fund_type"].(string)
	if !ok || fromFundType == "" {
		return nil, fmt.Errorf("gaap: fund transfer requires metadata key \"from_fund_type\"")
	}
	toFundType, ok := tx.Metadata["to_fund_type"].(string)
	if !ok || toFundType == "" {
		return nil, fmt.Errorf("gaap: fund transfer requires metadata key \"to_fund_type\"")
	}

	fromCashNum := cashAccountForFundType(FundType(fromFundType))
	toCashNum := cashAccountForFundType(FundType(toFundType))

	fromCash, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, fromCashNum)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup source cash account %d: %w", fromCashNum, err)
	}
	if fromCash == nil {
		return nil, fmt.Errorf("gaap: source cash account %d not found for org %s", fromCashNum, tx.OrgID)
	}

	toCash, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, toCashNum)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup dest cash account %d: %w", toCashNum, err)
	}
	if toCash == nil {
		return nil, fmt.Errorf("gaap: dest cash account %d not found for org %s", toCashNum, tx.OrgID)
	}

	transferOut, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, 3100)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup account 3100: %w", err)
	}
	if transferOut == nil {
		return nil, fmt.Errorf("gaap: account 3100 (Interfund Transfer Out) not found for org %s", tx.OrgID)
	}

	transferIn, err := gl.FindAccountByOrgAndNumber(ctx, tx.OrgID, 3110)
	if err != nil {
		return nil, fmt.Errorf("gaap: lookup account 3110: %w", err)
	}
	if transferIn == nil {
		return nil, fmt.Errorf("gaap: account 3110 (Interfund Transfer In) not found for org %s", tx.OrgID)
	}

	return []GLJournalLine{
		{AccountID: transferOut.ID, DebitCents: tx.AmountCents, CreditCents: 0},
		{AccountID: fromCash.ID, DebitCents: 0, CreditCents: tx.AmountCents},
		{AccountID: toCash.ID, DebitCents: tx.AmountCents, CreditCents: 0},
		{AccountID: transferIn.ID, DebitCents: 0, CreditCents: tx.AmountCents},
	}, nil
}

var gaapChartOfAccounts = []GLAccountSeed{
	// Headers
	{1000, "Assets", GLAccountTypeAsset, true, true, "", 0},
	{2000, "Liabilities", GLAccountTypeLiability, true, true, "", 0},
	{3000, "Fund Balances", GLAccountTypeEquity, true, true, "", 0},
	{4000, "Revenue", GLAccountTypeRevenue, true, true, "", 0},
	{5000, "Operating Expenses", GLAccountTypeExpense, true, true, "", 0},

	// Under 1000 Assets
	{1010, "Cash-Operating", GLAccountTypeAsset, false, true, "operating", 1000},
	{1020, "Cash-Reserve", GLAccountTypeAsset, false, true, "reserve", 1000},
	{1100, "AR-Assessments", GLAccountTypeAsset, false, true, "operating", 1000},
	{1110, "AR-Other", GLAccountTypeAsset, false, false, "operating", 1000},
	{1200, "Prepaid Expenses", GLAccountTypeAsset, false, false, "operating", 1000},

	// Under 2000 Liabilities
	{2100, "AP", GLAccountTypeLiability, false, true, "operating", 2000},
	{2200, "Prepaid Assessments", GLAccountTypeLiability, false, false, "operating", 2000},

	// Under 3000 Fund Balances
	{3010, "Operating Fund Balance", GLAccountTypeEquity, false, true, "operating", 3000},
	{3020, "Reserve Fund Balance", GLAccountTypeEquity, false, true, "reserve", 3000},
	{3100, "Interfund Transfer Out", GLAccountTypeEquity, false, true, "", 3000},
	{3110, "Interfund Transfer In", GLAccountTypeEquity, false, true, "", 3000},

	// Under 4000 Revenue
	{4010, "Assessment Revenue-Operating", GLAccountTypeRevenue, false, true, "operating", 4000},
	{4020, "Assessment Revenue-Reserve", GLAccountTypeRevenue, false, true, "reserve", 4000},
	{4100, "Late Fee Revenue", GLAccountTypeRevenue, false, true, "operating", 4000},
	{4200, "Interest Income", GLAccountTypeRevenue, false, false, "", 4000},

	// Under 5000 Operating Expenses
	{5010, "Management Fee", GLAccountTypeExpense, false, false, "operating", 5000},
	{5020, "Insurance", GLAccountTypeExpense, false, false, "operating", 5000},
	{5030, "Utilities", GLAccountTypeExpense, false, false, "operating", 5000},
	{5040, "Landscaping", GLAccountTypeExpense, false, false, "operating", 5000},
	{5050, "Maintenance and Repairs", GLAccountTypeExpense, false, false, "operating", 5000},
	{5060, "Professional Services", GLAccountTypeExpense, false, false, "operating", 5000},
}
