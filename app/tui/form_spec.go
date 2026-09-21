package tui

import (
	"fmt"
	"regexp"
	"strings"

	xprogress "github.com/iyear/tdl/pkg/progress"
)

// FieldKind describes the value contract of a form field. Display text and
// command values stay separate, so localisation can never alter arguments.
type FieldKind uint8

const (
	FieldText FieldKind = iota
	FieldNumber
	FieldBool
	FieldChoice
	FieldPaths
	FieldDirectory
	FieldSaveFile
	FieldOpenFile
	FieldChat
	FieldExtraArgs
)

type Choice struct {
	Value    string
	LabelKey string
}

type FieldSpec struct {
	ID          string
	Kind        FieldKind
	LabelKey    string
	HelpKey     string
	Flag        string
	Required    bool
	Choices     []Choice
	Picker      *PickerRequest
	DefaultText string
}

type FieldError struct {
	FieldID string
	Message string
}

type FormValues map[string]any

func (v FormValues) String(id string) string {
	value, _ := v[id].(string)
	return strings.TrimSpace(value)
}

func (v FormValues) Bool(id string) bool {
	value, _ := v[id].(bool)
	return value
}

func (v FormValues) Strings(id string) []string {
	value, _ := v[id].([]string)
	return append([]string(nil), value...)
}

type RunDisplay struct {
	Title   string
	Summary string
}

type InputRef struct {
	Path string
	Kind string
}

// RunSpec is immutable once a task starts. It prevents later form edits from
// changing retry or display behaviour.
type RunSpec struct {
	ActionID string
	Args     []string
	Display  RunDisplay
	Inputs   []InputRef
}

type RetryClass uint8

const (
	RetrySafe RetryClass = iota
	RetryRestartItem
	RetryCleanupOnly
	RetryUncertain
	RetryNotAllowed
)

type ItemResult struct {
	ID, DisplayName, SourcePath, Phase, Err string
	Status                                  string
	Retry                                   RetryClass
}

type RunResult struct {
	Items []ItemResult
}

func buildRunResult(events []xprogress.Event) RunResult {
	result := RunResult{Items: make([]ItemResult, 0, len(events))}
	for _, event := range events {
		if event.Status != xprogress.StatusFailed && event.Status != xprogress.StatusCanceled {
			continue
		}
		retry := RetryRestartItem
		if event.Direction == xprogress.DirectionUpload {
			switch {
			case event.Phase == "uploaded_cleanup":
				retry = RetryCleanupOnly
			case strings.Contains(event.Err, "send message"):
				retry = RetryUncertain
			case strings.Contains(event.Err, "prepare upload"), strings.Contains(event.Err, "prepare video cover"):
				retry = RetrySafe
			}
		}
		result.Items = append(result.Items, ItemResult{
			ID: event.TaskID, DisplayName: event.FileName, SourcePath: event.SourcePath, Phase: event.Phase,
			Err: event.Err, Status: string(event.Status), Retry: retry,
		})
	}
	return result
}

type FormSpec struct {
	ID       string
	BaseArgs []string
	Fields   []FieldSpec
	Validate func(FormValues) []FieldError
	BuildRun func(FormValues, settings) (RunSpec, error)
}

func (s FormSpec) validate(values FormValues) []FieldError {
	var errs []FieldError
	for _, f := range s.Fields {
		if !f.Required {
			continue
		}
		empty := values.String(f.ID) == "" && len(values.Strings(f.ID)) == 0
		if empty {
			errs = append(errs, FieldError{FieldID: f.ID, Message: "required"})
		}
	}
	if s.Validate != nil {
		errs = append(errs, s.Validate(values)...)
	}
	return errs
}

func (s FormSpec) build(values FormValues, cfg settings) (RunSpec, error) {
	if errs := s.validate(values); len(errs) > 0 {
		return RunSpec{}, fmt.Errorf("%s: %s", errs[0].FieldID, errs[0].Message)
	}
	if s.BuildRun != nil {
		return s.BuildRun(values, cfg)
	}
	args := append([]string{}, cfg.globalArgs()...)
	args = append(args, s.BaseArgs...)
	var inputs []InputRef
	for _, field := range s.Fields {
		switch field.Kind {
		case FieldBool:
			if values.Bool(field.ID) {
				args = append(args, field.Flag)
			}
		case FieldPaths:
			for _, path := range values.Strings(field.ID) {
				args = append(args, field.Flag, path)
				inputs = append(inputs, InputRef{Path: path, Kind: "file"})
			}
		case FieldExtraArgs:
			args = append(args, strings.Fields(values.String(field.ID))...)
		default:
			value := values.String(field.ID)
			if value != "" && value != field.DefaultText {
				args = append(args, field.Flag, value)
				if field.Kind == FieldOpenFile {
					inputs = append(inputs, InputRef{Path: value, Kind: "file"})
				}
			}
		}
	}
	return RunSpec{ActionID: s.ID, Args: args, Inputs: inputs}, nil
}

var fieldIDInvalid = regexp.MustCompile(`[^a-z0-9]+`)

func stableFieldID(flag, label string) string {
	id := strings.TrimLeft(strings.ToLower(flag), "-")
	if id == "" {
		id = strings.ToLower(label)
	}
	id = strings.Trim(fieldIDInvalid.ReplaceAllString(id, "_"), "_")
	if id == "" {
		return "field"
	}
	return id
}

func fieldKindOf(f field) FieldKind {
	if f.kind == kChat {
		return FieldChat
	}
	if f.kind == kPicker && f.picker != nil {
		switch f.picker.Mode {
		case PickDirectory:
			return FieldDirectory
		case PickFiles, PickFilesAndDirectories:
			return FieldPaths
		case PickSaveFile:
			return FieldSaveFile
		case PickOpenFile:
			return FieldOpenFile
		}
	}
	switch f.kind {
	case kBool:
		return FieldBool
	case kChoice:
		return FieldChoice
	case kExtra:
		return FieldExtraArgs
	default:
		return FieldText
	}
}

func (a action) formSpec() FormSpec {
	fields := make([]FieldSpec, 0, len(a.fields))
	for _, f := range a.fields {
		choices := make([]Choice, 0, len(f.choices))
		for _, value := range f.choices {
			choices = append(choices, Choice{Value: value, LabelKey: value})
		}
		fields = append(fields, FieldSpec{
			ID: stableFieldID(f.flag, f.labelKey), Kind: fieldKindOf(f), LabelKey: f.labelKey,
			HelpKey: f.helpKey, Flag: f.flag, Choices: choices, DefaultText: f.def, Picker: f.picker,
		})
	}
	return FormSpec{ID: a.id, BaseArgs: append([]string(nil), a.base...), Fields: fields}
}

func (a action) formValues() FormValues {
	values := make(FormValues, len(a.fields))
	for _, f := range a.fields {
		id := stableFieldID(f.flag, f.labelKey)
		switch f.kind {
		case kBool:
			values[id] = f.boolVal
		case kPicker:
			kind := fieldKindOf(f)
			if kind == FieldPaths {
				if len(f.paths) > 0 {
					values[id] = append([]string(nil), f.paths...)
				} else if value := strings.TrimSpace(f.ti.Value()); value != "" {
					values[id] = []string{value}
				}
			} else if len(f.paths) > 0 {
				values[id] = f.paths[0]
			} else {
				values[id] = f.value()
			}
		default:
			values[id] = f.value()
		}
	}
	return values
}
