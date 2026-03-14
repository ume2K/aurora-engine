package worker

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

type ObjectStore interface {
	DownloadToFile(ctx context.Context, bucket, key, destPath string) error
	UploadFromFile(ctx context.Context, bucket, key, srcPath, contentType string) error
}

type S3ObjectStore struct {
	client *minio.Client
}

func NewS3ObjectStore(client *minio.Client) *S3ObjectStore {
	return &S3ObjectStore{client: client}
}

func (s *S3ObjectStore) DownloadToFile(ctx context.Context, bucket, key, destPath string) error {
	if err := s.client.FGetObject(ctx, bucket, key, destPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("s3 download %s/%s: %w", bucket, key, err)
	}
	return nil
}

func (s *S3ObjectStore) UploadFromFile(ctx context.Context, bucket, key, srcPath, contentType string) error {
	_, err := s.client.FPutObject(ctx, bucket, key, srcPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("s3 upload %s/%s: %w", bucket, key, err)
	}
	return nil
}
