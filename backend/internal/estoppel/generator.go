package estoppel

// CertificateGenerator produces PDF documents for estoppel certificates and
// lender questionnaires.
type CertificateGenerator interface {
	// GenerateEstoppel renders an estoppel certificate PDF from the aggregated
	// data and rules, returning the raw PDF bytes.
	GenerateEstoppel(data *AggregatedData, rules *EstoppelRules) ([]byte, error)

	// GenerateLenderQuestionnaire renders a lender questionnaire PDF from the
	// aggregated data and rules, returning the raw PDF bytes.
	GenerateLenderQuestionnaire(data *AggregatedData, rules *EstoppelRules) ([]byte, error)
}

// templateBuilders maps StatutoryFormID values to their corresponding PDF
// builder functions. An empty or unrecognised form ID falls back to the generic
// buildEstoppelPDF builder.
var templateBuilders = map[string]func(*AggregatedData, *EstoppelRules) ([]byte, error){
	"generic":      buildEstoppelPDF,
	"fl_720_30851": buildFloridaEstoppelPDF,
	"ca_4528":      buildCaliforniaEstoppelPDF,
	"tx_207":       buildTexasEstoppelPDF,
	"nv_116":       buildNevadaEstoppelPDF,
	"va_55_1":      buildVirginiaEstoppelPDF,
}

// MarotoGenerator implements CertificateGenerator using the Maroto v2 PDF
// library.
type MarotoGenerator struct{}

// NewMarotoGenerator returns a new MarotoGenerator.
func NewMarotoGenerator() *MarotoGenerator {
	return &MarotoGenerator{}
}

// GenerateEstoppel renders an estoppel certificate PDF, dispatching to a
// state-specific builder based on rules.StatutoryFormID when one is registered.
// Unknown form IDs fall back to the generic builder.
func (g *MarotoGenerator) GenerateEstoppel(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	formID := rules.StatutoryFormID
	if formID == "" {
		formID = "generic"
	}
	builder, ok := templateBuilders[formID]
	if !ok {
		builder = buildEstoppelPDF // fallback to generic
	}
	return builder(data, rules)
}

// GenerateLenderQuestionnaire renders a lender questionnaire PDF.
func (g *MarotoGenerator) GenerateLenderQuestionnaire(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	return buildLenderQuestionnairePDF(data, rules)
}
