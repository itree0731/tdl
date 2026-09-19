package tui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/iyear/tdl/pkg/consts"
)

type Lang string

const (
	LangEn Lang = "en"
	LangZh Lang = "zh"
)

type dict map[string]string

var en = dict{
	// menu
	"menu.title":           "What do you want to do?",
	"menu.login":           "Login",
	"menu.login.desc":      "Log in to a Telegram account (QR by default)",
	"menu.dl":              "Download",
	"menu.dl.desc":         "Download media from messages, exports or links",
	"menu.up":              "Upload",
	"menu.up.desc":         "Upload local files to a chat",
	"menu.chatls":          "Chat: List",
	"menu.chatls.desc":     "List your chats",
	"menu.chatexport":      "Chat: Export",
	"menu.chatexport.desc": "Export messages for download",
	"menu.chatusers":       "Chat: Users",
	"menu.chatusers.desc":  "Export users from channels",
	"menu.forward":         "Forward",
	"menu.forward.desc":    "Forward messages to another peer",
	"menu.backup":          "Backup",
	"menu.backup.desc":     "Backup sessions and data",
	"menu.recover":         "Recover",
	"menu.recover.desc":    "Recover from a backup file",
	"menu.update":          "Update",
	"menu.update.desc":     "Self-update tdl binary",
	"menu.version":         "Version",
	"menu.version.desc":    "Show tdl version",
	"menu.settings":        "Settings",
	"menu.settings.desc":   "Language and global options",
	"menu.quit":            "Quit",
	"menu.quit.desc":       "Exit the TUI",

	// form
	"form.run":        "Run",
	"form.back":       "Back",
	"form.required":   "required",
	"form.optional":   "optional",
	"form.bool.on":    "on",
	"form.bool.off":   "off",
	"form.extra":      "Extra args",
	"form.extra.desc": "Appended verbatim, e.g. --takeout --limit 4",

	// settings
	"set.language": "Language",
	"set.global":   "Global options (applied to every command)",
	"set.ns":       "Namespace",
	"set.proxy":    "Proxy",
	"set.threads":  "Threads",
	"set.limit":    "Limit",

	// status
	"status.running":  "Running",
	"status.done":     "Done in",
	"status.failed":   "Failed after",
	"status.stopped":  "Stopped after",
	"status.stopping": "stopping… ctrl+c again to force quit",
	"status.loginext": "'tdl login' needs an interactive terminal: quit the TUI (q) and run it there",
	"status.empty":    "No output",
	"status.ready":    "Ready. Output will stream here.",

	// shortcuts
	"sc.updown":    "navigate",
	"sc.enter":     "select",
	"sc.space":     "toggle",
	"sc.esc":       "back",
	"sc.ctrlc":     "stop",
	"sc.typing":    "type",
	"sc.leftright": "switch",
	"sc.quit":      "quit",
	"sc.scroll":    "scroll",

	// misc
	"banner.title":  "tdl — Telegram Downloader, but more than a downloader",
	"banner.sub":    "TUI mode · grok-build style",
	"hdr.nosession": "no session",
}

var zh = dict{
	"menu.title":           "想做什么？",
	"menu.login":           "登录",
	"menu.login.desc":      "登录 Telegram 账号（默认二维码）",
	"menu.dl":              "下载",
	"menu.dl.desc":         "从消息链接或导出文件下载媒体",
	"menu.up":              "上传",
	"menu.up.desc":         "上传本地文件到会话",
	"menu.chatls":          "会话：列出",
	"menu.chatls.desc":     "列出你的会话",
	"menu.chatexport":      "会话：导出",
	"menu.chatexport.desc": "导出消息用于下载",
	"menu.chatusers":       "会话：用户",
	"menu.chatusers.desc":  "导出频道用户",
	"menu.forward":         "转发",
	"menu.forward.desc":    "转发消息到目标会话",
	"menu.backup":          "备份",
	"menu.backup.desc":     "备份会话与数据",
	"menu.recover":         "恢复",
	"menu.recover.desc":    "从备份文件恢复",
	"menu.update":          "更新",
	"menu.update.desc":     "自更新 tdl 二进制",
	"menu.version":         "版本",
	"menu.version.desc":    "查看 tdl 版本",
	"menu.settings":        "设置",
	"menu.settings.desc":   "语言与全局选项",
	"menu.quit":            "退出",
	"menu.quit.desc":       "退出 TUI",

	"form.run":        "运行",
	"form.back":       "返回",
	"form.required":   "必填",
	"form.optional":   "可选",
	"form.bool.on":    "开",
	"form.bool.off":   "关",
	"form.extra":      "附加参数",
	"form.extra.desc": "原样追加，如 --takeout --limit 4",

	"set.language": "语言",
	"set.global":   "全局选项（对所有命令生效）",
	"set.ns":       "命名空间",
	"set.proxy":    "代理",
	"set.threads":  "线程数",
	"set.limit":    "并发数",

	"status.running":  "运行中",
	"status.done":     "完成，用时",
	"status.failed":   "失败，用时",
	"status.stopped":  "已停止，用时",
	"status.stopping": "正在停止…再按一次 ctrl+c 强制退出",
	"status.loginext": "tdl login 需要交互式终端：退出 TUI（q）后在终端运行",
	"status.empty":    "无输出",
	"status.ready":    "就绪。输出将在这里滚动。",

	"sc.updown":    "移动",
	"sc.enter":     "确认",
	"sc.space":     "切换",
	"sc.esc":       "返回",
	"sc.ctrlc":     "停止",
	"sc.typing":    "输入",
	"sc.leftright": "切换",
	"sc.quit":      "退出",
	"sc.scroll":    "滚动",

	"banner.title":  "tdl — Telegram 下载器，不止于下载器",
	"banner.sub":    "TUI 模式 · grok-build 风格",
	"hdr.nosession": "暂无会话",
}

func (l Lang) t(key string) string {
	var d dict
	switch l {
	case LangZh:
		d = zh
	default:
		d = en
	}
	if s, ok := d[key]; ok {
		return s
	}
	return en[key]
}

// settings persisted to <data>/tui.json
type settings struct {
	Language string `json:"language"`
	NS       string `json:"ns,omitempty"`
	Proxy    string `json:"proxy,omitempty"`
	Threads  string `json:"threads,omitempty"`
	Limit    string `json:"limit,omitempty"`
}

func settingsPath() string {
	return filepath.Join(consts.DataDir, "tui.json")
}

func loadSettings() settings {
	s := settings{Language: string(LangEn)}
	b, err := os.ReadFile(settingsPath())
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Language != string(LangZh) && s.Language != string(LangEn) {
		s.Language = string(LangEn)
	}
	return s
}

func saveSettings(s settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), b, 0o600)
}
