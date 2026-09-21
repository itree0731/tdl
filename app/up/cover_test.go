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
	"time"
)

func TestParseCoverMode(t *testing.T) {
	for _, tc := range []struct {
		value string
		old   bool
		want  CoverMode
	}{
		{"", false, CoverVideoCover},
		{"video-cover", false, CoverVideoCover},
		{"thumbnail", false, CoverThumbnail},
		{"off", false, CoverOff},
		{"", true, CoverOff},
	} {
		got, err := parseCoverMode(tc.value, tc.old)
		if err != nil || got != tc.want {
			t.Fatalf("parseCoverMode(%q,%v) = %q,%v", tc.value, tc.old, got, err)
		}
	}
	if _, err := parseCoverMode("bad", false); err == nil {
		t.Fatal("invalid cover mode accepted")
	}
}

func TestCoverTimestampManualAndAutoClamp(t *testing.T) {
	got, err := coverTimestamp("unused", "12.5s")
	if err != nil || got != 12500*time.Millisecond {
		t.Fatalf("manual timestamp = %v, %v", got, err)
	}
	if _, err := coverTimestamp("unused", "yesterday"); err == nil {
		t.Fatal("invalid timestamp accepted")
	}
}

func TestPrepareCoversUsesLanczosQ2AndCleansByCaller(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	writeMinimalMP4(t, video)
	oldFind, oldRun := findFFmpeg, runFFmpeg
	findFFmpeg = func() (string, error) { return "ffmpeg", nil }
	var calls [][]string
	runFFmpeg = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		out := args[len(args)-1]
		longest := 1280
		for _, arg := range args {
			if strings.Contains(arg, "scale=320:320") {
				longest = 320
			}
		}
		f, err := os.Create(out)
		if err != nil {
			return nil, err
		}
		img := image.NewRGBA(image.Rect(0, 0, longest, max(1, longest/2)))
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				img.Set(x, y, color.RGBA{R: 160, G: 100, B: 70, A: 255})
			}
		}
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
		_ = f.Close()
		return nil, err
	}
	t.Cleanup(func() { findFFmpeg, runFFmpeg = oldFind, oldRun })

	prepared, err := prepareCovers(context.Background(), video, "", "", CoverVideoCover, "1s")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || prepared.Cover == "" || prepared.Thumb == "" || prepared.VideoTimestamp != 1 {
		t.Fatalf("prepared=%+v calls=%d", prepared, len(calls))
	}
	for _, call := range calls {
		joined := strings.Join(call, " ")
		if !strings.Contains(joined, "flags=lanczos") || !strings.Contains(joined, "-q:v 2") {
			t.Fatalf("low-quality ffmpeg args: %s", joined)
		}
	}
	for _, path := range prepared.Temporary {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}
