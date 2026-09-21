package up

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 80, A: 255})
		}
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeMinimalMP4(t *testing.T, path string) {
	t.Helper()
	// Enough for mimetype.DetectFile to classify the fixture as video/mp4.
	data := []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '2'}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareThumbnailGeneratesForVideoWithoutSidecar(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	writeMinimalMP4(t, video)

	oldRunner := runFFmpeg
	oldFinder := findFFmpeg
	findFFmpeg = func() (string, error) { return "test-ffmpeg", nil }
	runFFmpeg = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		writeJPEG(t, args[len(args)-1], 320, 180)
		return nil, nil
	}
	t.Cleanup(func() {
		runFFmpeg = oldRunner
		findFFmpeg = oldFinder
	})

	thumb, temporary, err := prepareThumbnail(context.Background(), video, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if thumb == "" || !temporary {
		t.Fatalf("thumb=%q temporary=%v", thumb, temporary)
	}
	if err := validateThumbnail(thumb); err != nil {
		t.Fatalf("generated thumbnail invalid: %v", err)
	}
	if err := os.Remove(thumb); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareThumbnailPrefersValidSidecar(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	sidecar := filepath.Join(dir, "clip.thumb")
	writeMinimalMP4(t, video)
	writeJPEG(t, sidecar, 160, 90)

	called := false
	oldRunner := runFFmpeg
	oldFinder := findFFmpeg
	findFFmpeg = func() (string, error) { return "test-ffmpeg", nil }
	runFFmpeg = func(context.Context, string, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	t.Cleanup(func() {
		runFFmpeg = oldRunner
		findFFmpeg = oldFinder
	})

	thumb, temporary, err := prepareThumbnail(context.Background(), video, sidecar, false)
	if err != nil {
		t.Fatal(err)
	}
	if thumb != sidecar || temporary || called {
		t.Fatalf("thumb=%q temporary=%v ffmpegCalled=%v", thumb, temporary, called)
	}
}

func TestPrepareThumbnailCanBeDisabled(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	writeMinimalMP4(t, video)

	called := false
	oldRunner := runFFmpeg
	oldFinder := findFFmpeg
	findFFmpeg = func() (string, error) { return "test-ffmpeg", nil }
	runFFmpeg = func(context.Context, string, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	t.Cleanup(func() {
		runFFmpeg = oldRunner
		findFFmpeg = oldFinder
	})

	thumb, temporary, err := prepareThumbnail(context.Background(), video, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if thumb != "" || temporary || called {
		t.Fatalf("thumb=%q temporary=%v ffmpegCalled=%v", thumb, temporary, called)
	}
}

func TestPrepareThumbnailReportsMissingFFmpeg(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	writeMinimalMP4(t, video)

	oldFinder := findFFmpeg
	findFFmpeg = func() (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { findFFmpeg = oldFinder })

	_, _, err := prepareThumbnail(context.Background(), video, "", false)
	if err == nil || !strings.Contains(err.Error(), "--no-auto-thumb") {
		t.Fatalf("error=%v", err)
	}
}

func TestPrepareThumbnailWithRealFFmpeg(t *testing.T) {
	ffmpeg, err := findFFmpeg()
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "portrait.mp4")
	output, err := runFFmpeg(context.Background(), ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=blue:s=720x1280:d=2",
		"-pix_fmt", "yuv420p", "-c:v", "libx264", video,
	)
	if err != nil {
		t.Fatalf("create video fixture: %v: %s", err, output)
	}

	thumb, temporary, err := prepareThumbnail(context.Background(), video, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !temporary {
		t.Fatal("real ffmpeg thumbnail must be temporary")
	}
	if err := validateThumbnail(thumb); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(thumb); err != nil {
		t.Fatal(err)
	}
}
