package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

// MinIOStorage implements MediaStorage interface using MinIO's native SDK
type MinIOStorage struct {
	client         *minio.Client
	bucket         string
	publicURL      string
	endpoint       string
	useServerProxy bool
	region         string
}

// NewMinIOStorage creates a new MinIO storage instance using native SDK
func NewMinIOStorage(cfg S3Config) (*MinIOStorage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket name is required")
	}

	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("S3 access key and secret key are required")
	}

	// Parse endpoint to remove protocol
	endpoint := cfg.Endpoint
	useSSL := true
	if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		useSSL = true
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		useSSL = false
	}

	// Initialize MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	logrus.Debugf("Initialized MinIO storage: endpoint=%s, bucket=%s, region=%s, SSL=%v, serverProxy=%v",
		endpoint, cfg.Bucket, cfg.Region, useSSL, cfg.UseServerProxy)

	return &MinIOStorage{
		client:         minioClient,
		bucket:         cfg.Bucket,
		publicURL:      cfg.PublicURL,
		endpoint:       cfg.Endpoint,
		useServerProxy: cfg.UseServerProxy,
		region:         cfg.Region,
	}, nil
}

// Save saves media data to MinIO
func (s *MinIOStorage) Save(ctx context.Context, data []byte, filename string) (string, error) {
	reader := bytes.NewReader(data)

	// Upload to MinIO
	_, err := s.client.PutObject(ctx, s.bucket, filename, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	return filename, nil
}

// SaveStream saves media from a reader stream to MinIO
func (s *MinIOStorage) SaveStream(ctx context.Context, reader io.Reader, filename string) (string, error) {
	// Upload to MinIO with unknown size (-1)
	_, err := s.client.PutObject(ctx, s.bucket, filename, reader, -1, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload stream to MinIO: %w", err)
	}

	return filename, nil
}

// Get retrieves media data from MinIO
func (s *MinIOStorage) Get(ctx context.Context, path string) ([]byte, error) {
	// Download from MinIO
	object, err := s.client.GetObject(ctx, s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from MinIO: %w", err)
	}
	defer object.Close()

	// Read all data
	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read MinIO object: %w", err)
	}

	return data, nil
}

// Delete removes media from MinIO
func (s *MinIOStorage) Delete(ctx context.Context, path string) error {
	err := s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object from MinIO: %w", err)
	}

	return nil
}

// Exists checks if a file exists in MinIO storage
func (s *MinIOStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		// Check if error indicates the object doesn't exist
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" || errResponse.Code == "NotFound" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}
	return true, nil
}

// GetURL returns a publicly accessible URL for the media
func (s *MinIOStorage) GetURL(path string) string {
	// If using server proxy for private bucket, return server download endpoint
	if s.useServerProxy {
		return fmt.Sprintf("/media/download/%s", path)
	}

	// If custom public URL is configured, use it for public bucket
	if s.publicURL != "" {
		base := strings.TrimRight(s.publicURL, "/")
		return fmt.Sprintf("%s/%s/%s", base, s.bucket, path)
	}

	// Otherwise, construct the direct S3/MinIO URL for public bucket
	base := strings.TrimRight(s.endpoint, "/")
	return fmt.Sprintf("%s/%s/%s", base, s.bucket, path)
}

// EnsureBucketExists checks if the bucket exists and creates it if it doesn't
func (s *MinIOStorage) EnsureBucketExists(ctx context.Context) error {
	// Check if bucket exists
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}

	if exists {
		return nil
	}

	// Try to create bucket
	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{
		Region: "us-east-1",
	})
	if err != nil {
		// Check if bucket was created by someone else in the meantime
		exists, existsErr := s.client.BucketExists(ctx, s.bucket)
		if existsErr == nil && exists {
			return nil
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	logrus.Debugf("Created MinIO bucket: %s", s.bucket)
	return nil
}

// TestConnection performs a comprehensive test of MinIO connectivity
func (s *MinIOStorage) TestConnection(ctx context.Context) error {
	// Test 1: List buckets
	_, err := s.client.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	// Test 2: Check if our bucket exists
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket '%s' does not exist", s.bucket)
	}

	// Test 3: Upload test object
	testKey := fmt.Sprintf("test/connection-test-%d.txt", ctx.Value("timestamp"))
	testData := []byte("Connection test from WhatsApp Web API")

	_, err = s.client.PutObject(ctx, s.bucket, testKey, bytes.NewReader(testData),
		int64(len(testData)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload test object: %w", err)
	}

	// Test 4: Download test object
	object, err := s.client.GetObject(ctx, s.bucket, testKey, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get test object: %w", err)
	}
	object.Close()

	// Test 5: Delete test object
	err = s.client.RemoveObject(ctx, s.bucket, testKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete test object: %w", err)
	}

	logrus.Debug("MinIO connection test passed")
	return nil
}
