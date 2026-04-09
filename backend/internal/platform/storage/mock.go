package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// MockStorageClient is an in-memory implementation of StorageClient.
// It is intended for use in development environments where MinIO is not available
// and in test doubles outside the storage package.
type MockStorageClient struct {
	uploaded map[string][]byte
}

// NewMockStorageClient creates a new MockStorageClient with empty state.
func NewMockStorageClient() *MockStorageClient {
	return &MockStorageClient{
		uploaded: make(map[string][]byte),
	}
}

// Upload stores the content of reader under bucket/key.
func (m *MockStorageClient) Upload(_ context.Context, bucket, key, _ string, reader io.Reader, _ int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("mock upload read: %w", err)
	}
	m.uploaded[bucket+"/"+key] = data
	return nil
}

// PresignedGetURL returns a fake pre-signed GET URL for an existing object.
func (m *MockStorageClient) PresignedGetURL(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	storageKey := bucket + "/" + key
	if _, ok := m.uploaded[storageKey]; !ok {
		// For dev use: return a URL even if the object was never explicitly uploaded.
		return "https://mock-storage.example.com/" + storageKey + "?X-Mock-Sig=get", nil
	}
	return "https://mock-storage.example.com/" + storageKey + "?X-Mock-Sig=get", nil
}

// PresignedPutURL returns a fake pre-signed PUT URL.
func (m *MockStorageClient) PresignedPutURL(_ context.Context, bucket, key, _ string, _ time.Duration) (string, error) {
	return "https://mock-storage.example.com/" + bucket + "/" + key + "?X-Mock-Sig=put", nil
}

// Delete removes an object from the mock store; no error if missing.
func (m *MockStorageClient) Delete(_ context.Context, bucket, key string) error {
	delete(m.uploaded, bucket+"/"+key)
	return nil
}

// Uploaded returns the raw bytes stored under bucket/key, for test assertions.
func (m *MockStorageClient) Uploaded(bucket, key string) ([]byte, bool) {
	data, ok := m.uploaded[bucket+"/"+key]
	return data, ok
}

// compile-time interface check
var _ StorageClient = (*MockStorageClient)(nil)
