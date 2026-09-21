package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
)

type kind int

const (
	kText kind = iota
	kBool
	kChoice
	kExtra
	kPicker
	kChat
)

func (k kind) editable() bool { return k == kText || k == kExtra || k == kPicker || k == kChat }

type field struct {
	kind        kind
	labelKey    string // i18n key, or plain text when it contains no dot
	helpKey     string
	placeholder string
	flag        string
	def         string // text default: value == def means "use tdl's own default"
	choices     []string
	picker      *PickerRequest
	paths       []string

	ti       textinput.Model
	boolVal  bool
	choiceIx int
}

func (f *field) label(l Lang) string {
	if strings.Contains(f.labelKey, ".") {
		return l.t(f.labelKey)
	}
	return localizedFieldLabel(l, f.labelKey)
}

func (f *field) help(l Lang) string {
	if h, ok := fieldHelp[f.helpKey]; ok {
		if l == LangZh {
			return h[1]
		}
		return h[0]
	}
	return l.t("form.extra.desc")
}

func textField(labelKey, flag, def, ph string) field {
	ti := textinput.New()
	ti.Placeholder = ph
	ti.SetValue(def)
	return field{kind: kText, labelKey: labelKey, helpKey: labelKey, placeholder: ph, flag: flag, def: def, ti: ti}
}

func boolField(labelKey, flag string) field {
	return field{kind: kBool, labelKey: labelKey, helpKey: labelKey, flag: flag}
}

func choiceField(labelKey, flag string, choices []string) field {
	return field{kind: kChoice, labelKey: labelKey, helpKey: labelKey, flag: flag, choices: choices}
}

func extraField() field {
	ti := textinput.New()
	ti.Placeholder = "--takeout --limit 4"
	return field{kind: kExtra, labelKey: "form.extra", helpKey: "form.extra", placeholder: ti.Placeholder, flag: "", ti: ti}
}

func pickerField(labelKey, flag, def, ph string, request PickerRequest) field {
	f := textField(labelKey, flag, def, ph)
	f.kind = kPicker
	f.picker = &request
	return f
}

func chatField(labelKey, flag, ph string) field {
	f := textField(labelKey, flag, "", ph)
	f.kind = kChat
	return f
}

func (f *field) value() string {
	switch f.kind {
	case kExtra, kText, kPicker, kChat:
		if len(f.paths) > 0 {
			return strings.Join(f.paths, " · ")
		}
		return strings.TrimSpace(f.ti.Value())
	case kChoice:
		return f.choices[f.choiceIx]
	case kBool:
		if f.boolVal {
			return "true"
		}
		return ""
	}
	return ""
}

// args appends this field's flag arguments to dst.
func (f *field) args(dst []string) []string {
	switch f.kind {
	case kExtra:
		if v := f.value(); v != "" {
			return append(dst, strings.Fields(v)...)
		}
	case kChoice:
		if v := f.value(); v != "" {
			return append(dst, f.flag, v)
		}
	case kBool:
		if f.boolVal {
			return append(dst, f.flag)
		}
	default:
		if f.kind == kPicker && len(f.paths) > 0 {
			for _, path := range f.paths {
				dst = append(dst, f.flag, path)
			}
			return dst
		}
		if v := f.value(); v != "" && v != f.def {
			return append(dst, f.flag, v)
		}
	}
	return dst
}

// action is one menu entry with its argument form.
type action struct {
	id       string
	titleKey string
	descKey  string
	base     []string // e.g. ["chat","export"]; nil = not executable
	fields   []field
}

func (a *action) title(l Lang) string { return l.t(a.titleKey) }
func (a *action) desc(l Lang) string  { return l.t(a.descKey) }

// argv assembles the full argument vector for the runner.
func (a *action) argv(global []string) []string {
	spec := a.formSpec()
	cfg := settings{}
	// Global flags have already been normalised by the caller. Build the
	// command through the typed spec, then prepend those exact flags.
	run, err := spec.build(a.formValues(), cfg)
	if err != nil {
		return append(append([]string{}, global...), a.base...)
	}
	return append(append([]string{}, global...), run.Args...)
}

func newActions() []action {
	return []action{
		{
			id: "login", titleKey: "menu.login", descKey: "menu.login.desc",
			base: []string{"login"},
			fields: []field{
				choiceField("Login type", "-T", []string{"", "code", "desktop"}),
				textField("Desktop path", "-d", "", "official client path"),
				textField("Passcode", "-p", "", "empty if none"),
			},
		},
		{
			id: "dl", titleKey: "menu.dl", descKey: "menu.dl.desc",
			base: []string{"dl"},
			fields: []field{
				textField("Message URLs (comma separated)", "-u", "", "https://t.me/..."),
				pickerField("Export files (comma separated)", "-f", "", "result.json", PickerRequest{Purpose: "download_exports", Mode: PickFiles, Multi: true, AllowedExt: []string{"json"}}),
				pickerField("Output dir", "-d", "downloads", "downloads", PickerRequest{Purpose: "download_output", Mode: PickDirectory}),
				textField("Include ext", "-i", "", "mp4,mp3"),
				textField("Exclude ext", "-e", "", "png,jpg"),
				boolField("Rewrite ext", "--rewrite-ext"),
				boolField("Skip same name", "--skip-same"),
				boolField("Newest first", "--desc"),
				boolField("Grouped media", "--group"),
				boolField("Takeout session", "--takeout"),
				boolField("Serve over HTTP", "--serve"),
				extraField(),
			},
		},
		{
			id: "up", titleKey: "menu.up", descKey: "menu.up.desc",
			base: []string{"up"},
			fields: []field{
				pickerField("Paths (comma separated)", "-p", "", "dirs or files", PickerRequest{Purpose: "upload_paths", Mode: PickFilesAndDirectories, Multi: true, Recursive: true}),
				chatField("Chat", "-c", "empty = Saved Messages"),
				textField("Topic id", "--topic", "", "0"),
				textField("To (router expr)", "--to", "", "CHAT expr"),
				textField("Include ext", "-i", "", "mp4,mp3"),
				textField("Exclude ext", "-e", "", "png,jpg"),
				boolField("Remove after upload", "--rm"),
				boolField("As photo", "--photo"),
				choiceField("Cover mode", "--cover-mode", []string{"video-cover", "thumbnail", "off"}),
				textField("Cover time", "--cover-at", "auto", "auto or 12s"),
				extraField(),
			},
		},
		{
			id: "chatls", titleKey: "menu.chatls", descKey: "menu.chatls.desc",
			base: []string{"chat", "ls"},
			fields: []field{
				choiceField("Output", "-o", []string{"", "json", "table"}),
				textField("Filter expr", "-f", "", "true"),
			},
		},
		{
			id: "chatexport", titleKey: "menu.chatexport", descKey: "menu.chatexport.desc",
			base: []string{"chat", "export"},
			fields: []field{
				choiceField("Export type", "-T", []string{"", "time", "id", "last"}),
				chatField("Chat", "-c", "empty = Saved Messages"),
				textField("Input (comma separated)", "-i", "", "depends on type"),
				textField("Filter expr", "-f", "", "true"),
				pickerField("Output file", "-o", "tdl-export.json", "tdl-export.json", PickerRequest{Purpose: "chat_export", Mode: PickSaveFile, AllowedExt: []string{"json"}}),
				boolField("With content", "--with-content"),
				boolField("All messages", "--all"),
				boolField("Raw struct", "--raw"),
				extraField(),
			},
		},
		{
			id: "chatusers", titleKey: "menu.chatusers", descKey: "menu.chatusers.desc",
			base: []string{"chat", "users"},
			fields: []field{
				chatField("Chat domain", "-c", "channel domain"),
				pickerField("Output file", "-o", "tdl-users.json", "tdl-users.json", PickerRequest{Purpose: "chat_users", Mode: PickSaveFile, AllowedExt: []string{"json"}}),
				boolField("Raw struct", "--raw"),
			},
		},
		{
			id: "forward", titleKey: "menu.forward", descKey: "menu.forward.desc",
			base: []string{"forward"},
			fields: []field{
				pickerField("From (comma separated)", "--from", "", "links or export files", PickerRequest{Purpose: "forward_sources", Mode: PickFiles, Multi: true, AllowedExt: []string{"json"}}),
				textField("To", "--to", "", "CHAT or router expr"),
				textField("Edit expr", "--edit", "", "empty = no edit"),
				choiceField("Mode", "--mode", []string{"", "copy", "forward"}),
				boolField("Silent", "--silent"),
				boolField("Dry run", "--dry-run"),
				boolField("No grouped detect", "--single"),
				boolField("Reverse order", "--desc"),
				extraField(),
			},
		},
		{
			id: "backup", titleKey: "menu.backup", descKey: "menu.backup.desc",
			base: []string{"backup"},
			fields: []field{
				pickerField("Destination", "-d", "", "<date>.backup.tdl", PickerRequest{Purpose: "backup_destination", Mode: PickSaveFile, AllowedExt: []string{"tdl"}}),
			},
		},
		{
			id: "recover", titleKey: "menu.recover", descKey: "menu.recover.desc",
			base: []string{"recover"},
			fields: []field{
				pickerField("Backup file", "-f", "", "xxx.backup.tdl", PickerRequest{Purpose: "recover_source", Mode: PickOpenFile, AllowedExt: []string{"tdl"}}),
			},
		},
		{
			id: "update", titleKey: "menu.update", descKey: "menu.update.desc",
			base: []string{"update"},
			fields: []field{
				boolField("No confirmation", "-y"),
				boolField("Dry run", "--dry-run"),
				textField("Target version", "-v", "", "v0.20.4"),
				boolField("Force reinstall", "-f"),
			},
		},
		{
			id: "version", titleKey: "menu.version", descKey: "menu.version.desc",
			base:   []string{"version"},
			fields: []field{},
		},
		{
			id: "settings", titleKey: "menu.settings", descKey: "menu.settings.desc",
			base:   nil, // handled by the model, never executed
			fields: nil,
		},
		{
			id: "quit", titleKey: "menu.quit", descKey: "menu.quit.desc",
			base:   nil, // handled by the model, never executed
			fields: nil,
		},
	}
}

// quoteJoin renders argv like a shell line, for display only.
func quoteJoin(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		if strings.ContainsAny(a, " \t") {
			parts = append(parts, fmt.Sprintf("%q", a))
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}
