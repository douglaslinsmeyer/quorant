package fin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── PaymentTerms tests ──────────────────────────────────────────────

func TestPaymentTerms_Net30(t *testing.T) {
	engine := newTestGaapEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PaymentTerms(context.Background(), PayableContext{
		PayableID:   uuid.New(),
		InvoiceDate: invoiceDate,
		VendorTerms: "Net 30",
		AmountCents: 50000,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), result.DueDate)
	assert.Nil(t, result.DiscountDate)
	assert.Nil(t, result.DiscountPercent)
}

func TestPaymentTerms_DiscountTerms(t *testing.T) {
	engine := newTestGaapEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PaymentTerms(context.Background(), PayableContext{
		PayableID:   uuid.New(),
		InvoiceDate: invoiceDate,
		VendorTerms: "2/10 Net 30",
		AmountCents: 50000,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), result.DueDate)
	require.NotNil(t, result.DiscountDate)
	assert.Equal(t, time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC), *result.DiscountDate)
	require.NotNil(t, result.DiscountPercent)
	assert.Equal(t, 2.0, *result.DiscountPercent)
}

func TestPaymentTerms_Net60(t *testing.T) {
	engine := newTestGaapEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PaymentTerms(context.Background(), PayableContext{
		PayableID:   uuid.New(),
		InvoiceDate: invoiceDate,
		VendorTerms: "Net 60",
		AmountCents: 50000,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC), result.DueDate)
	assert.Nil(t, result.DiscountDate)
	assert.Nil(t, result.DiscountPercent)
}

func TestPaymentTerms_EmptyString_DefaultsNet30(t *testing.T) {
	engine := newTestGaapEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PaymentTerms(context.Background(), PayableContext{
		PayableID:   uuid.New(),
		InvoiceDate: invoiceDate,
		VendorTerms: "",
		AmountCents: 50000,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), result.DueDate)
	assert.Nil(t, result.DiscountDate)
	assert.Nil(t, result.DiscountPercent)
}

func TestPaymentTerms_CaseInsensitive(t *testing.T) {
	engine := newTestGaapEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	cases := []string{"net 30", "NET 30", "Net 30", "nEt 30"}
	expected := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	for _, terms := range cases {
		t.Run(terms, func(t *testing.T) {
			result, err := engine.PaymentTerms(context.Background(), PayableContext{
				PayableID:   uuid.New(),
				InvoiceDate: invoiceDate,
				VendorTerms: terms,
				AmountCents: 50000,
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, expected, result.DueDate)
		})
	}
}

// ── PayableRecognitionDate tests ────────────────────────────────────

func TestPayableRecognitionDate_AccrualWithServiceDate(t *testing.T) {
	engine := newTestGaapEngine() // accrual basis
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	serviceDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	result, err := engine.PayableRecognitionDate(context.Background(), ExpenseContext{
		InvoiceDate: invoiceDate,
		ServiceDate: &serviceDate,
		AmountCents: 50000,
	})

	require.NoError(t, err)
	assert.Equal(t, serviceDate, result)
}

func TestPayableRecognitionDate_AccrualWithoutServiceDate(t *testing.T) {
	engine := newTestGaapEngine() // accrual basis
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PayableRecognitionDate(context.Background(), ExpenseContext{
		InvoiceDate: invoiceDate,
		AmountCents: 50000,
	})

	require.NoError(t, err)
	assert.Equal(t, invoiceDate, result)
}

func TestPayableRecognitionDate_CashBasis(t *testing.T) {
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisCash,
		FiscalYearStart:  1,
	})

	_, err := engine.PayableRecognitionDate(context.Background(), ExpenseContext{
		InvoiceDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		AmountCents: 50000,
	})

	require.ErrorIs(t, err, ErrCashBasisNoPayable)
}
