package worker

import "time"

type Job struct {
	ID           string     `json:"id"`
	VideoID      string     `json:"video_id"`
	JobType      string     `json:"job_type"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	Attempts     int        `json:"attempts"`
	ClaimedBy    *string    `json:"claimed_by,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
