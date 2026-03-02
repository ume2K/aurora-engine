package video

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

type UploadResult struct {
	Key  string
	Size int64
}

type ObjectStorage interface {
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error)
	DeleteObject(ctx context.Context, bucket, key string) error
}

type S3Storage struct {
	client *minio.Client
}

func NewS3Storage(client *minio.Client) *S3Storage {
	return &S3Storage{client: client}
}

func (s *S3Storage) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error) {
	info, err := s.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return UploadResult{}, fmt.Errorf("s3 put object: %w", err)
	}
	return UploadResult{Key: info.Key, Size: info.Size}, nil
}

func (s *S3Storage) DeleteObject(ctx context.Context, bucket, key string) error {
	if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}
