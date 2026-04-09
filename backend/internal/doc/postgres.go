package doc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDocRepository implements DocRepository using a pgxpool.
type PostgresDocRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresDocRepository creates a new PostgresDocRepository backed by pool.
func NewPostgresDocRepository(pool *pgxpool.Pool) *PostgresDocRepository {
	return &PostgresDocRepository{pool: pool}
}

// ─── Documents ────────────────────────────────────────────────────────────────

// Create inserts a new document. It generates a storage_key of the form
// {org_id}/{new_uuid}/{file_name}, sets version_number=1, is_current=true.
func (r *PostgresDocRepository) Create(ctx context.Context, d *Document) (*Document, error) {
	docID := uuid.New()
	storageKey := fmt.Sprintf("%s/%s/%s", d.OrgID, docID, d.FileName)

	metaJSON, err := marshalDocMetadata(d.Metadata)
	if err != nil {
		return nil, fmt.Errorf("doc: Create marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO documents (
			id, org_id, category_id, uploaded_by,
			title, file_name, content_type, size_bytes,
			storage_key, visibility, version_number, parent_doc_id,
			is_current, metadata
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, 1, NULL,
			TRUE, $11
		)
		RETURNING id, org_id, category_id, uploaded_by,
		          title, file_name, content_type, size_bytes,
		          storage_key, visibility, version_number, parent_doc_id,
		          is_current, metadata, created_at, updated_at, deleted_at`

	visibility := d.Visibility
	if visibility == "" {
		visibility = "members"
	}

	row := r.pool.QueryRow(ctx, q,
		docID,
		d.OrgID,
		d.CategoryID,
		d.UploadedBy,
		d.Title,
		d.FileName,
		d.ContentType,
		d.SizeBytes,
		storageKey,
		visibility,
		metaJSON,
	)
	result, err := scanDocument(row)
	if err != nil {
		return nil, fmt.Errorf("doc: Create: %w", err)
	}
	return result, nil
}

// FindByID returns the document with the given ID, or nil if not found or soft-deleted.
func (r *PostgresDocRepository) FindByID(ctx context.Context, id uuid.UUID) (*Document, error) {
	const q = `
		SELECT id, org_id, category_id, uploaded_by,
		       title, file_name, content_type, size_bytes,
		       storage_key, visibility, version_number, parent_doc_id,
		       is_current, metadata, created_at, updated_at, deleted_at
		FROM documents
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanDocument(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("doc: FindByID: %w", err)
	}
	return result, nil
}

// ListByOrg returns all current, non-deleted documents for the given org, ordered by title.
func (r *PostgresDocRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Document, error) {
	const q = `
		SELECT id, org_id, category_id, uploaded_by,
		       title, file_name, content_type, size_bytes,
		       storage_key, visibility, version_number, parent_doc_id,
		       is_current, metadata, created_at, updated_at, deleted_at
		FROM documents
		WHERE org_id = $1 AND is_current = TRUE AND deleted_at IS NULL
		ORDER BY title`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("doc: ListByOrg: %w", err)
	}
	defer rows.Close()

	return collectDocuments(rows, "ListByOrg")
}

// Update persists changes to an existing document and returns the updated row.
func (r *PostgresDocRepository) Update(ctx context.Context, d *Document) (*Document, error) {
	metaJSON, err := marshalDocMetadata(d.Metadata)
	if err != nil {
		return nil, fmt.Errorf("doc: Update marshal metadata: %w", err)
	}

	const q = `
		UPDATE documents SET
			title        = $1,
			category_id  = $2,
			visibility   = $3,
			metadata     = $4,
			updated_at   = now()
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING id, org_id, category_id, uploaded_by,
		          title, file_name, content_type, size_bytes,
		          storage_key, visibility, version_number, parent_doc_id,
		          is_current, metadata, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		d.Title,
		d.CategoryID,
		d.Visibility,
		metaJSON,
		d.ID,
	)
	result, err := scanDocument(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("doc: Update: %s not found or already deleted", d.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("doc: Update: %w", err)
	}
	return result, nil
}

// SoftDelete marks a document as deleted without removing the row.
func (r *PostgresDocRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE documents SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("doc: SoftDelete: %w", err)
	}
	return nil
}

// ─── Versioning ───────────────────────────────────────────────────────────────

// CreateVersion creates a new version in a transaction:
// sets old doc is_current=false, inserts new doc with parent_doc_id and version_number+1.
func (r *PostgresDocRepository) CreateVersion(ctx context.Context, parentDocID uuid.UUID, d *Document) (*Document, error) {
	metaJSON, err := marshalDocMetadata(d.Metadata)
	if err != nil {
		return nil, fmt.Errorf("doc: CreateVersion marshal metadata: %w", err)
	}

	var result *Document

	err = pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// Fetch the current version's version_number and resolve the root doc ID.
		// The root is the document without a parent; COALESCE(parent_doc_id, id) gives root.
		var currentVersionNumber int
		var rootDocID uuid.UUID
		err := tx.QueryRow(ctx,
			`SELECT version_number, COALESCE(parent_doc_id, id) FROM documents WHERE id = $1 AND deleted_at IS NULL`,
			parentDocID,
		).Scan(&currentVersionNumber, &rootDocID)
		if err != nil {
			return fmt.Errorf("fetch parent version_number: %w", err)
		}

		// Find the maximum version_number in the chain to handle non-linear branching.
		var maxVersionNumber int
		err = tx.QueryRow(ctx,
			`SELECT MAX(version_number) FROM documents WHERE COALESCE(parent_doc_id, id) = $1`,
			rootDocID,
		).Scan(&maxVersionNumber)
		if err != nil {
			return fmt.Errorf("fetch max version_number: %w", err)
		}

		// Mark the current doc as no longer current.
		_, err = tx.Exec(ctx,
			`UPDATE documents SET is_current = FALSE WHERE id = $1`,
			parentDocID,
		)
		if err != nil {
			return fmt.Errorf("mark parent not current: %w", err)
		}

		// Insert the new version. parent_doc_id always points to the chain root.
		newDocID := uuid.New()
		storageKey := fmt.Sprintf("%s/%s/%s", d.OrgID, newDocID, d.FileName)
		visibility := d.Visibility
		if visibility == "" {
			visibility = "members"
		}

		const insertQ = `
			INSERT INTO documents (
				id, org_id, category_id, uploaded_by,
				title, file_name, content_type, size_bytes,
				storage_key, visibility, version_number, parent_doc_id,
				is_current, metadata
			) VALUES (
				$1, $2, $3, $4,
				$5, $6, $7, $8,
				$9, $10, $11, $12,
				TRUE, $13
			)
			RETURNING id, org_id, category_id, uploaded_by,
			          title, file_name, content_type, size_bytes,
			          storage_key, visibility, version_number, parent_doc_id,
			          is_current, metadata, created_at, updated_at, deleted_at`

		row := tx.QueryRow(ctx, insertQ,
			newDocID,
			d.OrgID,
			d.CategoryID,
			d.UploadedBy,
			d.Title,
			d.FileName,
			d.ContentType,
			d.SizeBytes,
			storageKey,
			visibility,
			maxVersionNumber+1,
			rootDocID,
			metaJSON,
		)

		var scanErr error
		result, scanErr = scanDocument(row)
		return scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("doc: CreateVersion: %w", err)
	}
	return result, nil
}

// ListVersions returns all versions of a document chain sorted by version_number DESC.
// It accepts any document ID in the chain and resolves to the root.
func (r *PostgresDocRepository) ListVersions(ctx context.Context, docID uuid.UUID) ([]Document, error) {
	const q = `
		WITH root AS (
			SELECT COALESCE(parent_doc_id, id) AS root_id
			FROM documents
			WHERE id = $1
		)
		SELECT d.id, d.org_id, d.category_id, d.uploaded_by,
		       d.title, d.file_name, d.content_type, d.size_bytes,
		       d.storage_key, d.visibility, d.version_number, d.parent_doc_id,
		       d.is_current, d.metadata, d.created_at, d.updated_at, d.deleted_at
		FROM documents d, root
		WHERE COALESCE(d.parent_doc_id, d.id) = root.root_id
		ORDER BY d.version_number DESC`

	rows, err := r.pool.Query(ctx, q, docID)
	if err != nil {
		return nil, fmt.Errorf("doc: ListVersions: %w", err)
	}
	defer rows.Close()

	return collectDocuments(rows, "ListVersions")
}

// ─── Categories ───────────────────────────────────────────────────────────────

// CreateCategory inserts a new document category and returns the persisted record.
func (r *PostgresDocRepository) CreateCategory(ctx context.Context, cat *DocumentCategory) (*DocumentCategory, error) {
	const q = `
		INSERT INTO document_categories (org_id, name, parent_id, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, name, parent_id, sort_order, created_at`

	row := r.pool.QueryRow(ctx, q,
		cat.OrgID,
		cat.Name,
		cat.ParentID,
		cat.SortOrder,
	)
	result, err := scanCategory(row)
	if err != nil {
		return nil, fmt.Errorf("doc: CreateCategory: %w", err)
	}
	return result, nil
}

// ListCategoriesByOrg returns all categories for the given org, ordered by sort_order, name.
func (r *PostgresDocRepository) ListCategoriesByOrg(ctx context.Context, orgID uuid.UUID) ([]DocumentCategory, error) {
	const q = `
		SELECT id, org_id, name, parent_id, sort_order, created_at
		FROM document_categories
		WHERE org_id = $1
		ORDER BY sort_order, name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("doc: ListCategoriesByOrg: %w", err)
	}
	defer rows.Close()

	results := []DocumentCategory{}
	for rows.Next() {
		cat, err := scanCategoryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("doc: ListCategoriesByOrg scan: %w", err)
		}
		results = append(results, *cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("doc: ListCategoriesByOrg rows: %w", err)
	}
	return results, nil
}

// UpdateCategory persists changes to a category and returns the updated record.
func (r *PostgresDocRepository) UpdateCategory(ctx context.Context, cat *DocumentCategory) (*DocumentCategory, error) {
	const q = `
		UPDATE document_categories SET
			name       = $1,
			parent_id  = $2,
			sort_order = $3
		WHERE id = $4
		RETURNING id, org_id, name, parent_id, sort_order, created_at`

	row := r.pool.QueryRow(ctx, q,
		cat.Name,
		cat.ParentID,
		cat.SortOrder,
		cat.ID,
	)
	result, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("doc: UpdateCategory: %s not found", cat.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("doc: UpdateCategory: %w", err)
	}
	return result, nil
}

// DeleteCategory hard-deletes a category by ID.
func (r *PostgresDocRepository) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM document_categories WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("doc: DeleteCategory: %w", err)
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func marshalDocMetadata(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func scanDocument(row pgx.Row) (*Document, error) {
	var d Document
	var metaRaw []byte
	err := row.Scan(
		&d.ID,
		&d.OrgID,
		&d.CategoryID,
		&d.UploadedBy,
		&d.Title,
		&d.FileName,
		&d.ContentType,
		&d.SizeBytes,
		&d.StorageKey,
		&d.Visibility,
		&d.VersionNumber,
		&d.ParentDocID,
		&d.IsCurrent,
		&metaRaw,
		&d.CreatedAt,
		&d.UpdatedAt,
		&d.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &d.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	return &d, nil
}

func collectDocuments(rows pgx.Rows, op string) ([]Document, error) {
	results := []Document{}
	for rows.Next() {
		var d Document
		var metaRaw []byte
		if err := rows.Scan(
			&d.ID,
			&d.OrgID,
			&d.CategoryID,
			&d.UploadedBy,
			&d.Title,
			&d.FileName,
			&d.ContentType,
			&d.SizeBytes,
			&d.StorageKey,
			&d.Visibility,
			&d.VersionNumber,
			&d.ParentDocID,
			&d.IsCurrent,
			&metaRaw,
			&d.CreatedAt,
			&d.UpdatedAt,
			&d.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("doc: %s scan: %w", op, err)
		}
		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &d.Metadata); err != nil {
				return nil, fmt.Errorf("doc: %s unmarshal metadata: %w", op, err)
			}
		}
		if d.Metadata == nil {
			d.Metadata = map[string]any{}
		}
		results = append(results, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("doc: %s rows: %w", op, err)
	}
	return results, nil
}

func scanCategory(row pgx.Row) (*DocumentCategory, error) {
	var c DocumentCategory
	err := row.Scan(
		&c.ID,
		&c.OrgID,
		&c.Name,
		&c.ParentID,
		&c.SortOrder,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func scanCategoryRow(rows pgx.Rows) (*DocumentCategory, error) {
	var c DocumentCategory
	err := rows.Scan(
		&c.ID,
		&c.OrgID,
		&c.Name,
		&c.ParentID,
		&c.SortOrder,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
