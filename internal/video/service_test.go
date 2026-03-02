package video

import (
	"context"
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
	svc := NewService(repo)

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
	repo := videoRepoStub{
		createFn: func(ctx context.Context, ownerID string, input CreateInput) (Video, error) { return Video{}, nil },
		listFn:   func(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) { return nil, nil },
	}
	svc := NewService(repo)

	_, err := svc.ListByOwnerWithQuery(context.Background(), "user-1", ListQuery{
		Page:   1,
		Limit:  20,
		Status: "unknown",
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}
