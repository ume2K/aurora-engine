package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrJobNotFound = errors.New("job not found")

type JobRepository interface {
	Create(ctx context.Context, videoID, jobType, consumerID string) (Job, error)
	GetByVideoID(ctx context.Context, videoID, jobType string) (Job, error)
	UpdateStatus(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error
	IncrementAttempts(ctx context.Context, jobID, consumerID string) error
}

type PostgresJobRepository struct {
	db *pgxpool.Pool
}

func NewPostgresJobRepository(db *pgxpool.Pool) *PostgresJobRepository {
	return &PostgresJobRepository{db: db}
}

func (r *PostgresJobRepository) Create(ctx context.Context, videoID, jobType, consumerID string) (Job, error) {
	const q = `
		INSERT INTO processing_jobs (video_id, job_type, status, attempts, claimed_by, started_at)
		VALUES ($1::uuid, $2, 'processing', 1, $3, NOW())
		RETURNING id::text, video_id::text, job_type, status, error_message, attempts,
		          claimed_by, started_at, finished_at, created_at, updated_at
	`

	var j Job
	err := r.db.QueryRow(ctx, q, videoID, jobType, consumerID).Scan(
		&j.ID, &j.VideoID, &j.JobType, &j.Status, &j.ErrorMessage, &j.Attempts,
		&j.ClaimedBy, &j.StartedAt, &j.FinishedAt, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return Job{}, fmt.Errorf("insert job: %w", err)
	}
	return j, nil
}

func (r *PostgresJobRepository) GetByVideoID(ctx context.Context, videoID, jobType string) (Job, error) {
	const q = `
		SELECT id::text, video_id::text, job_type, status, error_message, attempts,
		       claimed_by, started_at, finished_at, created_at, updated_at
		FROM processing_jobs
		WHERE video_id = $1::uuid AND job_type = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var j Job
	err := r.db.QueryRow(ctx, q, videoID, jobType).Scan(
		&j.ID, &j.VideoID, &j.JobType, &j.Status, &j.ErrorMessage, &j.Attempts,
		&j.ClaimedBy, &j.StartedAt, &j.FinishedAt, &j.CreatedAt, &j.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("get job by video: %w", err)
	}
	return j, nil
}

func (r *PostgresJobRepository) UpdateStatus(ctx context.Context, jobID, status string, errorMsg *string, consumerID string) error {
	const q = `
		UPDATE processing_jobs
		SET status = $2,
		    error_message = $3,
		    claimed_by = $4,
		    finished_at = CASE WHEN $2 IN ('completed', 'failed') THEN NOW() ELSE finished_at END,
		    updated_at = NOW()
		WHERE id = $1::uuid
	`

	tag, err := r.db.Exec(ctx, q, jobID, status, errorMsg, consumerID)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}
	return nil
}

func (r *PostgresJobRepository) IncrementAttempts(ctx context.Context, jobID, consumerID string) error {
	const q = `
		UPDATE processing_jobs
		SET attempts = attempts + 1,
		    claimed_by = $2,
		    started_at = NOW(),
		    status = 'processing',
		    updated_at = NOW()
		WHERE id = $1::uuid
	`

	tag, err := r.db.Exec(ctx, q, jobID, consumerID)
	if err != nil {
		return fmt.Errorf("increment job attempts: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}
	return nil
}
