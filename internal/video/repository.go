package video

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, ownerID string, input CreateInput) (Video, error)
	ListByOwner(ctx context.Context, ownerID string, query ListQuery) ([]Video, error)
	GetByID(ctx context.Context, ownerID, videoID string) (Video, error)
	Update(ctx context.Context, ownerID, videoID string, input UpdateInput) (Video, error)
	Delete(ctx context.Context, ownerID, videoID string) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, ownerID string, input CreateInput) (Video, error) {
	const q = `
		INSERT INTO videos (owner_id, object_key, filename, content_type, size_bytes, status)
		VALUES ($1::uuid, $2, $3, $4, $5, 'uploaded')
		RETURNING id::text, owner_id::text, object_key, filename, content_type, size_bytes, status, created_at, updated_at;
	`

	var v Video
	err := r.db.QueryRow(ctx, q, ownerID, input.ObjectKey, input.Filename, input.ContentType, input.SizeBytes).Scan(
		&v.ID, &v.OwnerID, &v.ObjectKey, &v.Filename, &v.ContentType, &v.SizeBytes, &v.Status, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return Video{}, fmt.Errorf("insert video: %w", err)
	}
	return v, nil
}

func (r *PostgresRepository) ListByOwner(ctx context.Context, ownerID string, query ListQuery) ([]Video, error) {
	const baseQuery = `
		SELECT id::text, owner_id::text, object_key, filename, content_type, size_bytes, status, created_at, updated_at
		FROM videos
		WHERE owner_id = $1::uuid
	`

	args := []any{ownerID}
	conditions := []string{}
	param := 2

	if query.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", param))
		args = append(args, query.Status)
		param++
	}
	if query.Q != "" {
		conditions = append(conditions, fmt.Sprintf("(filename ILIKE $%d OR object_key ILIKE $%d)", param, param))
		args = append(args, "%"+query.Q+"%")
		param++
	}

	builder := strings.Builder{}
	builder.WriteString(baseQuery)
	if len(conditions) > 0 {
		builder.WriteString(" AND ")
		builder.WriteString(strings.Join(conditions, " AND "))
	}
	builder.WriteString(fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", param, param+1))
	args = append(args, query.Limit, (query.Page-1)*query.Limit)

	rows, err := r.db.Query(ctx, builder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()

	videos := make([]Video, 0)
	for rows.Next() {
		var v Video
		if scanErr := rows.Scan(&v.ID, &v.OwnerID, &v.ObjectKey, &v.Filename, &v.ContentType, &v.SizeBytes, &v.Status, &v.CreatedAt, &v.UpdatedAt); scanErr != nil {
			return nil, fmt.Errorf("scan video: %w", scanErr)
		}
		videos = append(videos, v)
	}
	return videos, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, ownerID, videoID string) (Video, error) {
	const q = `
		SELECT id::text, owner_id::text, object_key, filename, content_type, size_bytes, status, created_at, updated_at
		FROM videos
		WHERE id = $1::uuid AND owner_id = $2::uuid;
	`

	var v Video
	err := r.db.QueryRow(ctx, q, videoID, ownerID).Scan(
		&v.ID, &v.OwnerID, &v.ObjectKey, &v.Filename, &v.ContentType, &v.SizeBytes, &v.Status, &v.CreatedAt, &v.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrVideoNotFound
	}
	if err != nil {
		return Video{}, fmt.Errorf("get video by id: %w", err)
	}
	return v, nil
}

func (r *PostgresRepository) Update(ctx context.Context, ownerID, videoID string, input UpdateInput) (Video, error) {
	const q = `
		UPDATE videos
		SET
			filename = COALESCE($3, filename),
			content_type = COALESCE($4, content_type),
			status = COALESCE($5, status),
			updated_at = NOW()
		WHERE id = $1::uuid AND owner_id = $2::uuid
		RETURNING id::text, owner_id::text, object_key, filename, content_type, size_bytes, status, created_at, updated_at;
	`

	var v Video
	err := r.db.QueryRow(ctx, q, videoID, ownerID, input.Filename, input.ContentType, input.Status).Scan(
		&v.ID, &v.OwnerID, &v.ObjectKey, &v.Filename, &v.ContentType, &v.SizeBytes, &v.Status, &v.CreatedAt, &v.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Video{}, ErrVideoNotFound
	}
	if err != nil {
		return Video{}, fmt.Errorf("update video: %w", err)
	}
	return v, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, ownerID, videoID string) error {
	const q = `
		DELETE FROM videos
		WHERE id = $1::uuid AND owner_id = $2::uuid;
	`
	tag, err := r.db.Exec(ctx, q, videoID, ownerID)
	if err != nil {
		return fmt.Errorf("delete video: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrVideoNotFound
	}
	return nil
}
