package fin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors returned by engine methods.
var (
	ErrNotImplemented     = errors.New("accounting engine: method not implemented in this phase")
	ErrCashBasisNoPayable = errors.New("accounting engine: cash basis does not recognize payables")
	ErrClosedPeriod       = errors.New("accounting engine: cannot post to closed period")
	ErrSoftClosedPeriod   = errors.New("accounting engine: only adjusting entries allowed in soft-closed period")
)

// AccountingEngine defines the contract for accounting standard drivers.
type AccountingEngine interface {
	Standard() AccountingStandard
	ChartOfAccounts() []GLAccountSeed
	RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error)
	ValidateTransaction(ctx context.Context, tx FinancialTransaction) error
	PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error)
	PaymentTerms(ctx context.Context, pc PayableContext) (*PaymentTermsResult, error)
	PayableRecognitionDate(ctx context.Context, ec ExpenseContext) (time.Time, error)
	RevenueRecognitionDate(ctx context.Context, tx FinancialTransaction) (time.Time, error)
}

// AccountResolver provides account lookups for the engine.
type AccountResolver interface {
	FindAccountByOrgAndNumber(ctx context.Context, orgID uuid.UUID, number int) (*GLAccount, error)
}
