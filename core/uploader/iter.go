package uploader

import (
	"context"
	"io"

	"github.com/gotd/td/tg"
)

type Iter interface {
	Next(ctx context.Context) bool
	Value() Elem
	Err() error
}

type File interface {
	io.ReadSeeker
	Name() string
	Size() int64
}

type Elem interface {
	File() File
	Thumb() (File, bool)
	Caption() (string, []tg.MessageEntityClass)
	To() tg.InputPeerClass
	Thread() int
	AsPhoto() bool
}

// PreparedElem can report a per-item preparation failure. The uploader checks
// it before making network requests, so a batch can continue with later items
// while preserving an accurate aggregate error.
type PreparedElem interface {
	PreparationError() error
}

// CoverElem optionally provides a high-resolution video cover. It remains
// separate from Thumb because Telegram stores the two in different fields.
type CoverElem interface {
	Cover() (File, bool)
	CoverTimestamp() int
}
