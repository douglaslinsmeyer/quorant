package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBillingRepository implements BillingRepository using a pgxpool.
type PostgresBillingRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresBillingRepository creates a new PostgresBillingRepository.
func NewPostgresBillingRepository(pool *pgxpool.Pool) *PostgresBillingRepository {
	return &PostgresBillingRepository{pool: pool}
}

// Pool returns the underlying pgxpool.Pool for test helpers.
func (r *PostgresBillingRepository) Pool() *pgxpool.Pool { return r.pool }

// ─── Accounts ─────────────────────────────────────────────────────────────────

func (r *PostgresBillingRepository) CreateAccount(ctx context.Context, a *BillingAccount) (*BillingAccount, error) {
	const q = `
		INSERT INTO billing_accounts (org_id, stripe_customer_id, billing_email, billing_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, stripe_customer_id, billing_email, billing_name, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q, a.OrgID, a.StripeCustomerID, a.BillingEmail, a.BillingName)
	result, err := scanAccount(row)
	if err != nil {
		return nil, fmt.Errorf("billing: CreateAccount: %w", err)
	}
	return result, nil
}

func (r *PostgresBillingRepository) FindAccountByOrg(ctx context.Context, orgID uuid.UUID) (*BillingAccount, error) {
	const q = `
		SELECT id, org_id, stripe_customer_id, billing_email, billing_name, created_at, updated_at
		FROM billing_accounts WHERE org_id = $1`

	row := r.pool.QueryRow(ctx, q, orgID)
	result, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("billing: FindAccountByOrg: %w", err)
	}
	return result, nil
}

func (r *PostgresBillingRepository) UpdateAccount(ctx context.Context, a *BillingAccount) (*BillingAccount, error) {
	const q = `
		UPDATE billing_accounts SET
			stripe_customer_id = $1,
			billing_email      = $2,
			billing_name       = $3,
			updated_at         = now()
		WHERE id = $4
		RETURNING id, org_id, stripe_customer_id, billing_email, billing_name, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q, a.StripeCustomerID, a.BillingEmail, a.BillingName, a.ID)
	result, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("billing: UpdateAccount: account %s not found", a.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("billing: UpdateAccount: %w", err)
	}
	return result, nil
}

// ─── Invoices ─────────────────────────────────────────────────────────────────

func (r *PostgresBillingRepository) CreateInvoice(ctx context.Context, inv *Invoice) (*Invoice, error) {
	const q = `
		INSERT INTO invoices
			(billing_account_id, org_id, stripe_invoice_id, status, subtotal_cents, tax_cents, total_cents, period_start, period_end, due_date, paid_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, billing_account_id, org_id, stripe_invoice_id, status,
		          subtotal_cents, tax_cents, total_cents, period_start, period_end,
		          due_date, paid_at, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		inv.BillingAccountID, inv.OrgID, inv.StripeInvoiceID,
		inv.Status, inv.SubtotalCents, inv.TaxCents, inv.TotalCents,
		inv.PeriodStart, inv.PeriodEnd, inv.DueDate, inv.PaidAt,
	)
	result, err := scanInvoice(row)
	if err != nil {
		return nil, fmt.Errorf("billing: CreateInvoice: %w", err)
	}
	return result, nil
}

func (r *PostgresBillingRepository) FindInvoiceByID(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	const q = `
		SELECT id, billing_account_id, org_id, stripe_invoice_id, status,
		       subtotal_cents, tax_cents, total_cents, period_start, period_end,
		       due_date, paid_at, created_at, updated_at
		FROM invoices WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("billing: FindInvoiceByID: %w", err)
	}
	return result, nil
}

func (r *PostgresBillingRepository) ListInvoicesByOrg(ctx context.Context, orgID uuid.UUID) ([]Invoice, error) {
	const q = `
		SELECT id, billing_account_id, org_id, stripe_invoice_id, status,
		       subtotal_cents, tax_cents, total_cents, period_start, period_end,
		       due_date, paid_at, created_at, updated_at
		FROM invoices WHERE org_id = $1 ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("billing: ListInvoicesByOrg: %w", err)
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		inv, err := scanInvoiceRow(rows)
		if err != nil {
			return nil, fmt.Errorf("billing: ListInvoicesByOrg scan: %w", err)
		}
		invoices = append(invoices, *inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("billing: ListInvoicesByOrg rows: %w", err)
	}
	return invoices, nil
}

func (r *PostgresBillingRepository) UpdateInvoice(ctx context.Context, inv *Invoice) (*Invoice, error) {
	const q = `
		UPDATE invoices SET
			stripe_invoice_id = $1,
			status            = $2,
			subtotal_cents    = $3,
			tax_cents         = $4,
			total_cents       = $5,
			due_date          = $6,
			paid_at           = $7,
			updated_at        = now()
		WHERE id = $8
		RETURNING id, billing_account_id, org_id, stripe_invoice_id, status,
		          subtotal_cents, tax_cents, total_cents, period_start, period_end,
		          due_date, paid_at, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		inv.StripeInvoiceID, inv.Status,
		inv.SubtotalCents, inv.TaxCents, inv.TotalCents,
		inv.DueDate, inv.PaidAt, inv.ID,
	)
	result, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("billing: UpdateInvoice: invoice %s not found", inv.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("billing: UpdateInvoice: %w", err)
	}
	return result, nil
}

// ─── Line Items ───────────────────────────────────────────────────────────────

func (r *PostgresBillingRepository) CreateLineItem(ctx context.Context, item *InvoiceLineItem) (*InvoiceLineItem, error) {
	metaJSON, err := marshalMeta(item.Metadata)
	if err != nil {
		return nil, fmt.Errorf("billing: CreateLineItem marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO invoice_line_items
			(invoice_id, description, quantity, unit_price_cents, total_cents, line_type, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, invoice_id, description, quantity, unit_price_cents, total_cents, line_type, metadata, created_at`

	row := r.pool.QueryRow(ctx, q,
		item.InvoiceID, item.Description, item.Quantity,
		item.UnitPriceCents, item.TotalCents, item.LineType, metaJSON,
	)
	result, err := scanLineItem(row)
	if err != nil {
		return nil, fmt.Errorf("billing: CreateLineItem: %w", err)
	}
	return result, nil
}

func (r *PostgresBillingRepository) ListLineItemsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]InvoiceLineItem, error) {
	const q = `
		SELECT id, invoice_id, description, quantity, unit_price_cents, total_cents, line_type, metadata, created_at
		FROM invoice_line_items WHERE invoice_id = $1 ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("billing: ListLineItemsByInvoice: %w", err)
	}
	defer rows.Close()

	var items []InvoiceLineItem
	for rows.Next() {
		item, err := scanLineItemRow(rows)
		if err != nil {
			return nil, fmt.Errorf("billing: ListLineItemsByInvoice scan: %w", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("billing: ListLineItemsByInvoice rows: %w", err)
	}
	return items, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func marshalMeta(v map[string]any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
}

func unmarshalMeta(raw []byte, dst *map[string]any) error {
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil {
			return fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if *dst == nil {
		*dst = map[string]any{}
	}
	return nil
}

func scanAccount(row pgx.Row) (*BillingAccount, error) {
	var a BillingAccount
	err := row.Scan(
		&a.ID, &a.OrgID, &a.StripeCustomerID,
		&a.BillingEmail, &a.BillingName,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanInvoice(row pgx.Row) (*Invoice, error) {
	var inv Invoice
	err := row.Scan(
		&inv.ID, &inv.BillingAccountID, &inv.OrgID,
		&inv.StripeInvoiceID, &inv.Status,
		&inv.SubtotalCents, &inv.TaxCents, &inv.TotalCents,
		&inv.PeriodStart, &inv.PeriodEnd,
		&inv.DueDate, &inv.PaidAt,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func scanInvoiceRow(rows pgx.Rows) (*Invoice, error) {
	var inv Invoice
	err := rows.Scan(
		&inv.ID, &inv.BillingAccountID, &inv.OrgID,
		&inv.StripeInvoiceID, &inv.Status,
		&inv.SubtotalCents, &inv.TaxCents, &inv.TotalCents,
		&inv.PeriodStart, &inv.PeriodEnd,
		&inv.DueDate, &inv.PaidAt,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func scanLineItem(row pgx.Row) (*InvoiceLineItem, error) {
	var item InvoiceLineItem
	var metaRaw []byte
	err := row.Scan(
		&item.ID, &item.InvoiceID, &item.Description,
		&item.Quantity, &item.UnitPriceCents, &item.TotalCents,
		&item.LineType, &metaRaw, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := unmarshalMeta(metaRaw, &item.Metadata); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanLineItemRow(rows pgx.Rows) (*InvoiceLineItem, error) {
	var item InvoiceLineItem
	var metaRaw []byte
	err := rows.Scan(
		&item.ID, &item.InvoiceID, &item.Description,
		&item.Quantity, &item.UnitPriceCents, &item.TotalCents,
		&item.LineType, &metaRaw, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := unmarshalMeta(metaRaw, &item.Metadata); err != nil {
		return nil, err
	}
	return &item, nil
}
