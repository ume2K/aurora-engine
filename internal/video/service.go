package video

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput           = errors.New("invalid input")
	ErrVideoNotFound          = errors.New("video not found")
	ErrFileTooLarge           = errors.New("file exceeds maximum allowed size")
	ErrVideoNotReady          = errors.New("video not ready")
	ErrProcessedAssetNotFound = errors.New("processed asset not found")
	ErrStreamURLUnavailable   = errors.New("stream url unavailable")
)

const streamURLTTL = 15 * time.Minute

var allowedStatuses = map[string]struct{}{
	"uploaded":   {},
	"processing": {},
	"ready":      {},
	"failed":     {},
}

type Service struct {
	repo            Repository
	storage         ObjectStorage
	events          EventPublisher
	bucketUploads   string
	bucketProcessed string
}

func NewService(repo Repository, storage ObjectStorage, events EventPublisher, bucketUploads, bucketProcessed string) *Service {
	return &Service{
		repo:            repo,
		storage:         storage,
		events:          events,
		bucketUploads:   bucketUploads,
		bucketProcessed: bucketProcessed,
	}
}

func (s *Service) Upload(ctx context.Context, ownerID string, file io.Reader, filename, contentType string) (Video, error) {
	filename = strings.TrimSpace(filename)
	contentType = strings.TrimSpace(contentType)

	if filename == "" {
		return Video{}, ErrInvalidInput
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	objectKey := fmt.Sprintf("%s/%s/%s", ownerID, uuid.New().String(), filename)

	info, err := s.storage.PutObject(ctx, s.bucketUploads, objectKey, file, -1, contentType)
	if err != nil {
		return Video{}, fmt.Errorf("upload to storage: %w", err)
	}

	input := CreateInput{
		ObjectKey:   objectKey,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   info.Size,
	}

	v, err := s.repo.Create(ctx, ownerID, input)
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if delErr := s.storage.DeleteObject(cleanupCtx, s.bucketUploads, objectKey); delErr != nil {
			log.Printf("cleanup s3 object %q failed: %v", objectKey, delErr)
		}
		return Video{}, fmt.Errorf("save video metadata: %w", err)
	}

	s.events.Publish(ctx, Event{
		Type:      "video_uploaded",
		VideoID:   v.ID,
		OwnerID:   ownerID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"object_key":   objectKey,
			"filename":     filename,
			"content_type": contentType,
			"size_bytes":   info.Size,
		},
	})

	return v, nil
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
	return s.ListByOwnerWithQuery(ctx, ownerID, ListQuery{Page: 1, Limit: 20})
}

func (s *Service) ListByOwnerWithQuery(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	query.Status = strings.TrimSpace(query.Status)
	if query.Status != "" {
		if _, ok := allowedStatuses[query.Status]; !ok {
			return nil, ErrInvalidInput
		}
	}
	query.Q = strings.TrimSpace(query.Q)
	return s.repo.ListByOwner(ctx, ownerID, query)
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

func (s *Service) OpenThumbnail(ctx context.Context, ownerID, videoID string) (io.ReadCloser, string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, "", ErrInvalidInput
	}

	v, err := s.repo.GetByID(ctx, ownerID, videoID)
	if err != nil {
		return nil, "", err
	}
	if v.Status != "ready" {
		return nil, "", ErrVideoNotReady
	}

	key := fmt.Sprintf("%s/thumb.jpg", videoID)
	reader, contentType, err := s.storage.GetObject(ctx, s.bucketProcessed, key)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such key") {
			return nil, "", ErrProcessedAssetNotFound
		}
		return nil, "", fmt.Errorf("open thumbnail: %w", err)
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "image/jpeg"
	}
	return reader, contentType, nil
}

func (s *Service) OpenProcessedVideo(ctx context.Context, ownerID, videoID string) (io.ReadCloser, string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, "", ErrInvalidInput
	}

	v, err := s.repo.GetByID(ctx, ownerID, videoID)
	if err != nil {
		return nil, "", err
	}
	if v.Status != "ready" {
		return nil, "", ErrVideoNotReady
	}

	key := fmt.Sprintf("%s/720p.mp4", videoID)
	reader, contentType, err := s.storage.GetObject(ctx, s.bucketProcessed, key)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such key") {
			return nil, "", ErrProcessedAssetNotFound
		}
		return nil, "", fmt.Errorf("open processed video: %w", err)
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "video/mp4"
	}
	return reader, contentType, nil
}

func (s *Service) GetStreamURL(ctx context.Context, ownerID, videoID string) (string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return "", ErrInvalidInput
	}

	v, err := s.repo.GetByID(ctx, ownerID, videoID)
	if err != nil {
		return "", err
	}
	if v.Status != "ready" {
		return "", ErrVideoNotReady
	}

	key := fmt.Sprintf("%s/720p.mp4", videoID)
	streamURL, err := s.storage.PresignGetObjectURL(ctx, s.bucketProcessed, key, streamURLTTL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrStreamURLUnavailable, err)
	}
	return streamURL, nil
}
