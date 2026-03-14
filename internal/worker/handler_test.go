package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type jobRepoStub struct {
	createFn            func(ctx context.Context, videoID, jobType, consumerID string) (Job, error)
	getByVideoIDFn      func(ctx context.Context, videoID, jobType string) (Job, error)
	updateStatusFn      func(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error
	incrementAttemptsFn func(ctx context.Context, jobID, consumerID string) error
}

func (s jobRepoStub) Create(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
	return s.createFn(ctx, videoID, jobType, consumerID)
}

func (s jobRepoStub) GetByVideoID(ctx context.Context, videoID, jobType string) (Job, error) {
	return s.getByVideoIDFn(ctx, videoID, jobType)
}

func (s jobRepoStub) UpdateStatus(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
	return s.updateStatusFn(ctx, jobID, status, errorMsg, consumerID)
}

func (s jobRepoStub) IncrementAttempts(ctx context.Context, jobID, consumerID string) error {
	return s.incrementAttemptsFn(ctx, jobID, consumerID)
}

type videoUpdaterStub struct {
	updateStatusByIDFn func(ctx context.Context, videoID, status string) error
}

func (s videoUpdaterStub) UpdateStatusByID(ctx context.Context, videoID, status string) error {
	return s.updateStatusByIDFn(ctx, videoID, status)
}

type objectStoreStub struct {
	downloadFn func(ctx context.Context, bucket, key, dest string) error
	uploadFn   func(ctx context.Context, bucket, key, src, contentType string) error
}

func (s objectStoreStub) DownloadToFile(ctx context.Context, bucket, key, dest string) error {
	if s.downloadFn != nil {
		return s.downloadFn(ctx, bucket, key, dest)
	}
	return nil
}

func (s objectStoreStub) UploadFromFile(ctx context.Context, bucket, key, src, contentType string) error {
	if s.uploadFn != nil {
		return s.uploadFn(ctx, bucket, key, src, contentType)
	}
	return nil
}

type transcoderStub struct {
	thumbnailFn    func(ctx context.Context, in, out string) error
	transcode720Fn func(ctx context.Context, in, out string) error
}

func (s transcoderStub) Thumbnail(ctx context.Context, in, out string) error {
	if s.thumbnailFn != nil {
		return s.thumbnailFn(ctx, in, out)
	}
	return nil
}

func (s transcoderStub) Transcode720p(ctx context.Context, in, out string) error {
	if s.transcode720Fn != nil {
		return s.transcode720Fn(ctx, in, out)
	}
	return nil
}

func newTestHandler(jobs jobRepoStub, videos videoUpdaterStub, store objectStoreStub, tc transcoderStub) *EventHandler {
	return NewEventHandler(jobs, videos, store, tc, "test-consumer", "test-uploads", "test-processed")
}

func testPayload() string {
	b, _ := json.Marshal(map[string]any{"object_key": "user-1/abc/demo.mp4"})
	return string(b)
}

func testValues() map[string]interface{} {
	return map[string]interface{}{
		"type":     "video_uploaded",
		"video_id": "vid-42",
		"payload":  testPayload(),
	}
}

func TestHandleMessage_HappyPath(t *testing.T) {
	var createdJobTypes []string
	var videoStatuses []string
	var completedJobs []string

	jobs := jobRepoStub{
		getByVideoIDFn: func(ctx context.Context, videoID, jobType string) (Job, error) {
			return Job{}, ErrJobNotFound
		},
		createFn: func(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
			createdJobTypes = append(createdJobTypes, jobType)
			return Job{ID: "job-" + jobType, VideoID: videoID, JobType: jobType, Status: "processing"}, nil
		},
		updateStatusFn: func(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
			if status == "completed" {
				completedJobs = append(completedJobs, jobID)
			}
			return nil
		},
		incrementAttemptsFn: func(ctx context.Context, jobID, consumerID string) error {
			return nil
		},
	}

	videos := videoUpdaterStub{
		updateStatusByIDFn: func(ctx context.Context, videoID, status string) error {
			videoStatuses = append(videoStatuses, status)
			return nil
		},
	}

	var thumbCalled, transcodeCalled bool
	tc := transcoderStub{
		thumbnailFn: func(ctx context.Context, in, out string) error {
			thumbCalled = true
			return nil
		},
		transcode720Fn: func(ctx context.Context, in, out string) error {
			transcodeCalled = true
			return nil
		},
	}

	h := newTestHandler(jobs, videos, objectStoreStub{}, tc)
	err := h.HandleMessage(context.Background(), "msg-1", testValues())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(createdJobTypes) != 2 || createdJobTypes[0] != "thumbnail" || createdJobTypes[1] != "transcode" {
		t.Fatalf("expected job types [thumbnail, transcode], got %v", createdJobTypes)
	}
	if !thumbCalled {
		t.Fatal("Thumbnail transcoder was not called")
	}
	if !transcodeCalled {
		t.Fatal("Transcode720p transcoder was not called")
	}
	if len(videoStatuses) != 2 || videoStatuses[0] != "processing" || videoStatuses[1] != "ready" {
		t.Fatalf("expected video statuses [processing, ready], got %v", videoStatuses)
	}
	if len(completedJobs) != 2 {
		t.Fatalf("expected 2 completed jobs, got %d", len(completedJobs))
	}
}

func TestHandleMessage_Idempotent_AllCompleted(t *testing.T) {
	var createCalled bool

	jobs := jobRepoStub{
		getByVideoIDFn: func(ctx context.Context, videoID, jobType string) (Job, error) {
			return Job{ID: "job-" + jobType, VideoID: videoID, Status: "completed"}, nil
		},
		createFn: func(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
			createCalled = true
			return Job{}, nil
		},
		updateStatusFn: func(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
			t.Fatal("UpdateStatus should not be called when all jobs completed")
			return nil
		},
		incrementAttemptsFn: func(ctx context.Context, jobID, consumerID string) error {
			t.Fatal("IncrementAttempts should not be called when all jobs completed")
			return nil
		},
	}

	videos := videoUpdaterStub{
		updateStatusByIDFn: func(ctx context.Context, videoID, status string) error {
			t.Fatal("UpdateStatusByID should not be called when all jobs completed")
			return nil
		},
	}

	tc := transcoderStub{
		thumbnailFn: func(ctx context.Context, in, out string) error {
			t.Fatal("Thumbnail should not be called when all jobs completed")
			return nil
		},
		transcode720Fn: func(ctx context.Context, in, out string) error {
			t.Fatal("Transcode720p should not be called when all jobs completed")
			return nil
		},
	}

	h := newTestHandler(jobs, videos, objectStoreStub{}, tc)
	err := h.HandleMessage(context.Background(), "msg-1", testValues())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if createCalled {
		t.Fatal("Create should not be called when all jobs completed")
	}
}

func TestHandleMessage_ContextCancellation(t *testing.T) {
	var jobFinalStatus string
	var videoFinalStatus string

	jobs := jobRepoStub{
		getByVideoIDFn: func(ctx context.Context, videoID, jobType string) (Job, error) {
			return Job{}, ErrJobNotFound
		},
		createFn: func(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
			return Job{ID: "job-1", VideoID: videoID, Status: "processing"}, nil
		},
		updateStatusFn: func(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
			jobFinalStatus = status
			return nil
		},
		incrementAttemptsFn: func(ctx context.Context, jobID, consumerID string) error {
			return nil
		},
	}

	videos := videoUpdaterStub{
		updateStatusByIDFn: func(ctx context.Context, videoID, status string) error {
			videoFinalStatus = status
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	tc := transcoderStub{
		thumbnailFn: func(ctx context.Context, in, out string) error {
			cancel()
			return ctx.Err()
		},
	}

	h := newTestHandler(jobs, videos, objectStoreStub{}, tc)
	err := h.HandleMessage(ctx, "msg-1", testValues())
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if jobFinalStatus != "failed" {
		t.Fatalf("expected job status failed, got %s", jobFinalStatus)
	}
	if videoFinalStatus != "failed" {
		t.Fatalf("expected video status failed, got %s", videoFinalStatus)
	}
}

func TestHandleMessage_Retry_ThumbnailDone_TranscodeFailed(t *testing.T) {
	var incrementCalled bool
	var transcodeCalled bool

	jobs := jobRepoStub{
		getByVideoIDFn: func(ctx context.Context, videoID, jobType string) (Job, error) {
			if jobType == "thumbnail" {
				return Job{ID: "job-thumb", VideoID: videoID, Status: "completed"}, nil
			}
			return Job{ID: "job-trans", VideoID: videoID, Status: "failed", Attempts: 1}, nil
		},
		createFn: func(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
			t.Fatalf("Create should not be called for existing jobs, got jobType=%s", jobType)
			return Job{}, nil
		},
		updateStatusFn: func(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
			return nil
		},
		incrementAttemptsFn: func(ctx context.Context, jobID, consumerID string) error {
			if jobID != "job-trans" {
				t.Fatalf("expected IncrementAttempts for job-trans, got %s", jobID)
			}
			incrementCalled = true
			return nil
		},
	}

	videos := videoUpdaterStub{
		updateStatusByIDFn: func(ctx context.Context, videoID, status string) error {
			return nil
		},
	}

	tc := transcoderStub{
		thumbnailFn: func(ctx context.Context, in, out string) error {
			t.Fatal("Thumbnail should not be called when already completed")
			return nil
		},
		transcode720Fn: func(ctx context.Context, in, out string) error {
			transcodeCalled = true
			return nil
		},
	}

	h := newTestHandler(jobs, videos, objectStoreStub{}, tc)
	err := h.HandleMessage(context.Background(), "msg-1", testValues())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !incrementCalled {
		t.Fatal("IncrementAttempts should be called for failed transcode job")
	}
	if !transcodeCalled {
		t.Fatal("Transcode720p should be called for retry")
	}
}

func TestHandleMessage_DBError(t *testing.T) {
	dbErr := errors.New("connection refused")

	jobs := jobRepoStub{
		getByVideoIDFn: func(ctx context.Context, videoID, jobType string) (Job, error) {
			return Job{}, dbErr
		},
		createFn: func(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
			return Job{}, nil
		},
		updateStatusFn: func(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
			return nil
		},
		incrementAttemptsFn: func(ctx context.Context, jobID, consumerID string) error {
			return nil
		},
	}

	videos := videoUpdaterStub{
		updateStatusByIDFn: func(ctx context.Context, videoID, status string) error {
			return nil
		},
	}

	h := newTestHandler(jobs, videos, objectStoreStub{}, transcoderStub{})
	err := h.HandleMessage(context.Background(), "msg-1", testValues())
	if err == nil {
		t.Fatal("expected error from DB failure")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped dbErr, got: %v", err)
	}
}

func TestHandleMessage_TranscoderError(t *testing.T) {
	var jobFinalStatus string
	var videoFinalStatus string

	jobs := jobRepoStub{
		getByVideoIDFn: func(ctx context.Context, videoID, jobType string) (Job, error) {
			return Job{}, ErrJobNotFound
		},
		createFn: func(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
			return Job{ID: "job-1", VideoID: videoID, Status: "processing"}, nil
		},
		updateStatusFn: func(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
			jobFinalStatus = status
			return nil
		},
		incrementAttemptsFn: func(ctx context.Context, jobID, consumerID string) error {
			return nil
		},
	}

	videos := videoUpdaterStub{
		updateStatusByIDFn: func(ctx context.Context, videoID, status string) error {
			videoFinalStatus = status
			return nil
		},
	}

	ffmpegErr := errors.New("ffmpeg: exit status 1")
	tc := transcoderStub{
		thumbnailFn: func(ctx context.Context, in, out string) error {
			return ffmpegErr
		},
	}

	h := newTestHandler(jobs, videos, objectStoreStub{}, tc)
	err := h.HandleMessage(context.Background(), "msg-1", testValues())
	if err == nil {
		t.Fatal("expected error from transcoder failure")
	}
	if jobFinalStatus != "failed" {
		t.Fatalf("expected job status failed, got %s", jobFinalStatus)
	}
	if videoFinalStatus != "failed" {
		t.Fatalf("expected video status failed, got %s", videoFinalStatus)
	}
}

