package ai

import (
	"context"

	"github.com/google/uuid"
)

// JurisdictionRuleRepository persists jurisdiction rules.
type JurisdictionRuleRepository interface {
	Create(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error)
	Update(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error)
	FindByID(ctx context.Context, id uuid.UUID) (*JurisdictionRule, error)
	GetActiveRule(ctx context.Context, jurisdiction, category, key string) (*JurisdictionRule, error)
	ListActiveRules(ctx context.Context, jurisdiction, category string) ([]JurisdictionRule, error)
	ListActiveRulesByJurisdiction(ctx context.Context, jurisdiction string) ([]JurisdictionRule, error)
	ListAllRules(ctx context.Context, jurisdiction string, limit int, afterID *uuid.UUID) ([]JurisdictionRule, bool, error)
	ListUpcomingRules(ctx context.Context, withinDays int) ([]JurisdictionRule, error)
}
