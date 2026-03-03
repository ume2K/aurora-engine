package worker

import (
	"context"
	"log"
)

type EventHandler struct{}

func NewEventHandler() *EventHandler {
	return &EventHandler{}
}

func (h *EventHandler) HandleMessage(ctx context.Context, msgID string, values map[string]interface{}) error {
	msgType, _ := values["type"].(string)
	videoID, _ := values["video_id"].(string)

	log.Printf("[handler] processing event %s: type=%s video_id=%s", msgID, msgType, videoID)
	return nil
}
