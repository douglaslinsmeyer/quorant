package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/queue"
)

// IngestionWorker handles domain events that should be ingested into the context lake.
// It embeds content from various sources and stores it as context chunks.
type IngestionWorker struct {
	chunkRepo ContextChunkRepository
	embedFn   EmbeddingFunc
	logger    *slog.Logger
}

// NewIngestionWorker creates a context lake ingestion worker.
func NewIngestionWorker(chunkRepo ContextChunkRepository, embedFn EmbeddingFunc, logger *slog.Logger) *IngestionWorker {
	return &IngestionWorker{
		chunkRepo: chunkRepo,
		embedFn:   embedFn,
		logger:    logger,
	}
}

// RegisterHandlers registers NATS event handlers for context lake ingestion.
// Called during worker binary startup.
func (w *IngestionWorker) RegisterHandlers(consumer *queue.Consumer) {
	// Document uploads → chunk and embed
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.ingest_document",
		Subject: "quorant.doc.DocumentUploaded.>",
		Handler: w.handleDocumentUploaded,
	})

	// Announcements → embed directly
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.ingest_announcement",
		Subject: "quorant.com.AnnouncementPublished.>",
		Handler: w.handleAnnouncementPublished,
	})

	// Violation resolved → lifecycle narrative
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.ingest_violation_resolved",
		Subject: "quorant.gov.ViolationResolved.>",
		Handler: w.handleViolationResolved,
	})

	// Meeting completed → embed motions
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.ingest_meeting_completed",
		Subject: "quorant.gov.MeetingCompleted.>",
		Handler: w.handleMeetingCompleted,
	})

	// Task completed → lifecycle narrative for significant tasks
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.ingest_task_completed",
		Subject: "quorant.task.TaskCompleted.>",
		Handler: w.handleTaskCompleted,
	})

	w.logger.Info("registered context lake ingestion handlers", "count", 5)
}

func (w *IngestionWorker) handleDocumentUploaded(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID       uuid.UUID `json:"org_id"`
		DocumentID  uuid.UUID `json:"document_id"`
		Title       string    `json:"title"`
		ContentType string    `json:"content_type"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing DocumentUploaded event: %w", err)
	}

	w.logger.Info("ingesting document", "doc_id", payload.DocumentID, "title", payload.Title)

	// For now, create a single chunk with the document title as content.
	// Full text extraction (OCR, PDF parsing) would happen here in production.
	return w.ingestSingleChunk(ctx, payload.OrgID, "document", payload.DocumentID,
		fmt.Sprintf("Document: %s (type: %s)", payload.Title, payload.ContentType))
}

func (w *IngestionWorker) handleAnnouncementPublished(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID          uuid.UUID `json:"org_id"`
		AnnouncementID uuid.UUID `json:"announcement_id"`
		Title          string    `json:"title"`
		Body           string    `json:"body"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing AnnouncementPublished event: %w", err)
	}

	content := fmt.Sprintf("Announcement: %s\n\n%s", payload.Title, payload.Body)
	return w.ingestSingleChunk(ctx, payload.OrgID, "announcement", payload.AnnouncementID, content)
}

func (w *IngestionWorker) handleViolationResolved(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID       uuid.UUID `json:"org_id"`
		ViolationID uuid.UUID `json:"violation_id"`
		Title       string    `json:"title"`
		Category    string    `json:"category"`
		Resolution  string    `json:"resolution"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing ViolationResolved event: %w", err)
	}

	content := fmt.Sprintf("Violation resolved: %s (category: %s). Resolution: %s",
		payload.Title, payload.Category, payload.Resolution)
	return w.ingestSingleChunk(ctx, payload.OrgID, "violation_narrative", payload.ViolationID, content)
}

func (w *IngestionWorker) handleMeetingCompleted(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID     uuid.UUID `json:"org_id"`
		MeetingID uuid.UUID `json:"meeting_id"`
		Title     string    `json:"title"`
		Summary   string    `json:"summary"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing MeetingCompleted event: %w", err)
	}

	content := fmt.Sprintf("Meeting: %s\n\n%s", payload.Title, payload.Summary)
	return w.ingestSingleChunk(ctx, payload.OrgID, "meeting_motion", payload.MeetingID, content)
}

func (w *IngestionWorker) handleTaskCompleted(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID  uuid.UUID `json:"org_id"`
		TaskID uuid.UUID `json:"task_id"`
		Title  string    `json:"title"`
		Type   string    `json:"task_type"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing TaskCompleted event: %w", err)
	}

	content := fmt.Sprintf("Task completed: %s (type: %s)", payload.Title, payload.Type)
	return w.ingestSingleChunk(ctx, payload.OrgID, "task_narrative", payload.TaskID, content)
}

// ingestSingleChunk embeds text and stores it as a single context chunk.
func (w *IngestionWorker) ingestSingleChunk(ctx context.Context, orgID uuid.UUID, sourceType string, sourceID uuid.UUID, content string) error {
	embedding, err := w.embedFn(ctx, content)
	if err != nil {
		return fmt.Errorf("embedding %s %s: %w", sourceType, sourceID, err)
	}

	chunk := &ContextChunk{
		Scope:      "org",
		OrgID:      &orgID,
		SourceType: sourceType,
		SourceID:   sourceID,
		ChunkIndex: 0,
		Content:    content,
		Embedding:  embedding,
		TokenCount: estimateTokens(content),
		Metadata:   map[string]any{},
	}

	return w.chunkRepo.CreateBatch(ctx, []*ContextChunk{chunk})
}
