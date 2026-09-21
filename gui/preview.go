package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/iyear/tdl/core/util/mediautil"
)

type MediaPreview struct {
	DataURL string `json:"dataURL"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

var previewFFmpeg = func() (string, error) { return exec.LookPath("ffmpeg") }
var previewCommand = func(ctx context.Context, executable string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, executable, args...).Output()
}

func (a *App) MediaPreview(paths []string) (MediaPreview, error) {
	path, kind, err := firstPreviewMedia(paths)
	if err != nil {
		return MediaPreview{}, err
	}
	ffmpeg, err := previewFFmpeg()
	if err != nil {
		return MediaPreview{}, fmt.Errorf("生成媒体预览需要 ffmpeg: %w", err)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if kind == "video" {
		args = append(args, "-ss", "1")
	}
	args = append(args, "-i", path, "-map", "0:v:0", "-frames:v", "1",
		"-vf", "scale=720:720:force_original_aspect_ratio=decrease:flags=lanczos",
		"-q:v", "3", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1")
	data, err := previewCommand(ctx, ffmpeg, args...)
	if err != nil && kind == "video" {
		// Sub-second videos may not have a frame at one second.
		args = removePreviewSeek(args)
		data, err = previewCommand(ctx, ffmpeg, args...)
	}
	if err != nil {
		return MediaPreview{}, fmt.Errorf("生成媒体预览失败: %w", err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return MediaPreview{}, fmt.Errorf("读取媒体预览失败: %w", err)
	}
	return MediaPreview{
		DataURL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data),
		Kind:    kind,
		Path:    path,
		Name:    filepath.Base(path),
		Width:   config.Width,
		Height:  config.Height,
	}, nil
}

func firstPreviewMedia(paths []string) (string, string, error) {
	checked := 0
	for _, input := range paths {
		info, err := os.Stat(input)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if kind := previewKind(input); kind != "" {
				return input, kind, nil
			}
			continue
		}
		var foundPath, foundKind string
		stop := fmt.Errorf("preview found")
		err = filepath.WalkDir(input, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			checked++
			if checked > 2000 {
				return fs.SkipAll
			}
			if kind := previewKind(path); kind != "" {
				foundPath, foundKind = path, kind
				return stop
			}
			return nil
		})
		if foundPath != "" {
			return foundPath, foundKind, nil
		}
		if err != nil && err != stop {
			continue
		}
	}
	return "", "", fmt.Errorf("所选内容中没有可预览的图片或视频")
}

func previewKind(path string) string {
	mime, err := mimetype.DetectFile(path)
	if err != nil {
		return ""
	}
	value := mime.String()
	if strings.HasPrefix(value, "image/") {
		return "image"
	}
	if mediautil.IsVideo(value) {
		return "video"
	}
	return ""
}

func removePreviewSeek(args []string) []string {
	result := make([]string, 0, len(args)-2)
	for i := 0; i < len(args); i++ {
		if args[i] == "-ss" && i+1 < len(args) {
			i++
			continue
		}
		result = append(result, args[i])
	}
	return result
}
