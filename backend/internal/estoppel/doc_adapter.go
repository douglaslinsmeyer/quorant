package estoppel

import (
	"context"

	"github.com/google/uuid"
)

// DocumentUploader is a narrow interface for uploading generated documents
// (e.g. estoppel PDFs) from raw bytes into document storage. It is satisfied
// by doc.EstoppelDocumentAdapter.
type DocumentUploader interface {
	UploadFromBytes(ctx context.Context, orgID uuid.UUID, title, fileName, contentType string, data []byte, uploadedBy uuid.UUID) (documentID uuid.UUID, err error)
}

// DocumentDownloader is a narrow interface for retrieving pre-signed download
// URLs for previously stored documents. It is satisfied by
// doc.EstoppelDocumentAdapter.
type DocumentDownloader interface {
	GetDownloadURL(ctx context.Context, documentID uuid.UUID) (string, error)
}
