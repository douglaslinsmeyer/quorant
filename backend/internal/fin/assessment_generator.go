package fin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/policy"
)

// UnitLister provides a list of unit IDs for an org. Implemented by the org
// module's unit repository.
type UnitLister interface {
	ListUnitIDsByOrg(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
}

// IsDueThisPeriod reports whether the given schedule should generate
// assessments for the billing period containing `now`.
func IsDueThisPeriod(sched AssessmentSchedule, now time.Time) bool {
	if now.Before(sched.StartsAt) {
		return false
	}
	if sched.EndsAt != nil && now.After(*sched.EndsAt) {
		return false
	}

	day := 1
	if sched.DayOfMonth != nil {
		day = *sched.DayOfMonth
	}
	if now.Day() != day {
		return false
	}

	// How many months since the schedule started?
	monthsSinceStart := (now.Year()-sched.StartsAt.Year())*12 + int(now.Month()) - int(sched.StartsAt.Month())

	switch sched.Frequency {
	case AssessmentFreqMonthly:
		return true
	case AssessmentFreqQuarterly:
		return monthsSinceStart%3 == 0
	case AssessmentFreqSemiAnnually:
		return monthsSinceStart%6 == 0
	case AssessmentFreqAnnually:
		return monthsSinceStart%12 == 0
	default:
		return false
	}
}

// AssessmentAmountContext carries unit-level data for amount resolution.
type AssessmentAmountContext struct {
	BaseAmountCents int64
	UnitType        string
	LotSizeSqft     int
}

// AssessmentAmountRuling holds the decoded assessment_amount_strategy ruling.
type AssessmentAmountRuling struct {
	Strategy    string `json:"strategy"`
	AmountCents *int64 `json:"amount_cents,omitempty"`
}

// resolveAssessmentAmount determines the assessment amount for a unit via the
// policy engine. When no policy is configured, returns the schedule's base amount.
func resolveAssessmentAmount(ctx context.Context, registry *policy.Registry, orgID uuid.UUID, ac AssessmentAmountContext) (int64, error) {
	if registry == nil {
		return ac.BaseAmountCents, nil
	}

	resolution, err := registry.Resolve(ctx, orgID, nil, "assessment_amount_strategy")
	if err != nil || resolution == nil || resolution.Ruling == nil {
		return ac.BaseAmountCents, nil
	}

	var ruling AssessmentAmountRuling
	if err := json.Unmarshal(resolution.Ruling, &ruling); err != nil {
		return ac.BaseAmountCents, nil
	}

	switch ruling.Strategy {
	case "flat":
		if ruling.AmountCents != nil {
			return *ruling.AmountCents, nil
		}
		return ac.BaseAmountCents, nil
	default:
		return ac.BaseAmountCents, nil
	}
}


// GenerateScheduledAssessments creates assessments for all units in the
// schedule's org that don't already have one for the current billing period.
// Returns the count of assessments created.
func (s *FinService) GenerateScheduledAssessments(ctx context.Context, sched AssessmentSchedule, lister UnitLister) (int, error) {
	unitIDs, err := lister.ListUnitIDsByOrg(ctx, sched.OrgID)
	if err != nil {
		return 0, fmt.Errorf("fin: GenerateScheduledAssessments list units: %w", err)
	}

	// Determine due date for this period.
	now := time.Now()
	day := 1
	if sched.DayOfMonth != nil {
		day = *sched.DayOfMonth
	}
	dueDate := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, time.UTC)

	// Load existing assessments to detect duplicates.
	existingByUnit := make(map[uuid.UUID]bool)
	for _, unitID := range unitIDs {
		assessments, listErr := s.assessments.ListAssessmentsByUnit(ctx, unitID)
		if listErr != nil {
			return 0, fmt.Errorf("fin: GenerateScheduledAssessments list assessments: %w", listErr)
		}
		for _, a := range assessments {
			if a.ScheduleID != nil && *a.ScheduleID == sched.ID &&
				a.DueDate.Year() == dueDate.Year() && a.DueDate.Month() == dueDate.Month() {
				existingByUnit[unitID] = true
				break
			}
		}
	}

	description := fmt.Sprintf("%s - %s %d", sched.Name, dueDate.Month().String(), dueDate.Year())

	count := 0
	for _, unitID := range unitIDs {
		if existingByUnit[unitID] {
			continue
		}

		req := CreateAssessmentRequest{
			UnitID:      unitID,
			Description: description,
			AmountCents: sched.BaseAmountCents,
			DueDate:     dueDate,
			GraceDays:   sched.GraceDays,
			ScheduleID:  &sched.ID,
		}

		if _, err := s.CreateAssessment(ctx, sched.OrgID, req); err != nil {
			return count, fmt.Errorf("fin: GenerateScheduledAssessments unit %s: %w", unitID, err)
		}
		count++
	}

	return count, nil
}
