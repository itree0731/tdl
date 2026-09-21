package up

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"strings"

	"github.com/gabriel-vasile/mimetype"

	"github.com/iyear/tdl/core/util/mediautil"
)

const (
	telegramThumbnailMaxDimension = 320
	telegramThumbnailMaxBytes     = 200 * 1024
)

var findFFmpeg = func() (string, error) {
	return exec.LookPath("ffmpeg")
}

var runFFmpeg = func(ctx context.Context, executable string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, executable, args...).CombinedOutput()
}

// prepareThumbnail keeps an explicit sidecar when one exists. Otherwise it
// generates a temporary JPEG for video files, unless automatic thumbnails were
// disabled for this upload.
func prepareThumbnail(ctx context.Context, mediaPath, sidecarPath string, disableAuto bool) (path string, temporary bool, err error) {
	if sidecarPath != "" {
		if err := validateThumbnail(sidecarPath); err != nil {
			return "", false, fmt.Errorf("invalid thumbnail %q: %w", sidecarPath, err)
		}
		return sidecarPath, false, nil
	}
	if disableAuto {
		return "", false, nil
	}

	mime, err := mimetype.DetectFile(mediaPath)
	if err != nil {
		return "", false, fmt.Errorf("detect media type for automatic thumbnail: %w", err)
	}
	if !mediautil.IsVideo(mime.String()) {
		return "", false, nil
	}

	ffmpeg, err := findFFmpeg()
	if err != nil {
		return "", false, fmt.Errorf("automatic video thumbnail requires ffmpeg in PATH; install ffmpeg or pass --no-auto-thumb: %w", err)
	}

	tmp, err := os.CreateTemp("", "tdl-video-thumb-*.jpg")
	if err != nil {
		return "", false, fmt.Errorf("create temporary video thumbnail: %w", err)
	}
	thumbPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(thumbPath)
		return "", false, fmt.Errorf("close temporary video thumbnail: %w", err)
	}
	_ = os.Remove(thumbPath) // ffmpeg creates the output itself.

	generate := func(seek string) ([]byte, error) {
		args := []string{"-hide_banner", "-loglevel", "error", "-y"}
		if seek != "" {
			args = append(args, "-ss", seek)
		}
		args = append(args,
			"-i", mediaPath,
			"-map", "0:v:0",
			"-frames:v", "1",
			"-vf", "scale=320:320:force_original_aspect_ratio=decrease",
			"-q:v", "5",
			"-f", "image2",
			thumbPath,
		)
		return runFFmpeg(ctx, ffmpeg, args...)
	}

	output, generateErr := generate("1")
	if generateErr != nil {
		_ = os.Remove(thumbPath)
		output, generateErr = generate("") // also supports sub-second videos.
	}
	if generateErr != nil {
		_ = os.Remove(thumbPath)
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = generateErr.Error()
		}
		return "", false, fmt.Errorf("generate automatic video thumbnail: %s", message)
	}
	if err := validateThumbnail(thumbPath); err != nil {
		_ = os.Remove(thumbPath)
		return "", false, fmt.Errorf("generated video thumbnail is invalid: %w", err)
	}
	return thumbPath, true, nil
}

func validateThumbnail(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() <= 0 {
		return fmt.Errorf("file is empty")
	}
	if info.Size() > telegramThumbnailMaxBytes {
		return fmt.Errorf("file is %d bytes; Telegram thumbnails must be at most %d bytes", info.Size(), telegramThumbnailMaxBytes)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	config, format, err := image.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("decode JPEG: %w", err)
	}
	if format != "jpeg" {
		return fmt.Errorf("format is %s; Telegram thumbnails must be JPEG", format)
	}
	if config.Width < 1 || config.Height < 1 {
		return fmt.Errorf("invalid dimensions %dx%d", config.Width, config.Height)
	}
	if config.Width > telegramThumbnailMaxDimension || config.Height > telegramThumbnailMaxDimension {
		return fmt.Errorf("dimensions are %dx%d; maximum is %dx%d", config.Width, config.Height, telegramThumbnailMaxDimension, telegramThumbnailMaxDimension)
	}
	return nil
}
