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
)

type field struct {
	kind        kind
	labelKey    string // i18n key, or plain text when it contains no dot
	helpKey     string
	placeholder string
	flag        string
	def         string // text default: value == def means "use tdl's own default"
	choices     []string

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

func (f *field) value() string {
	switch f.kind {
	case kExtra, kText:
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
	argv := append([]string{}, global...)
	argv = append(argv, a.base...)
	for i := range a.fields {
		argv = a.fields[i].args(argv)
	}
	return argv
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
				textField("Export files (comma separated)", "-f", "", "result.json"),
				textField("Output dir", "-d", "downloads", "downloads"),
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
				textField("Paths (comma separated)", "-p", "", "dirs or files"),
				textField("Chat", "-c", "", "empty = Saved Messages"),
				textField("Topic id", "--topic", "", "0"),
				textField("To (router expr)", "--to", "", "CHAT expr"),
				textField("Include ext", "-i", "", "mp4,mp3"),
				textField("Exclude ext", "-e", "", "png,jpg"),
				boolField("Remove after upload", "--rm"),
				boolField("As photo", "--photo"),
				boolField("Disable auto thumbnail", "--no-auto-thumb"),
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
				textField("Chat", "-c", "", "empty = Saved Messages"),
				textField("Input (comma separated)", "-i", "", "depends on type"),
				textField("Filter expr", "-f", "", "true"),
				textField("Output file", "-o", "tdl-export.json", "tdl-export.json"),
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
				textField("Chat domain", "-c", "", "channel domain"),
				textField("Output file", "-o", "tdl-users.json", "tdl-users.json"),
				boolField("Raw struct", "--raw"),
			},
		},
		{
			id: "forward", titleKey: "menu.forward", descKey: "menu.forward.desc",
			base: []string{"forward"},
			fields: []field{
				textField("From (comma separated)", "--from", "", "links or export files"),
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
				textField("Destination", "-d", "", "<date>.backup.tdl"),
			},
		},
		{
			id: "recover", titleKey: "menu.recover", descKey: "menu.recover.desc",
			base: []string{"recover"},
			fields: []field{
				textField("Backup file", "-f", "", "xxx.backup.tdl"),
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
