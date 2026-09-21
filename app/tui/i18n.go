package tui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/iyear/tdl/core/util/netutil"
	"github.com/iyear/tdl/pkg/consts"
)

type Lang string

const (
	LangEn Lang = "en"
	LangZh Lang = "zh"
)

type dict map[string]string

var zhFieldLabels = map[string]string{
	"Login type": "登录方式", "Desktop path": "桌面客户端路径", "Passcode": "登录密码",
	"Message URLs (comma separated)": "消息链接（逗号分隔）", "Export files (comma separated)": "导出文件（逗号分隔）", "Output dir": "输出目录",
	"Include ext": "包含扩展名", "Exclude ext": "排除扩展名", "Rewrite ext": "重写扩展名", "Skip same name": "跳过同名文件",
	"Newest first": "最新优先", "Grouped media": "媒体分组", "Takeout session": "Takeout 会话", "Serve over HTTP": "通过 HTTP 提供服务",
	"Paths (comma separated)": "路径（逗号分隔）", "Chat": "会话目标", "Topic id": "主题 ID", "To (router expr)": "目标（路由表达式）",
	"Remove after upload": "上传后删除", "As photo": "作为图片发送", "Disable auto thumbnail": "禁用自动视频封面", "Output": "输出格式", "Filter expr": "过滤表达式",
	"Export type": "导出类型", "Input (comma separated)": "输入（逗号分隔）", "Output file": "输出文件", "With content": "包含内容",
	"All messages": "全部消息", "Raw struct": "原始结构", "Chat domain": "会话域名", "From (comma separated)": "来源（逗号分隔）",
	"To": "目标", "Edit expr": "编辑表达式", "Mode": "模式", "Silent": "静默发送", "Dry run": "演练模式",
	"No grouped detect": "不检测分组", "Reverse order": "倒序", "Destination": "目标文件", "Backup file": "备份文件",
	"No confirmation": "跳过确认", "Target version": "目标版本", "Force reinstall": "强制重装",
}

var zhChoiceLabels = map[string]string{
	"code": "验证码", "desktop": "桌面客户端", "json": "JSON", "csv": "CSV", "table": "表格",
	"time": "时间", "id": "ID", "last": "最后一条", "copy": "复制", "forward": "转发",
	"en": "英文", "zh": "中文",
}

var zhPlaceholders = map[string]string{
	"dirs or files": "文件或目录，可用逗号分隔", "protocol://host:port": "协议://主机:端口",
	"official client path": "官方客户端路径", "empty if none": "没有密码则留空", "https://t.me/...": "https://t.me/...",
	"result.json": "result.json", "downloads": "下载目录", "mp4,mp3": "mp4,mp3", "png,jpg": "png,jpg",
	"D:\\videos": "D:\\videos", "empty = Saved Messages": "留空表示收藏夹", "0": "0", "CHAT expr": "会话或路由表达式",
	"true": "true", "depends on type": "根据导出类型填写", "tdl-export.json": "tdl-export.json", "channel domain": "频道域名",
	"links or export files": "链接或导出文件", "CHAT or router expr": "会话或路由表达式", "empty = no edit": "留空表示不编辑",
	"<date>.backup.tdl": "<日期>.backup.tdl", "xxx.backup.tdl": "xxx.backup.tdl", "v0.20.4": "v0.20.4",
}

func localizedFieldLabel(l Lang, raw string) string {
	if l == LangZh {
		if value, ok := zhFieldLabels[raw]; ok {
			return value
		}
	}
	return raw
}

func localizedChoice(l Lang, raw string) string {
	if l == LangZh {
		if value, ok := zhChoiceLabels[raw]; ok {
			return value
		}
	}
	return raw
}

func localizedPlaceholder(l Lang, raw string) string {
	if l == LangZh {
		if value, ok := zhPlaceholders[raw]; ok {
			return value
		}
	}
	return raw
}

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
	"form.run":             "Run",
	"form.back":            "Back",
	"form.required":        "required",
	"form.optional":        "optional",
	"form.bool.on":         "on",
	"form.bool.off":        "off",
	"form.extra":           "Extra args",
	"form.extra.desc":      "Appended verbatim, e.g. --takeout --limit 4",
	"field.login.type":     "Login type",
	"field.login.desktop":  "Desktop path",
	"field.login.passcode": "Passcode",
	"field.dl.urls":        "Message URLs",
	"field.dl.files":       "Export files",
	"field.dl.dir":         "Output directory",
	"field.up.paths":       "Upload paths",
	"field.up.chat":        "Chat",
	"field.chat.filter":    "Filter expression",
	"field.common.include": "Include extensions",
	"field.common.exclude": "Exclude extensions",
	"field.common.extra":   "Extra arguments",

	// settings
	"set.language": "Language",
	"set.global":   "Global options (applied to every command)",
	"set.ns":       "Namespace",
	"set.proxy":    "Proxy",
	"set.threads":  "Threads",
	"set.limit":    "Limit",
	"set.unsaved":  "Unsaved changes: [s] save  [d] discard  [esc] continue editing",

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

	"form.run":             "运行",
	"form.back":            "返回",
	"form.required":        "必填",
	"form.optional":        "可选",
	"form.bool.on":         "开",
	"form.bool.off":        "关",
	"form.extra":           "附加参数",
	"form.extra.desc":      "原样追加，如 --takeout --limit 4",
	"field.login.type":     "登录方式",
	"field.login.desktop":  "桌面客户端路径",
	"field.login.passcode": "登录密码",
	"field.dl.urls":        "消息链接",
	"field.dl.files":       "导出文件",
	"field.dl.dir":         "输出目录",
	"field.up.paths":       "上传路径",
	"field.up.chat":        "会话目标",
	"field.chat.filter":    "过滤表达式",
	"field.common.include": "包含扩展名",
	"field.common.exclude": "排除扩展名",
	"field.common.extra":   "附加参数",

	"set.language": "语言",
	"set.global":   "全局选项（对所有命令生效）",
	"set.ns":       "命名空间",
	"set.proxy":    "代理",
	"set.threads":  "线程数",
	"set.limit":    "并发数",
	"set.unsaved":  "有未保存修改：[s] 保存  [d] 放弃  [esc] 继续编辑",

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
	if pair, ok := uiMessages[key]; ok {
		if l == LangZh {
			return pair[1]
		}
		return pair[0]
	}
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

func validateSettings(s settings) error {
	for _, v := range []struct{ name, value string }{{"set.threads", s.Threads}, {"set.limit", s.Limit}} {
		if v.value != "" {
			n, err := strconv.Atoi(v.value)
			if err != nil || n < 1 {
				return fmt.Errorf("%s: %s", Lang(s.Language).t(v.name), Lang(s.Language).t("set.positive"))
			}
		}
	}
	if s.Proxy != "" {
		u, err := url.Parse(s.Proxy)
		_, dialerErr := netutil.NewProxy(s.Proxy)
		if err != nil || u.Scheme == "" || u.Host == "" || dialerErr != nil {
			return fmt.Errorf("%s: %s", Lang(s.Language).t("set.proxy"), Lang(s.Language).t("set.proxy.invalid"))
		}
	}
	return nil
}
func saveSettings(s settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(settingsPath()), ".tui-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, settingsPath())
}
