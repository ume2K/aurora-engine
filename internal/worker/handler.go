package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var jobTypes = []string{"thumbnail", "transcode"}

type VideoUpdater interface {
	UpdateStatusByID(ctx context.Context, videoID, status string) error
}

type EventHandler struct {
	jobs       JobRepository
	videos     VideoUpdater
	store      ObjectStore
	transcoder Transcoder
	consumerID string
	bucketIn   string
	bucketOut  string
}

func NewEventHandler(jobs JobRepository, videos VideoUpdater, store ObjectStore,
	transcoder Transcoder, consumerID, bucketIn, bucketOut string) *EventHandler {
	return &EventHandler{
		jobs:       jobs,
		videos:     videos,
		store:      store,
		transcoder: transcoder,
		consumerID: consumerID,
		bucketIn:   bucketIn,
		bucketOut:  bucketOut,
	}
}

func (h *EventHandler) HandleMessage(ctx context.Context, msgID string, values map[string]interface{}) error {
	videoID, _ := values["video_id"].(string)
	msgType, _ := values["type"].(string)

	if videoID == "" {
		log.Printf("[handler] skipping message %s: missing video_id", msgID)
		return nil
	}

	log.Printf("[handler] processing event %s: type=%s video_id=%s", msgID, msgType, videoID)

	objectKey, err := extractObjectKey(values)
	if err != nil {
		log.Printf("[handler] skipping message %s: %v", msgID, err)
		return nil
	}

	if h.allJobsCompleted(ctx, videoID) {
		log.Printf("[handler] all jobs for video %s already completed, skipping", videoID)
		return nil
	}

	if err := h.videos.UpdateStatusByID(ctx, videoID, "processing"); err != nil {
		return fmt.Errorf("set video processing: %w", err)
	}

	for _, jobType := range jobTypes {
		if err := h.processJob(ctx, videoID, objectKey, jobType); err != nil {
			return err
		}
	}

	if err := h.videos.UpdateStatusByID(ctx, videoID, "ready"); err != nil {
		return fmt.Errorf("set video ready: %w", err)
	}

	log.Printf("[handler] all jobs for video %s completed successfully", videoID)
	return nil
}

func (h *EventHandler) processJob(ctx context.Context, videoID, objectKey, jobType string) error {
	job, err := h.jobs.GetByVideoID(ctx, videoID, jobType)
	if err != nil && !errors.Is(err, ErrJobNotFound) {
		return fmt.Errorf("check existing %s job: %w", jobType, err)
	}

	if err == nil && job.Status == "completed" {
		log.Printf("[handler] %s job %s for video %s already completed, skipping", jobType, job.ID, videoID)
		return nil
	}

	var jobID string
	if errors.Is(err, ErrJobNotFound) {
		created, createErr := h.jobs.Create(ctx, videoID, jobType, h.consumerID)
		if createErr != nil {
			return fmt.Errorf("create %s job: %w", jobType, createErr)
		}
		jobID = created.ID
		log.Printf("[handler] created %s job %s for video %s", jobType, jobID, videoID)
	} else {
		jobID = job.ID
		if err := h.jobs.IncrementAttempts(ctx, jobID, h.consumerID); err != nil {
			return fmt.Errorf("increment %s attempts: %w", jobType, err)
		}
		log.Printf("[handler] retrying %s job %s for video %s (was %s)", jobType, jobID, videoID, job.Status)
	}

	if err := h.runTranscode(ctx, videoID, objectKey, jobType); err != nil {
		errMsg := err.Error()
		h.markJobFailed(ctx, jobID, videoID, errMsg)
		return err
	}

	if err := h.jobs.UpdateStatus(ctx, jobID, "completed", nil, h.consumerID); err != nil {
		errMsg := fmt.Sprintf("update %s job completed: %v", jobType, err)
		h.markJobFailed(ctx, jobID, videoID, errMsg)
		return fmt.Errorf("set %s job completed: %w", jobType, err)
	}

	log.Printf("[handler] %s job %s for video %s completed", jobType, jobID, videoID)
	return nil
}

func (h *EventHandler) runTranscode(ctx context.Context, videoID, objectKey, jobType string) error {
	tmpDir, err := os.MkdirTemp("", "aurora-"+jobType+"-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input")
	if err := h.store.DownloadToFile(ctx, h.bucketIn, objectKey, inputPath); err != nil {
		return fmt.Errorf("download source: %w", err)
	}

	var outputFile, outputKey, contentType string
	switch jobType {
	case "thumbnail":
		outputFile = "thumb.jpg"
		outputKey = videoID + "/thumb.jpg"
		contentType = "image/jpeg"
	case "transcode":
		outputFile = "720p.mp4"
		outputKey = videoID + "/720p.mp4"
		contentType = "video/mp4"
	default:
		return fmt.Errorf("unknown job type: %s", jobType)
	}

	outputPath := filepath.Join(tmpDir, outputFile)

	switch jobType {
	case "thumbnail":
		if err := h.transcoder.Thumbnail(ctx, inputPath, outputPath); err != nil {
			return fmt.Errorf("ffmpeg thumbnail: %w", err)
		}
	case "transcode":
		if err := h.transcoder.Transcode720p(ctx, inputPath, outputPath); err != nil {
			return fmt.Errorf("ffmpeg transcode: %w", err)
		}
	}

	if err := h.store.UploadFromFile(ctx, h.bucketOut, outputKey, outputPath, contentType); err != nil {
		return fmt.Errorf("upload result: %w", err)
	}

	return nil
}

func (h *EventHandler) allJobsCompleted(ctx context.Context, videoID string) bool {
	for _, jt := range jobTypes {
		job, err := h.jobs.GetByVideoID(ctx, videoID, jt)
		if err != nil || job.Status != "completed" {
			return false
		}
	}
	return true
}

func (h *EventHandler) markJobFailed(ctx context.Context, jobID, videoID, errMsg string) {
	if updateErr := h.jobs.UpdateStatus(ctx, jobID, "failed", &errMsg, h.consumerID); updateErr != nil {
		log.Printf("[handler] failed to mark job %s as failed: %v", jobID, updateErr)
	}
	if updateErr := h.videos.UpdateStatusByID(ctx, videoID, "failed"); updateErr != nil {
		log.Printf("[handler] failed to mark video %s as failed: %v", videoID, updateErr)
	}
}

func extractObjectKey(values map[string]interface{}) (string, error) {
	payloadStr, ok := values["payload"].(string)
	if !ok || payloadStr == "" {
		return "", fmt.Errorf("missing payload field")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}
	objectKey, ok := payload["object_key"].(string)
	if !ok || objectKey == "" {
		return "", fmt.Errorf("missing object_key in payload")
	}
	return objectKey, nil
}
