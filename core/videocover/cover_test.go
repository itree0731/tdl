package videocover

import (
	"context"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	for _, tc := range []struct {
		at       string
		duration time.Duration
		want     time.Duration
	}{
		{"12.5s", 0, 12500 * time.Millisecond},
		{"auto", 500 * time.Millisecond, 0},
		{"auto", 50 * time.Second, 5 * time.Second},
		{"auto", 10 * time.Minute, 30 * time.Second},
	} {
		got, err := Timestamp("missing.mp4", tc.at, tc.duration)
		if err != nil || got != tc.want {
			t.Fatalf("at=%q duration=%s: got=%s err=%v want=%s", tc.at, tc.duration, got, err, tc.want)
		}
	}
	if _, err := Timestamp("missing.mp4", "yesterday", 0); err == nil {
		t.Fatal("accepted invalid cover time")
	}
}

func TestPrepareRealShortVideoAndCleanup(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	video := filepath.Join(t.TempDir(), "short.mp4")
	command := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=640x360:r=2:d=1.5", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", video)
	if output, runErr := command.CombinedOutput(); runErr != nil {
		t.Fatalf("make video: %v: %s", runErr, output)
	}
	prepared, err := Prepare(context.Background(), video, "50s", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{prepared.Cover, prepared.Thumb} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing generated image %s: %v", path, err)
		}
	}
	if err := validateJPEG(prepared.Cover, 1280); err != nil {
		t.Fatal(err)
	}
	coverFile, err := os.Open(prepared.Cover)
	if err != nil {
		t.Fatal(err)
	}
	coverSize, _, err := image.DecodeConfig(coverFile)
	_ = coverFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cover dimensions: %dx%d", coverSize.Width, coverSize.Height)
	if err := validateJPEG(prepared.Thumb, 320); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{prepared.Cover, prepared.Thumb} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary image remains %s: %v", path, err)
		}
	}
}

func TestProbeNonMP4VideoWithFFprobe(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	if _, err = exec.LookPath("ffprobe"); err != nil {
		t.Skipf("ffprobe unavailable: %v", err)
	}
	video := filepath.Join(t.TempDir(), "portrait.mkv")
	command := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=360x640:r=2:d=2", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", video)
	if output, runErr := command.CombinedOutput(); runErr != nil {
		t.Fatalf("make mkv: %v: %s", runErr, output)
	}
	duration, width, height, err := Probe(context.Background(), video)
	if err != nil || duration <= 0 || width != 360 || height != 640 {
		t.Fatalf("duration=%s size=%dx%d err=%v", duration, width, height, err)
	}
	prepared, err := Prepare(context.Background(), video, "auto", duration)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	if err := validateJPEG(prepared.Cover, 1280); err != nil {
		t.Fatal(err)
	}
}
