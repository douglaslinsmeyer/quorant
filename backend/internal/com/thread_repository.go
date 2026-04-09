package com

import (
	"context"

	"github.com/google/uuid"
)

// ThreadRepository persists and retrieves threads and their messages.
type ThreadRepository interface {
	CreateThread(ctx context.Context, t *Thread) (*Thread, error)
	FindThreadByID(ctx context.Context, id uuid.UUID) (*Thread, error)
	ListThreadsByOrg(ctx context.Context, orgID uuid.UUID) ([]Thread, error)

	CreateMessage(ctx context.Context, m *Message) (*Message, error)
	ListMessagesByThread(ctx context.Context, threadID uuid.UUID) ([]Message, error)
	UpdateMessage(ctx context.Context, m *Message) (*Message, error)
	SoftDeleteMessage(ctx context.Context, id uuid.UUID) error
}
