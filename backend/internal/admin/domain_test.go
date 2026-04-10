package admin_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── FeatureFlag JSON serialization ─────────────────────────────────────────

func TestFeatureFlag_JSONSerialization(t *testing.T) {
	desc := "Controls new UI"
	flag := admin.FeatureFlag{
		ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Key:         "new_ui",
		Description: &desc,
		Enabled:     true,
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(flag)
	require.NoError(t, err)

	var got admin.FeatureFlag
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, flag.ID, got.ID)
	assert.Equal(t, flag.Key, got.Key)
	assert.Equal(t, flag.Enabled, got.Enabled)
	require.NotNil(t, got.Description)
	assert.Equal(t, *flag.Description, *got.Description)
}

func TestFeatureFlag_JSONSerialization_NilDescription(t *testing.T) {
	flag := admin.FeatureFlag{
		ID:      uuid.New(),
		Key:     "no_desc",
		Enabled: false,
	}

	data, err := json.Marshal(flag)
	require.NoError(t, err)

	// description should be omitted when nil
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	_, hasDesc := raw["description"]
	assert.False(t, hasDesc, "description should be omitted when nil")
}

// ─── CreateFeatureFlagRequest validation ────────────────────────────────────

func TestCreateFeatureFlagRequest_Validate_ValidKey(t *testing.T) {
	req := admin.CreateFeatureFlagRequest{Key: "my_feature"}
	assert.NoError(t, req.Validate())
}

func TestCreateFeatureFlagRequest_Validate_MissingKey(t *testing.T) {
	req := admin.CreateFeatureFlagRequest{}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation.required")
}

// ─── UpdateFeatureFlagRequest validation ────────────────────────────────────

func TestUpdateFeatureFlagRequest_Validate_WithEnabled(t *testing.T) {
	enabled := true
	req := admin.UpdateFeatureFlagRequest{Enabled: &enabled}
	assert.NoError(t, req.Validate())
}

func TestUpdateFeatureFlagRequest_Validate_WithDescription(t *testing.T) {
	desc := "updated desc"
	req := admin.UpdateFeatureFlagRequest{Description: &desc}
	assert.NoError(t, req.Validate())
}

func TestUpdateFeatureFlagRequest_Validate_NoFields(t *testing.T) {
	req := admin.UpdateFeatureFlagRequest{}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation.at_least_one")
}

// ─── SetFlagOverrideRequest validation ──────────────────────────────────────

func TestSetFlagOverrideRequest_Validate_Valid(t *testing.T) {
	req := admin.SetFlagOverrideRequest{OrgID: uuid.New().String(), Enabled: true}
	assert.NoError(t, req.Validate())
}

func TestSetFlagOverrideRequest_Validate_MissingOrgID(t *testing.T) {
	req := admin.SetFlagOverrideRequest{Enabled: true}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation.required")
}
