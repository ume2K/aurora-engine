package video

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisPublisher struct {
	client *redis.Client
	stream string
}

func NewRedisPublisher(client *redis.Client, stream string) *RedisPublisher {
	return &RedisPublisher{client: client, stream: stream}
}

func (p *RedisPublisher) Publish(ctx context.Context, event Event) error {
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: p.stream,
		ID:     "*",
		Values: map[string]any{
			"type":      event.Type,
			"video_id":  event.VideoID,
			"owner_id":  event.OwnerID,
			"timestamp": event.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			"payload":   string(payloadJSON),
		},
	}

	if err := p.client.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("xadd to stream %q: %w", p.stream, err)
	}
	return nil
}
