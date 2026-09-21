package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	tdlcmd "github.com/iyear/tdl/cmd"
	"github.com/iyear/tdl/pkg/key"
	"github.com/iyear/tdl/pkg/kv"
	"github.com/iyear/tdl/pkg/tclient"
	"github.com/skip2/go-qrcode"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var namespacePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func (a *App) StartQRLogin(namespace, proxy string) error {
	namespace = strings.TrimSpace(namespace)
	if !namespacePattern.MatchString(namespace) {
		return fmt.Errorf("账号名称只能包含字母、数字、点、下划线和连字符，长度 1 到 64")
	}
	store, err := kv.NewWithMap(tdlcmd.DefaultBoltStorage)
	if err != nil {
		return err
	}
	existing, listErr := store.Namespaces()
	closeErr := store.Close()
	if listErr != nil {
		return listErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, value := range existing {
		if value == namespace {
			return fmt.Errorf("账号名称 %q 已存在，请使用新的名称", namespace)
		}
	}
	a.mu.Lock()
	if a.running || a.chatLoading || a.settingsBusy || a.loginRunning {
		a.mu.Unlock()
		return fmt.Errorf("当前有其他操作正在运行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	passwords := make(chan string, 1)
	a.loginCancel = cancel
	a.loginPass = passwords
	a.loginRunning = true
	a.mu.Unlock()
	go a.runQRLogin(ctx, namespace, proxy, passwords)
	return nil
}

func (a *App) SubmitLoginPassword(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return fmt.Errorf("请输入两步验证密码")
	}
	a.mu.Lock()
	channel := a.loginPass
	running := a.loginRunning
	a.mu.Unlock()
	if !running || channel == nil {
		return fmt.Errorf("当前没有等待密码的登录")
	}
	select {
	case channel <- password:
		return nil
	default:
		return fmt.Errorf("密码已经提交")
	}
}

func (a *App) CancelLogin() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.loginRunning || a.loginCancel == nil {
		return false
	}
	a.loginCancel()
	return true
}

func (a *App) runQRLogin(ctx context.Context, namespace, proxy string, passwords <-chan string) {
	store, err := kv.NewWithMap(tdlcmd.DefaultBoltStorage)
	if err != nil {
		a.finishLogin(namespace, err)
		return
	}
	db, err := store.Open(namespace)
	if err != nil {
		_ = store.Close()
		a.finishLogin(namespace, err)
		return
	}
	if err = db.Set(ctx, key.App(), []byte(tclient.AppDesktop)); err != nil {
		_ = store.Close()
		a.finishLogin(namespace, err)
		return
	}
	dispatcher := tg.NewUpdateDispatcher()
	client, err := tclient.New(ctx, tclient.Options{KV: db, Proxy: proxy, ReconnectTimeout: 30 * time.Second, UpdateHandler: dispatcher}, true)
	if err != nil {
		_ = store.Close()
		a.finishLogin(namespace, err)
		return
	}
	var userID int64
	var username string
	err = client.Run(ctx, func(runCtx context.Context) error {
		_, authErr := client.QR().Auth(runCtx, qrlogin.OnLoginToken(dispatcher), func(_ context.Context, token qrlogin.Token) error {
			png, encodeErr := qrcode.Encode(token.URL(), qrcode.Medium, 512)
			if encodeErr != nil {
				return encodeErr
			}
			runtime.EventsEmit(a.ctx, "login:qr", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png))
			return nil
		})
		if authErr != nil {
			if !tgerr.Is(authErr, "SESSION_PASSWORD_NEEDED") {
				return authErr
			}
			runtime.EventsEmit(a.ctx, "login:password-required", true)
			var password string
			select {
			case password = <-passwords:
			case <-runCtx.Done():
				return runCtx.Err()
			}
			if _, authErr = client.Auth().Password(runCtx, password); authErr != nil {
				return authErr
			}
		}
		self, selfErr := client.Self(runCtx)
		if selfErr != nil {
			return selfErr
		}
		userID, username = self.ID, self.Username
		return nil
	})
	closeErr := store.Close()
	if err == nil {
		err = closeErr
	}
	a.finishLogin(namespace, err)
	if err == nil {
		runtime.EventsEmit(a.ctx, "login:done", map[string]any{"namespace": namespace, "userID": userID, "username": username})
	}
}

func (a *App) finishLogin(namespace string, err error) {
	a.mu.Lock()
	a.loginRunning = false
	a.loginCancel = nil
	a.loginPass = nil
	a.mu.Unlock()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		runtime.EventsEmit(a.ctx, "login:error", map[string]any{"namespace": namespace, "error": err.Error()})
	}
}
