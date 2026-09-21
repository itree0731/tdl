package tui

import (
	"strings"
	"testing"
)

func actionByID(t *testing.T, id string) action {
	t.Helper()
	for _, action := range newActions() {
		if action.id == id {
			return action
		}
	}
	t.Fatalf("missing action %q", id)
	return action{}
}

func TestUploadFormValidationRunsBeforeCommand(t *testing.T) {
	action := actionByID(t, "up")
	spec := action.formSpec()
	values := action.formValues()
	if _, err := spec.build(values, defaultSettings()); err == nil || !strings.Contains(err.Error(), "select at least one") {
		t.Fatalf("empty upload error=%v", err)
	}
	values["p"] = []string{"clip.mp4"}
	values["topic"] = "7"
	if _, err := spec.build(values, defaultSettings()); err == nil || !strings.Contains(err.Error(), "chat is required") {
		t.Fatalf("topic dependency error=%v", err)
	}
	values["c"] = "123"
	values["to"] = `"456"`
	if _, err := spec.build(values, defaultSettings()); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("destination conflict error=%v", err)
	}
}

func TestDownloadRequiresURLOrExport(t *testing.T) {
	action := actionByID(t, "dl")
	if _, err := action.formSpec().build(action.formValues(), defaultSettings()); err == nil {
		t.Fatal("empty download form passed validation")
	}
	values := action.formValues()
	values["f"] = []string{`C:\exports\messages, one.json`}
	run, err := action.formSpec().build(values, defaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := 0; i+1 < len(run.Args); i++ {
		if run.Args[i] == "-f" && run.Args[i+1] == `C:\exports\messages, one.json` {
			found = true
		}
	}
	if !found {
		t.Fatalf("export path was not preserved: %v", run.Args)
	}
}
