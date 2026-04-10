package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	platformai "github.com/quorant/quorant/internal/platform/ai"
)

// PostgresPolicyResolver implements PolicyResolver using PolicyService + LLM.
// GetPolicy performs a cache lookup from policy_extractions.
// QueryPolicy performs RAG inference: embed question → retrieve chunks → LLM synthesis.
type PostgresPolicyResolver struct {
	service        *PolicyService
	contextService *ContextLakeService
	llmClient      platformai.Client
}

// NewPostgresPolicyResolver constructs a resolver with optional LLM client.
// If llmClient is nil, QueryPolicy returns nil (graceful degradation).
func NewPostgresPolicyResolver(service *PolicyService) *PostgresPolicyResolver {
	return &PostgresPolicyResolver{service: service}
}

// NewPostgresPolicyResolverWithLLM constructs a resolver with full RAG capability.
func NewPostgresPolicyResolverWithLLM(service *PolicyService, contextService *ContextLakeService, llmClient platformai.Client) *PostgresPolicyResolver {
	return &PostgresPolicyResolver{
		service:        service,
		contextService: contextService,
		llmClient:      llmClient,
	}
}

// GetPolicy implements PolicyResolver — looks up the active extraction for the policy key.
func (r *PostgresPolicyResolver) GetPolicy(ctx context.Context, orgID uuid.UUID, policyKey string) (*PolicyResult, error) {
	return r.service.GetActivePolicy(ctx, orgID, policyKey)
}

// QueryPolicy implements PolicyResolver — RAG inference against the context lake.
// Steps:
// 1. Search context lake for relevant chunks using the query
// 2. Send chunks + query to LLM for synthesis
// 3. Log resolution and return structured result
func (r *PostgresPolicyResolver) QueryPolicy(ctx context.Context, orgID uuid.UUID, query string, qctx QueryContext) (*ResolutionResult, error) {
	if r.llmClient == nil || r.contextService == nil {
		return nil, fmt.Errorf("AI inference not configured: LLM client or context service not available")
	}

	start := time.Now()

	// Step 1: Retrieve relevant context chunks
	chunks, err := r.contextService.Search(ctx, orgID, query, ContextFilters{
		MaxResults: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("context lake search: %w", err)
	}

	// Step 2: Build prompt with retrieved context
	contextText := ""
	sourcePassages := make([]SourcePassage, 0, len(chunks))
	for _, chunk := range chunks {
		contextText += fmt.Sprintf("--- Source: %s (section: %s) ---\n%s\n\n",
			chunk.SourceType, chunk.SectionRef, chunk.Content)
		sourcePassages = append(sourcePassages, SourcePassage{
			DocID:   chunk.SourceID,
			Section: chunk.SectionRef,
			Text:    chunk.Content,
		})
	}

	systemPrompt := `You are an AI assistant for an HOA (Homeowners Association) management platform.
You answer policy questions by analyzing the provided governing documents and community context.
Always cite specific sections from the source material.
If you cannot find a definitive answer in the provided context, say so clearly.
Respond in JSON format with fields: "answer" (string), "confidence" (0.0-1.0), "citations" (array of section references).`

	userPrompt := fmt.Sprintf("Context from community documents and history:\n\n%s\n\nQuestion: %s", contextText, query)

	resp, err := r.llmClient.Complete(ctx, platformai.CompletionRequest{
		System:    systemPrompt,
		Messages:  []platformai.Message{{Role: "user", Content: userPrompt}},
		MaxTokens: 2048,
		ResponseFormat: &platformai.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM inference: %w", err)
	}

	latencyMs := int(time.Since(start).Milliseconds())

	// Step 3: Parse LLM response
	var llmResponse struct {
		Answer     string   `json:"answer"`
		Confidence float64  `json:"confidence"`
		Citations  []string `json:"citations"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &llmResponse); err != nil {
		// If JSON parsing fails, treat the raw content as the answer
		llmResponse.Answer = resp.Content
		llmResponse.Confidence = 0.5
	}

	resolution, _ := json.Marshal(map[string]any{
		"answer":    llmResponse.Answer,
		"citations": llmResponse.Citations,
	})

	passagesJSON, _ := json.Marshal(sourcePassages)

	// Step 4: Log the resolution
	resolutionRecord := &PolicyResolution{
		OrgID:             orgID,
		Query:             query,
		PolicyKeys:        []string{}, // natural language query, not a specific key
		Resolution:        resolution,
		Reasoning:         llmResponse.Answer,
		SourcePassages:    passagesJSON,
		Confidence:        llmResponse.Confidence,
		ResolutionType:    "ai_inference",
		ModelVersion:      &resp.Model,
		LatencyMs:         &latencyMs,
		RequestingModule:  qctx.Module,
		RequestingContext:  json.RawMessage("{}"),
	}

	if _, err := r.service.repo.CreateResolution(ctx, resolutionRecord); err != nil {
		// Log but don't fail — the resolution was successful even if logging fails
	}

	return &ResolutionResult{
		Resolution:     resolution,
		Reasoning:      llmResponse.Answer,
		SourcePassages: sourcePassages,
		Confidence:     llmResponse.Confidence,
		Escalated:      llmResponse.Confidence < 0.5, // low confidence → suggest human review
	}, nil
}
