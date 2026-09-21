package tui

import (
	"bytes"
	"container/list"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type MediaPreview interface {
	Render(path string, widthCells, heightCells int, profile ColorProfile) (string, error)
}

type mediaPreviewMsg struct {
	screenID uint64
	runID    uint64
	path     string
	text     string
	err      error
}

type previewCacheEntry struct {
	key, value string
}

type localMediaPreview struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	lru      *list.List
}

func newLocalMediaPreview(capacity int) *localMediaPreview {
	return &localMediaPreview{capacity: max(1, capacity), items: make(map[string]*list.Element), lru: list.New()}
}

func (p *localMediaPreview) Render(path string, widthCells, heightCells int, profile ColorProfile) (string, error) {
	if widthCells < 1 || heightCells < 1 {
		return "", fmt.Errorf("invalid preview size")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s|%d|%d|%d|%d|%d", path, info.Size(), info.ModTime().UnixNano(), widthCells, heightCells, profile)
	if value, ok := p.get(key); ok {
		return value, nil
	}
	if profile == ColorNone || profile == ColorANSI16 {
		value := mediaInfoCard(path, info.Size())
		p.put(key, value)
		return value, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(f)
	_ = f.Close()
	if err != nil {
		img, err = decodeVideoFrame(path, widthCells, heightCells)
		if err != nil {
			value := mediaInfoCard(path, info.Size())
			p.put(key, value)
			return value, nil
		}
	}
	value := renderHalfBlocks(img, widthCells, heightCells, profile)
	p.put(key, value)
	return value, nil
}

func decodeVideoFrame(path string, widthCells, heightCells int) (image.Image, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}
	maxW := max(2, widthCells)
	maxH := max(2, heightCells*2)
	cmd := exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-ss", "1", "-i", path,
		"-map", "0:v:0", "-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos", maxW, maxH),
		"-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1",
	)
	data, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func (p *localMediaPreview) get(key string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.items[key]
	if !ok {
		return "", false
	}
	p.lru.MoveToFront(e)
	return e.Value.(previewCacheEntry).value, true
}

func (p *localMediaPreview) put(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.items[key]; ok {
		e.Value = previewCacheEntry{key: key, value: value}
		p.lru.MoveToFront(e)
		return
	}
	e := p.lru.PushFront(previewCacheEntry{key: key, value: value})
	p.items[key] = e
	for p.lru.Len() > p.capacity {
		last := p.lru.Back()
		delete(p.items, last.Value.(previewCacheEntry).key)
		p.lru.Remove(last)
	}
}

func renderHalfBlocks(src image.Image, widthCells, heightCells int, profile ColorProfile) string {
	b := src.Bounds()
	scale := math.Min(float64(widthCells)/float64(b.Dx()), float64(heightCells*2)/float64(b.Dy()))
	if scale <= 0 {
		return ""
	}
	w := max(1, min(widthCells, int(math.Round(float64(b.Dx())*scale))))
	h := max(2, min(heightCells*2, int(math.Round(float64(b.Dy())*scale))))
	if h%2 != 0 {
		h++
	}
	var out strings.Builder
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			top := sampleRGB(src, b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h)
			bottom := sampleRGB(src, b.Min.X+x*b.Dx()/w, b.Min.Y+min(h-1, y+1)*b.Dy()/h)
			if profile == ColorANSI256 {
				fmt.Fprintf(&out, "\x1b[38;5;%dm\x1b[48;5;%dm▀", rgb256(top), rgb256(bottom))
			} else {
				fmt.Fprintf(&out, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", top[0], top[1], top[2], bottom[0], bottom[1], bottom[2])
			}
		}
		out.WriteString("\x1b[0m")
		if y+2 < h {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func sampleRGB(img image.Image, x, y int) [3]uint8 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

func rgb256(rgb [3]uint8) int {
	r := int(math.Round(float64(rgb[0]) / 255 * 5))
	g := int(math.Round(float64(rgb[1]) / 255 * 5))
	b := int(math.Round(float64(rgb[2]) / 255 * 5))
	return 16 + 36*r + 6*g + b
}

func mediaInfoCard(path string, size int64) string {
	ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(path)), ".")
	if ext == "" {
		ext = "FILE"
	}
	return fmt.Sprintf("%s / %s / %s", ext, filepath.Base(path), formatBytes(size))
}
