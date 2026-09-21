package tui

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaPreviewHalfBlocksPreserveAspectAndCacheBound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wide.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 40, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 5), G: uint8(y * 20), B: 80, A: 255})
		}
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	p := newLocalMediaPreview(1)
	out, err := p.Render(path, 20, 8, ColorTrue)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) > 3 { // 4:1 source should use about 2.5 terminal rows at 20 cols.
		t.Fatalf("preview aspect was stretched: %d rows", len(lines))
	}
	if !strings.Contains(out, "\x1b[38;2;") || !strings.Contains(out, "▀") {
		t.Fatalf("true-colour half blocks missing: %q", out)
	}
	if _, err := p.Render(path, 10, 4, ColorANSI256); err != nil {
		t.Fatal(err)
	}
	if p.lru.Len() != 1 {
		t.Fatalf("LRU size = %d", p.lru.Len())
	}
}

func TestMediaPreviewNoColorUsesMetadataCard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(path, []byte("video"), 0600); err != nil {
		t.Fatal(err)
	}
	p := newLocalMediaPreview(2)
	out, err := p.Render(path, 20, 5, ColorNone)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "MP4 / clip.mp4") || strings.Contains(out, "\x1b[") {
		t.Fatalf("metadata fallback = %q", out)
	}
}
