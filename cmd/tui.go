package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tgerr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/iyear/tdl/app/chat"
	"github.com/iyear/tdl/app/tui"
	"github.com/iyear/tdl/core/logctx"
	"github.com/iyear/tdl/core/storage"
	"github.com/iyear/tdl/pkg/consts"
	"github.com/iyear/tdl/pkg/kv"
	tclientpkg "github.com/iyear/tdl/pkg/tclient"
)

const namespaceCheckTimeout = 15 * time.Second

var (
	errNamespaceUnauthorized = errors.New("namespace is not authorized")
	errNamespaceNoSession    = errors.New("namespace has no session")
)

// NewTUI creates the fullscreen, mouse-interactive tdl transfer workbench.
func NewTUI() *cobra.Command {
	return &cobra.Command{
		Use:     "tui",
		Short:   "Interactive media transfer workbench for tdl",
		GroupID: groupTools.ID,
		RunE: func(cmd *cobra.Command, args []string) error {
			stg := kv.From(cmd.Context())
			namespaces, err := validNamespaces(cmd.Context(), stg)
			if err != nil {
				return err
			}
			source := &tuiChatSource{storage: stg, cursors: make(map[string]map[int]*chat.DialogCursor)}
			return tui.Run(cmd.Context(), execInTUI, namespaces, tui.WithChatSource(source))
		},
	}
}

type tuiChatSource struct {
	storage kv.Storage
	mu      sync.Mutex
	nextID  int
	cursors map[string]map[int]*chat.DialogCursor
}

func (s *tuiChatSource) Page(ctx context.Context, namespace string, cursor *tui.ChatCursor, limit int) (tui.ChatPage, error) {
	if limit <= 0 {
		limit = 100
	}
	var nativeCursor *chat.DialogCursor
	if cursor != nil {
		s.mu.Lock()
		nativeCursor = s.cursors[namespace][cursor.OffsetID]
		s.mu.Unlock()
		if nativeCursor == nil {
			return tui.ChatPage{}, errors.Errorf("unknown chat cursor %d for namespace %q", cursor.OffsetID, namespace)
		}
	}
	kvd, err := s.storage.Open(namespace)
	if err != nil {
		return tui.ChatPage{}, errors.Wrap(err, "open chat namespace")
	}
	client, err := tclientpkg.New(ctx, tclientpkg.Options{
		KV: kvd, Proxy: viper.GetString(consts.FlagProxy), NTP: viper.GetString(consts.FlagNTP), ReconnectTimeout: viper.GetDuration(consts.FlagReconnectTimeout),
	}, false)
	if err != nil {
		return tui.ChatPage{}, errors.Wrap(err, "create chat selector client")
	}
	var dialogs []*chat.Dialog
	var next *chat.DialogCursor
	var skipped int
	err = client.Run(ctx, func(runCtx context.Context) error {
		var listErr error
		dialogs, next, skipped, listErr = chat.ListDialogsPage(runCtx, client, kvd, nativeCursor, limit)
		return listErr
	})
	if err != nil {
		return tui.ChatPage{}, errors.Wrap(err, "load chat page")
	}
	items := make([]tui.ChatRef, 0, len(dialogs))
	for _, dialog := range dialogs {
		topics := make([]tui.TopicRef, 0, len(dialog.Topics))
		for _, topic := range dialog.Topics {
			topics = append(topics, tui.TopicRef{ID: topic.ID, Title: topic.Title})
		}
		items = append(items, tui.ChatRef{ID: dialog.ID, Username: dialog.Username, Title: dialog.VisibleName, Type: dialog.Type, Topics: topics})
	}
	page := tui.ChatPage{Items: items, Skipped: skipped}
	if next != nil {
		s.mu.Lock()
		s.nextID++
		token := s.nextID
		if s.cursors[namespace] == nil {
			s.cursors[namespace] = make(map[int]*chat.DialogCursor)
		}
		s.cursors[namespace][token] = next
		s.mu.Unlock()
		page.Next = &tui.ChatCursor{OffsetID: token}
	}
	return page, nil
}

// validNamespaces checks every stored session before the TUI starts. Only
// explicit authorization failures are removed; transient network failures
// leave the namespace visible so a temporary outage cannot destroy a session.
func validNamespaces(parent context.Context, stg kv.Storage) ([]string, error) {
	ns, err := stg.Namespaces()
	if err != nil {
		return nil, errors.Wrap(err, "list namespaces")
	}
	sort.Strings(ns)

	valid := make([]string, 0, len(ns))
	for _, name := range ns {
		ctx, cancel := context.WithTimeout(parent, namespaceCheckTimeout)
		err := checkNamespace(ctx, stg, name)
		cancel()
		if errors.Is(err, errNamespaceUnauthorized) {
			if removeErr := stg.RemoveNamespace(name); removeErr != nil {
				return nil, errors.Wrapf(removeErr, "remove invalid namespace %q", name)
			}
			logctx.From(parent).Warn("Removed unauthorized namespace", zap.String("namespace", name))
			continue
		}
		if errors.Is(err, errNamespaceNoSession) {
			logctx.From(parent).Debug("Ignoring namespace without Telegram session", zap.String("namespace", name))
			continue
		}
		if err != nil {
			logctx.From(parent).Warn("Could not validate namespace; keeping it", zap.String("namespace", name), zap.Error(err))
		}
		valid = append(valid, name)
	}
	return valid, nil
}

func checkNamespace(ctx context.Context, stg kv.Storage, name string) error {
	kvd, err := stg.Open(name)
	if err != nil {
		return errors.Wrap(err, "open namespace")
	}
	hasSession, err := storage.HasSession(ctx, kvd)
	if err != nil {
		return errors.Wrap(err, "check session")
	}
	if !hasSession {
		return errNamespaceNoSession
	}
	client, err := tclientpkg.New(ctx, tclientpkg.Options{
		KV:               kvd,
		Proxy:            viper.GetString(consts.FlagProxy),
		NTP:              viper.GetString(consts.FlagNTP),
		ReconnectTimeout: viper.GetDuration(consts.FlagReconnectTimeout),
	}, false)
	if err != nil {
		if isUnauthorizedNamespaceError(err) {
			return errNamespaceUnauthorized
		}
		return errors.Wrap(err, "create telegram client")
	}
	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			if isUnauthorizedNamespaceError(err) {
				return errNamespaceUnauthorized
			}
			return err
		}
		if !status.Authorized {
			return errNamespaceUnauthorized
		}
		return nil
	})
}

func isUnauthorizedNamespaceError(err error) bool {
	return auth.IsUnauthorized(err) || tgerr.Is(err,
		"AUTH_KEY_UNREGISTERED",
		"SESSION_EXPIRED",
		"AUTH_KEY_DUPLICATED",
	)
}

// listNamespaces remains useful to callers that only need the stored names.
func listNamespaces() []string {
	stg, err := kv.NewWithMap(DefaultBoltStorage)
	if err != nil {
		return nil
	}
	defer func() { _ = stg.Close() }()
	ns, err := stg.Namespaces()
	if err != nil {
		return nil
	}
	sort.Strings(ns)
	return ns
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
	oldColorOutput, oldColorError := color.Output, color.Error
	color.Output, color.Error = out, out
	defer func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
		color.Output, color.Error = oldColorOutput, oldColorError
	}()

	var f *os.File
	if _, ok := out.(*os.File); ok {
		f = out.(*os.File)
	}

	// prefer a real *os.File so isatty-adjacent code paths behave
	if f != nil {
		os.Stdout, os.Stderr = f, f
	}

	// No interactive input exists inside the TUI: the console is owned by
	// the renderer. survey prompts read os.Stdin directly and would block
	// forever on the raw-mode console, so hand them an EOF-ing handle.
	if nullIn, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0); err == nil {
		oldStdin := os.Stdin
		os.Stdin = nullIn
		defer func() {
			os.Stdin = oldStdin
			_ = nullIn.Close()
		}()
	}

	root := New()
	root.SetArgs(argv)
	root.SetOut(out)
	root.SetErr(out)
	root.SetIn(bytes.NewReader(nil))

	err := root.ExecuteContext(ctx)

	return err
}
