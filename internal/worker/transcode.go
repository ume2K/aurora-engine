package worker

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
)

type Transcoder interface {
	Thumbnail(ctx context.Context, inputPath, outputPath string) error
	Transcode720p(ctx context.Context, inputPath, outputPath string) error
}

type FFmpegTranscoder struct{}

func NewFFmpegTranscoder() *FFmpegTranscoder {
	return &FFmpegTranscoder{}
}

func (t *FFmpegTranscoder) Thumbnail(ctx context.Context, inputPath, outputPath string) error {
	return runFFmpeg(ctx, "-y",
		"-ss", "00:00:01",
		"-i", inputPath,
		"-vframes", "1",
		"-vf", "scale=320:-2",
		outputPath,
	)
}

func (t *FFmpegTranscoder) Transcode720p(ctx context.Context, inputPath, outputPath string) error {
	return runFFmpeg(ctx, "-y",
		"-i", inputPath,
		"-vf", "scale=-2:720",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "28",
		"-c:a", "aac",
		"-movflags", "+faststart",
		outputPath,
	)
}

func runFFmpeg(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("[ffmpeg] running: ffmpeg %v", args)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("ffmpeg cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("ffmpeg failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}
