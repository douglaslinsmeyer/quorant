package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	platformai "github.com/quorant/quorant/internal/platform/ai"
)

// DocumentPipeline processes governing documents through the AI pipeline:
// 1. Text chunking (split document into semantic chunks)
// 2. Embedding (generate vectors for each chunk)
// 3. Policy extraction (AI reads document and extracts structured policies)
type DocumentPipeline struct {
	chunkRepo  ContextChunkRepository
	policyRepo PolicyRepository
	llmClient  platformai.Client
	embedFn    EmbeddingFunc
	logger     *slog.Logger
}

// NewDocumentPipeline creates a governing document processing pipeline.
func NewDocumentPipeline(chunkRepo ContextChunkRepository, policyRepo PolicyRepository, llmClient platformai.Client, embedFn EmbeddingFunc, logger *slog.Logger) *DocumentPipeline {
	return &DocumentPipeline{
		chunkRepo:  chunkRepo,
		policyRepo: policyRepo,
		llmClient:  llmClient,
		embedFn:    embedFn,
		logger:     logger,
	}
}

// ProcessDocument runs the full pipeline on a governing document.
// Called when a document is registered or reindexed.
func (p *DocumentPipeline) ProcessDocument(ctx context.Context, doc *GoverningDocument, documentText string) error {
	p.logger.Info("processing governing document", "doc_id", doc.ID, "title", doc.Title)

	// Step 1: Update status to processing
	doc.IndexingStatus = "processing"
	if _, err := p.policyRepo.UpdateGoverningDoc(ctx, doc); err != nil {
		return fmt.Errorf("updating doc status: %w", err)
	}

	// Step 2: Delete existing chunks for this document (for re-indexing)
	if err := p.chunkRepo.DeleteBySource(ctx, doc.ID); err != nil {
		return fmt.Errorf("deleting existing chunks: %w", err)
	}

	// Step 3: Chunk the document text
	chunks := chunkText(documentText, 500, 50) // ~500 tokens per chunk, 50 token overlap
	p.logger.Info("chunked document", "doc_id", doc.ID, "chunk_count", len(chunks))

	// Step 4: Embed and store each chunk
	contextChunks := make([]*ContextChunk, 0, len(chunks))
	for i, chunkText := range chunks {
		embedding, err := p.embedFn(ctx, chunkText)
		if err != nil {
			p.logger.Error("embedding failed", "doc_id", doc.ID, "chunk", i, "error", err)
			continue // skip this chunk, process the rest
		}

		contextChunks = append(contextChunks, &ContextChunk{
			Scope:      "org",
			OrgID:      &doc.OrgID,
			SourceType: "governing_document",
			SourceID:   doc.ID,
			ChunkIndex: i,
			Content:    chunkText,
			SectionRef: func() *string { s := fmt.Sprintf("chunk_%d", i); return &s }(),
			Embedding:  embedding,
			TokenCount: estimateTokens(chunkText),
			Metadata:   map[string]any{"doc_type": doc.DocType, "title": doc.Title},
		})
	}

	if len(contextChunks) > 0 {
		if err := p.chunkRepo.CreateBatch(ctx, contextChunks); err != nil {
			return fmt.Errorf("storing chunks: %w", err)
		}
	}

	// Step 5: Extract structured policies via LLM
	extractionCount := 0
	if p.llmClient != nil && len(documentText) > 0 {
		extractions, err := p.extractPolicies(ctx, doc, documentText)
		if err != nil {
			p.logger.Error("policy extraction failed", "doc_id", doc.ID, "error", err)
			// Don't fail the whole pipeline — chunks are already stored
		} else {
			extractionCount = len(extractions)
		}
	}

	// Step 6: Update document status to indexed
	chunkCount := len(contextChunks)
	doc.IndexingStatus = "indexed"
	doc.ChunkCount = &chunkCount
	doc.ExtractionCount = &extractionCount
	if _, err := p.policyRepo.UpdateGoverningDoc(ctx, doc); err != nil {
		return fmt.Errorf("updating doc status: %w", err)
	}

	p.logger.Info("document processing complete",
		"doc_id", doc.ID, "chunks", chunkCount, "extractions", extractionCount)
	return nil
}

// extractPolicies uses the LLM to extract structured policy rules from the document.
func (p *DocumentPipeline) extractPolicies(ctx context.Context, doc *GoverningDocument, text string) ([]*PolicyExtraction, error) {
	systemPrompt := `You are analyzing an HOA governing document. Extract all structured policy rules you can find.
For each policy, provide:
- domain: one of "financial", "governance", "compliance", "use_restrictions", "operational"
- policy_key: a snake_case identifier (e.g., "late_fee_schedule", "voting_eligibility", "pet_policy")
- config: the structured rule as a JSON object
- source_text: the exact passage this was extracted from
- source_section: the section reference (e.g., "Article IV, Section 3(b)")
- confidence: 0.0-1.0 how confident you are in the extraction

Respond with a JSON array of extractions.`

	// Truncate text to fit model context window (rough limit)
	if len(text) > 50000 {
		text = text[:50000]
	}

	resp, err := p.llmClient.Complete(ctx, platformai.CompletionRequest{
		System:         systemPrompt,
		Messages:       []platformai.Message{{Role: "user", Content: text}},
		MaxTokens:      4096,
		ResponseFormat: &platformai.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM extraction: %w", err)
	}

	var rawExtractions []struct {
		Domain        string          `json:"domain"`
		PolicyKey     string          `json:"policy_key"`
		Config        json.RawMessage `json:"config"`
		SourceText    string          `json:"source_text"`
		SourceSection string          `json:"source_section"`
		Confidence    float64         `json:"confidence"`
	}

	// Try to parse as array first, then as object with "extractions" key
	if err := json.Unmarshal([]byte(resp.Content), &rawExtractions); err != nil {
		var wrapper struct {
			Extractions json.RawMessage `json:"extractions"`
		}
		if json.Unmarshal([]byte(resp.Content), &wrapper) == nil {
			json.Unmarshal(wrapper.Extractions, &rawExtractions)
		}
	}

	var results []*PolicyExtraction
	for _, raw := range rawExtractions {
		if raw.PolicyKey == "" || raw.Domain == "" {
			continue
		}
		section := raw.SourceSection
		extraction := &PolicyExtraction{
			OrgID:         doc.OrgID,
			Domain:        raw.Domain,
			PolicyKey:     raw.PolicyKey,
			Config:        raw.Config,
			Confidence:    raw.Confidence,
			SourceDocID:   doc.ID,
			SourceText:    raw.SourceText,
			SourceSection: &section,
			ReviewStatus:  "pending", // requires board review
			ModelVersion:  resp.Model,
		}

		created, err := p.policyRepo.CreateExtraction(ctx, extraction)
		if err != nil {
			p.logger.Error("storing extraction", "policy_key", raw.PolicyKey, "error", err)
			continue
		}
		results = append(results, created)
	}

	return results, nil
}

// chunkText splits text into overlapping chunks of approximately maxTokens size.
func chunkText(text string, maxTokens, overlap int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	// Rough approximation: 1 token ≈ 0.75 words
	wordsPerChunk := int(float64(maxTokens) * 0.75)
	overlapWords := int(float64(overlap) * 0.75)

	if wordsPerChunk <= 0 {
		wordsPerChunk = 100
	}
	if overlapWords < 0 {
		overlapWords = 0
	}

	var chunks []string
	for i := 0; i < len(words); {
		end := i + wordsPerChunk
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
		i += wordsPerChunk - overlapWords
		if i >= len(words) {
			break
		}
	}
	return chunks
}

// estimateTokens provides a rough token count estimate.
func estimateTokens(text string) int {
	// GPT-family models: ~4 chars per token on average
	return len(text) / 4
}
