package com

import (
	"context"

	"github.com/google/uuid"
)

// NotificationRepository persists notification preferences and push tokens.
type NotificationRepository interface {
	UpsertPreference(ctx context.Context, p *NotificationPreference) (*NotificationPreference, error)
	ListPreferencesByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]NotificationPreference, error)

	CreatePushToken(ctx context.Context, t *PushToken) (*PushToken, error)
	DeletePushToken(ctx context.Context, id uuid.UUID) error
	ListPushTokensByUser(ctx context.Context, userID uuid.UUID) ([]PushToken, error)
}
