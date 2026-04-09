package ai

// AIConfig holds per-org AI autonomy thresholds.
// Stored as JSON in organizations.settings under the key "ai_config".
type AIConfig struct {
	AutoApplyThreshold        float64  `json:"auto_apply_threshold"`        // default 0.95
	SuggestThreshold          float64  `json:"suggest_threshold"`           // default 0.80
	EscalateBelowSuggest      bool     `json:"escalate_below_suggest"`      // default true
	HighStakesActions         []string `json:"high_stakes_actions"`         // e.g., ["lien_filing", "account_suspension"]
	HomeownerQueryAutoRespond bool     `json:"homeowner_query_auto_respond"` // default true
	HomeownerQueryDisclaimer  bool     `json:"homeowner_query_disclaimer"`   // default true
}

// DefaultAIConfig returns the default configuration.
func DefaultAIConfig() AIConfig {
	return AIConfig{
		AutoApplyThreshold:        0.95,
		SuggestThreshold:          0.80,
		EscalateBelowSuggest:      true,
		HighStakesActions:         []string{"lien_filing", "account_suspension", "fine_over_50000", "membership_termination"},
		HomeownerQueryAutoRespond: true,
		HomeownerQueryDisclaimer:  true,
	}
}
