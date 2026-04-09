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

// MarotoGenerator implements CertificateGenerator using the Maroto v2 PDF
// library.
type MarotoGenerator struct{}

// NewMarotoGenerator returns a new MarotoGenerator.
func NewMarotoGenerator() *MarotoGenerator {
	return &MarotoGenerator{}
}

// GenerateEstoppel renders a standard estoppel certificate PDF.
func (g *MarotoGenerator) GenerateEstoppel(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	return buildEstoppelPDF(data, rules)
}

// GenerateLenderQuestionnaire renders a lender questionnaire PDF.
func (g *MarotoGenerator) GenerateLenderQuestionnaire(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	return buildLenderQuestionnairePDF(data, rules)
}
