package video

import (
	"context"
	"log"
	"time"
)

type Event struct {
	Type      string         `json:"type"`
	VideoID   string         `json:"video_id"`
	OwnerID   string         `json:"owner_id"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type EventPublisher interface {
	Publish(ctx context.Context, event Event) error
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, event Event) error {
	log.Printf("[event-stub] %s video_id=%s owner=%s", event.Type, event.VideoID, event.OwnerID)
	return nil
}
