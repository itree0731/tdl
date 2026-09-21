package uploader

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/tg"
)

type preparationTestFile struct{ *bytes.Reader }

func (f preparationTestFile) Name() string { return "video.mp4" }
func (f preparationTestFile) Size() int64  { return int64(f.Len()) }

type preparationTestElem struct {
	file File
	err  error
}

func (e preparationTestElem) File() File                                 { return e.file }
func (e preparationTestElem) Thumb() (File, bool)                        { return nil, false }
func (e preparationTestElem) Caption() (string, []tg.MessageEntityClass) { return "", nil }
func (e preparationTestElem) To() tg.InputPeerClass                      { return &tg.InputPeerSelf{} }
func (e preparationTestElem) Thread() int                                { return 0 }
func (e preparationTestElem) AsPhoto() bool                              { return false }
func (e preparationTestElem) PreparationError() error                    { return e.err }

func TestUploadReturnsPreparationErrorBeforeNetworkUse(t *testing.T) {
	sentinel := errors.New("thumbnail generation failed")
	elem := preparationTestElem{
		file: preparationTestFile{Reader: bytes.NewReader([]byte("video"))},
		err:  sentinel,
	}
	u := &Uploader{}
	err := u.upload(context.Background(), elem)
	if !errors.Is(err, sentinel) {
		t.Fatalf("error=%v", err)
	}
}
