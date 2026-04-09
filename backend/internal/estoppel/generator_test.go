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
// TestMarotoGenerator_InterfaceCompliance
// ---------------------------------------------------------------------------

// TestMarotoGenerator_InterfaceCompliance is a compile-time check that
// MarotoGenerator satisfies CertificateGenerator.
var _ estoppel.CertificateGenerator = (*estoppel.MarotoGenerator)(nil)
