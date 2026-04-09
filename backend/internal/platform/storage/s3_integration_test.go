//go:build integration

package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/quorant/quorant/internal/platform/config"
	"github.com/quorant/quorant/internal/platform/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	integrationMinIOEndpoint = "localhost:9000"
	integrationAccessKey     = "minioadmin"
	integrationSecretKey     = "minioadmin"
	integrationBucket        = "quorant-test-docs"
)

// newIntegrationS3Client creates an S3Client pointed at the local MinIO instance
// and ensures the test bucket exists.
func newIntegrationS3Client(t *testing.T) *storage.S3Client {
	t.Helper()

	cfg := config.S3Config{
		Endpoint:  integrationMinIOEndpoint,
		AccessKey: integrationAccessKey,
		SecretKey: integrationSecretKey,
		Bucket:    integrationBucket,
		UseSSL:    false,
	}

	client, err := storage.NewS3Client(cfg)
	require.NoError(t, err, "creating S3 client")

	err = client.EnsureBucket(context.Background())
	require.NoError(t, err, "ensuring test bucket %q exists", integrationBucket)

	return client
}

// uniqueKey generates a test-scoped object key to avoid inter-test collisions.
func uniqueKey(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("integration-tests/%s-%d%s", t.Name(), time.Now().UnixNano(), suffix)
}

// TestS3Client_UploadAndPresignedGet uploads a file, generates a pre-signed GET
// URL, and verifies the URL is reachable and returns the correct content.
func TestS3Client_UploadAndPresignedGet(t *testing.T) {
	client := newIntegrationS3Client(t)
	ctx := context.Background()

	content := []byte("integration test content for pre-signed GET")
	key := uniqueKey(t, ".txt")

	// Upload the file.
	err := client.Upload(ctx, integrationBucket, key, "text/plain",
		bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err, "uploading file")

	// Obtain a pre-signed GET URL.
	rawURL, err := client.PresignedGetURL(ctx, integrationBucket, key, 15*time.Minute)
	require.NoError(t, err, "generating pre-signed GET URL")
	assert.NotEmpty(t, rawURL, "pre-signed GET URL must not be empty")

	// The URL must be parseable.
	parsed, parseErr := url.Parse(rawURL)
	require.NoError(t, parseErr)
	assert.NotEmpty(t, parsed.Scheme)
	assert.NotEmpty(t, parsed.Host)

	// Fetch through the pre-signed URL and verify the body matches.
	resp, err := http.Get(rawURL) //nolint:noctx
	require.NoError(t, err, "GET pre-signed URL")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"expected 200 from pre-signed URL; body: %s", body)
	assert.Equal(t, content, body)
}

// TestS3Client_Delete uploads a file, deletes it, and confirms the object is gone.
func TestS3Client_Delete(t *testing.T) {
	client := newIntegrationS3Client(t)
	ctx := context.Background()

	content := []byte("file to be deleted")
	key := uniqueKey(t, ".txt")

	// Upload the file.
	err := client.Upload(ctx, integrationBucket, key, "text/plain",
		bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err, "uploading file before delete")

	// Confirm object is reachable via pre-signed URL before deletion.
	rawURL, err := client.PresignedGetURL(ctx, integrationBucket, key, 5*time.Minute)
	require.NoError(t, err)

	resp, err := http.Get(rawURL) //nolint:noctx
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "object should exist before delete")

	// Delete the object.
	err = client.Delete(ctx, integrationBucket, key)
	require.NoError(t, err, "deleting object")

	// After deletion, a fresh pre-signed URL for the same key should yield 403/404.
	rawURLAfter, err := client.PresignedGetURL(ctx, integrationBucket, key, 5*time.Minute)
	require.NoError(t, err, "generating pre-signed URL post-delete (URL generation itself is offline)")

	respAfter, err := http.Get(rawURLAfter) //nolint:noctx
	require.NoError(t, err)
	respAfter.Body.Close()
	assert.NotEqual(t, http.StatusOK, respAfter.StatusCode,
		"expected non-200 after deletion; got %d", respAfter.StatusCode)
}

// TestS3Client_PresignedPutURL verifies a pre-signed PUT URL is generated successfully.
func TestS3Client_PresignedPutURL(t *testing.T) {
	client := newIntegrationS3Client(t)
	ctx := context.Background()

	key := uniqueKey(t, ".bin")
	rawURL, err := client.PresignedPutURL(ctx, integrationBucket, key, "application/octet-stream", 15*time.Minute)
	require.NoError(t, err, "generating pre-signed PUT URL")
	assert.NotEmpty(t, rawURL)

	parsed, parseErr := url.Parse(rawURL)
	require.NoError(t, parseErr)
	assert.NotEmpty(t, parsed.Scheme)
	assert.NotEmpty(t, parsed.Host)
}

// TestS3Client_Bucket verifies the bucket name is returned correctly.
func TestS3Client_Bucket(t *testing.T) {
	client := newIntegrationS3Client(t)
	assert.Equal(t, integrationBucket, client.Bucket())
}

