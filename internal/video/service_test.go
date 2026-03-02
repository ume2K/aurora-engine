package video

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
)

type videoRepoStub struct {
	createFn func(ctx context.Context, ownerID string, input CreateInput) (Video, error)
	listFn   func(ctx context.Context, ownerID string, query ListQuery) ([]Video, error)
}

func (s videoRepoStub) Create(ctx context.Context, ownerID string, input CreateInput) (Video, error) {
	return s.createFn(ctx, ownerID, input)
}

func (s videoRepoStub) ListByOwner(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) {
	return s.listFn(ctx, ownerID, query)
}

func (s videoRepoStub) GetByID(ctx context.Context, ownerID, videoID string) (Video, error) {
	return Video{}, nil
}

func (s videoRepoStub) Update(ctx context.Context, ownerID, videoID string, input UpdateInput) (Video, error) {
	return Video{}, nil
}

func (s videoRepoStub) Delete(ctx context.Context, ownerID, videoID string) error {
	return nil
}

type storageStub struct {
	putFn    func(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error)
	deleteFn func(ctx context.Context, bucket, key string) error
}

func (s storageStub) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error) {
	if s.putFn != nil {
		return s.putFn(ctx, bucket, key, reader, size, contentType)
	}
	return UploadResult{Key: key, Size: 1024}, nil
}

func (s storageStub) DeleteObject(ctx context.Context, bucket, key string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, bucket, key)
	}
	return nil
}

func defaultRepo() videoRepoStub {
	return videoRepoStub{
		createFn: func(ctx context.Context, ownerID string, input CreateInput) (Video, error) {
			return Video{ID: "vid-1", OwnerID: ownerID, ObjectKey: input.ObjectKey, Status: "uploaded", SizeBytes: input.SizeBytes}, nil
		},
		listFn: func(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) {
			return nil, nil
		},
	}
}

func newTestService(repo videoRepoStub, storage storageStub) *Service {
	return NewService(repo, storage, NoopPublisher{}, "test-bucket")
}

func TestCreate_HappyPath(t *testing.T) {
	repo := videoRepoStub{
		createFn: func(ctx context.Context, ownerID string, input CreateInput) (Video, error) {
			if ownerID != "user-1" {
				t.Fatalf("unexpected ownerID: %s", ownerID)
			}
			return Video{ID: "vid-1", OwnerID: ownerID, Status: "uploaded"}, nil
		},
		listFn: func(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) { return nil, nil },
	}
	svc := newTestService(repo, storageStub{})

	video, err := svc.Create(context.Background(), "user-1", CreateInput{
		ObjectKey:   "uploads/a.mp4",
		Filename:    "a.mp4",
		ContentType: "video/mp4",
		SizeBytes:   42,
	})
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if video.ID == "" {
		t.Fatalf("expected created video id")
	}
}

func TestListByOwnerWithQuery_InvalidStatus(t *testing.T) {
	svc := newTestService(defaultRepo(), storageStub{})

	_, err := svc.ListByOwnerWithQuery(context.Background(), "user-1", ListQuery{
		Page:   1,
		Limit:  20,
		Status: "unknown",
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestUpload_HappyPath(t *testing.T) {
	var capturedKey string
	repo := videoRepoStub{
		createFn: func(ctx context.Context, ownerID string, input CreateInput) (Video, error) {
			capturedKey = input.ObjectKey
			return Video{
				ID:        "vid-99",
				OwnerID:   ownerID,
				ObjectKey: input.ObjectKey,
				Filename:  input.Filename,
				SizeBytes: input.SizeBytes,
				Status:    "uploaded",
			}, nil
		},
		listFn: func(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) { return nil, nil },
	}
	storage := storageStub{
		putFn: func(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error) {
			if bucket != "test-bucket" {
				t.Fatalf("unexpected bucket: %s", bucket)
			}
			data, _ := io.ReadAll(reader)
			return UploadResult{Key: key, Size: int64(len(data))}, nil
		},
	}
	svc := newTestService(repo, storage)

	body := strings.NewReader("fake-video-content")
	v, err := svc.Upload(context.Background(), "user-42", body, "demo.mp4", "video/mp4")
	if err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	if v.ID != "vid-99" {
		t.Fatalf("expected vid-99, got %s", v.ID)
	}
	if !strings.HasPrefix(capturedKey, "user-42/") {
		t.Fatalf("object key should start with owner ID, got: %s", capturedKey)
	}
	if !strings.HasSuffix(capturedKey, "/demo.mp4") {
		t.Fatalf("object key should end with filename, got: %s", capturedKey)
	}
}

func TestUpload_EmptyFilename(t *testing.T) {
	svc := newTestService(defaultRepo(), storageStub{})

	_, err := svc.Upload(context.Background(), "user-1", strings.NewReader("data"), "", "video/mp4")
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestUpload_StorageFailure(t *testing.T) {
	storage := storageStub{
		putFn: func(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error) {
			return UploadResult{}, fmt.Errorf("s3 connection refused")
		},
	}
	svc := newTestService(defaultRepo(), storage)

	_, err := svc.Upload(context.Background(), "user-1", strings.NewReader("data"), "test.mp4", "video/mp4")
	if err == nil {
		t.Fatal("expected error on storage failure")
	}
}

func TestUpload_DBFailure_CleansUpS3(t *testing.T) {
	var deletedKey string
	repo := videoRepoStub{
		createFn: func(ctx context.Context, ownerID string, input CreateInput) (Video, error) {
			return Video{}, fmt.Errorf("db connection lost")
		},
		listFn: func(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) { return nil, nil },
	}
	storage := storageStub{
		putFn: func(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (UploadResult, error) {
			return UploadResult{Key: key, Size: 100}, nil
		},
		deleteFn: func(ctx context.Context, bucket, key string) error {
			deletedKey = key
			return nil
		},
	}
	svc := newTestService(repo, storage)

	_, err := svc.Upload(context.Background(), "user-1", strings.NewReader("data"), "test.mp4", "video/mp4")
	if err == nil {
		t.Fatal("expected error on DB failure")
	}
	if deletedKey == "" {
		t.Fatal("expected S3 cleanup after DB failure")
	}
}
