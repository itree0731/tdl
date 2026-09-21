package tui

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

type Clipboard interface {
	Copy(string) error
}

type terminalClipboard struct{ out io.Writer }

func (c terminalClipboard) Copy(value string) error {
	if c.out == nil {
		return fmt.Errorf("clipboard output is unavailable")
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(value))
	_, err := fmt.Fprintf(c.out, "\x1b]52;c;%s\a", encoded)
	return err
}

func defaultClipboard() Clipboard { return terminalClipboard{out: os.Stderr} }
