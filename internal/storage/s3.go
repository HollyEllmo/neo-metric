package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// S3Config holds S3/MinIO configuration
type S3Config struct {
	Endpoint        string // e.g., "http://localhost:9000" for MinIO
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Region          string
	PublicURL       string // Public URL for accessing files (e.g., "http://localhost:9000/media")
}

// S3Storage provides S3-compatible storage operations
type S3Storage struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

// NewS3Storage creates a new S3 storage client
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	// Create S3 client with static credentials and custom endpoint
	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		),
		UsePathStyle: true, // Required for MinIO
	})

	return &S3Storage{
		client:    client,
		bucket:    cfg.Bucket,
		publicURL: cfg.PublicURL,
	}, nil
}

// UploadInput represents input for uploading a file
type UploadInput struct {
	Reader      io.Reader
	ContentType string
	Size        int64
	Filename    string // Optional: original filename for extension extraction
}

// UploadOutput represents output from uploading a file
type UploadOutput struct {
	Key       string // Object key in S3
	URL       string // Public URL to access the file
	Size      int64
	UploadedAt time.Time
}

// Upload uploads a file to S3 and returns the public URL
func (s *S3Storage) Upload(ctx context.Context, in UploadInput) (*UploadOutput, error) {
	// Generate unique key
	ext := path.Ext(in.Filename)
	if ext == "" {
		ext = getExtensionFromContentType(in.ContentType)
	}
	key := fmt.Sprintf("%s/%s%s", time.Now().Format("2006/01/02"), uuid.New().String(), ext)

	// Upload to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          in.Reader,
		ContentType:   aws.String(in.ContentType),
		ContentLength: aws.Int64(in.Size),
	})
	if err != nil {
		return nil, fmt.Errorf("uploading to s3: %w", err)
	}

	// Build public URL
	publicURL := fmt.Sprintf("%s/%s", s.publicURL, key)

	return &UploadOutput{
		Key:        key,
		URL:        publicURL,
		Size:       in.Size,
		UploadedAt: time.Now(),
	}, nil
}

// Delete removes a file from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("deleting from s3: %w", err)
	}
	return nil
}

// getExtensionFromContentType returns file extension based on content type
func getExtensionFromContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	default:
		return ""
	}
}
