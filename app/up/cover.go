package up

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/iyear/tdl/core/util/mediautil"
)

type CoverMode string

const (
	CoverVideoCover CoverMode = "video-cover"
	CoverThumbnail  CoverMode = "thumbnail"
	CoverOff        CoverMode = "off"
)

const coverSidecarSuffix = ".cover.jpg"

type preparedCovers struct {
	Thumb, Cover   string
	Temporary      []string
	VideoTimestamp int
}

func parseCoverMode(value string, noAutoThumb bool) (CoverMode, error) {
	if noAutoThumb {
		if value != "" {
			return "", fmt.Errorf("--no-auto-thumb conflicts with --cover-mode")
		}
		return CoverOff, nil
	}
	if value == "" {
		return CoverVideoCover, nil
	}
	mode := CoverMode(value)
	switch mode {
	case CoverVideoCover, CoverThumbnail, CoverOff:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid cover mode %q; use video-cover, thumbnail, or off", value)
	}
}

func coverTimestamp(path, at string) (time.Duration, error) {
	if at != "" && at != "auto" {
		d, err := time.ParseDuration(at)
		if err != nil || d < 0 {
			return 0, fmt.Errorf("invalid --cover-at %q; use auto or a duration such as 12s", at)
		}
		return d, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	duration, _, _, err := mediautil.GetMP4Info(f)
	if err != nil {
		return time.Second, nil
	}
	seconds := float64(duration) * 0.1
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func prepareCovers(ctx context.Context, mediaPath, thumbSidecar, coverSidecar string, mode CoverMode, at string) (preparedCovers, error) {
	if mode == CoverOff {
		return preparedCovers{}, nil
	}
	if mode == CoverThumbnail {
		thumb, temporary, err := prepareThumbnail(ctx, mediaPath, thumbSidecar, false)
		result := preparedCovers{Thumb: thumb}
		if temporary {
			result.Temporary = append(result.Temporary, thumb)
		}
		return result, err
	}

	mime, err := mimetype.DetectFile(mediaPath)
	if err != nil {
		return preparedCovers{}, fmt.Errorf("detect media type for video cover: %w", err)
	}
	if !mediautil.IsVideo(mime.String()) {
		// Non-video documents may still use an explicit legacy thumbnail.
		if thumbSidecar == "" {
			return preparedCovers{}, nil
		}
		if err := validateThumbnail(thumbSidecar); err != nil {
			return preparedCovers{}, err
		}
		return preparedCovers{Thumb: thumbSidecar}, nil
	}

	ffmpeg, err := findFFmpeg()
	if err != nil {
		return preparedCovers{}, fmt.Errorf("video-cover requires ffmpeg in PATH; use --cover-mode off to disable: %w", err)
	}
	seek, err := coverTimestamp(mediaPath, at)
	if err != nil {
		return preparedCovers{}, err
	}
	result := preparedCovers{VideoTimestamp: int(seek.Seconds())}
	cleanup := func() {
		for _, path := range result.Temporary {
			_ = os.Remove(path)
		}
	}

	source := mediaPath
	seekArg := formatFFmpegSeek(seek)
	if coverSidecar != "" {
		source = coverSidecar
		seekArg = ""
	}
	result.Cover, err = generateCoverJPEG(ctx, ffmpeg, source, seekArg, 1280)
	if err != nil {
		cleanup()
		return preparedCovers{}, fmt.Errorf("generate high-resolution video cover: %w", err)
	}
	result.Temporary = append(result.Temporary, result.Cover)
	if err := validateVideoCover(result.Cover); err != nil {
		cleanup()
		return preparedCovers{}, fmt.Errorf("generated video cover is invalid: %w", err)
	}

	if thumbSidecar != "" {
		if err := validateThumbnail(thumbSidecar); err != nil {
			cleanup()
			return preparedCovers{}, fmt.Errorf("invalid thumbnail %q: %w", thumbSidecar, err)
		}
		result.Thumb = thumbSidecar
	} else {
		result.Thumb, err = generateCoverJPEG(ctx, ffmpeg, source, seekArg, telegramThumbnailMaxDimension)
		if err != nil {
			cleanup()
			return preparedCovers{}, fmt.Errorf("generate video thumbnail: %w", err)
		}
		result.Temporary = append(result.Temporary, result.Thumb)
		if err := validateThumbnail(result.Thumb); err != nil {
			cleanup()
			return preparedCovers{}, fmt.Errorf("generated video thumbnail is invalid: %w", err)
		}
	}
	return result, nil
}

func validateVideoCover(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	config, format, err := image.DecodeConfig(f)
	if err != nil {
		return err
	}
	if format != "jpeg" {
		return fmt.Errorf("format is %s; video cover must be JPEG", format)
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 1280 || config.Height > 1280 {
		return fmt.Errorf("invalid dimensions %dx%d; longest edge must be at most 1280", config.Width, config.Height)
	}
	return nil
}

func generateCoverJPEG(ctx context.Context, ffmpeg, source, seek string, longest int) (string, error) {
	tmp, err := os.CreateTemp("", "tdl-video-cover-*.jpg")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	_ = os.Remove(path)
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if seek != "" {
		args = append(args, "-ss", seek)
	}
	args = append(args, "-i", source, "-map", "0:v:0", "-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos", longest, longest),
		"-q:v", "2", "-f", "image2", path)
	output, err := runFFmpeg(ctx, ffmpeg, args...)
	if err != nil {
		_ = os.Remove(path)
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s", message)
	}
	return path, nil
}

func formatFFmpegSeek(d time.Duration) string {
	seconds := d.Seconds()
	return strconv.FormatFloat(seconds, 'f', 3, 64)
}

func coverSidecarPath(mediaPath string) string {
	return strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath)) + coverSidecarSuffix
}
