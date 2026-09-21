package downloader

import (
	"context"
	"io"
	"testing"

	"github.com/gotd/td/tg"
)

type preparedDownloadElem struct{}

func (preparedDownloadElem) File() File         { return preparedDownloadFile{} }
func (preparedDownloadElem) To() io.WriterAt    { return nil }
func (preparedDownloadElem) AsTakeout() bool    { return false }
func (preparedDownloadElem) SkipTransfer() bool { return true }

type preparedDownloadFile struct{}

func (preparedDownloadFile) Location() tg.InputFileLocationClass { return nil }
func (preparedDownloadFile) Size() int64                         { return 1 }
func (preparedDownloadFile) DC() int                             { return 1 }

func TestCompleteTemporaryFileSkipsNetworkDownload(t *testing.T) {
	d := Downloader{}
	if err := d.download(context.Background(), preparedDownloadElem{}); err != nil {
		t.Fatal(err)
	}
}
