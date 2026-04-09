package com

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresThreadRepository implements ThreadRepository using pgxpool.
type PostgresThreadRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresThreadRepository creates a new PostgresThreadRepository.
func NewPostgresThreadRepository(pool *pgxpool.Pool) *PostgresThreadRepository {
	return &PostgresThreadRepository{pool: pool}
}

// ─── CreateThread ─────────────────────────────────────────────────────────────

// CreateThread inserts a new thread and returns the fully-populated row.
func (r *PostgresThreadRepository) CreateThread(ctx context.Context, t *Thread) (*Thread, error) {
	const q = `
		INSERT INTO threads (org_id, subject, thread_type, is_closed, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, subject, thread_type, is_closed, created_by,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		t.OrgID,
		t.Subject,
		t.ThreadType,
		t.IsClosed,
		t.CreatedBy,
	)
	result, err := scanThread(row)
	if err != nil {
		return nil, fmt.Errorf("com: thread CreateThread: %w", err)
	}
	return result, nil
}

// ─── FindThreadByID ───────────────────────────────────────────────────────────

// FindThreadByID returns the thread with the given ID, or nil if not found or soft-deleted.
func (r *PostgresThreadRepository) FindThreadByID(ctx context.Context, id uuid.UUID) (*Thread, error) {
	const q = `
		SELECT id, org_id, subject, thread_type, is_closed, created_by,
		       created_at, updated_at, deleted_at
		FROM threads
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanThread(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("com: thread FindThreadByID: %w", err)
	}
	return result, nil
}

// ─── ListThreadsByOrg ─────────────────────────────────────────────────────────

// ListThreadsByOrg returns all non-deleted threads for an org, ordered by updated_at desc.
func (r *PostgresThreadRepository) ListThreadsByOrg(ctx context.Context, orgID uuid.UUID) ([]Thread, error) {
	const q = `
		SELECT id, org_id, subject, thread_type, is_closed, created_by,
		       created_at, updated_at, deleted_at
		FROM threads
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY updated_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("com: thread ListThreadsByOrg: %w", err)
	}
	defer rows.Close()

	results := []Thread{}
	for rows.Next() {
		var t Thread
		if err := rows.Scan(
			&t.ID, &t.OrgID, &t.Subject, &t.ThreadType, &t.IsClosed, &t.CreatedBy,
			&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("com: thread ListThreadsByOrg scan: %w", err)
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: thread ListThreadsByOrg rows: %w", err)
	}
	return results, nil
}

// ─── CreateMessage ────────────────────────────────────────────────────────────

// CreateMessage inserts a new message into a thread and returns the fully-populated row.
func (r *PostgresThreadRepository) CreateMessage(ctx context.Context, m *Message) (*Message, error) {
	const q = `
		INSERT INTO messages (thread_id, sender_id, body, attachment_ids)
		VALUES ($1, $2, $3, $4)
		RETURNING id, thread_id, sender_id, body, attachment_ids,
		          edited_at, created_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		m.ThreadID,
		m.SenderID,
		m.Body,
		m.AttachmentIDs,
	)
	result, err := scanMessage(row)
	if err != nil {
		return nil, fmt.Errorf("com: thread CreateMessage: %w", err)
	}
	return result, nil
}

// ─── ListMessagesByThread ─────────────────────────────────────────────────────

// ListMessagesByThread returns all non-deleted messages for the given thread, ordered by created_at asc.
func (r *PostgresThreadRepository) ListMessagesByThread(ctx context.Context, threadID uuid.UUID) ([]Message, error) {
	const q = `
		SELECT id, thread_id, sender_id, body, attachment_ids,
		       edited_at, created_at, deleted_at
		FROM messages
		WHERE thread_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, threadID)
	if err != nil {
		return nil, fmt.Errorf("com: thread ListMessagesByThread: %w", err)
	}
	defer rows.Close()

	results := []Message{}
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, fmt.Errorf("com: thread ListMessagesByThread scan: %w", err)
		}
		results = append(results, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: thread ListMessagesByThread rows: %w", err)
	}
	return results, nil
}

// ─── UpdateMessage ────────────────────────────────────────────────────────────

// UpdateMessage persists body and attachment changes to an existing message.
func (r *PostgresThreadRepository) UpdateMessage(ctx context.Context, m *Message) (*Message, error) {
	const q = `
		UPDATE messages SET
			body           = $1,
			attachment_ids = $2,
			edited_at      = now()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, thread_id, sender_id, body, attachment_ids,
		          edited_at, created_at, deleted_at`

	row := r.pool.QueryRow(ctx, q, m.Body, m.AttachmentIDs, m.ID)
	result, err := scanMessage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("com: thread UpdateMessage: %s not found or already deleted", m.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("com: thread UpdateMessage: %w", err)
	}
	return result, nil
}

// ─── SoftDeleteMessage ────────────────────────────────────────────────────────

// SoftDeleteMessage marks a message as deleted without removing the row.
func (r *PostgresThreadRepository) SoftDeleteMessage(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE messages SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("com: thread SoftDeleteMessage: %w", err)
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func scanThread(row pgx.Row) (*Thread, error) {
	var t Thread
	err := row.Scan(
		&t.ID, &t.OrgID, &t.Subject, &t.ThreadType, &t.IsClosed, &t.CreatedBy,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanMessage(row pgx.Row) (*Message, error) {
	var m Message
	err := row.Scan(
		&m.ID, &m.ThreadID, &m.SenderID, &m.Body, &m.AttachmentIDs,
		&m.EditedAt, &m.CreatedAt, &m.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if m.AttachmentIDs == nil {
		m.AttachmentIDs = []uuid.UUID{}
	}
	return &m, nil
}

func scanMessageRow(rows pgx.Rows) (*Message, error) {
	var m Message
	err := rows.Scan(
		&m.ID, &m.ThreadID, &m.SenderID, &m.Body, &m.AttachmentIDs,
		&m.EditedAt, &m.CreatedAt, &m.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if m.AttachmentIDs == nil {
		m.AttachmentIDs = []uuid.UUID{}
	}
	return &m, nil
}
