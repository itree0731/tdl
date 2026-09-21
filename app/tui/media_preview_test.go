package tui

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type countingPreview struct{ calls int }

func (p *countingPreview) Render(path string, _, _ int, _ ColorProfile) (string, error) {
	p.calls++
	if path == "bad" {
		return "", fmt.Errorf("bad preview")
	}
	return "PREVIEW:" + path, nil
}

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

func TestModelLoadsPreviewOffRenderPathAndDropsStaleResult(t *testing.T) {
	isolateSettings(t)
	renderer := &countingPreview{}
	m := sized(t, newModel(stubExec, nil), 120, 30)
	m.mediaPreview = renderer
	m.running = true
	m.showOutput = true
	m.runID = 7
	m.screenID = 3
	cmd := m.startMediaPreview("clip.mp4")
	if cmd == nil || !m.previewLoading || renderer.calls != 0 {
		t.Fatalf("preview did not schedule asynchronously: loading=%v calls=%d", m.previewLoading, renderer.calls)
	}
	_ = m.View()
	if renderer.calls != 0 {
		t.Fatal("View performed synchronous media decoding")
	}
	msg := cmd()
	if renderer.calls != 1 {
		t.Fatalf("preview calls=%d", renderer.calls)
	}
	r, _ := m.Update(msg)
	m = asModel(r)
	if m.previewText != "PREVIEW:clip.mp4" || m.previewLoading {
		t.Fatalf("preview result not applied: text=%q loading=%v", m.previewText, m.previewLoading)
	}

	m.previewText = "current"
	r, _ = m.Update(mediaPreviewMsg{screenID: 2, runID: 7, path: "clip.mp4", text: "stale"})
	if got := asModel(r).previewText; got != "current" {
		t.Fatalf("stale preview replaced current result: %q", got)
	}
}
