package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	tdlcmd "github.com/iyear/tdl/cmd"
	"github.com/iyear/tdl/pkg/kv"
	xprogress "github.com/iyear/tdl/pkg/progress"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}
type Capabilities struct {
	Product       string `json:"product"`
	VideoCover    bool   `json:"videoCover"`
	StartAtZero   bool   `json:"startAtZero"`
	SafeBatch     bool   `json:"safeBatch"`
	SafeDownloads bool   `json:"safeDownloads"`
}
type UploadRequest struct {
	Namespace string   `json:"namespace"`
	Chat      string   `json:"chat"`
	CoverMode string   `json:"coverMode"`
	CoverAt   string   `json:"coverAt"`
	Topic     int      `json:"topic"`
	Threads   int      `json:"threads"`
	Limit     int      `json:"limit"`
	Paths     []string `json:"paths"`
	Remove    bool     `json:"remove"`
	AsPhoto   bool     `json:"asPhoto"`
}
type StartResult struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

func NewApp() *App                         { return &App{} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) Capabilities() Capabilities  { return Capabilities{"TDL Desktop", true, true, true, true} }
func (a *App) Namespaces() ([]string, error) {
	store, err := kv.NewWithMap(tdlcmd.DefaultBoltStorage)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.Namespaces()
}
func (a *App) SelectUploadFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择上传文件"})
}
func (a *App) SelectUploadDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择上传目录"})
}

func (a *App) StartUpload(req UploadRequest) (StartResult, error) {
	if len(req.Paths) == 0 {
		return StartResult{}, fmt.Errorf("请选择至少一个文件或目录")
	}
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return StartResult{}, fmt.Errorf("已有传输任务正在运行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel, a.running = cancel, true
	a.mu.Unlock()
	go a.runUpload(ctx, req)
	return StartResult{true, "上传任务已启动"}, nil
}
func (a *App) StopTransfer() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running || a.cancel == nil {
		return false
	}
	a.cancel()
	return true
}

func (a *App) runUpload(ctx context.Context, req UploadRequest) {
	collector := xprogress.NewCollector()
	ctx = xprogress.WithSink(ctx, collector)
	stopUpdates := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stopUpdates:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				runtime.EventsEmit(a.ctx, "transfer:snapshot", collector.Snapshot())
			}
		}
	}()
	args := buildUploadArgs(req)
	root := tdlcmd.New()
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(ctx)
	close(stopUpdates)
	<-done
	final := collector.FinishContext(ctx, err)
	runtime.EventsEmit(a.ctx, "transfer:snapshot", final)
	runtime.EventsEmit(a.ctx, "transfer:done", map[string]any{"error": errorString(err), "canceled": errors.Is(ctx.Err(), context.Canceled)})
	a.mu.Lock()
	a.running = false
	a.cancel = nil
	a.mu.Unlock()
}

func buildUploadArgs(req UploadRequest) []string {
	args := make([]string, 0, 24+len(req.Paths)*2)
	if req.Namespace != "" {
		args = append(args, "--ns", req.Namespace)
	}
	if req.Threads > 0 {
		args = append(args, "--threads", strconv.Itoa(req.Threads))
	}
	if req.Limit > 0 {
		args = append(args, "--limit", strconv.Itoa(req.Limit))
	}
	args = append(args, "up")
	for _, p := range req.Paths {
		args = append(args, "-p", p)
	}
	if req.Chat != "" {
		args = append(args, "--chat", req.Chat)
	}
	if req.Topic > 0 {
		args = append(args, "--topic", strconv.Itoa(req.Topic))
	}
	if req.CoverMode != "" {
		args = append(args, "--cover-mode", req.CoverMode)
	}
	if req.CoverAt != "" {
		args = append(args, "--cover-at", req.CoverAt)
	}
	if req.Remove {
		args = append(args, "--rm")
	}
	if req.AsPhoto {
		args = append(args, "--photo")
	}
	return args
}
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
