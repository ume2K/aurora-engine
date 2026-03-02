package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, id string) (User, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id::text, email, role, password_hash, created_at;
	`

	var u User
	err := r.db.QueryRow(ctx, q, strings.ToLower(strings.TrimSpace(email)), passwordHash).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.PasswordHash,
		&u.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrEmailAlreadyExists
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `
		SELECT id::text, email, role, password_hash, created_at
		FROM users
		WHERE email = $1;
	`

	var u User
	err := r.db.QueryRow(ctx, q, strings.ToLower(strings.TrimSpace(email))).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.PasswordHash,
		&u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("select user by email: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, id string) (User, error) {
	const q = `
		SELECT id::text, email, role, password_hash, created_at
		FROM users
		WHERE id = $1;
	`

	var u User
	err := r.db.QueryRow(ctx, q, id).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.PasswordHash,
		&u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user by id: %w", err)
	}
	return u, nil
}
