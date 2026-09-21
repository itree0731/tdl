package chat

import (
	"context"
	"fmt"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/iyear/tdl/core/logctx"
	"github.com/iyear/tdl/core/storage"
	"github.com/iyear/tdl/core/util/tutil"
)

// DialogCursor is the native Telegram getDialogs cursor. The TUI adapter keeps
// it opaque and exposes only a short token, avoiding text parsing and repeated
// full-account scans.
type DialogCursor struct {
	OffsetDate int
	OffsetID   int
	OffsetPeer tg.InputPeerClass
}

// ListDialogsPage fetches and converts exactly one Telegram dialogs page.
func ListDialogsPage(ctx context.Context, c *telegram.Client, kvd storage.Storage, cursor *DialogCursor, limit int) ([]*Dialog, *DialogCursor, int, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	offsetPeer := tg.InputPeerClass(&tg.InputPeerEmpty{})
	offsetDate, offsetID := 0, 0
	if cursor != nil {
		offsetDate, offsetID = cursor.OffsetDate, cursor.OffsetID
		if cursor.OffsetPeer != nil {
			offsetPeer = cursor.OffsetPeer
		}
	}
	response, err := c.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetDate: offsetDate, OffsetID: offsetID, OffsetPeer: offsetPeer, Limit: limit,
	})
	if err != nil {
		return nil, nil, 0, err
	}
	dialogClasses, messages, users, chats, err := unpackDialogsPage(response)
	if err != nil {
		return nil, nil, 0, err
	}
	if len(dialogClasses) == 0 {
		return nil, nil, 0, nil
	}
	entities := pageEntities(users, chats)
	messageMap := make(map[int]tg.NotEmptyMessage, len(messages))
	for _, message := range messages {
		if value, ok := message.AsNotEmpty(); ok {
			messageMap[value.GetID()] = value
		}
	}
	blocked, err := tutil.GetBlockedDialogs(ctx, c.API())
	if err != nil {
		return nil, nil, 0, err
	}
	manager := peers.Options{Storage: storage.NewPeers(kvd)}.Build(c.API())
	result := make([]*Dialog, 0, len(dialogClasses))
	skipped := 0
	seen := make(map[int64]bool)
	for _, class := range dialogClasses {
		dialog, ok := class.(*tg.Dialog)
		if !ok {
			continue
		}
		id := peerID(dialog.Peer)
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		inputPeer, extractErr := entities.ExtractPeer(dialog.Peer)
		if extractErr != nil {
			skipped++
			logctx.From(ctx).Warn("skipping dialog with missing peer", zap.Int64("peer_id", id), zap.Error(extractErr))
			continue
		}
		if _, blockedPeer := blocked[id]; blockedPeer {
			continue
		}
		if applyErr := applyPeers(ctx, manager, entities, id); applyErr != nil {
			logctx.From(ctx).Warn("failed to apply peer updates", zap.Int64("id", id), zap.Error(applyErr))
		}
		var item *Dialog
		switch value := inputPeer.(type) {
		case *tg.InputPeerUser:
			item = processUser(value.UserID, entities)
		case *tg.InputPeerChannel:
			item = processChannel(ctx, c.API(), value.ChannelID, entities)
		case *tg.InputPeerChat:
			item = processChat(value.ChatID, entities)
		}
		if item != nil {
			result = append(result, item)
		}
	}
	next, err := nextDialogCursor(dialogClasses, messageMap, entities, limit)
	if err != nil {
		return result, nil, skipped, err
	}
	return result, next, skipped, nil
}

func unpackDialogsPage(response tg.MessagesDialogsClass) ([]tg.DialogClass, []tg.MessageClass, []tg.UserClass, []tg.ChatClass, error) {
	switch value := response.(type) {
	case *tg.MessagesDialogs:
		return value.Dialogs, value.Messages, value.Users, value.Chats, nil
	case *tg.MessagesDialogsSlice:
		return value.Dialogs, value.Messages, value.Users, value.Chats, nil
	case *tg.MessagesDialogsNotModified:
		return nil, nil, nil, nil, nil
	default:
		return nil, nil, nil, nil, fmt.Errorf("unexpected dialog type %T", response)
	}
}

func pageEntities(users []tg.UserClass, chats []tg.ChatClass) peer.Entities {
	userMap := make(map[int64]*tg.User)
	chatMap := make(map[int64]*tg.Chat)
	channelMap := make(map[int64]*tg.Channel)
	for _, class := range users {
		if value, ok := class.(*tg.User); ok {
			userMap[value.ID] = value
		}
	}
	for _, class := range chats {
		switch value := class.(type) {
		case *tg.Chat:
			chatMap[value.ID] = value
		case *tg.Channel:
			channelMap[value.ID] = value
		}
	}
	return peer.NewEntities(userMap, chatMap, channelMap)
}

func peerID(value tg.PeerClass) int64 {
	switch p := value.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChat:
		return p.ChatID
	case *tg.PeerChannel:
		return p.ChannelID
	default:
		return 0
	}
}

func nextDialogCursor(dialogs []tg.DialogClass, messages map[int]tg.NotEmptyMessage, entities peer.Entities, limit int) (*DialogCursor, error) {
	if len(dialogs) < limit || len(dialogs) == 0 {
		return nil, nil
	}
	last, ok := dialogs[len(dialogs)-1].(*tg.Dialog)
	if !ok {
		return nil, fmt.Errorf("last dialog is %T", dialogs[len(dialogs)-1])
	}
	inputPeer, err := entities.ExtractPeer(last.Peer)
	if err != nil {
		return nil, fmt.Errorf("extract pagination peer: %w", err)
	}
	date := 0
	if message, ok := messages[last.TopMessage]; ok {
		date = message.GetDate()
	}
	return &DialogCursor{OffsetDate: date, OffsetID: last.TopMessage, OffsetPeer: inputPeer}, nil
}
