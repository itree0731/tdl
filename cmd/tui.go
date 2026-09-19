package cmd

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/iyear/tdl/app/tui"
)

// NewTUI creates the `tdl tui` command: a fullscreen, mouse-interactive
// shell around tdl's own commands, styled after xai-org/grok-build.
func NewTUI() *cobra.Command {
	return &cobra.Command{
		Use:     "tui",
		Short:   "Interactive TUI for tdl (grok-build style)",
		GroupID: groupTools.ID,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(execInTUI)
		},
	}
}

// execInTUI runs a fresh tdl command tree in-process.
//
// Stdout/stderr are swapped to the given writer for the duration of the run:
// tdl and its progress bars (go-pretty) write to os.Stdout directly instead
// of the cobra output writer, so SetOut alone would leak to the real terminal
// behind the TUI. The bubbletea renderer captured the terminal handle at
// startup and is unaffected by the swap.
func execInTUI(ctx context.Context, argv []string, out io.Writer) error {
	oldStdout, oldStderr := os.Stdout, os.Stderr

	var f *os.File
	if _, ok := out.(*os.File); ok {
		f = out.(*os.File)
	}

	// prefer a real *os.File so isatty-adjacent code paths behave
	if f != nil {
		os.Stdout, os.Stderr = f, f
	}

	root := New()
	root.SetArgs(argv)
	root.SetOut(out)
	root.SetErr(out)
	root.SetIn(bytes.NewReader(nil))

	err := root.ExecuteContext(ctx)

	if f != nil {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	}
	return err
}
