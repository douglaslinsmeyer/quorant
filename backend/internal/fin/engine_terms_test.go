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
