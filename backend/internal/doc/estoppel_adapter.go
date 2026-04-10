package doc

import (
	"context"

	"github.com/google/uuid"
)

// EstoppelDocumentAdapter adapts DocService to the narrow DocumentUploader and
// DocumentDownloader interfaces expected by the estoppel package, avoiding a
// direct import cycle between the two packages.
type EstoppelDocumentAdapter struct {
	service *DocService
}

// NewEstoppelDocumentAdapter constructs an EstoppelDocumentAdapter wrapping the
// given DocService.
func NewEstoppelDocumentAdapter(service *DocService) *EstoppelDocumentAdapter {
	return &EstoppelDocumentAdapter{service: service}
}

// UploadFromBytes uploads raw bytes as a new document and returns the document
// ID on success. It delegates to DocService.UploadFromBytes.
func (a *EstoppelDocumentAdapter) UploadFromBytes(ctx context.Context, orgID uuid.UUID, title, fileName, contentType string, data []byte, uploadedBy uuid.UUID) (uuid.UUID, error) {
	req := UploadFromBytesRequest{
		Title:       title,
		FileName:    fileName,
		ContentType: contentType,
	}
	doc, err := a.service.UploadFromBytes(ctx, orgID, req, data, uploadedBy)
	if err != nil {
		return uuid.Nil, err
	}
	return doc.ID, nil
}

// GetDownloadURL returns a pre-signed download URL for the document with the
// given ID. It delegates to DocService.GetDownloadURL.
func (a *EstoppelDocumentAdapter) GetDownloadURL(ctx context.Context, documentID uuid.UUID) (string, error) {
	return a.service.GetDownloadURL(ctx, documentID)
}
