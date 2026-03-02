package video

import "time"

type Video struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	ObjectKey   string    `json:"object_key"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateInput struct {
	ObjectKey   string `json:"object_key"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type UpdateInput struct {
	Filename    *string `json:"filename"`
	ContentType *string `json:"content_type"`
	Status      *string `json:"status"`
}
