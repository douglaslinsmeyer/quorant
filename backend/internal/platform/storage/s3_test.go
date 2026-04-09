package storage_test

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/quorant/quorant/internal/platform/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// MockStorageClient
// ---------------------------------------------------------------------------

// MockStorageClient is an in-memory implementation of StorageClient for use in
// tests that depend on file storage without requiring a real S3/MinIO instance.
type MockStorageClient struct {
	uploaded map[string][]byte
	urls     map[string]string
}

// NewMockStorageClient creates a new MockStorageClient with empty state.
func NewMockStorageClient() *MockStorageClient {
	return &MockStorageClient{
		uploaded: make(map[string][]byte),
		urls:     make(map[string]string),
	}
}

func (m *MockStorageClient) Upload(_ context.Context, bucket, key, _ string, reader io.Reader, _ int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("mock upload read: %w", err)
	}
	m.uploaded[bucket+"/"+key] = data
	return nil
}

func (m *MockStorageClient) PresignedGetURL(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	storageKey := bucket + "/" + key
	if _, ok := m.uploaded[storageKey]; !ok {
		return "", fmt.Errorf("mock: object not found: %s", storageKey)
	}
	return "https://mock-storage.example.com/" + storageKey + "?X-Mock-Sig=get", nil
}

func (m *MockStorageClient) PresignedPutURL(_ context.Context, bucket, key, _ string, _ time.Duration) (string, error) {
	return "https://mock-storage.example.com/" + bucket + "/" + key + "?X-Mock-Sig=put", nil
}

func (m *MockStorageClient) Delete(_ context.Context, bucket, key string) error {
	storageKey := bucket + "/" + key
	if _, ok := m.uploaded[storageKey]; !ok {
		return fmt.Errorf("mock: object not found: %s", storageKey)
	}
	delete(m.uploaded, storageKey)
	return nil
}

// Uploaded returns the raw bytes stored under bucket/key, for test assertions.
func (m *MockStorageClient) Uploaded(bucket, key string) ([]byte, bool) {
	data, ok := m.uploaded[bucket+"/"+key]
	return data, ok
}

// ---------------------------------------------------------------------------
// Unit tests — MockStorageClient satisfies StorageClient interface
// ---------------------------------------------------------------------------

// Compile-time proof that *MockStorageClient satisfies the StorageClient interface.
var _ storage.StorageClient = (*MockStorageClient)(nil)

func TestMockStorageClient_ImplementsInterface(t *testing.T) {
	var client storage.StorageClient = NewMockStorageClient()
	assert.NotNil(t, client)
}

func TestMockStorageClient_Upload(t *testing.T) {
	mock := NewMockStorageClient()
	ctx := context.Background()

	content := "hello world"
	err := mock.Upload(ctx, "test-bucket", "docs/hello.txt", "text/plain",
		strings.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	got, ok := mock.Uploaded("test-bucket", "docs/hello.txt")
	require.True(t, ok, "expected uploaded file to exist")
	assert.Equal(t, content, string(got))
}

func TestMockStorageClient_PresignedGetURL_ReturnsValidURL(t *testing.T) {
	mock := NewMockStorageClient()
	ctx := context.Background()

	// Upload first so the object exists.
	content := "some content"
	err := mock.Upload(ctx, "test-bucket", "docs/test.pdf", "application/pdf",
		strings.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	rawURL, err := mock.PresignedGetURL(ctx, "test-bucket", "docs/test.pdf", 15*time.Minute)
	require.NoError(t, err)

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err, "presigned GET URL must be a valid URL")
	assert.NotEmpty(t, parsed.Scheme)
	assert.NotEmpty(t, parsed.Host)
}

func TestMockStorageClient_PresignedGetURL_ErrorForMissingObject(t *testing.T) {
	mock := NewMockStorageClient()
	ctx := context.Background()

	_, err := mock.PresignedGetURL(ctx, "test-bucket", "nonexistent.pdf", 15*time.Minute)
	assert.Error(t, err)
}

func TestMockStorageClient_PresignedPutURL_ReturnsValidURL(t *testing.T) {
	mock := NewMockStorageClient()
	ctx := context.Background()

	rawURL, err := mock.PresignedPutURL(ctx, "test-bucket", "docs/upload.pdf", "application/pdf", 15*time.Minute)
	require.NoError(t, err)

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err, "presigned PUT URL must be a valid URL")
	assert.NotEmpty(t, parsed.Scheme)
	assert.NotEmpty(t, parsed.Host)
}

func TestMockStorageClient_Delete(t *testing.T) {
	mock := NewMockStorageClient()
	ctx := context.Background()

	content := "delete me"
	err := mock.Upload(ctx, "test-bucket", "docs/remove.txt", "text/plain",
		strings.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	err = mock.Delete(ctx, "test-bucket", "docs/remove.txt")
	require.NoError(t, err)

	_, ok := mock.Uploaded("test-bucket", "docs/remove.txt")
	assert.False(t, ok, "expected object to be deleted")
}

func TestMockStorageClient_Delete_ErrorForMissingObject(t *testing.T) {
	mock := NewMockStorageClient()
	ctx := context.Background()

	err := mock.Delete(ctx, "test-bucket", "nonexistent.txt")
	assert.Error(t, err)
}

