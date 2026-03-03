package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Stream     string
	Group      string
	ConsumerID string

	ReadCount    int64
	BlockTime    time.Duration
	ClaimMinIdle time.Duration
	ClaimTick    time.Duration
}

func DefaultConfig(stream, group, consumerID string) Config {
	return Config{
		Stream:       stream,
		Group:        group,
		ConsumerID:   consumerID,
		ReadCount:    10,
		BlockTime:    2 * time.Second,
		ClaimMinIdle: 5 * time.Second,
		ClaimTick:    3 * time.Second,
	}
}

type MessageHandler func(ctx context.Context, msgID string, values map[string]interface{}) error

type Worker struct {
	redis   *redis.Client
	cfg     Config
	handler MessageHandler
}

func New(redisClient *redis.Client, cfg Config, handler MessageHandler) *Worker {
	return &Worker{
		redis:   redisClient,
		cfg:     cfg,
		handler: handler,
	}
}

func (w *Worker) EnsureConsumerGroup(ctx context.Context) error {
	err := w.redis.XGroupCreateMkStream(ctx, w.cfg.Stream, w.cfg.Group, "$").Err()
	if err != nil {
		if strings.Contains(err.Error(), "BUSYGROUP") {
			log.Printf("[worker] consumer group %q already exists on stream %q", w.cfg.Group, w.cfg.Stream)
			return nil
		}
		return fmt.Errorf("xgroup create: %w", err)
	}
	log.Printf("[worker] created consumer group %q on stream %q", w.cfg.Group, w.cfg.Stream)
	return nil
}

// Run starts the main read loop. It blocks until ctx is cancelled (Graceful Shutdown).
func (w *Worker) Run(ctx context.Context) {
	log.Printf("[worker] starting consumer %q in group %q on stream %q",
		w.cfg.ConsumerID, w.cfg.Group, w.cfg.Stream)

	for {
		if ctx.Err() != nil {
			log.Printf("[worker] context cancelled, stopping read loop")
			return
		}

		streams, err := w.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    w.cfg.Group,
			Consumer: w.cfg.ConsumerID,
			Streams:  []string{w.cfg.Stream, ">"},
			Count:    w.cfg.ReadCount,
			Block:    w.cfg.BlockTime,
		}).Result()

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err == redis.Nil {
				continue
			}
			log.Printf("[worker] xreadgroup error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				w.processMessage(ctx, msg)
			}
		}
	}
}

// RunClaimer periodically checks for pending messages from dead consumers
// and claims them. It blocks until ctx is cancelled.
func (w *Worker) RunClaimer(ctx context.Context) {
	log.Printf("[claimer] starting PEL claimer (interval=%s, min-idle=%s)",
		w.cfg.ClaimTick, w.cfg.ClaimMinIdle)

	ticker := time.NewTicker(w.cfg.ClaimTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[claimer] context cancelled, stopping")
			return
		case <-ticker.C:
			w.claimPending(ctx)
		}
	}
}

func (w *Worker) claimPending(ctx context.Context) {
	pending, err := w.redis.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.cfg.Stream,
		Group:  w.cfg.Group,
		Start:  "-",
		End:    "+",
		Count:  w.cfg.ReadCount,
		Idle:   w.cfg.ClaimMinIdle,
	}).Result()
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("[claimer] xpending error: %v", err)
		}
		return
	}

	if len(pending) == 0 {
		return
	}

	ids := make([]string, 0, len(pending))
	for _, p := range pending {
		if p.Consumer == w.cfg.ConsumerID {
			continue
		}
		ids = append(ids, p.ID)
	}

	if len(ids) == 0 {
		return
	}

	log.Printf("[claimer] attempting to claim %d pending messages", len(ids))

	claimed, err := w.redis.XClaim(ctx, &redis.XClaimArgs{
		Stream:   w.cfg.Stream,
		Group:    w.cfg.Group,
		Consumer: w.cfg.ConsumerID,
		MinIdle:  w.cfg.ClaimMinIdle,
		Messages: ids,
	}).Result()
	if err != nil {
		log.Printf("[claimer] xclaim error: %v", err)
		return
	}

	for _, msg := range claimed {
		log.Printf("[claimer] claimed message %s from dead consumer", msg.ID)
		w.processMessage(ctx, msg)
	}
}

func (w *Worker) processMessage(ctx context.Context, msg redis.XMessage) {
	msgType, _ := msg.Values["type"].(string)
	log.Printf("[worker] processing message %s type=%s", msg.ID, msgType)

	if err := w.handler(ctx, msg.ID, msg.Values); err != nil {
		log.Printf("[worker] handler error for %s: %v (will remain pending)", msg.ID, err)
		return
	}

	if err := w.redis.XAck(ctx, w.cfg.Stream, w.cfg.Group, msg.ID).Err(); err != nil {
		log.Printf("[worker] xack failed for %s: %v", msg.ID, err)
	}
}
