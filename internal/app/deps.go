package app

import (
	"context"
	"fmt"
	"time"

	"gocore/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
	S3    *minio.Client
}

func NewDeps(ctx context.Context, cfg config.Config) (*Deps, error) {
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pingPostgres(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	if err := pingRedis(ctx, redisClient); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}

	s3Client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
		Region: cfg.S3Region,
	})
	if err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("create s3 client: %w", err)
	}

	if err := pingS3(ctx, s3Client); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}
	if err := ensureBucket(ctx, s3Client, cfg.S3BucketUploads, cfg.S3Region); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}
	if err := ensureBucket(ctx, s3Client, cfg.S3BucketProcessed, cfg.S3Region); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}

	return &Deps{
		DB:    db,
		Redis: redisClient,
		S3:    s3Client,
	}, nil
}

func (d *Deps) Close() {
	if d == nil {
		return
	}
	if d.Redis != nil {
		_ = d.Redis.Close()
	}
	if d.DB != nil {
		d.DB.Close()
	}
}

func pingPostgres(parent context.Context, db *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

func pingRedis(parent context.Context, rdb *redis.Client) error {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

func pingS3(parent context.Context, s3 *minio.Client) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	if _, err := s3.ListBuckets(ctx); err != nil {
		return fmt.Errorf("ping s3/rustfs: %w", err)
	}
	return nil
}

func ensureBucket(parent context.Context, s3 *minio.Client, bucketName, region string) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	exists, err := s3.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("check bucket %q: %w", bucketName, err)
	}
	if exists {
		return nil
	}

	if err := s3.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: region}); err != nil {
		return fmt.Errorf("create bucket %q: %w", bucketName, err)
	}
	return nil
}
