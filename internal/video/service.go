package video

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrVideoNotFound = errors.New("video not found")
)

var allowedStatuses = map[string]struct{}{
	"uploaded":   {},
	"processing": {},
	"ready":      {},
	"failed":     {},
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, ownerID string, input CreateInput) (Video, error) {
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.Filename = strings.TrimSpace(input.Filename)
	input.ContentType = strings.TrimSpace(input.ContentType)

	if input.ObjectKey == "" || input.Filename == "" || input.ContentType == "" || input.SizeBytes <= 0 {
		return Video{}, ErrInvalidInput
	}
	return s.repo.Create(ctx, ownerID, input)
}

func (s *Service) ListByOwner(ctx context.Context, ownerID string) ([]Video, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

func (s *Service) GetByID(ctx context.Context, ownerID, videoID string) (Video, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return Video{}, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, ownerID, videoID)
}

func (s *Service) Update(ctx context.Context, ownerID, videoID string, input UpdateInput) (Video, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return Video{}, ErrInvalidInput
	}
	if input.Filename != nil {
		v := strings.TrimSpace(*input.Filename)
		if v == "" {
			return Video{}, ErrInvalidInput
		}
		input.Filename = &v
	}
	if input.ContentType != nil {
		v := strings.TrimSpace(*input.ContentType)
		if v == "" {
			return Video{}, ErrInvalidInput
		}
		input.ContentType = &v
	}
	if input.Status != nil {
		v := strings.TrimSpace(*input.Status)
		if _, ok := allowedStatuses[v]; !ok {
			return Video{}, ErrInvalidInput
		}
		input.Status = &v
	}

	if input.Filename == nil && input.ContentType == nil && input.Status == nil {
		return Video{}, ErrInvalidInput
	}
	return s.repo.Update(ctx, ownerID, videoID, input)
}

func (s *Service) Delete(ctx context.Context, ownerID, videoID string) error {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return ErrInvalidInput
	}
	return s.repo.Delete(ctx, ownerID, videoID)
}
