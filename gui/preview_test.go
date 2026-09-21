package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaPreviewFindsImageAndReturnsBoundedDataURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.jpg")
	data := jpegBytes(t, 4, 3)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	oldFinder, oldCommand := previewFFmpeg, previewCommand
	previewFFmpeg = func() (string, error) { return "test-ffmpeg", nil }
	previewCommand = func(context.Context, string, ...string) ([]byte, error) { return data, nil }
	t.Cleanup(func() { previewFFmpeg, previewCommand = oldFinder, oldCommand })
	app := NewApp()
	app.ctx = context.Background()
	preview, err := app.MediaPreview([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Kind != "image" || preview.Path != path || preview.Width != 4 || preview.Height != 3 {
		t.Fatalf("preview=%+v", preview)
	}
	if !strings.HasPrefix(preview.DataURL, "data:image/jpeg;base64,") {
		t.Fatalf("unexpected data URL: %q", preview.DataURL)
	}
}

func TestRemovePreviewSeek(t *testing.T) {
	got := strings.Join(removePreviewSeek([]string{"-y", "-ss", "1", "-i", "clip.mp4"}), " ")
	if got != "-y -i clip.mp4" {
		t.Fatalf("args=%q", got)
	}
}

func TestMediaPreviewWithRealFFmpegVideo(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	video := filepath.Join(t.TempDir(), "portrait.mp4")
	command := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=360x640:d=1.5", "-pix_fmt", "yuv420p", video)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate video: %v: %s", err, output)
	}
	app := NewApp()
	app.ctx = context.Background()
	preview, err := app.MediaPreview([]string{video})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Kind != "video" || preview.Width != 405 || preview.Height != 720 || preview.DataURL == "" {
		t.Fatalf("kind=%s size=%dx%d data=%t", preview.Kind, preview.Width, preview.Height, preview.DataURL != "")
	}
}

func jpegBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 32, G: 96, B: 160, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
