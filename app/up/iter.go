package up

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/expr-lang/expr/vm"
	"github.com/go-faster/errors"
	"github.com/go-viper/mapstructure/v2"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/telegram/peers"

	"github.com/iyear/tdl/core/uploader"
	"github.com/iyear/tdl/core/util/tutil"
	"github.com/iyear/tdl/pkg/texpr"
)

type File struct {
	File  string
	Thumb string
	Cover string
}

type dest struct {
	Peer   string
	Thread int
}

type iter struct {
	files       []*File
	to          *vm.Program
	caption     *vm.Program
	chat        string
	topic       int
	photo       bool
	remove      bool
	noAutoThumb bool
	coverMode   CoverMode
	coverAt     string
	delay       time.Duration
	manager     *peers.Manager

	cur  int
	err  error
	file uploader.Elem
}

func newIter(files []*File, to, caption *vm.Program, chat string, topic int, photo, remove, noAutoThumb bool, coverMode CoverMode, coverAt string, delay time.Duration, manager *peers.Manager) *iter {
	return &iter{
		files:       files,
		to:          to,
		caption:     caption,
		chat:        chat,
		topic:       topic,
		photo:       photo,
		remove:      remove,
		noAutoThumb: noAutoThumb,
		coverMode:   coverMode,
		coverAt:     coverAt,
		delay:       delay,
		manager:     manager,

		cur:  0,
		err:  nil,
		file: nil,
	}
}

func (i *iter) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		i.err = ctx.Err()
		return false
	default:
	}

	if i.cur >= len(i.files) || i.err != nil {
		return false
	}

	// if delay is set, sleep for a while for each iteration
	if i.delay > 0 && i.cur > 0 { // skip first delay
		timer := time.NewTimer(i.delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			i.err = ctx.Err()
			return false
		case <-timer.C:
		}
	}

	cur := i.files[i.cur]
	i.cur++

	file, err := i.next(ctx, cur)
	if err != nil {
		i.err = err
		return false
	}

	i.file = file
	return true
}

func (i *iter) next(ctx context.Context, cur *File) (*iterElem, error) {
	file, err := i.resolveFile(cur.File)
	if err != nil {
		return nil, errors.Wrap(err, "resolve file")
	}
	var (
		thumb          *uploaderFile
		cover          *uploaderFile
		temporaryFiles []string
		keepFiles      bool
	)
	defer func() {
		if keepFiles {
			return
		}
		_ = file.Close()
		if thumb != nil {
			_ = thumb.Close()
		}
		if cover != nil {
			_ = cover.Close()
		}
		for _, path := range temporaryFiles {
			_ = os.Remove(path)
		}
	}()

	env := exprEnv(ctx, cur)

	to, thread, err := i.resolveDest(ctx, env)
	if err != nil {
		return nil, errors.Wrap(err, "resolve destination")
	}

	caption, err := i.resolveCaption(env)
	if err != nil {
		return nil, errors.Wrap(err, "resolve caption")
	}

	prepared, err := prepareCovers(ctx, cur.File, cur.Thumb, cur.Cover, i.coverMode, i.coverAt)
	if err != nil {
		keepFiles = true
		return &iterElem{
			file:           file,
			to:             to,
			caption:        caption,
			thread:         thread,
			asPhoto:        i.photo,
			remove:         i.remove,
			preparationErr: errors.Wrap(err, "prepare video cover"),
		}, nil
	}
	temporaryFiles = append(temporaryFiles, prepared.Temporary...)
	thumb, err = i.resolveThumb(prepared.Thumb)
	if err != nil {
		return nil, errors.Wrap(err, "resolve thumbnail")
	}
	cover, err = i.resolveCover(prepared.Cover)
	if err != nil {
		return nil, errors.Wrap(err, "resolve video cover")
	}

	keepFiles = true
	return &iterElem{
		file:    file,
		thumb:   thumb,
		cover:   cover,
		to:      to,
		caption: caption,
		thread:  thread,

		asPhoto:        i.photo,
		remove:         i.remove,
		temporaryFiles: temporaryFiles,
	}, nil
}

func (i *iter) resolveCover(path string) (*uploaderFile, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.Size() <= 0 {
		_ = f.Close()
		return nil, errors.New("video cover is empty")
	}
	return &uploaderFile{File: f, size: info.Size()}, nil
}

func (i *iter) resolveFile(path string) (*uploaderFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "open file")
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "stat file")
	}

	return &uploaderFile{
		File: f,
		size: stat.Size(),
	}, nil
}

func (i *iter) resolveDest(ctx context.Context, env Env) (peers.Peer, int, error) {
	if i.chat != "" { // compatible with old version
		to, err := i.resolvePeer(ctx, i.chat)
		if err != nil {
			return nil, 0, errors.Wrap(err, "resolve chat")
		}

		return to, i.topic, nil
	}

	// message routing
	result, err := texpr.Run(i.to, env)
	if err != nil {
		return nil, 0, errors.Wrap(err, "parse expression")
	}

	var (
		to     peers.Peer
		thread int
	)

	switch r := result.(type) {
	case string:
		// pure chat, no reply to, which is a compatible with old version
		// and a convenient way to send message to self
		to, err = i.resolvePeer(ctx, r)
	case map[string]interface{}:
		// chat with reply to topic or message
		var d dest

		if err = mapstructure.WeakDecode(r, &d); err != nil {
			return nil, 0, errors.Wrapf(err, "decode dest: %v", result)
		}

		to, err = i.resolvePeer(ctx, d.Peer)
		thread = d.Thread
	default:
		return nil, 0, errors.Errorf("message router must return string or dest: %T", result)
	}

	if err != nil {
		return nil, 0, errors.Wrap(err, "resolve peer")
	}

	return to, thread, nil
}

func (i *iter) resolvePeer(ctx context.Context, peer string) (peers.Peer, error) {
	if peer == "" { // self
		return i.manager.Self(ctx)
	}

	return tutil.GetInputPeer(ctx, i.manager, peer)
}

func (i *iter) resolveCaption(env Env) (*entity.Builder, error) {
	// parse caption
	captionStr, err := texpr.Run(i.caption, env)
	if err != nil {
		return nil, errors.Wrap(err, "parse caption")
	}

	r, ok := captionStr.(string)
	if !ok {
		return nil, errors.Errorf("caption must return string, got %T", captionStr)
	}

	caption := &entity.Builder{}
	if len(r) > 0 {
		if err = html.HTML(strings.NewReader(r), caption, html.Options{
			UserResolver:          nil,
			DisableTelegramEscape: false,
		}); err != nil {
			return nil, errors.Wrap(err, "parse caption HTML")
		}
	}

	return caption, nil
}

func (i *iter) resolveThumb(path string) (*uploaderFile, error) {
	if path == "" {
		return nil, nil
	}

	if err := validateThumbnail(path); err != nil {
		return nil, errors.Wrapf(err, "invalid thumbnail file: %v", path)
	}

	thumb, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "open thumbnail file")
	}
	info, err := thumb.Stat()
	if err != nil {
		_ = thumb.Close()
		return nil, errors.Wrap(err, "stat thumbnail file")
	}

	return &uploaderFile{
		File: thumb,
		size: info.Size(),
	}, nil
}

func (i *iter) Value() uploader.Elem {
	return i.file
}

func (i *iter) Err() error {
	return i.err
}
