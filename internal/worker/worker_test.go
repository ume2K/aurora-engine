package worker

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("my-stream", "my-group", "consumer-1")

	if cfg.Stream != "my-stream" {
		t.Fatalf("expected stream my-stream, got %s", cfg.Stream)
	}
	if cfg.Group != "my-group" {
		t.Fatalf("expected group my-group, got %s", cfg.Group)
	}
	if cfg.ConsumerID != "consumer-1" {
		t.Fatalf("expected consumer consumer-1, got %s", cfg.ConsumerID)
	}
	if cfg.ReadCount <= 0 {
		t.Fatal("expected positive ReadCount")
	}
	if cfg.BlockTime <= 0 {
		t.Fatal("expected positive BlockTime")
	}
	if cfg.ClaimMinIdle <= 0 {
		t.Fatal("expected positive ClaimMinIdle")
	}
	if cfg.ClaimTick <= 0 {
		t.Fatal("expected positive ClaimTick")
	}
}

func TestMessageHandler_CalledWithValues(t *testing.T) {
	var capturedID string
	var capturedType string

	handler := MessageHandler(func(ctx context.Context, msgID string, values map[string]interface{}) error {
		capturedID = msgID
		if v, ok := values["type"].(string); ok {
			capturedType = v
		}
		return nil
	})

	err := handler(context.Background(), "1234-0", map[string]interface{}{
		"type":     "video_uploaded",
		"video_id": "vid-42",
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if capturedID != "1234-0" {
		t.Fatalf("expected msgID 1234-0, got %s", capturedID)
	}
	if capturedType != "video_uploaded" {
		t.Fatalf("expected type video_uploaded, got %s", capturedType)
	}
}

func TestMessageHandler_ErrorPropagation(t *testing.T) {
	handler := MessageHandler(func(ctx context.Context, msgID string, values map[string]interface{}) error {
		return errors.New("processing failed")
	})

	err := handler(context.Background(), "1234-0", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error from handler")
	}
	if err.Error() != "processing failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}
