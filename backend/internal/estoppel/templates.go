package estoppel

import (
	"fmt"
	"time"

	marotolib "github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// newMarotoDoc creates a new Maroto document with the standard Letter page
// size and 15mm margins on all sides.
func newMarotoDoc() core.Maroto {
	cfg := config.NewBuilder().
		WithPageSize(pagesize.Letter).
		WithLeftMargin(15).
		WithTopMargin(15).
		WithRightMargin(15).
		WithBottomMargin(15).
		Build()
	return marotolib.New(cfg)
}

// ---------------------------------------------------------------------------
// Estoppel certificate template
// ---------------------------------------------------------------------------

// buildEstoppelPDF constructs the Maroto document for a standard estoppel
// certificate and returns the raw PDF bytes.
func buildEstoppelPDF(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	m := newMarotoDoc()

	addSectionHeader(m, "ESTOPPEL CERTIFICATE")
	addSpacer(m, 5)

	// ── Property information ──────────────────────────────────────────────────
	addSectionHeader(m, "Property Information")
	addLabelValue(m, "Unit Number", data.Property.UnitNumber)
	addLabelValue(m, "Address", data.Property.Address)
	addLabelValue(m, "Parcel Number", data.Property.ParcelNumber)
	addLabelValue(m, "Unit Type", data.Property.UnitType)
	addSpacer(m, 3)

	// ── Financial information ─────────────────────────────────────────────────
	addSectionHeader(m, "Financial Information")
	addLabelValue(m, "Regular Assessment", formatCents(data.Financial.RegularAssessmentCents))
	addLabelValue(m, "Assessment Frequency", data.Financial.AssessmentFrequency)
	addLabelValue(m, "Current Balance", formatCents(data.Financial.CurrentBalanceCents))
	addLabelValue(m, "Transfer Fee", formatCents(data.Financial.TransferFeeCents))
	addLabelValue(m, "Capital Contribution", formatCents(data.Financial.CapitalContributionCents))
	addLabelValue(m, "Reserve Fund Fee", formatCents(data.Financial.ReserveFundFeeCents))
	addSpacer(m, 3)

	// Delinquency section
	if data.Financial.HasActiveCollection {
		addSectionHeader(m, "Delinquency Information")
		addLabelValue(m, "Collection Status", data.Financial.CollectionStatus)
		addLabelValue(m, "Total Delinquent Amount", formatCents(data.Financial.TotalDelinquentCents))
		addLabelValue(m, "Late Fees", formatCents(data.Financial.LateFeesCents))
		addLabelValue(m, "Interest", formatCents(data.Financial.InterestCents))
		addSpacer(m, 3)
	}

	// ── Compliance information ────────────────────────────────────────────────
	addSectionHeader(m, "Compliance Information")
	addLabelValue(m, "Open Violations", fmt.Sprintf("%d", data.Compliance.Violations.OpenCount))
	addLabelValue(m, "Active Fines", fmt.Sprintf("%v", data.Compliance.Violations.HasActiveFine))
	addLabelValue(m, "Total Fines", formatCents(data.Compliance.Violations.TotalFinesCents))
	addSpacer(m, 3)

	// ── Narrative sections ────────────────────────────────────────────────────
	if data.Narratives != nil {
		addSectionHeader(m, "Assessment Summary")
		for _, f := range data.Narratives.AssessmentSummary {
			addNarrativeField(m, f)
		}
		addSpacer(m, 3)

		addSectionHeader(m, "Additional Disclosures")
		for _, f := range data.Narratives.AdditionalDisclosures {
			addNarrativeField(m, f)
		}
		addSpacer(m, 3)
	}

	// ── Rules & effective information ─────────────────────────────────────────
	addSectionHeader(m, "Certificate Information")
	addLabelValue(m, "As of Date", data.AsOfTime.Format(time.DateOnly))
	addLabelValue(m, "Statute Reference", rules.StatuteRef)
	addSpacer(m, 5)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("estoppel pdf: generate: %w", err)
	}
	return doc.GetBytes(), nil
}

// ---------------------------------------------------------------------------
// Lender questionnaire template
// ---------------------------------------------------------------------------

// buildLenderQuestionnairePDF constructs the Maroto document for a lender
// questionnaire and returns the raw PDF bytes.
func buildLenderQuestionnairePDF(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	m := newMarotoDoc()

	addSectionHeader(m, "LENDER QUESTIONNAIRE")
	addSpacer(m, 5)

	// ── Property information ──────────────────────────────────────────────────
	addSectionHeader(m, "Property Information")
	addLabelValue(m, "Unit Number", data.Property.UnitNumber)
	addLabelValue(m, "Address", data.Property.Address)
	addLabelValue(m, "Unit Type", data.Property.UnitType)
	addLabelValue(m, "Square Footage", fmt.Sprintf("%.0f sq ft", data.Property.SquareFootage))
	addLabelValue(m, "Bedrooms", fmt.Sprintf("%d", data.Property.Bedrooms))
	addLabelValue(m, "Bathrooms", fmt.Sprintf("%.1f", data.Property.Bathrooms))
	addSpacer(m, 3)

	// ── Association financials ────────────────────────────────────────────────
	addSectionHeader(m, "Association Financial Information")
	addLabelValue(m, "Regular Assessment", formatCents(data.Financial.RegularAssessmentCents))
	addLabelValue(m, "Assessment Frequency", data.Financial.AssessmentFrequency)
	addLabelValue(m, "Reserve Balance", formatCents(data.Financial.ReserveBalanceCents))
	addLabelValue(m, "Reserve Target", formatCents(data.Financial.ReserveTargetCents))
	addLabelValue(m, "Budget Status", data.Financial.BudgetStatus)
	addLabelValue(m, "Delinquency Rate (60 days)", fmt.Sprintf("%.2f%%", data.Financial.DelinquencyRate60Days*100))
	addSpacer(m, 3)

	// ── Insurance ─────────────────────────────────────────────────────────────
	addSectionHeader(m, "Insurance")
	addLabelValue(m, "Master Policy on File", fmt.Sprintf("%v", data.Compliance.Insurance.HasMasterPolicy))
	addLabelValue(m, "Policy Number", data.Compliance.Insurance.PolicyNumber)
	addLabelValue(m, "Carrier", data.Compliance.Insurance.Carrier)
	addLabelValue(m, "Flood Coverage", fmt.Sprintf("%v", data.Compliance.Insurance.FloodCoverage))
	addLabelValue(m, "Earthquake Coverage", fmt.Sprintf("%v", data.Compliance.Insurance.EarthquakeCoverage))
	addSpacer(m, 3)

	// ── Litigation ────────────────────────────────────────────────────────────
	addSectionHeader(m, "Litigation")
	addLabelValue(m, "Active Litigation", fmt.Sprintf("%v", data.Compliance.Litigation.HasActiveLitigation))
	if data.Compliance.Litigation.HasActiveLitigation {
		addLabelValue(m, "Case Description", data.Compliance.Litigation.CaseDescription)
	}
	addSpacer(m, 3)

	// ── Governing documents ───────────────────────────────────────────────────
	addSectionHeader(m, "Governing Documents")
	addLabelValue(m, "CC&R Version", data.Compliance.CCRVersion)
	addLabelValue(m, "By-Laws Version", data.Compliance.ByLawVersion)
	addLabelValue(m, "Rules Version", data.Compliance.RulesVersion)
	addSpacer(m, 3)

	// ── Rules & effective information ─────────────────────────────────────────
	addSectionHeader(m, "Questionnaire Information")
	addLabelValue(m, "As of Date", data.AsOfTime.Format(time.DateOnly))
	addLabelValue(m, "Statute Reference", rules.StatuteRef)
	addSpacer(m, 5)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("lender questionnaire pdf: generate: %w", err)
	}
	return doc.GetBytes(), nil
}

// ---------------------------------------------------------------------------
// Template helper functions
// ---------------------------------------------------------------------------

// addSectionHeader adds a bold, full-width section title row.
func addSectionHeader(m core.Maroto, title string) {
	m.AddRow(8,
		text.NewCol(12, title, props.Text{
			Style: fontstyle.Bold,
			Size:  11,
			Align: align.Left,
		}),
	)
}

// addLabelValue adds a two-column row with a label on the left and value on
// the right.
func addLabelValue(m core.Maroto, label, value string) {
	m.AddRow(6,
		text.NewCol(4, label, props.Text{
			Style: fontstyle.Bold,
			Size:  9,
		}),
		text.NewCol(8, value, props.Text{
			Size: 9,
		}),
	)
}

// addNarrativeField renders a NarrativeField as a label+value pair with a
// slightly larger value area.
func addNarrativeField(m core.Maroto, f NarrativeField) {
	if f.Label != "" {
		m.AddRow(5,
			text.NewCol(12, f.Label, props.Text{
				Style: fontstyle.Bold,
				Size:  9,
			}),
		)
	}
	if f.Value != "" {
		m.AddRow(6,
			text.NewCol(12, f.Value, props.Text{
				Size: 9,
			}),
		)
	}
}

// addSpacer inserts an empty row of the given height (in mm) to provide
// whitespace between sections.
func addSpacer(m core.Maroto, height float64) {
	m.AddRow(height, text.NewCol(12, ""))
}

// formatCents formats an integer cent amount as a US dollar string with
// comma grouping (e.g. 250000 → "$2,500.00", -5050 → "-$50.50").
func formatCents(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	dollars := cents / 100
	pennies := cents % 100
	formatted := insertCommas(dollars)
	result := fmt.Sprintf("$%s.%02d", formatted, pennies)
	if negative {
		result = "-" + result
	}
	return result
}

// insertCommas returns the decimal string representation of n with commas
// inserted every three digits from the right (e.g. 1234567 → "1,234,567").
func insertCommas(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// Work from right to left, inserting commas every three digits.
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}
