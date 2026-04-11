package fin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Assessment fund split policy tests ───────────────────────────────

func TestResolveAssessmentFundSplit_PolicyWithSplit(t *testing.T) {
	ruling, _ := json.Marshal(assessmentFundSplitRuling{
		Allocations: []fundSplitEntry{
			{FundType: "operating", Percent: 0.80},
			{FundType: "reserve", Percent: 0.20},
		},
	})
	registry := newRegistryWithCap(t, "assessment_fund_allocation", ruling)

	split, err := resolveAssessmentFundSplit(context.Background(), registry, uuid.New())
	require.NoError(t, err)
	require.Len(t, split, 2)
	assert.Equal(t, "operating", split[0].FundType)
	assert.Equal(t, 0.80, split[0].Percent)
	assert.Equal(t, "reserve", split[1].FundType)
	assert.Equal(t, 0.20, split[1].Percent)
}

func TestResolveAssessmentFundSplit_NoPolicy_DefaultsOperating(t *testing.T) {
	split, err := resolveAssessmentFundSplit(context.Background(), nil, uuid.New())
	require.NoError(t, err)
	require.Len(t, split, 1)
	assert.Equal(t, "operating", split[0].FundType)
	assert.Equal(t, 1.0, split[0].Percent)
}

func TestResolveAssessmentFundSplit_PolicyAllOperating(t *testing.T) {
	ruling, _ := json.Marshal(assessmentFundSplitRuling{
		Allocations: []fundSplitEntry{
			{FundType: "operating", Percent: 1.0},
		},
	})
	registry := newRegistryWithCap(t, "assessment_fund_allocation", ruling)

	split, err := resolveAssessmentFundSplit(context.Background(), registry, uuid.New())
	require.NoError(t, err)
	require.Len(t, split, 1)
	assert.Equal(t, "operating", split[0].FundType)
	assert.Equal(t, 1.0, split[0].Percent)
}
