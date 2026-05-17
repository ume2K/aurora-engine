package video

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

type UploadResult struct {
	Key  string
	Size int64
}

type ObjectStorage interface {
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, string, error)
	PresignGetObjectURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error)
}

type S3Storage struct {
	client        *minio.Client
	presignClient *minio.Client
}

func NewS3Storage(client, presignClient *minio.Client) *S3Storage {
	if presignClient == nil {
		presignClient = client
	}
	return &S3Storage{client: client, presignClient: presignClient}
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

func (s *S3Storage) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, string, error) {
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("s3 get object: %w", err)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, "", fmt.Errorf("s3 stat object: %w", err)
	}

	return obj, info.ContentType, nil
}

func (s *S3Storage) PresignGetObjectURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	u, err := s.presignClient.PresignedGetObject(ctx, bucket, key, expires, url.Values{})
	if err != nil {
		return "", fmt.Errorf("s3 presign get object: %w", err)
	}
	return u.String(), nil
}
