package org

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresOrgRepository implements OrgRepository using a pgxpool.
type PostgresOrgRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresOrgRepository creates a new PostgresOrgRepository backed by pool.
func NewPostgresOrgRepository(pool *pgxpool.Pool) *PostgresOrgRepository {
	return &PostgresOrgRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new organization. It generates a URL-safe slug from the org
// name, computes the ltree path (prepending the parent's path if set), and
// returns the fully-populated row.
func (r *PostgresOrgRepository) Create(ctx context.Context, org *Organization) (*Organization, error) {
	baseSlug := slugify(org.Name)
	slug, err := r.uniqueSlug(ctx, baseSlug)
	if err != nil {
		return nil, fmt.Errorf("org: Create slug: %w", err)
	}

	// Compute ltree path.
	// ltree labels cannot contain hyphens, so we replace them with underscores.
	ltreeLabel := strings.ReplaceAll(slug, "-", "_")
	path := ltreeLabel

	if org.ParentID != nil {
		parent, err := r.FindByID(ctx, *org.ParentID)
		if err != nil {
			return nil, fmt.Errorf("org: Create find parent: %w", err)
		}
		if parent == nil {
			return nil, fmt.Errorf("org: Create: parent %s not found", *org.ParentID)
		}
		path = parent.Path + "." + ltreeLabel
	}

	settingsJSON, err := json.Marshal(org.Settings)
	if err != nil {
		return nil, fmt.Errorf("org: Create marshal settings: %w", err)
	}

	const q = `
		INSERT INTO organizations (
			parent_id, type, name, slug, path,
			address_line1, address_line2, city, state, jurisdiction, zip,
			phone, email, website, logo_url,
			locale, timezone, currency_code, country,
			settings
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20
		)
		RETURNING id, parent_id, type, name, slug, path,
		          address_line1, address_line2, city, state, jurisdiction, zip,
		          phone, email, website, logo_url,
		          locale, timezone, currency_code, country,
		          settings,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		org.ParentID,
		org.Type,
		org.Name,
		slug,
		path,
		org.AddressLine1,
		org.AddressLine2,
		org.City,
		org.State,
		org.Jurisdiction,
		org.Zip,
		org.Phone,
		org.Email,
		org.Website,
		org.LogoURL,
		org.Locale,
		org.Timezone,
		org.CurrencyCode,
		org.Country,
		settingsJSON,
	)

	result, err := scanOrg(row)
	if err != nil {
		return nil, fmt.Errorf("org: Create: %w", err)
	}
	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the organization with the given ID, or nil if not found or soft-deleted.
func (r *PostgresOrgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	const q = `
		SELECT id, parent_id, type, name, slug, path,
		       address_line1, address_line2, city, state, jurisdiction, zip,
		       phone, email, website, logo_url,
		       locale, timezone, currency_code, country,
		       settings,
		       created_at, updated_at, deleted_at
		FROM organizations
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanOrg(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("org: FindByID: %w", err)
	}
	return result, nil
}

// ─── FindBySlug ──────────────────────────────────────────────────────────────

// FindBySlug returns the organization with the given slug, or nil if not found or soft-deleted.
func (r *PostgresOrgRepository) FindBySlug(ctx context.Context, slug string) (*Organization, error) {
	const q = `
		SELECT id, parent_id, type, name, slug, path,
		       address_line1, address_line2, city, state, jurisdiction, zip,
		       phone, email, website, logo_url,
		       locale, timezone, currency_code, country,
		       settings,
		       created_at, updated_at, deleted_at
		FROM organizations
		WHERE slug = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, slug)
	result, err := scanOrg(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("org: FindBySlug: %w", err)
	}
	return result, nil
}

// ─── ListByUserAccess ────────────────────────────────────────────────────────

// ListByUserAccess returns non-deleted orgs the given user has active memberships in,
// supporting cursor-based pagination ordered by id. Fetch limit+1 rows to determine
// whether more results exist. afterID is the ID of the last item from the previous page.
func (r *PostgresOrgRepository) ListByUserAccess(ctx context.Context, userID uuid.UUID, limit int, afterID *uuid.UUID) ([]Organization, bool, error) {
	const q = `
		SELECT DISTINCT o.id, o.parent_id, o.type, o.name, o.slug, o.path,
		                o.address_line1, o.address_line2, o.city, o.state, o.jurisdiction, o.zip,
		                o.phone, o.email, o.website, o.logo_url,
		                o.locale, o.timezone, o.currency_code, o.country,
		                o.settings,
		                o.created_at, o.updated_at, o.deleted_at
		FROM organizations o
		JOIN memberships m ON m.org_id = o.id
		WHERE m.user_id = $1
		  AND m.deleted_at IS NULL
		  AND m.status = 'active'
		  AND o.deleted_at IS NULL
		  AND ($3::uuid IS NULL OR o.id > $3)
		ORDER BY o.id
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, userID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("org: ListByUserAccess: %w", err)
	}
	defer rows.Close()

	orgs, err := collectOrgs(rows, "ListByUserAccess")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(orgs) > limit
	if hasMore {
		orgs = orgs[:limit]
	}
	return orgs, hasMore, nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to an existing organization and returns the updated row.
func (r *PostgresOrgRepository) Update(ctx context.Context, org *Organization) (*Organization, error) {
	settingsJSON, err := json.Marshal(org.Settings)
	if err != nil {
		return nil, fmt.Errorf("org: Update marshal settings: %w", err)
	}

	const q = `
		UPDATE organizations SET
			name          = $1,
			address_line1 = $2,
			address_line2 = $3,
			city          = $4,
			state         = $5,
			jurisdiction  = $6,
			zip           = $7,
			phone         = $8,
			email         = $9,
			website       = $10,
			logo_url      = $11,
			locale        = $12,
			timezone      = $13,
			currency_code = $14,
			country       = $15,
			settings      = $16,
			updated_at    = now()
		WHERE id = $17 AND deleted_at IS NULL
		RETURNING id, parent_id, type, name, slug, path,
		          address_line1, address_line2, city, state, jurisdiction, zip,
		          phone, email, website, logo_url,
		          locale, timezone, currency_code, country,
		          settings,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		org.Name,
		org.AddressLine1,
		org.AddressLine2,
		org.City,
		org.State,
		org.Jurisdiction,
		org.Zip,
		org.Phone,
		org.Email,
		org.Website,
		org.LogoURL,
		org.Locale,
		org.Timezone,
		org.CurrencyCode,
		org.Country,
		settingsJSON,
		org.ID,
	)

	result, err := scanOrg(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("org: Update: org %s not found or already deleted", org.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("org: Update: %w", err)
	}
	return result, nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

// SoftDelete marks an organization as deleted without removing the row.
func (r *PostgresOrgRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE organizations SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("org: SoftDelete: %w", err)
	}
	return nil
}

// ─── ListChildren ────────────────────────────────────────────────────────────

// ListChildren returns the direct children of an organization ordered by name.
func (r *PostgresOrgRepository) ListChildren(ctx context.Context, parentID uuid.UUID) ([]Organization, error) {
	const q = `
		SELECT id, parent_id, type, name, slug, path,
		       address_line1, address_line2, city, state, jurisdiction, zip,
		       phone, email, website, logo_url,
		       locale, timezone, currency_code, country,
		       settings,
		       created_at, updated_at, deleted_at
		FROM organizations
		WHERE parent_id = $1 AND deleted_at IS NULL
		ORDER BY name`

	rows, err := r.pool.Query(ctx, q, parentID)
	if err != nil {
		return nil, fmt.Errorf("org: ListChildren: %w", err)
	}
	defer rows.Close()

	return collectOrgs(rows, "ListChildren")
}

// ─── ListByJurisdiction ──────────────────────────────────────────────────────

// ListByJurisdiction returns all non-deleted orgs in the given jurisdiction, ordered by name.
func (r *PostgresOrgRepository) ListByJurisdiction(ctx context.Context, jurisdiction string) ([]Organization, error) {
	const q = `
		SELECT id, parent_id, type, name, slug, path,
		       address_line1, address_line2, city, state, jurisdiction, zip,
		       phone, email, website, logo_url,
		       locale, timezone, currency_code, country,
		       settings,
		       created_at, updated_at, deleted_at
		FROM organizations
		WHERE jurisdiction = $1 AND deleted_at IS NULL
		ORDER BY name`

	rows, err := r.pool.Query(ctx, q, jurisdiction)
	if err != nil {
		return nil, fmt.Errorf("org: ListByJurisdiction: %w", err)
	}
	defer rows.Close()
	return collectOrgs(rows, "ListByJurisdiction")
}

// ─── ConnectManagement ───────────────────────────────────────────────────────

// ConnectManagement creates an active management relationship between a firm and an HOA.
// Returns an error if the HOA already has an active management firm.
func (r *PostgresOrgRepository) ConnectManagement(ctx context.Context, firmOrgID, hoaOrgID uuid.UUID) (*OrgManagement, error) {
	const q = `
		INSERT INTO organizations_management (firm_org_id, hoa_org_id)
		VALUES ($1, $2)
		RETURNING id, firm_org_id, hoa_org_id, started_at, ended_at, created_at`

	row := r.pool.QueryRow(ctx, q, firmOrgID, hoaOrgID)
	result, err := scanOrgManagement(row)
	if err != nil {
		return nil, fmt.Errorf("org: ConnectManagement: %w", err)
	}
	return result, nil
}

// ─── DisconnectManagement ────────────────────────────────────────────────────

// DisconnectManagement ends the active management relationship for an HOA.
func (r *PostgresOrgRepository) DisconnectManagement(ctx context.Context, hoaOrgID uuid.UUID) error {
	const q = `
		UPDATE organizations_management
		SET ended_at = now()
		WHERE hoa_org_id = $1 AND ended_at IS NULL`

	_, err := r.pool.Exec(ctx, q, hoaOrgID)
	if err != nil {
		return fmt.Errorf("org: DisconnectManagement: %w", err)
	}
	return nil
}

// ─── ListManagementHistory ───────────────────────────────────────────────────

// ListManagementHistory returns all management relationships for an HOA, ordered by started_at.
func (r *PostgresOrgRepository) ListManagementHistory(ctx context.Context, hoaOrgID uuid.UUID) ([]OrgManagement, error) {
	const q = `
		SELECT id, firm_org_id, hoa_org_id, started_at, ended_at, created_at
		FROM organizations_management
		WHERE hoa_org_id = $1
		ORDER BY started_at`

	rows, err := r.pool.Query(ctx, q, hoaOrgID)
	if err != nil {
		return nil, fmt.Errorf("org: ListManagementHistory: %w", err)
	}
	defer rows.Close()

	var results []OrgManagement
	for rows.Next() {
		var m OrgManagement
		if err := rows.Scan(&m.ID, &m.FirmOrgID, &m.HOAOrgID, &m.StartedAt, &m.EndedAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("org: ListManagementHistory scan: %w", err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("org: ListManagementHistory rows: %w", err)
	}
	return results, nil
}

// ─── FindActiveManagement ────────────────────────────────────────────────────

// FindActiveManagement returns the active management relationship for an HOA, or nil if self-managed.
func (r *PostgresOrgRepository) FindActiveManagement(ctx context.Context, hoaOrgID uuid.UUID) (*OrgManagement, error) {
	const q = `
		SELECT id, firm_org_id, hoa_org_id, started_at, ended_at, created_at
		FROM organizations_management
		WHERE hoa_org_id = $1 AND ended_at IS NULL`

	row := r.pool.QueryRow(ctx, q, hoaOrgID)
	result, err := scanOrgManagement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("org: FindActiveManagement: %w", err)
	}
	return result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// slugify converts a name into a URL-safe, lowercase, hyphen-delimited slug.
// Special characters are stripped; spaces become hyphens.
func slugify(name string) string {
	// Lowercase.
	s := strings.ToLower(name)

	// Replace whitespace sequences with a single hyphen.
	wsRe := regexp.MustCompile(`\s+`)
	s = wsRe.ReplaceAllString(s, "-")

	// Strip characters that are not alphanumeric or hyphens.
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	s = b.String()

	// Collapse multiple consecutive hyphens.
	hyphenRe := regexp.MustCompile(`-{2,}`)
	s = hyphenRe.ReplaceAllString(s, "-")

	// Trim leading/trailing hyphens.
	return strings.Trim(s, "-")
}

// uniqueSlug returns base if it is not taken, otherwise appends a random suffix.
func (r *PostgresOrgRepository) uniqueSlug(ctx context.Context, base string) (string, error) {
	candidate := base
	for {
		var exists bool
		err := r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM organizations WHERE slug = $1)`,
			candidate,
		).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		// Append a 4-character random hex suffix and retry.
		candidate = fmt.Sprintf("%s-%x", base, rand.Int31n(0xFFFF)) //nolint:gosec
	}
}

// scanOrg reads a single organization row.
func scanOrg(row pgx.Row) (*Organization, error) {
	var o Organization
	var settingsRaw []byte
	err := row.Scan(
		&o.ID,
		&o.ParentID,
		&o.Type,
		&o.Name,
		&o.Slug,
		&o.Path,
		&o.AddressLine1,
		&o.AddressLine2,
		&o.City,
		&o.State,
		&o.Jurisdiction,
		&o.Zip,
		&o.Phone,
		&o.Email,
		&o.Website,
		&o.LogoURL,
		&o.Locale,
		&o.Timezone,
		&o.CurrencyCode,
		&o.Country,
		&settingsRaw,
		&o.CreatedAt,
		&o.UpdatedAt,
		&o.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(settingsRaw) > 0 {
		if err := json.Unmarshal(settingsRaw, &o.Settings); err != nil {
			return nil, fmt.Errorf("unmarshal settings: %w", err)
		}
	}
	if o.Settings == nil {
		o.Settings = map[string]any{}
	}
	return &o, nil
}

// collectOrgs drains a pgx.Rows into a slice of Organization values.
func collectOrgs(rows pgx.Rows, op string) ([]Organization, error) {
	var orgs []Organization
	for rows.Next() {
		var settingsRaw []byte
		var o Organization
		if err := rows.Scan(
			&o.ID,
			&o.ParentID,
			&o.Type,
			&o.Name,
			&o.Slug,
			&o.Path,
			&o.AddressLine1,
			&o.AddressLine2,
			&o.City,
			&o.State,
			&o.Jurisdiction,
			&o.Zip,
			&o.Phone,
			&o.Email,
			&o.Website,
			&o.LogoURL,
			&o.Locale,
			&o.Timezone,
			&o.CurrencyCode,
			&o.Country,
			&settingsRaw,
			&o.CreatedAt,
			&o.UpdatedAt,
			&o.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("org: %s scan: %w", op, err)
		}
		if len(settingsRaw) > 0 {
			if err := json.Unmarshal(settingsRaw, &o.Settings); err != nil {
				return nil, fmt.Errorf("org: %s unmarshal settings: %w", op, err)
			}
		}
		if o.Settings == nil {
			o.Settings = map[string]any{}
		}
		orgs = append(orgs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("org: %s rows: %w", op, err)
	}
	return orgs, nil
}

// scanOrgManagement reads a single organizations_management row.
func scanOrgManagement(row pgx.Row) (*OrgManagement, error) {
	var m OrgManagement
	err := row.Scan(&m.ID, &m.FirmOrgID, &m.HOAOrgID, &m.StartedAt, &m.EndedAt, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
