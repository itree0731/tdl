package videocover

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iyear/tdl/core/util/mediautil"
)

// Prepared contains temporary JPEGs. The caller must Close it after the
// Telegram request completes, including when that request fails.
type Prepared struct {
	Cover string
	Thumb string
}

func (p Prepared) Close() error {
	var err error
	for _, path := range []string{p.Cover, p.Thumb} {
		if path != "" {
			err = errors.Join(err, os.Remove(path))
		}
	}
	return err
}

// Prepare extracts a high-resolution cover and Telegram-compatible thumbnail.
// A requested frame beyond the end of a short clip falls back to the first
// frame. CoverAt never becomes a playback timestamp.
func Prepare(ctx context.Context, videoPath, coverAt string, durationHint time.Duration) (Prepared, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return Prepared{}, fmt.Errorf("video cover requires ffmpeg in PATH: %w", err)
	}
	seek, err := Timestamp(videoPath, coverAt, durationHint)
	if err != nil {
		return Prepared{}, err
	}
	result := Prepared{}
	result.Cover, err = extract(ctx, ffmpeg, videoPath, seek, 1280)
	if err != nil {
		return Prepared{}, fmt.Errorf("generate video cover: %w", err)
	}
	result.Thumb, err = extract(ctx, ffmpeg, videoPath, seek, 320)
	if err != nil {
		_ = result.Close()
		return Prepared{}, fmt.Errorf("generate video thumbnail: %w", err)
	}
	return result, nil
}

// Timestamp accepts auto or a Go duration such as 12s. For videos under one
// second, auto uses the first frame rather than seeking past the end.
func Timestamp(videoPath, at string, durationHint time.Duration) (time.Duration, error) {
	if at != "" && at != "auto" {
		seek, err := time.ParseDuration(at)
		if err != nil || seek < 0 {
			return 0, fmt.Errorf("invalid cover time %q; use auto or a duration such as 12s", at)
		}
		return seek, nil
	}
	duration := durationHint
	if file, err := os.Open(videoPath); err == nil {
		if seconds, _, _, infoErr := mediautil.GetMP4Info(file); infoErr == nil && seconds > 0 {
			duration = time.Duration(seconds) * time.Second
		}
		_ = file.Close()
	}
	if duration > 0 && duration < time.Second {
		return 0, nil
	}
	if duration <= 0 {
		return time.Second, nil
	}
	seek := duration / 10
	if seek < time.Second {
		seek = time.Second
	}
	if seek > 30*time.Second {
		seek = 30 * time.Second
	}
	return seek, nil
}

func extract(ctx context.Context, ffmpeg, source string, seek time.Duration, longest int) (string, error) {
	file, err := os.CreateTemp("", "tdl-video-cover-*.jpg")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err = file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	_ = os.Remove(path)
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(path)
		}
	}()
	for attempt, position := range []time.Duration{seek, 0} {
		if attempt > 0 && seek == 0 {
			break
		}
		args := []string{"-hide_banner", "-loglevel", "error", "-y"}
		if position > 0 {
			args = append(args, "-ss", strconv.FormatFloat(position.Seconds(), 'f', 3, 64))
		}
		args = append(args, "-i", source, "-map", "0:v:0", "-frames:v", "1",
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos", longest, longest),
			"-q:v", "2", "-f", "image2", path)
		output, runErr := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput()
		if runErr == nil {
			if validErr := validateJPEG(path, longest); validErr == nil {
				keep = true
				return path, nil
			} else {
				err = validErr
			}
		} else {
			err = fmt.Errorf("ffmpeg: %s: %w", strings.TrimSpace(string(output)), runErr)
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		_ = os.Remove(path)
	}
	return "", err
}

func validateJPEG(path string, longest int) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return err
	}
	if format != "jpeg" || config.Width < 1 || config.Height < 1 || config.Width > longest || config.Height > longest {
		return fmt.Errorf("invalid JPEG cover dimensions %dx%d", config.Width, config.Height)
	}
	return nil
}
