// Package storage provides file storage operations using an S3-compatible backend.
package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/quorant/quorant/internal/platform/config"
)

// StorageClient provides file storage operations.
type StorageClient interface {
	// Upload stores a file in the given bucket at the given key.
	Upload(ctx context.Context, bucket, key, contentType string, reader io.Reader, size int64) error

	// PresignedGetURL returns a pre-signed URL for downloading a file.
	PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)

	// PresignedPutURL returns a pre-signed URL for uploading a file directly.
	PresignedPutURL(ctx context.Context, bucket, key, contentType string, expiry time.Duration) (string, error)

	// Delete removes a file from storage.
	Delete(ctx context.Context, bucket, key string) error
}

// S3Client implements StorageClient using the MinIO SDK (S3-compatible).
type S3Client struct {
	client *minio.Client
	bucket string
}

// NewS3Client creates an S3-compatible storage client from the provided configuration.
func NewS3Client(cfg config.S3Config) (*S3Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}

	return &S3Client{client: client, bucket: cfg.Bucket}, nil
}

// EnsureBucket creates the default bucket if it does not already exist.
func (s *S3Client) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("checking bucket: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("creating bucket: %w", err)
		}
	}
	return nil
}

// Upload stores a file and returns an error if the operation fails.
func (s *S3Client) Upload(ctx context.Context, bucket, key, contentType string, reader io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// PresignedGetURL returns a pre-signed URL for downloading a file.
func (s *S3Client) PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, bucket, key, expiry, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// PresignedPutURL returns a pre-signed URL for uploading a file directly.
func (s *S3Client) PresignedPutURL(ctx context.Context, bucket, key, contentType string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, bucket, key, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Delete removes a file from storage.
func (s *S3Client) Delete(ctx context.Context, bucket, key string) error {
	return s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

// Bucket returns the default bucket name configured for this client.
func (s *S3Client) Bucket() string {
	return s.bucket
}
