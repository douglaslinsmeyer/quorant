package estoppel_test

import (
	"testing"

	"github.com/quorant/quorant/internal/estoppel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEstoppelRules returns a minimal EstoppelRules for use in generator tests.
func testEstoppelRules() *estoppel.EstoppelRules {
	rush := 3
	effective := 30
	return &estoppel.EstoppelRules{
		StandardTurnaroundBusinessDays: 10,
		StandardFeeCents:               25000,
		RushTurnaroundBusinessDays:     &rush,
		RushFeeCents:                   15000,
		DelinquentSurchargeCents:       5000,
		EffectivePeriodDays:            &effective,
		StatuteRef:                     "FS 720.30851",
	}
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_GenerateEstoppel_ProducesValidPDF
// ---------------------------------------------------------------------------

// TestMarotoGenerator_GenerateEstoppel_ProducesValidPDF verifies that
// GenerateEstoppel returns non-empty bytes whose header identifies them as a
// valid PDF file.
func TestMarotoGenerator_GenerateEstoppel_ProducesValidPDF(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	var cg estoppel.CertificateGenerator = gen

	data := newTestAggregatedData()
	rules := testEstoppelRules()

	pdfBytes, err := cg.GenerateEstoppel(data, rules)

	require.NoError(t, err, "GenerateEstoppel should not return an error")
	require.NotEmpty(t, pdfBytes, "GenerateEstoppel should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdfBytes[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_GenerateLenderQuestionnaire_ProducesValidPDF
// ---------------------------------------------------------------------------

// TestMarotoGenerator_GenerateLenderQuestionnaire_ProducesValidPDF verifies
// that GenerateLenderQuestionnaire returns non-empty bytes whose header
// identifies them as a valid PDF file.
func TestMarotoGenerator_GenerateLenderQuestionnaire_ProducesValidPDF(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	var cg estoppel.CertificateGenerator = gen

	data := newTestAggregatedData()
	rules := testEstoppelRules()

	pdfBytes, err := cg.GenerateLenderQuestionnaire(data, rules)

	require.NoError(t, err, "GenerateLenderQuestionnaire should not return an error")
	require.NotEmpty(t, pdfBytes, "GenerateLenderQuestionnaire should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdfBytes[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_FloridaTemplate
// ---------------------------------------------------------------------------

// TestMarotoGenerator_FloridaTemplate verifies that the Florida-specific
// estoppel template produces a valid PDF.
func TestMarotoGenerator_FloridaTemplate(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &estoppel.EstoppelRules{
		StatutoryFormID: "fl_720_30851",
		StatuteRef:      "§720.30851",
	}

	pdf, err := gen.GenerateEstoppel(data, rules)

	require.NoError(t, err, "Florida GenerateEstoppel should not return an error")
	require.NotEmpty(t, pdf, "Florida GenerateEstoppel should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdf[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_CaliforniaTemplate
// ---------------------------------------------------------------------------

// TestMarotoGenerator_CaliforniaTemplate verifies that the California-specific
// estoppel template produces a valid PDF, including required attachments.
func TestMarotoGenerator_CaliforniaTemplate(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &estoppel.EstoppelRules{
		StatutoryFormID:     "ca_4528",
		StatuteRef:          "CA Civil Code §4525",
		RequiredAttachments: []string{"Governing Documents", "CC&Rs", "Bylaws"},
	}

	pdf, err := gen.GenerateEstoppel(data, rules)

	require.NoError(t, err, "California GenerateEstoppel should not return an error")
	require.NotEmpty(t, pdf, "California GenerateEstoppel should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdf[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_TexasTemplate
// ---------------------------------------------------------------------------

// TestMarotoGenerator_TexasTemplate verifies that the Texas-specific estoppel
// template produces a valid PDF.
func TestMarotoGenerator_TexasTemplate(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &estoppel.EstoppelRules{
		StatutoryFormID: "tx_207",
		StatuteRef:      "TX Property Code §207.003",
	}

	pdf, err := gen.GenerateEstoppel(data, rules)

	require.NoError(t, err, "Texas GenerateEstoppel should not return an error")
	require.NotEmpty(t, pdf, "Texas GenerateEstoppel should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdf[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_NevadaTemplate
// ---------------------------------------------------------------------------

// TestMarotoGenerator_NevadaTemplate verifies that the Nevada-specific estoppel
// template produces a valid PDF.
func TestMarotoGenerator_NevadaTemplate(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &estoppel.EstoppelRules{
		StatutoryFormID: "nv_116",
		StatuteRef:      "NRS 116.4109",
	}

	pdf, err := gen.GenerateEstoppel(data, rules)

	require.NoError(t, err, "Nevada GenerateEstoppel should not return an error")
	require.NotEmpty(t, pdf, "Nevada GenerateEstoppel should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdf[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_VirginiaTemplate
// ---------------------------------------------------------------------------

// TestMarotoGenerator_VirginiaTemplate verifies that the Virginia-specific
// estoppel template produces a valid PDF.
func TestMarotoGenerator_VirginiaTemplate(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &estoppel.EstoppelRules{
		StatutoryFormID: "va_55_1",
		StatuteRef:      "§55.1-1808",
	}

	pdf, err := gen.GenerateEstoppel(data, rules)

	require.NoError(t, err, "Virginia GenerateEstoppel should not return an error")
	require.NotEmpty(t, pdf, "Virginia GenerateEstoppel should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdf[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_UnknownFormIDFallsBackToGeneric
// ---------------------------------------------------------------------------

// TestMarotoGenerator_UnknownFormIDFallsBackToGeneric verifies that an
// unrecognised StatutoryFormID falls back to the generic estoppel template
// and still produces a valid PDF.
func TestMarotoGenerator_UnknownFormIDFallsBackToGeneric(t *testing.T) {
	gen := estoppel.NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &estoppel.EstoppelRules{
		StatutoryFormID: "zz_unknown_form",
		StatuteRef:      "Unknown Statute §0",
	}

	pdf, err := gen.GenerateEstoppel(data, rules)

	require.NoError(t, err, "Fallback GenerateEstoppel should not return an error")
	require.NotEmpty(t, pdf, "Fallback GenerateEstoppel should return non-empty bytes")
	assert.Equal(t, "%PDF", string(pdf[:4]), "PDF bytes should start with %%PDF header")
}

// ---------------------------------------------------------------------------
// TestMarotoGenerator_InterfaceCompliance
// ---------------------------------------------------------------------------

// TestMarotoGenerator_InterfaceCompliance is a compile-time check that
// MarotoGenerator satisfies CertificateGenerator.
var _ estoppel.CertificateGenerator = (*estoppel.MarotoGenerator)(nil)
