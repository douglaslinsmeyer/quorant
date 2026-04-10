package fin_test

import (
	"testing"

	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
)

func TestTransactionType_IsValid(t *testing.T) {
	assert.True(t, fin.TxTypeAssessment.IsValid())
	assert.True(t, fin.TxTypePayment.IsValid())
	assert.True(t, fin.TxTypeFundTransfer.IsValid())
	assert.False(t, fin.TransactionType("bogus").IsValid())
}

func TestAccountingStandard_IsValid(t *testing.T) {
	assert.True(t, fin.AccountingStandardGAAP.IsValid())
	assert.True(t, fin.AccountingStandardIFRS.IsValid())
	assert.False(t, fin.AccountingStandard("bogus").IsValid())
}
