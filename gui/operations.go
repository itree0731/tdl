package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type TaskRecord struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"`
	Detail     string    `json:"detail"`
	Status     string    `json:"status"`
	Summary    string    `json:"summary"`
	Error      string    `json:"error"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

type ForwardRequest struct {
	Namespace string   `json:"namespace"`
	Proxy     string   `json:"proxy"`
	From      []string `json:"from"`
	To        string   `json:"to"`
	Mode      string   `json:"mode"`
	Threads   int      `json:"threads"`
	Silent    bool     `json:"silent"`
	DryRun    bool     `json:"dryRun"`
	Single    bool     `json:"single"`
	Desc      bool     `json:"desc"`
}

type ChatExportRequest struct {
	Namespace   string `json:"namespace"`
	Proxy       string `json:"proxy"`
	Chat        string `json:"chat"`
	Topic       int    `json:"topic"`
	Last        int    `json:"last"`
	Output      string `json:"output"`
	WithContent bool   `json:"withContent"`
	All         bool   `json:"all"`
}

func (a *App) TaskHistory() []TaskRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]TaskRecord, len(a.tasks))
	for i := range a.tasks {
		result[len(a.tasks)-1-i] = a.tasks[i]
	}
	return result
}

func (a *App) ClearTaskHistory() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("任务运行时不能清空历史")
	}
	a.tasks = nil
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "tasks:changed", []TaskRecord{})
	return nil
}

func (a *App) finishTask(id int64, status, errText, summary string) {
	a.mu.Lock()
	for i := range a.tasks {
		if a.tasks[i].ID == id {
			a.tasks[i].Status = status
			a.tasks[i].Error = errText
			a.tasks[i].Summary = summary
			a.tasks[i].FinishedAt = time.Now()
			break
		}
	}
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "tasks:changed", a.TaskHistory())
}

func operationLabel(direction string) string {
	switch direction {
	case "upload":
		return "上传"
	case "download":
		return "下载"
	case "forward":
		return "转发"
	case "chat-export":
		return "会话导出"
	case "backup":
		return "备份"
	case "recover":
		return "恢复"
	default:
		return "操作"
	}
}

func (a *App) SelectForwardFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "选择会话导出 JSON",
		Filters: []runtime.FileFilter{{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"}},
	})
}

func (a *App) SelectChatExportDestination() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存会话导出",
		DefaultFilename: "tmt-export.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"}},
	})
}

func (a *App) SelectBackupDestination() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 TMT 账号备份",
		DefaultFilename: time.Now().Format("2006-01-02-15_04_05") + ".backup.tmt",
		Filters:         []runtime.FileFilter{{DisplayName: "TMT 备份 (*.tmt)", Pattern: "*.tmt"}},
	})
}

func (a *App) SelectRecoveryFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 TMT 或旧版 TDL 备份",
		Filters: []runtime.FileFilter{
			{DisplayName: "TMT/TDL 备份 (*.tmt;*.tdl)", Pattern: "*.tmt;*.tdl"},
		},
	})
}

func (a *App) StartForward(req ForwardRequest) (StartResult, error) {
	if len(req.From) == 0 {
		return StartResult{}, fmt.Errorf("请至少输入一个消息链接或导出文件")
	}
	if req.Mode != "direct" && req.Mode != "clone" {
		return StartResult{}, fmt.Errorf("转发模式无效")
	}
	return a.startTransfer("forward", buildForwardArgs(req), fmt.Sprintf("%d 个来源", len(req.From)))
}

func (a *App) StartChatExport(req ChatExportRequest) (StartResult, error) {
	if req.Last < 1 || req.Last > 100000 {
		return StartResult{}, fmt.Errorf("导出消息数必须在 1 到 100000 之间")
	}
	if req.Output == "" {
		return StartResult{}, fmt.Errorf("请选择导出文件")
	}
	return a.startTransfer("chat-export", buildChatExportArgs(req), req.Output)
}

func (a *App) StartBackup(path string) (StartResult, error) {
	if path == "" {
		return StartResult{}, fmt.Errorf("请选择备份文件")
	}
	return a.startTransfer("backup", []string{"backup", "--dst", path}, path)
}

func (a *App) StartRecover(path string, confirmed bool) (StartResult, error) {
	if !confirmed {
		return StartResult{}, fmt.Errorf("恢复操作需要明确确认")
	}
	if path == "" {
		return StartResult{}, fmt.Errorf("请选择备份文件")
	}
	return a.startTransfer("recover", []string{"recover", "--file", path}, path)
}

func buildForwardArgs(req ForwardRequest) []string {
	args := globalOperationArgs(req.Namespace, req.Proxy, req.Threads)
	args = append(args, "forward")
	for _, source := range req.From {
		args = append(args, "--from", source)
	}
	args = append(args, "--to", req.To, "--mode", req.Mode)
	if req.Silent {
		args = append(args, "--silent")
	}
	if req.DryRun {
		args = append(args, "--dry-run")
	}
	if req.Single {
		args = append(args, "--single")
	}
	if req.Desc {
		args = append(args, "--desc")
	}
	return args
}

func buildChatExportArgs(req ChatExportRequest) []string {
	args := globalOperationArgs(req.Namespace, req.Proxy, 0)
	args = append(args, "chat", "export", "--type", "last", "--input", strconv.Itoa(req.Last), "--output", req.Output)
	if req.Chat != "" {
		args = append(args, "--chat", req.Chat)
	}
	if req.Topic > 0 {
		args = append(args, "--topic", strconv.Itoa(req.Topic))
	}
	if req.WithContent {
		args = append(args, "--with-content")
	}
	if req.All {
		args = append(args, "--all")
	}
	return args
}

func globalOperationArgs(namespace, proxy string, threads int) []string {
	args := make([]string, 0, 6)
	if namespace != "" {
		args = append(args, "--ns", namespace)
	}
	if proxy != "" {
		args = append(args, "--proxy", proxy)
	}
	if threads > 0 {
		args = append(args, "--threads", strconv.Itoa(threads))
	}
	return args
}
