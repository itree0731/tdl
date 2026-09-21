package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/iyear/tdl/app/chat"
	tdlcmd "github.com/iyear/tdl/cmd"
	"github.com/iyear/tdl/pkg/kv"
	xprogress "github.com/iyear/tdl/pkg/progress"
	"github.com/iyear/tdl/pkg/tclient"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	mu           sync.Mutex
	cancel       context.CancelFunc
	running      bool
	chatLoading  bool
	settingsBusy bool
	dialogMu     sync.Mutex
	chatCursorID int
	chatCursors  map[string]map[int]*chat.DialogCursor
	settings     DesktopSettings
	settingsErr  error
	tasks        []TaskRecord
	nextTaskID   int64
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
	Proxy     string   `json:"proxy"`
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

type DownloadRequest struct {
	Namespace string   `json:"namespace"`
	Proxy     string   `json:"proxy"`
	URLs      []string `json:"urls"`
	Files     []string `json:"files"`
	Directory string   `json:"directory"`
	Threads   int      `json:"threads"`
	Limit     int      `json:"limit"`
	Rewrite   bool     `json:"rewrite"`
	SkipSame  bool     `json:"skipSame"`
	Group     bool     `json:"group"`
	Takeout   bool     `json:"takeout"`
	Restart   bool     `json:"restart"`
}
type StartResult struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

type TopicRef struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type ChatRef struct {
	ID       int64      `json:"id"`
	Username string     `json:"username"`
	Title    string     `json:"title"`
	Type     string     `json:"type"`
	Topics   []TopicRef `json:"topics"`
	Self     bool       `json:"self"`
}

type ChatPage struct {
	Items   []ChatRef `json:"items"`
	Next    int       `json:"next"`
	Skipped int       `json:"skipped"`
}

func NewApp() *App {
	settings, err := loadDesktopSettings(desktopSettingsPath())
	return &App{chatCursors: make(map[string]map[int]*chat.DialogCursor), settings: settings, settingsErr: err}
}
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) Capabilities() Capabilities  { return Capabilities{"TMT Desktop", true, true, true, true} }
func (a *App) Namespaces() ([]string, error) {
	store, err := kv.NewWithMap(tdlcmd.DefaultBoltStorage)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.Namespaces()
}

func (a *App) GetSettings() (DesktopSettings, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.settingsErr != nil {
		return a.settings, a.settingsErr
	}
	return a.settings, nil
}

func (a *App) SaveSettings(settings DesktopSettings) (DesktopSettings, error) {
	a.mu.Lock()
	if a.running || a.chatLoading || a.settingsBusy {
		a.mu.Unlock()
		return DesktopSettings{}, fmt.Errorf("当前有任务正在运行，不能保存设置")
	}
	a.settingsBusy = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.settingsBusy = false
		a.mu.Unlock()
	}()
	settings.SchemaVersion = desktopSettingsVersion
	if err := saveDesktopSettings(desktopSettingsPath(), settings); err != nil {
		return DesktopSettings{}, err
	}
	a.mu.Lock()
	a.settings = settings
	a.settingsErr = nil
	a.mu.Unlock()
	return settings, nil
}
func (a *App) SelectUploadFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择上传文件"})
}
func (a *App) SelectUploadDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择上传目录"})
}

func (a *App) SelectDownloadDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择下载目录"})
}

func (a *App) SelectDownloadExportFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "选择 TMT 会话导出 JSON",
		Filters: []runtime.FileFilter{{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"}},
	})
}

func (a *App) ChatPage(namespace string, cursor, limit int) (ChatPage, error) {
	a.dialogMu.Lock()
	defer a.dialogMu.Unlock()

	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return ChatPage{}, fmt.Errorf("传输运行时不能加载会话")
	}
	a.chatLoading = true
	defer func() {
		a.mu.Lock()
		a.chatLoading = false
		a.mu.Unlock()
	}()
	var native *chat.DialogCursor
	if cursor > 0 {
		native = a.chatCursors[namespace][cursor]
		if native == nil {
			a.mu.Unlock()
			return ChatPage{}, fmt.Errorf("无效会话游标")
		}
	}
	a.mu.Unlock()
	store, err := kv.NewWithMap(tdlcmd.DefaultBoltStorage)
	if err != nil {
		return ChatPage{}, err
	}
	defer store.Close()
	db, err := store.Open(namespace)
	if err != nil {
		return ChatPage{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 45*time.Second)
	defer cancel()
	client, err := tclient.New(ctx, tclient.Options{KV: db, ReconnectTimeout: 30 * time.Second}, false)
	if err != nil {
		return ChatPage{}, err
	}
	var dialogs []*chat.Dialog
	var next *chat.DialogCursor
	var skipped int
	err = client.Run(ctx, func(runCtx context.Context) error {
		var pageErr error
		dialogs, next, skipped, pageErr = chat.ListDialogsPage(runCtx, client, db, native, limit)
		return pageErr
	})
	if err != nil {
		return ChatPage{}, err
	}
	page := ChatPage{Items: make([]ChatRef, 0, len(dialogs)+1), Skipped: skipped}
	if cursor == 0 {
		page.Items = append(page.Items, ChatRef{Title: "Saved Messages", Type: "self", Self: true})
	}
	for _, d := range dialogs {
		topics := make([]TopicRef, 0, len(d.Topics))
		for _, topic := range d.Topics {
			topics = append(topics, TopicRef{ID: topic.ID, Title: topic.Title})
		}
		page.Items = append(page.Items, ChatRef{ID: d.ID, Username: d.Username, Title: d.VisibleName, Type: d.Type, Topics: topics})
	}
	if next != nil {
		a.mu.Lock()
		a.chatCursorID++
		page.Next = a.chatCursorID
		if a.chatCursors[namespace] == nil {
			a.chatCursors[namespace] = make(map[int]*chat.DialogCursor)
		}
		a.chatCursors[namespace][page.Next] = next
		a.mu.Unlock()
	}
	return page, nil
}

func (a *App) StartUpload(req UploadRequest) (StartResult, error) {
	if len(req.Paths) == 0 {
		return StartResult{}, fmt.Errorf("请选择至少一个文件或目录")
	}
	return a.startTransfer("upload", buildUploadArgs(req), fmt.Sprintf("%d 个输入", len(req.Paths)))
}

func (a *App) StartDownload(req DownloadRequest) (StartResult, error) {
	if len(req.URLs) == 0 && len(req.Files) == 0 {
		return StartResult{}, fmt.Errorf("请至少输入一个 Telegram 消息链接或选择一个导出文件")
	}
	if req.Directory == "" {
		return StartResult{}, fmt.Errorf("请选择下载目录")
	}
	return a.startTransfer("download", buildDownloadArgs(req), fmt.Sprintf("%d 个链接，%d 个导出文件", len(req.URLs), len(req.Files)))
}

func (a *App) startTransfer(direction string, args []string, detail string) (StartResult, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return StartResult{}, fmt.Errorf("已有传输任务正在运行")
	}
	if a.chatLoading {
		a.mu.Unlock()
		return StartResult{}, fmt.Errorf("正在加载会话，请稍后开始传输")
	}
	if a.settingsBusy {
		a.mu.Unlock()
		return StartResult{}, fmt.Errorf("正在保存设置，请稍后开始传输")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel, a.running = cancel, true
	a.nextTaskID++
	taskID := a.nextTaskID
	a.tasks = append(a.tasks, TaskRecord{ID: taskID, Type: direction, Detail: detail, Status: "running", StartedAt: time.Now()})
	if len(a.tasks) > 100 {
		a.tasks = append([]TaskRecord(nil), a.tasks[len(a.tasks)-100:]...)
	}
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "tasks:changed", a.TaskHistory())
	go a.runTransfer(ctx, direction, args, taskID)
	message := operationLabel(direction) + "任务已启动"
	return StartResult{true, message}, nil
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

func (a *App) runTransfer(ctx context.Context, direction string, args []string, taskID int64) {
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
				runtime.EventsEmit(a.ctx, "transfer:items", collector.Items())
			}
		}
	}()
	root := tdlcmd.New()
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(ctx)
	close(stopUpdates)
	<-done
	final := collector.FinishContext(ctx, err)
	a.finishTask(taskID, string(final.Status), errorString(err), final.Summary(true))
	runtime.EventsEmit(a.ctx, "transfer:snapshot", final)
	runtime.EventsEmit(a.ctx, "transfer:items", collector.Items())
	runtime.EventsEmit(a.ctx, "transfer:done", map[string]any{"taskID": taskID, "direction": direction, "error": errorString(err), "canceled": errors.Is(ctx.Err(), context.Canceled)})
	a.mu.Lock()
	a.running = false
	a.cancel = nil
	a.mu.Unlock()
}

func buildDownloadArgs(req DownloadRequest) []string {
	args := make([]string, 0, 24+len(req.URLs)*2+len(req.Files)*2)
	if req.Namespace != "" {
		args = append(args, "--ns", req.Namespace)
	}
	if req.Proxy != "" {
		args = append(args, "--proxy", req.Proxy)
	}
	if req.Threads > 0 {
		args = append(args, "--threads", strconv.Itoa(req.Threads))
	}
	if req.Limit > 0 {
		args = append(args, "--limit", strconv.Itoa(req.Limit))
	}
	args = append(args, "dl", "--dir", req.Directory)
	for _, url := range req.URLs {
		args = append(args, "--url", url)
	}
	for _, file := range req.Files {
		args = append(args, "--file", file)
	}
	if req.Restart {
		args = append(args, "--restart")
	} else {
		args = append(args, "--continue")
	}
	if req.Rewrite {
		args = append(args, "--rewrite-ext")
	}
	if req.SkipSame {
		args = append(args, "--skip-same")
	}
	if req.Group {
		args = append(args, "--group")
	}
	if req.Takeout {
		args = append(args, "--takeout")
	}
	return args
}

func buildUploadArgs(req UploadRequest) []string {
	args := make([]string, 0, 24+len(req.Paths)*2)
	if req.Namespace != "" {
		args = append(args, "--ns", req.Namespace)
	}
	if req.Proxy != "" {
		args = append(args, "--proxy", req.Proxy)
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
