package downloader

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

type Elem interface {
	File() File
	To() io.WriterAt

	AsTakeout() bool
}

// PreparedElem can skip network transfer when an existing temporary file is
// already complete and only finalization remains.
type PreparedElem interface {
	SkipTransfer() bool
}

type File interface {
	Location() tg.InputFileLocationClass
	Size() int64
	DC() int
}
